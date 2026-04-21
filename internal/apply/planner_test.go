package apply

import (
	"testing"

	"github.com/ohanyere/kubernetes-cost-guard/internal/domain"
	"github.com/ohanyere/kubernetes-cost-guard/internal/patch"
	"github.com/ohanyere/kubernetes-cost-guard/internal/recommendation"
)

func TestPlanSkipsProtectedNamespace(t *testing.T) {
	rec := safeCPURecommendation("kube-system")

	got := Plan([]recommendation.Recommendation{rec}, patch.Preview([]recommendation.Recommendation{rec}), true)

	if len(got.Operations) != 0 {
		t.Fatalf("expected no apply operations, got %d", len(got.Operations))
	}
	if len(got.Skipped) != 1 {
		t.Fatalf("expected one skipped operation, got %d", len(got.Skipped))
	}
	if got.Skipped[0].SkipReason != "namespace is protected" {
		t.Fatalf("unexpected skip reason: %s", got.Skipped[0].SkipReason)
	}
}

func TestPlanSkipsUnsafeRecommendation(t *testing.T) {
	rec := safeCPURecommendation("default")
	rec.SafetyLevel = recommendation.SafetyLevelCautious

	got := Plan([]recommendation.Recommendation{rec}, patch.Preview([]recommendation.Recommendation{rec}), true)

	if len(got.Operations) != 0 {
		t.Fatalf("expected no apply operations, got %d", len(got.Operations))
	}
	if len(got.Skipped) != 1 {
		t.Fatalf("expected one skipped operation, got %d", len(got.Skipped))
	}
	if got.Skipped[0].SkipReason != "recommendation is not marked safe" {
		t.Fatalf("unexpected skip reason: %s", got.Skipped[0].SkipReason)
	}
}

func TestPlanSkipsUsageBasedWhenMetricsMissing(t *testing.T) {
	rec := safeCPURecommendation("default")

	got := Plan([]recommendation.Recommendation{rec}, patch.Preview([]recommendation.Recommendation{rec}), false)

	if len(got.Operations) != 0 {
		t.Fatalf("expected no apply operations, got %d", len(got.Operations))
	}
	if len(got.Skipped) != 1 {
		t.Fatalf("expected one skipped operation, got %d", len(got.Skipped))
	}
	if got.Skipped[0].SkipReason != "metrics are required for usage-based changes" {
		t.Fatalf("unexpected skip reason: %s", got.Skipped[0].SkipReason)
	}
}

func TestPlanAppliesOnlySafeOperations(t *testing.T) {
	rec := safeCPURecommendation("default")

	got := Plan([]recommendation.Recommendation{rec}, patch.Preview([]recommendation.Recommendation{rec}), true)

	if len(got.Operations) != 1 {
		t.Fatalf("expected one apply operation, got %d", len(got.Operations))
	}
	if len(got.Applied) != 1 {
		t.Fatalf("expected one applied result, got %d", len(got.Applied))
	}
	if got.Applied[0].Reason != rec.Reason {
		t.Fatalf("unexpected apply reason: %s", got.Applied[0].Reason)
	}
}

func safeCPURecommendation(namespace string) recommendation.Recommendation {
	return recommendation.Recommendation{
		Type:            recommendation.RecommendationReduceCPURequest,
		Target:          domain.WorkloadRef{Namespace: namespace, Name: "web", Kind: domain.WorkloadKindDeployment},
		TargetContainer: "app",
		CurrentValue:    "1000m",
		SuggestedValue:  "750m",
		Reason:          "CPU request can be reduced safely.",
		SafetyLevel:     recommendation.SafetyLevelSafe,
	}
}
