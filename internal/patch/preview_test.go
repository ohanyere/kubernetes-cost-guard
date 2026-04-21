package patch

import (
	"testing"

	"github.com/ohanyere/kubernetes-cost-guard/internal/domain"
	"github.com/ohanyere/kubernetes-cost-guard/internal/recommendation"
)

func TestPreviewBuildsPatchOperations(t *testing.T) {
	ref := domain.WorkloadRef{Namespace: "default", Name: "web", Kind: domain.WorkloadKindDeployment}
	recommendations := []recommendation.Recommendation{
		{
			Type:            recommendation.RecommendationReduceCPURequest,
			Target:          ref,
			TargetContainer: "app",
			CurrentValue:    "1000m",
			SuggestedValue:  "750m",
		},
		{
			Type:            recommendation.RecommendationAddMemoryRequest,
			Target:          ref,
			TargetContainer: "sidecar",
			CurrentValue:    "0Mi",
			SuggestedValue:  "128Mi",
		},
		{
			Type:           recommendation.RecommendationReduceReplicas,
			Target:         ref,
			CurrentValue:   "3",
			SuggestedValue: "2",
		},
	}

	got := Preview(recommendations)

	if len(got.Operations) != 3 {
		t.Fatalf("expected 3 operations, got %d", len(got.Operations))
	}

	assertOperation(t, got.Operations[0], ref, "app", cpuRequestPath, "1000m", "750m")
	assertOperation(t, got.Operations[1], ref, "sidecar", memoryRequestPath, "0Mi", "128Mi")
	assertOperation(t, got.Operations[2], ref, "", replicasPath, "3", "2")
}

func assertOperation(t *testing.T, got PatchOperation, ref domain.WorkloadRef, container, fieldPath, currentValue, newValue string) {
	t.Helper()

	if got.Target != ref {
		t.Fatalf("target: expected %#v, got %#v", ref, got.Target)
	}
	if got.TargetContainer != container {
		t.Fatalf("target container: expected %q, got %q", container, got.TargetContainer)
	}
	if got.FieldPath != fieldPath {
		t.Fatalf("field path: expected %q, got %q", fieldPath, got.FieldPath)
	}
	if got.CurrentValue != currentValue {
		t.Fatalf("current value: expected %q, got %q", currentValue, got.CurrentValue)
	}
	if got.NewValue != newValue {
		t.Fatalf("new value: expected %q, got %q", newValue, got.NewValue)
	}
}
