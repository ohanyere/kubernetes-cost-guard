package analysis

import (
	"testing"

	"github.com/ohanyere/kubernetes-cost-guard/internal/domain"
)

func TestAnalyzeNamespaceNoWorkloads(t *testing.T) {
	got := AnalyzeNamespace("empty", nil)

	if got.Namespace != "empty" {
		t.Fatalf("unexpected namespace: %s", got.Namespace)
	}
	if got.WorkloadCount != 0 {
		t.Fatalf("expected no workloads, got %d", got.WorkloadCount)
	}
	if len(got.Workloads) != 0 || len(got.TopWastefulWorkloads) != 0 || len(got.Findings) != 0 {
		t.Fatalf("expected empty analysis, got %#v", got)
	}
}

func TestAnalyzeNamespaceAggregatesWorkloadsWithFindings(t *testing.T) {
	got := AnalyzeNamespace("default", []WorkloadInput{
		{
			Resources: namespaceWorkload("web", 2, domain.ContainerResources{
				Name:     "app",
				Requests: domain.ResourceQuantity{CPUMilli: 1000, MemoryMiB: 512},
				Limits:   domain.ResourceQuantity{CPUMilli: 1500, MemoryMiB: 1024},
			}),
			Usage: namespaceUsage("web", true, domain.ContainerUsage{
				Name:          "app",
				CPUUsedMilli:  200,
				MemoryUsedMiB: 800,
			}),
		},
		{
			Resources: namespaceWorkload("worker", 1, domain.ContainerResources{
				Name:     "job",
				Requests: domain.ResourceQuantity{CPUMilli: 500, MemoryMiB: 256},
				Limits:   domain.ResourceQuantity{CPUMilli: 1000, MemoryMiB: 512},
			}),
			Usage: namespaceUsage("worker", true, domain.ContainerUsage{
				Name:          "job",
				CPUUsedMilli:  400,
				MemoryUsedMiB: 128,
			}),
		},
	})

	if got.WorkloadCount != 2 {
		t.Fatalf("expected two workloads, got %d", got.WorkloadCount)
	}
	if got.TotalRequested.CPUMilli != 2500 || got.TotalRequested.MemoryMiB != 1280 {
		t.Fatalf("unexpected requested totals: %#v", got.TotalRequested)
	}
	if got.TotalObserved.CPUMilli != 600 || got.TotalObserved.MemoryMiB != 928 {
		t.Fatalf("unexpected observed totals: %#v", got.TotalObserved)
	}
	if got.FindingsByType[FindingTypeCPUOverprovisioned] == 0 {
		t.Fatalf("expected CPU overprovisioning count, got %#v", got.FindingsByType)
	}
	if len(got.TopWastefulWorkloads) == 0 || got.TopWastefulWorkloads[0].Name != "web" {
		t.Fatalf("expected web to rank first, got %#v", got.TopWastefulWorkloads)
	}
}

func TestAnalyzeNamespaceMetricsMissing(t *testing.T) {
	got := AnalyzeNamespace("default", []WorkloadInput{
		{
			Resources: namespaceWorkload("web", 1, domain.ContainerResources{
				Name:     "app",
				Requests: domain.ResourceQuantity{CPUMilli: 500, MemoryMiB: 256},
				Limits:   domain.ResourceQuantity{CPUMilli: 1000, MemoryMiB: 512},
			}),
			Usage: domain.UsageSnapshot{
				Ref:              domain.WorkloadRef{Namespace: "default", Name: "web", Kind: domain.WorkloadKindDeployment},
				MetricsAvailable: false,
				Warnings:         []string{"metrics-server unavailable"},
			},
		},
	})

	if got.MetricsAvailable {
		t.Fatal("expected namespace metrics to be unavailable")
	}
	if got.FindingsByType[FindingTypeMetricsUnavailable] != 1 {
		t.Fatalf("expected workload metrics finding count, got %#v", got.FindingsByType)
	}
	if !hasAnyFinding(got.Findings, FindingTypeMetricsMissingWide) {
		t.Fatalf("expected namespace metrics finding, got %#v", got.Findings)
	}
}

func namespaceWorkload(name string, replicas int32, containers ...domain.ContainerResources) domain.WorkloadResources {
	return domain.WorkloadResources{
		Ref:             domain.WorkloadRef{Namespace: "default", Name: name, Kind: domain.WorkloadKindDeployment},
		Replicas:        replicas,
		DesiredReplicas: replicas,
		Containers:      containers,
	}
}

func namespaceUsage(name string, metricsAvailable bool, containers ...domain.ContainerUsage) domain.UsageSnapshot {
	return domain.UsageSnapshot{
		Ref:              domain.WorkloadRef{Namespace: "default", Name: name, Kind: domain.WorkloadKindDeployment},
		MetricsAvailable: metricsAvailable,
		Containers:       containers,
	}
}
