package admission

import (
	"encoding/json"
	"testing"
)

func TestArePoliciesEqual(t *testing.T) {
	tests := []struct {
		name               string
		newPoliciesYML     string
		currentPoliciesYML string
		expect             bool
	}{{"same nil settings",
		"{\"privileged-pods\":{\"url\":\"registry://ghcr.io/kubewarden/policies/pod-privileged:v0.1.5\",\"settings\":null}}",
		"{\"privileged-pods\":{\"url\":\"registry://ghcr.io/kubewarden/policies/pod-privileged:v0.1.5\",\"settings\":null}}",
		false},
		{"same empty",
			"{}",
			"{}",
			false},
		{"same with settings",
			"{\"privileged-pods\":{\"url\":\"registry://ghcr.io/kubewarden/policies/pod-privileged:v0.1.5\",\"settings\":{\"name\":\"test\", \"list\":[\"one\",\"two\"]}}}",
			"{\"privileged-pods\":{\"url\":\"registry://ghcr.io/kubewarden/policies/pod-privileged:v0.1.5\",\"settings\":{\"name\":\"test\", \"list\":[\"one\",\"two\"]}}}",
			false},
		{"same with settings different order",
			"{\"privileged-pods\":{\"url\":\"registry://ghcr.io/kubewarden/policies/pod-privileged:v0.1.5\",\"settings\":{\"name\":\"test\", \"list\":[\"one\",\"two\"]}}}",
			"{\"privileged-pods\":{\"settings\":{\"name\":\"test\", \"list\":[\"one\",\"two\"]},\"url\":\"registry://ghcr.io/kubewarden/policies/pod-privileged:v0.1.5\"}}",
			false},
		{"2 policies same different order",
			"{\"privileged-pods\":{\"url\":\"registry://ghcr.io/kubewarden/policies/pod-privileged:v0.1.5\",\"settings\":null},\"psp-capabilities\":{\"url\":\"registry://ghcr.io/kubewarden/policies/psp-capabilities:v0.1.5\",\"settings\":null}}",
			"{\"psp-capabilities\":{\"url\":\"registry://ghcr.io/kubewarden/policies/psp-capabilities:v0.1.5\",\"settings\":null},\"privileged-pods\":{\"url\":\"registry://ghcr.io/kubewarden/policies/pod-privileged:v0.1.5\",\"settings\":null}}",
			false},
		{"different",
			"{\"privileged-pods\":{\"url\":\"registry://ghcr.io/kubewarden/policies/pod-privileged:v0.1.5\",\"settings\":null}}",
			"{\"psp-capabilities\":{\"url\":\"registry://ghcr.io/kubewarden/policies/psp-capabilities:v0.1.5\",\"settings\":null},\"privileged-pods\":{\"url\":\"registry://ghcr.io/kubewarden/policies/pod-privileged:v0.1.5\",\"settings\":null}}",
			true},
		{"different settings",
			"{\"privileged-pods\":{\"url\":\"registry://ghcr.io/kubewarden/policies/pod-privileged:v0.1.5\",\"settings\":{\"name\":\"test\", \"list\":[\"one\",\"two\"]}}}",
			"{\"privileged-pods\":{\"url\":\"registry://ghcr.io/kubewarden/policies/pod-privileged:v0.1.5\",\"settings\":{\"name\":\"test\", \"list\":[\"one\"]}}}",
			true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var currentPoliciesMap map[string]policyServerConfigEntry
			if err := json.Unmarshal([]byte(test.newPoliciesYML), &currentPoliciesMap); err != nil {
				t.Errorf("unexpected error %s", err.Error())
			}
			got, err := shouldUpdatePolicyMap(test.currentPoliciesYML, currentPoliciesMap)
			if err != nil {
				t.Errorf("unexpected error %s", err.Error())
			}
			if got != test.expect {
				t.Errorf("got %t, want %t", got, test.expect)
			}
		})
	}
}