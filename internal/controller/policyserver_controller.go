/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	policiesv1 "github.com/kubewarden/kubewarden-controller/api/policies/v1"
	"github.com/kubewarden/kubewarden-controller/internal/constants"
)

// Warning: this controller is deployed by a helm chart which has its own
// templated RBAC rules. The rules are kept in sync between what is generated by
// `make manifests` and the helm chart by hand.
//
// We need access to these resources only inside of the namespace where the
// controller is deployed. Here we assume it's being deployed inside of the
// `kubewarden` namespace, this has to be parametrized in the helm chart
//+kubebuilder:rbac:groups=policies.kubewarden.io,resources=policyservers,verbs=get;list;watch;delete;create;update;patch
//+kubebuilder:rbac:groups=policies.kubewarden.io,resources=policyservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=policies.kubewarden.io,resources=policyservers/finalizers,verbs=update
//+kubebuilder:rbac:namespace=kubewarden,groups=core,resources=secrets;services;configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:namespace=kubewarden,groups=apps,resources=deployments,verbs=create;update;patch;delete;get;list;watch
//+kubebuilder:rbac:namespace=kubewarden,groups=apps,resources=replicasets,verbs=get;list;watch
//+kubebuilder:rbac:namespace=kubewarden,groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:namespace=kubewarden,groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete

// PolicyServerReconciler reconciles a PolicyServer object.
type PolicyServerReconciler struct {
	client.Client
	TelemetryConfiguration
	Log                                                logr.Logger
	Scheme                                             *runtime.Scheme
	DeploymentsNamespace                               string
	AlwaysAcceptAdmissionReviewsInDeploymentsNamespace bool
}

// TelemetryConfiguration is a struct that contains the configuration for the
// Telemetry configuration. Now, it only contains the configuration for the
// OpenTelemetry.
type TelemetryConfiguration struct {
	MetricsEnabled bool
	TracingEnabled bool
	// OpenTelemetry configuration.
	// OtelSidecarEnabled is a flag that enables the OpenTelemetry sidecar.
	OtelSidecarEnabled bool
	// OtelCertificateSecret and OtelClientCertificateSecret are the names of the
	// secrets that contain the certificates used with the communication between
	// controller and policy server with the remote OpenTelemetry collector.
	OtelCertificateSecret       string
	OtelClientCertificateSecret string
}

