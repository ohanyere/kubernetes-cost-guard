package recommendation

import (
	"testing"

	"github.com/ohanyere/kubernetes-cost-guard/internal/analysis"
	"github.com/ohanyere/kubernetes-cost-guard/internal/domain"
)

func TestGenerateRecommendations(t *testing.T) {
	tests := []struct {
		name          string
		result        analysis.AnalysisResult
		wantTypes     []RecommendationType
		wantSuggested []string
	}{
		{
			name: "cpu overprovisioned creates cpu recommendation",
			result: workloadAnalysis(
				domain.ResourceQuantity{CPUMilli: 1000, MemoryMiB: 512},
				domain.ContainerUsage{Name: "app", CPUUsedMilli: 100, MemoryUsedMiB: 400},
				true,
				analysis.Finding{
					Type:    analysis.FindingTypeCPUOverprovisioned,
					Details: map[string]string{"container": "app"},
				},
			),
			wantTypes:     []RecommendationType{RecommendationReduceCPURequest},
			wantSuggested: []string{"750m"},
		},
		{
			name: "memory overprovisioned creates memory recommendation",
			result: workloadAnalysis(
				domain.ResourceQuantity{CPUMilli: 500, MemoryMiB: 1024},
				domain.ContainerUsage{Name: "app", CPUUsedMilli: 200, MemoryUsedMiB: 128},
				true,
				analysis.Finding{
					Type:    analysis.FindingTypeMemoryOverprovisioned,
					Details: map[string]string{"container": "app"},
				},
			),
			wantTypes:     []RecommendationType{RecommendationReduceMemoryRequest},
			wantSuggested: []string{"768Mi"},
		},
		{
			name: "missing requests create baseline recommendations",
			result: workloadAnalysis(
				domain.ResourceQuantity{},
				domain.ContainerUsage{Name: "app"},
				false,
				analysis.Finding{
					Type:    analysis.FindingTypeCPURequestMissing,
					Details: map[string]string{"container": "app"},
				},
				analysis.Finding{
					Type:    analysis.FindingTypeMemoryRequestMissing,
					Details: map[string]string{"container": "app"},
				},
			),
			wantTypes:     []RecommendationType{RecommendationAddCPURequest, RecommendationAddMemoryRequest},
			wantSuggested: []string{"100m", "128Mi"},
		},
		{
			name: "underutilized replicas creates conservative replica recommendation",
			result: workloadAnalysis(
				domain.ResourceQuantity{CPUMilli: 500, MemoryMiB: 512},
				domain.ContainerUsage{Name: "app", CPUUsedMilli: 100, MemoryUsedMiB: 128},
				true,
				analysis.Finding{
					Type: analysis.FindingTypeUnderutilizedReplicas,
				},
			),
			wantTypes:     []RecommendationType{RecommendationReduceReplicas},
			wantSuggested: []string{"2"},
		},
		{
			name: "missing metrics skips usage based recommendation",
			result: workloadAnalysis(
				domain.ResourceQuantity{CPUMilli: 1000, MemoryMiB: 1024},
				domain.ContainerUsage{Name: "app", CPUUsedMilli: 100, MemoryUsedMiB: 128},
				false,
				analysis.Finding{
					Type:    analysis.FindingTypeCPUOverprovisioned,
					Details: map[string]string{"container": "app"},
				},
				analysis.Finding{
					Type: analysis.FindingTypeUnderutilizedReplicas,
				},
			),
			wantTypes:     nil,
			wantSuggested: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Generate(tt.result)
			if len(got.Recommendations) != len(tt.wantTypes) {
				t.Fatalf("expected %d recommendations, got %d: %#v", len(tt.wantTypes), len(got.Recommendations), got.Recommendations)
			}
			for i, wantType := range tt.wantTypes {
				if got.Recommendations[i].Type != wantType {
					t.Fatalf("recommendation %d type: expected %s, got %s", i, wantType, got.Recommendations[i].Type)
				}
				if got.Recommendations[i].SuggestedValue != tt.wantSuggested[i] {
					t.Fatalf("recommendation %d suggested value: expected %s, got %s", i, tt.wantSuggested[i], got.Recommendations[i].SuggestedValue)
				}
				if got.Recommendations[i].Reason == "" {
					t.Fatalf("recommendation %d has empty reason", i)
				}
			}
		})
	}
}

func workloadAnalysis(requests domain.ResourceQuantity, usage domain.ContainerUsage, metricsAvailable bool, findings ...analysis.Finding) analysis.AnalysisResult {
	ref := domain.WorkloadRef{Namespace: "default", Name: "web", Kind: domain.WorkloadKindDeployment}
	return analysis.AnalysisResult{
		Ref: ref,
		Resources: domain.WorkloadResources{
			Ref:             ref,
			DesiredReplicas: 3,
			Containers: []domain.ContainerResources{
				{Name: "app", Requests: requests},
			},
		},
		Usage: domain.UsageSnapshot{
			Ref:              ref,
			MetricsAvailable: metricsAvailable,
			Containers:       []domain.ContainerUsage{usage},
		},
		Findings: findings,
	}
}
