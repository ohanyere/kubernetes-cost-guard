package analysis

import (
	"time"

	"github.com/ohanyere/kubernetes-cost-guard/internal/domain"
)

type FindingType string

const (
	FindingTypeCPURequestMissing     FindingType = "cpu_request_missing"
	FindingTypeMemoryRequestMissing  FindingType = "memory_request_missing"
	FindingTypeCPULimitMissing       FindingType = "cpu_limit_missing"
	FindingTypeMemoryLimitMissing    FindingType = "memory_limit_missing"
	FindingTypeCPUOverprovisioned    FindingType = "cpu_overprovisioned"
	FindingTypeMemoryOverprovisioned FindingType = "memory_overprovisioned"
	FindingTypeUnderutilizedReplicas FindingType = "underutilized_replicas"
	FindingTypeMetricsUnavailable    FindingType = "metrics_unavailable"
	FindingTypeMissingRequestsWide   FindingType = "missing_requests_widespread"
	FindingTypeMetricsMissingWide    FindingType = "namespace_metrics_unavailable"
	FindingTypeHighOverprovisioning  FindingType = "namespace_high_overprovisioning"
)

type Severity string

const (
	SeverityLow    Severity = "low"
	SeverityMedium Severity = "medium"
	SeverityHigh   Severity = "high"
)

type Finding struct {
	Type     FindingType       `json:"type"`
	Severity Severity          `json:"severity"`
	Message  string            `json:"message"`
	Details  map[string]string `json:"details,omitempty"`
}

type AnalysisResult struct {
	Ref        domain.WorkloadRef       `json:"ref"`
	Resources  domain.WorkloadResources `json:"resources"`
	Usage      domain.UsageSnapshot     `json:"usage"`
	Findings   []Finding                `json:"findings"`
	AnalyzedAt time.Time                `json:"analyzedAt"`
}

type WorkloadInput struct {
	Resources domain.WorkloadResources
	Usage     domain.UsageSnapshot
}

type WorkloadSummary struct {
	Name          string        `json:"name"`
	Namespace     string        `json:"namespace"`
	Kind          string        `json:"kind"`
	Replicas      int32         `json:"replicas"`
	FindingsCount int           `json:"findingsCount"`
	HighCount     int           `json:"highCount"`
	MediumCount   int           `json:"mediumCount"`
	LowCount      int           `json:"lowCount"`
	KeyFindings   []FindingType `json:"keyFindings,omitempty"`
	SeverityScore int           `json:"severityScore"`
}

type NamespaceAnalysisResult struct {
	Namespace            string                  `json:"namespace"`
	WorkloadCount        int                     `json:"workloadCount"`
	TotalRequested       domain.ResourceQuantity `json:"totalRequested"`
	TotalObserved        domain.ResourceQuantity `json:"totalObserved"`
	MetricsAvailable     bool                    `json:"metricsAvailable"`
	Workloads            []WorkloadSummary       `json:"workloads"`
	TopWastefulWorkloads []WorkloadSummary       `json:"topWastefulWorkloads"`
	FindingsByType       map[FindingType]int     `json:"findingsByType"`
	Findings             []Finding               `json:"findings"`
	AnalyzedAt           time.Time               `json:"analyzedAt"`
}