func (r *PolicyServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var policyServer policiesv1.PolicyServer
	if err := r.Get(ctx, req.NamespacedName, &policyServer); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	policies, err := r.getPolicies(ctx, &policyServer)
	if err != nil {
		return ctrl.Result{}, errors.Join(errors.New("could not get policies"), err)
	}

	if policyServer.ObjectMeta.DeletionTimestamp != nil {
		return r.reconcileDeletion(ctx, &policyServer, policies)
	}

	err = r.reconcilePolicyServerCertSecret(ctx, &policyServer)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err = r.reconcilePolicyServerConfigMap(ctx, &policyServer, policies); err != nil {
		setFalseConditionType(
			&policyServer.Status.Conditions,
			string(policiesv1.PolicyServerConfigMapReconciled),
			fmt.Sprintf("error reconciling configmap: %v", err),
		)
		return ctrl.Result{}, err
	}

	setTrueConditionType(
		&policyServer.Status.Conditions,
		string(policiesv1.PolicyServerConfigMapReconciled),
	)

	if err = r.reconcilePolicyServerPodDisruptionBudget(ctx, &policyServer); err != nil {
		setFalseConditionType(
			&policyServer.Status.Conditions,
			string(policiesv1.PolicyServerPodDisruptionBudgetReconciled),
			fmt.Sprintf("error reconciling policy server PodDisruptionBudget: %v", err),
		)
		return ctrl.Result{}, err
	}

	setTrueConditionType(
		&policyServer.Status.Conditions,
		string(policiesv1.PolicyServerPodDisruptionBudgetReconciled),
	)

	if err = r.reconcilePolicyServerDeployment(ctx, &policyServer); err != nil {
		setFalseConditionType(
			&policyServer.Status.Conditions,
			string(policiesv1.PolicyServerDeploymentReconciled),
			fmt.Sprintf("error reconciling deployment: %v", err),
		)
		return ctrl.Result{}, err
	}

	setTrueConditionType(
		&policyServer.Status.Conditions,
		string(policiesv1.PolicyServerDeploymentReconciled),
	)

	if err = r.reconcilePolicyServerService(ctx, &policyServer); err != nil {
		setFalseConditionType(
			&policyServer.Status.Conditions,
			string(policiesv1.PolicyServerServiceReconciled),
			fmt.Sprintf("error reconciling service: %v", err),
		)
		return ctrl.Result{}, err
	}

	setTrueConditionType(
		&policyServer.Status.Conditions,
		string(policiesv1.PolicyServerServiceReconciled),
	)

	if err = r.Client.Status().Update(ctx, &policyServer); err != nil {
		return ctrl.Result{}, fmt.Errorf("update policy server status error: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PolicyServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := mgr.GetFieldIndexer().IndexField(context.Background(), &policiesv1.ClusterAdmissionPolicy{}, constants.PolicyServerIndexKey, func(object client.Object) []string {
		policy, ok := object.(*policiesv1.ClusterAdmissionPolicy)
		if !ok {
			r.Log.Error(nil, "object is not type of ClusterAdmissionPolicy: %#v", "policy", policy)
			return []string{}
		}
		return []string{policy.Spec.PolicyServer}
	})
	if err != nil {
		return fmt.Errorf("failed enrolling controller with manager: %w", err)
	}
	err = mgr.GetFieldIndexer().IndexField(context.Background(), &policiesv1.AdmissionPolicy{}, constants.PolicyServerIndexKey, func(object client.Object) []string {
		policy, ok := object.(*policiesv1.AdmissionPolicy)
		if !ok {
			r.Log.Error(nil, "object is not type of AdmissionPolicy: %#v", "policy", policy)
			return []string{}
		}
		return []string{policy.Spec.PolicyServer}
	})
	if err != nil {
		return fmt.Errorf("failed enrolling controller with manager: %w", err)
	}
	err = mgr.GetFieldIndexer().IndexField(context.Background(), &policiesv1.AdmissionPolicyGroup{}, constants.PolicyServerIndexKey, func(object client.Object) []string {
		policy, ok := object.(*policiesv1.AdmissionPolicyGroup)
		if !ok {
			r.Log.Error(nil, "object is not type of AdmissionPolicyGroup: %#v", "policy", policy)
			return []string{}
		}
		return []string{policy.Spec.PolicyServer}
	})
	if err != nil {
		return fmt.Errorf("failed enrolling controller with manager: %w", err)
	}
	err = mgr.GetFieldIndexer().IndexField(context.Background(), &policiesv1.ClusterAdmissionPolicyGroup{}, constants.PolicyServerIndexKey, func(object client.Object) []string {
		policy, ok := object.(*policiesv1.ClusterAdmissionPolicyGroup)
		if !ok {
			r.Log.Error(nil, "object is not type of ClusterAdmissionPolicyGroup: %#v", "policy", policy)
			return []string{}
		}
		return []string{policy.Spec.PolicyServer}
	})
	if err != nil {
		return fmt.Errorf("failed enrolling controller with manager: %w", err)
	}

	err = ctrl.NewControllerManagedBy(mgr).
		For(&policiesv1.PolicyServer{}).
		Watches(&policiesv1.AdmissionPolicy{}, handler.EnqueueRequestsFromMapFunc(r.enqueueAdmissionPolicy)).
		Watches(&policiesv1.AdmissionPolicyGroup{}, handler.EnqueueRequestsFromMapFunc(r.enqueueAdmissionPolicyGroup)).
		Watches(&policiesv1.ClusterAdmissionPolicy{}, handler.EnqueueRequestsFromMapFunc(r.enqueueClusterAdmissionPolicy)).
		Watches(&policiesv1.ClusterAdmissionPolicyGroup{}, handler.EnqueueRequestsFromMapFunc(r.enqueueClusterAdmissionPolicyGroup)).
		Complete(r)
	if err != nil {
		return errors.Join(errors.New("failed enrolling controller with manager"), err)
	}
	return nil
}

