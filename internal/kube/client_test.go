package kube

import (
	"encoding/json"
	"testing"

	"github.com/ohanyere/kubernetes-cost-guard/internal/change"
	"github.com/ohanyere/kubernetes-cost-guard/internal/domain"
)

func TestDeploymentPatchPayloadUsesMinimalSupportedFields(t *testing.T) {
	target := domain.WorkloadRef{Namespace: "default", Name: "web", Kind: domain.WorkloadKindDeployment}

	payloadBytes, err := deploymentPatchPayload([]change.Operation{
		{
			Target:       target,
			FieldPath:    "spec.replicas",
			CurrentValue: "3",
			NewValue:     "2",
		},
		{
			Target:          target,
			TargetContainer: "worker",
			FieldPath:       "spec.template.spec.containers[].resources.requests.cpu",
			CurrentValue:    "500m",
			NewValue:        "400m",
		},
		{
			Target:          target,
			TargetContainer: "app",
			FieldPath:       "spec.template.spec.containers[].resources.requests.cpu",
			CurrentValue:    "1000m",
			NewValue:        "750m",
		},
		{
			Target:          target,
			TargetContainer: "app",
			FieldPath:       "spec.template.spec.containers[].resources.requests.memory",
			CurrentValue:    "512Mi",
			NewValue:        "384Mi",
		},
	})
	if err != nil {
		t.Fatalf("unexpected payload error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		t.Fatalf("unmarshal patch payload: %v", err)
	}
	spec := payload["spec"].(map[string]any)
	if got := spec["replicas"]; got != float64(2) {
		t.Fatalf("unexpected replicas patch: %#v", got)
	}
	template := spec["template"].(map[string]any)
	podSpec := template["spec"].(map[string]any)
	containers := podSpec["containers"].([]any)
	if len(containers) != 2 {
		t.Fatalf("expected two patched containers, got %d", len(containers))
	}
	container := containers[0].(map[string]any)
	if got := container["name"]; got != "app" {
		t.Fatalf("expected deterministic first patched container, got %#v", got)
	}
	resources := container["resources"].(map[string]any)
	requests := resources["requests"].(map[string]any)
	if got := requests["cpu"]; got != "750m" {
		t.Fatalf("unexpected cpu request patch: %#v", got)
	}
	if got := requests["memory"]; got != "384Mi" {
		t.Fatalf("unexpected memory request patch: %#v", got)
	}
	if _, ok := resources["limits"]; ok {
		t.Fatalf("must not patch unsupported limits field: %#v", resources)
	}
	container = containers[1].(map[string]any)
	if got := container["name"]; got != "worker" {
		t.Fatalf("expected deterministic second patched container, got %#v", got)
	}
}

func TestDeploymentPatchPayloadRejectsUnsupportedFields(t *testing.T) {
	target := domain.WorkloadRef{Namespace: "default", Name: "web", Kind: domain.WorkloadKindDeployment}

	_, err := deploymentPatchPayload([]change.Operation{
		{
			Target:       target,
			FieldPath:    "spec.template.spec.containers[].resources.limits.cpu",
			CurrentValue: "1000m",
			NewValue:     "750m",
		},
	})
	if err == nil {
		t.Fatal("expected unsupported field error")
	}
}
