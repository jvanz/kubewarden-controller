/*
Copyright 2022.

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

package controllers

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1alpha2 "github.com/kubewarden/kubewarden-controller/apis/v1alpha2"
)

var _ = Describe("Given a ClusterAdmissionPolicy", func() {
	When("it does not have a status set", func() {
		Context("and it is not deleted", func() {
			Context("and it has an empty policy server set on its spec", func() {
				var (
					policyName = "unscheduled-policy"
				)
				BeforeEach(func() {
					Expect(
						k8sClient.Create(ctx, clusterAdmissionPolicyWithPolicyServerName(policyName, "")),
					).To(HaveSucceededOrAlreadyExisted())
				})
				It(fmt.Sprintf("should set its policy status to %q", v1alpha2.PolicyStatusUnscheduled), func() {
					Eventually(func(g Gomega) (*v1alpha2.ClusterAdmissionPolicy, error) {
						return getFreshClusterAdmissionPolicy(policyName)
					}).Should(
						WithTransform(
							func(clusterAdmissionPolicy *v1alpha2.ClusterAdmissionPolicy) v1alpha2.PolicyStatusEnum {
								return clusterAdmissionPolicy.Status.PolicyStatus
							},
							Equal(v1alpha2.PolicyStatusUnscheduled),
						),
					)
				})
			})
			Context("and it has a non-empty policy server set on its spec", func() {
				var (
					policyName       = "scheduled-policy"
					policyServerName = "other-policy-server"
				)
				BeforeEach(func() {
					Expect(
						k8sClient.Create(ctx, clusterAdmissionPolicyWithPolicyServerName(policyName, policyServerName)),
					).To(HaveSucceededOrAlreadyExisted())
				})
				It(fmt.Sprintf("should set its policy status to %q", v1alpha2.PolicyStatusScheduled), func() {
					Eventually(func(g Gomega) (*v1alpha2.ClusterAdmissionPolicy, error) {
						return getFreshClusterAdmissionPolicy(policyName)
					}).Should(
						WithTransform(
							func(clusterAdmissionPolicy *v1alpha2.ClusterAdmissionPolicy) v1alpha2.PolicyStatusEnum {
								return clusterAdmissionPolicy.Status.PolicyStatus
							},
							Equal(v1alpha2.PolicyStatusScheduled),
						),
					)
				})
				Context("and the targeted policy server is created", func() {
					BeforeEach(func() {
						Expect(
							k8sClient.Create(ctx, policyServer(policyServerName)),
						).To(HaveSucceededOrAlreadyExisted())
					})
					It(fmt.Sprintf("should set its policy status to %q", v1alpha2.PolicyStatusPending), func() {
						Eventually(func(g Gomega) (*v1alpha2.ClusterAdmissionPolicy, error) {
							return getFreshClusterAdmissionPolicy(policyName)
						}, 30*time.Second, 250*time.Millisecond).Should(
							WithTransform(
								func(clusterAdmissionPolicy *v1alpha2.ClusterAdmissionPolicy) v1alpha2.PolicyStatusEnum {
									return clusterAdmissionPolicy.Status.PolicyStatus
								},
								Equal(v1alpha2.PolicyStatusPending),
							),
						)
					})
				})
			})
		})
	})
})