func (r *PolicyServerReconciler) enqueueAdmissionPolicy(_ context.Context, object client.Object) []reconcile.Request {
	// The watch will trigger twice per object change; once with the old
	// object, and once the new object. We need to be mindful when doing
	// Updates since they will invalidate the newer versions of the
	// object.
	policy, ok := object.(*policiesv1.AdmissionPolicy)
	if !ok {
		r.Log.Info("object is not type of AdmissionPolicy: %+v", "policy", policy)
		return []ctrl.Request{}
	}

	return []ctrl.Request{
		{
			NamespacedName: client.ObjectKey{
				Name: policy.Spec.PolicyServer,
			},
		},
	}
}

func (r *PolicyServerReconciler) enqueueAdmissionPolicyGroup(_ context.Context, object client.Object) []reconcile.Request {
	// The watch will trigger twice per object change; once with the old
	// object, and once the new object. We need to be mindful when doing
	// Updates since they will invalidate the newer versions of the
	// object.
	policy, ok := object.(*policiesv1.AdmissionPolicyGroup)
	if !ok {
		r.Log.Info("object is not type of AdmissionPolicyGroup: %+v", "policy", policy)
		return []ctrl.Request{}
	}

	return []ctrl.Request{
		{
			NamespacedName: client.ObjectKey{
				Name: policy.Spec.PolicyServer,
			},
		},
	}
}
func (r *PolicyServerReconciler) enqueueClusterAdmissionPolicy(_ context.Context, object client.Object) []reconcile.Request {
	// The watch will trigger twice per object change; once with the old
	// object, and once the new object. We need to be mindful when doing
	// Updates since they will invalidate the newer versions of the
	// object.
	policy, ok := object.(*policiesv1.ClusterAdmissionPolicy)
	if !ok {
		r.Log.Info("object is not type of ClusterAdmissionPolicy: %+v", "policy", policy)
		return []ctrl.Request{}
	}

	return []ctrl.Request{
		{
			NamespacedName: client.ObjectKey{
				Name: policy.Spec.PolicyServer,
			},
		},
	}
}

func (r *PolicyServerReconciler) enqueueClusterAdmissionPolicyGroup(_ context.Context, object client.Object) []reconcile.Request {
	// The watch will trigger twice per object change; once with the old
	// object, and once the new object. We need to be mindful when doing
	// Updates since they will invalidate the newer versions of the
	// object.
	policy, ok := object.(*policiesv1.ClusterAdmissionPolicyGroup)
	if !ok {
		r.Log.Info("object is not type of ClusterAdmissionPolicyGroup: %+v", "policy", policy)
		return []ctrl.Request{}
	}

	return []ctrl.Request{
		{
			NamespacedName: client.ObjectKey{
				Name: policy.Spec.PolicyServer,
			},
		},
	}
}

