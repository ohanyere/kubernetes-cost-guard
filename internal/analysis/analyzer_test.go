package analysis

import (
	"testing"

	"github.com/ohanyere/kubernetes-cost-guard/internal/domain"
)

func TestAnalyzeWorkloadFindings(t *testing.T) {
	tests := []struct {
		name        string
		resources   domain.WorkloadResources
		usage       domain.UsageSnapshot
		wantFinding FindingType
	}{
		{
			name: "missing requests",
			resources: workloadResources(1, domain.ContainerResources{
				Name:     "app",
				Requests: domain.ResourceQuantity{},
				Limits:   domain.ResourceQuantity{CPUMilli: 500, MemoryMiB: 512},
			}),
			usage:       usageSnapshot(true, domain.ContainerUsage{Name: "app", CPUUsedMilli: 100, MemoryUsedMiB: 100}),
			wantFinding: FindingTypeCPURequestMissing,
		},
		{
			name: "overprovisioned cpu",
			resources: workloadResources(1, domain.ContainerResources{
				Name:     "app",
				Requests: domain.ResourceQuantity{CPUMilli: 1000, MemoryMiB: 512},
				Limits:   domain.ResourceQuantity{CPUMilli: 1500, MemoryMiB: 1024},
			}),
			usage:       usageSnapshot(true, domain.ContainerUsage{Name: "app", CPUUsedMilli: 200, MemoryUsedMiB: 400}),
			wantFinding: FindingTypeCPUOverprovisioned,
		},
		{
			name: "overprovisioned memory",
			resources: workloadResources(1, domain.ContainerResources{
				Name:     "app",
				Requests: domain.ResourceQuantity{CPUMilli: 500, MemoryMiB: 1000},
				Limits:   domain.ResourceQuantity{CPUMilli: 1000, MemoryMiB: 1500},
			}),
			usage:       usageSnapshot(true, domain.ContainerUsage{Name: "app", CPUUsedMilli: 400, MemoryUsedMiB: 200}),
			wantFinding: FindingTypeMemoryOverprovisioned,
		},
		{
			name: "metrics missing",
			resources: workloadResources(1, domain.ContainerResources{
				Name:     "app",
				Requests: domain.ResourceQuantity{CPUMilli: 500, MemoryMiB: 512},
				Limits:   domain.ResourceQuantity{CPUMilli: 1000, MemoryMiB: 1024},
			}),
			usage: domain.UsageSnapshot{
				MetricsAvailable: false,
				Warnings:         []string{"metrics-server unavailable"},
			},
			wantFinding: FindingTypeMetricsUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnalyzeWorkload(tt.resources, tt.usage)

			if !hasFinding(got.Findings, tt.wantFinding) {
				t.Fatalf("expected finding %q, got %#v", tt.wantFinding, got.Findings)
			}
		})
	}
}

func workloadResources(replicas int32, containers ...domain.ContainerResources) domain.WorkloadResources {
	return domain.WorkloadResources{
		Ref:             domain.WorkloadRef{Namespace: "default", Name: "web", Kind: domain.WorkloadKindDeployment},
		Replicas:        replicas,
		DesiredReplicas: replicas,
		Containers:      containers,
	}
}

func usageSnapshot(metricsAvailable bool, containers ...domain.ContainerUsage) domain.UsageSnapshot {
	return domain.UsageSnapshot{
		Ref:              domain.WorkloadRef{Namespace: "default", Name: "web", Kind: domain.WorkloadKindDeployment},
		MetricsAvailable: metricsAvailable,
		Containers:       containers,
	}
}

func hasFinding(findings []Finding, findingType FindingType) bool {
	for _, finding := range findings {
		if finding.Type == findingType {
			return true
		}
	}
	return false
}
