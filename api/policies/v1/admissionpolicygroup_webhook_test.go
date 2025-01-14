//go:build testing

/*
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

package v1

import (
	"testing"

	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"

	"github.com/kubewarden/kubewarden-controller/internal/constants"
)

func TestAdmissionPolicyGroupDefault(t *testing.T) {
	policy := AdmissionPolicyGroup{}
	policy.Default()

	require.Equal(t, constants.DefaultPolicyServer, policy.GetPolicyServer())
	require.Contains(t, policy.GetFinalizers(), constants.KubewardenFinalizer)
}

func TestAdmissionPolicyGroupValidateCreate(t *testing.T) {
	policy := NewAdmissionPolicyGroupFactory().Build()
	warnings, err := policy.ValidateCreate()
	require.NoError(t, err)
	require.Empty(t, warnings)
}

func TestClusterAdmissionPolicyValidateCreateWithNoMembers(t *testing.T) {
	policy := NewAdmissionPolicyGroupFactory().Build()
	policy.Spec.Policies = nil
	warnings, err := policy.ValidateCreate()
	require.Error(t, err)
	require.Empty(t, warnings)
	require.Contains(t, err.Error(), "policy groups must have at least one policy member")
}

func TestAdmissionPolicyGroupValidateUpdate(t *testing.T) {
	oldPolicy := NewAdmissionPolicyGroupFactory().Build()
	newPolicy := NewAdmissionPolicyGroupFactory().Build()
	warnings, err := newPolicy.ValidateUpdate(oldPolicy)
	require.NoError(t, err)
	require.Empty(t, warnings)

	oldPolicy = NewAdmissionPolicyGroupFactory().
		WithMode("monitor").
		Build()
	newPolicy = NewAdmissionPolicyGroupFactory().
		WithMode("protect").
		Build()
	warnings, err = newPolicy.ValidateUpdate(oldPolicy)
	require.NoError(t, err)
	require.Empty(t, warnings)
}

func TestInvalidAdmissionPolicyGroupValidateUpdate(t *testing.T) {
	oldPolicy := NewAdmissionPolicyFactory().
		WithPolicyServer("old").
		Build()
	newPolicy := NewAdmissionPolicyFactory().
		WithPolicyServer("new").
		Build()
	warnings, err := newPolicy.ValidateUpdate(oldPolicy)
	require.Error(t, err)
	require.Empty(t, warnings)

	newPolicy = NewAdmissionPolicyFactory().
		WithPolicyServer("new").
		WithMode("monitor").
		Build()

	warnings, err = newPolicy.ValidateUpdate(oldPolicy)
	require.Error(t, err)
	require.Empty(t, warnings)
}

func TestAdmissionPolicyGroupValidateUpdateWithInvalidOldPolicy(t *testing.T) {
	oldPolicy := NewClusterAdmissionPolicyGroupFactory().Build()
	newPolicy := NewAdmissionPolicyGroupFactory().Build()
	warnings, err := newPolicy.ValidateUpdate(oldPolicy)
	require.Empty(t, warnings)
	require.ErrorContains(t, err, "object is not of type AdmissionPolicyGroup")
}

func TestInvalidAdmissionPolicyGroupCreation(t *testing.T) {
	policy := NewAdmissionPolicyGroupFactory().
		WithPolicyServer("").
		WithRules([]admissionregistrationv1.RuleWithOperations{
			{},
			{
				Operations: []admissionregistrationv1.OperationType{},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{"*"},
					APIVersions: []string{"*"},
					Resources:   []string{"*/*"},
				}},
			{
				Operations: nil,
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{"*"},
					APIVersions: []string{"*"},
					Resources:   []string{"*/*"},
				},
			},
			{
				Operations: []admissionregistrationv1.OperationType{""},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{"*"},
					APIVersions: []string{"*"},
					Resources:   []string{"*/*"},
				},
			},
			{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{"*"},
					APIVersions: []string{},
					Resources:   []string{"*/*"},
				},
			},
			{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{"*"},
					APIVersions: []string{"*"},
					Resources:   []string{},
				},
			},
			{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{"*"},
					APIVersions: []string{""},
					Resources:   []string{"*/*"},
				},
			},
			{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{"*"},
					APIVersions: []string{"*"},
					Resources:   []string{""},
				},
			},
			{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"", "pods"},
				},
			},
		}).
		WithMatchConditions([]admissionregistrationv1.MatchCondition{
			{
				Name:       "foo",
				Expression: "1 + 1",
			},
			{
				Name:       "foo",
				Expression: "invalid expression",
			},
		}).
		Build()
	warnings, err := policy.ValidateCreate()
	require.Error(t, err)
	require.Empty(t, warnings)
}