// getPolicies returns all admission policies, cluster admission policy,
// admission policies groups and cluster admission policy groups bound to the
// given policyServer.
func (r *PolicyServerReconciler) getPolicies(ctx context.Context, policyServer *policiesv1.PolicyServer) ([]policiesv1.Policy, error) {
	var clusterAdmissionPolicies policiesv1.ClusterAdmissionPolicyList
	err := r.Client.List(ctx, &clusterAdmissionPolicies, client.MatchingFields{constants.PolicyServerIndexKey: policyServer.Name})
	if err != nil && apierrors.IsNotFound(err) {
		err = fmt.Errorf("failed obtaining ClusterAdmissionPolicies: %w", err)
		return nil, err
	}
	var admissionPolicies policiesv1.AdmissionPolicyList
	err = r.Client.List(ctx, &admissionPolicies, client.MatchingFields{constants.PolicyServerIndexKey: policyServer.Name})
	if err != nil && apierrors.IsNotFound(err) {
		err = fmt.Errorf("failed obtaining AdmissionPolicies: %w", err)
		return nil, err
	}

	var admissionPolicyGroupList policiesv1.AdmissionPolicyGroupList
	err = r.Client.List(ctx, &admissionPolicyGroupList, client.MatchingFields{constants.PolicyServerIndexKey: policyServer.Name})
	if err != nil && apierrors.IsNotFound(err) {
		err = fmt.Errorf("failed obtaining AdmissionPolicyGroups: %w", err)
		return nil, err
	}

	var clusterAdmissionPolicyGroupList policiesv1.ClusterAdmissionPolicyGroupList
	err = r.Client.List(ctx, &clusterAdmissionPolicyGroupList, client.MatchingFields{constants.PolicyServerIndexKey: policyServer.Name})
	if err != nil && apierrors.IsNotFound(err) {
		err = fmt.Errorf("failed obtaining ClusterAdmissionPolicyGroups: %w", err)
		return nil, err
	}

	policies := make([]policiesv1.Policy, 0)
	for _, clusterAdmissionPolicy := range clusterAdmissionPolicies.Items {
		policies = append(policies, clusterAdmissionPolicy.DeepCopy())
	}
	for _, admissionPolicy := range admissionPolicies.Items {
		policies = append(policies, admissionPolicy.DeepCopy())
	}
	for _, admissionPolicyGroup := range admissionPolicyGroupList.Items {
		policies = append(policies, admissionPolicyGroup.DeepCopy())
	}
	for _, clusterAdmissionPolicyGroup := range clusterAdmissionPolicyGroupList.Items {
		policies = append(policies, clusterAdmissionPolicyGroup.DeepCopy())
	}
	return policies, nil
}

func (r *PolicyServerReconciler) reconcileDeletion(ctx context.Context, policyServer *policiesv1.PolicyServer, policies []policiesv1.Policy) (ctrl.Result, error) {
	if len(policies) != 0 {
		// There are still policies scheduled on the PolicyServer, we have to
		// wait for them to be completely removed before going further with the cleanup
		return r.deletePoliciesAndRequeue(ctx, policyServer, policies)
	}

	// Remove the old finalizer used to ensure that the policy server created
	// before this controller version is delete as well. As the upgrade path
	// supported by the Kubewarden project does not allow jumping versions, we
	// can safely remove this line of code after a few releases.
	controllerutil.RemoveFinalizer(policyServer, constants.KubewardenFinalizerPre114)
	controllerutil.RemoveFinalizer(policyServer, constants.KubewardenFinalizer)
	if err := r.Update(ctx, policyServer); err != nil {
		// return if PolicyServer was previously deleted
		if apierrors.IsConflict(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("cannot update policy server: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *PolicyServerReconciler) deletePoliciesAndRequeue(ctx context.Context, policyServer *policiesv1.PolicyServer, policies []policiesv1.Policy) (ctrl.Result, error) {
	deleteError := make([]error, 0)
	for _, policy := range policies {
		if policy.GetDeletionTimestamp() != nil {
			// the policy is already pending deletion
			continue
		}
		if err := r.Delete(ctx, policy); err != nil && !apierrors.IsNotFound(err) {
			deleteError = append(deleteError, err)
		}
	}

	if len(deleteError) != 0 {
		r.Log.Error(errors.Join(deleteError...), "could not remove all policies bound to policy server", "policy-server", policyServer.Name)
		return ctrl.Result{}, fmt.Errorf("could not remove all policies bound to policy server %s", policyServer.Name)
	}

	return ctrl.Result{Requeue: true}, nil
}

func setFalseConditionType(
	conditions *[]metav1.Condition,
	conditionType string,
	message string,
) {
	apimeta.SetStatusCondition(
		conditions,
		metav1.Condition{
			Type:    conditionType,
			Status:  metav1.ConditionFalse,
			Reason:  string(policiesv1.ReconciliationFailed),
			Message: message,
		},
	)
}

func setTrueConditionType(conditions *[]metav1.Condition, conditionType string) {
	apimeta.SetStatusCondition(
		conditions,
		metav1.Condition{
			Type:   conditionType,
			Status: metav1.ConditionTrue,
			Reason: string(policiesv1.ReconciliationSucceeded),
		},
	)
}

func policyServerDeploymentName(policyServerName string) string {
	return "policy-server-" + policyServerName
}
