package analysis

import (
	"fmt"
	"sort"
	"time"

	"github.com/ohanyere/kubernetes-cost-guard/internal/domain"
)

const topWastefulWorkloadCount = 5

const (
	lowSeverityScore    = 1
	mediumSeverityScore = 2
	highSeverityScore   = 3
)

func AnalyzeNamespace(namespace string, workloads []WorkloadInput) NamespaceAnalysisResult {
	summaries := make([]WorkloadSummary, 0, len(workloads))
	findingsByType := make(map[FindingType]int)
	metricsAvailable := len(workloads) > 0
	var requestedTotals domain.ResourceQuantity
	var totalObserved domain.ResourceQuantity
	var missingRequestWorkloads int
	var metricsMissingWorkloads int
	var overprovisionedWorkloads int

	for _, workload := range workloads {
		analysis := AnalyzeWorkload(workload.Resources, workload.Usage)
		replicas := effectiveReplicas(workload.Resources)
		requestedCPU, requestedMemory := totalRequested(workload.Resources, replicas)
		observedCPU, observedMemory := totalUsed(workload.Usage)

		requestedTotals.CPUMilli += requestedCPU
		requestedTotals.MemoryMiB += requestedMemory
		if workload.Usage.MetricsAvailable {
			totalObserved.CPUMilli += observedCPU
			totalObserved.MemoryMiB += observedMemory
		} else {
			metricsAvailable = false
		}

		summary := summarizeWorkload(analysis)
		summaries = append(summaries, summary)

		if hasAnyFinding(analysis.Findings, FindingTypeCPURequestMissing, FindingTypeMemoryRequestMissing) {
			missingRequestWorkloads++
		}
		if hasAnyFinding(analysis.Findings, FindingTypeMetricsUnavailable) {
			metricsMissingWorkloads++
		}
		if hasAnyFinding(analysis.Findings, FindingTypeCPUOverprovisioned, FindingTypeMemoryOverprovisioned) {
			overprovisionedWorkloads++
		}
		for _, finding := range analysis.Findings {
			findingsByType[finding.Type]++
		}
	}

	return NamespaceAnalysisResult{
		Namespace:            namespace,
		WorkloadCount:        len(workloads),
		TotalRequested:       requestedTotals,
		TotalObserved:        totalObserved,
		MetricsAvailable:     metricsAvailable,
		Workloads:            summaries,
		TopWastefulWorkloads: topWastefulWorkloads(summaries, topWastefulWorkloadCount),
		FindingsByType:       findingsByType,
		Findings:             namespaceFindings(len(workloads), missingRequestWorkloads, metricsMissingWorkloads, overprovisionedWorkloads),
		AnalyzedAt:           time.Now().UTC(),
	}
}

func summarizeWorkload(result AnalysisResult) WorkloadSummary {
	summary := WorkloadSummary{
		Name:          result.Ref.Name,
		Namespace:     result.Ref.Namespace,
		Kind:          string(result.Ref.Kind),
		Replicas:      effectiveReplicas(result.Resources),
		FindingsCount: len(result.Findings),
	}

	seenKeyFindings := make(map[FindingType]bool)
	for _, finding := range result.Findings {
		switch finding.Severity {
		case SeverityHigh:
			summary.HighCount++
			summary.SeverityScore += highSeverityScore
		case SeverityMedium:
			summary.MediumCount++
			summary.SeverityScore += mediumSeverityScore
		default:
			summary.LowCount++
			summary.SeverityScore += lowSeverityScore
		}
		if !seenKeyFindings[finding.Type] && len(summary.KeyFindings) < 3 {
			summary.KeyFindings = append(summary.KeyFindings, finding.Type)
			seenKeyFindings[finding.Type] = true
		}
	}

	return summary
}

func topWastefulWorkloads(workloads []WorkloadSummary, limit int) []WorkloadSummary {
	ranked := append([]WorkloadSummary(nil), workloads...)
	ranked = workloadsWithFindings(ranked)
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].SeverityScore == ranked[j].SeverityScore {
			return ranked[i].FindingsCount > ranked[j].FindingsCount
		}
		return ranked[i].SeverityScore > ranked[j].SeverityScore
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}

func workloadsWithFindings(workloads []WorkloadSummary) []WorkloadSummary {
	withFindings := workloads[:0]
	for _, workload := range workloads {
		if workload.FindingsCount > 0 {
			withFindings = append(withFindings, workload)
		}
	}
	return withFindings
}

func namespaceFindings(workloadCount, missingRequestWorkloads, metricsMissingWorkloads, overprovisionedWorkloads int) []Finding {
	if workloadCount == 0 {
		return nil
	}

	var findings []Finding
	if missingRequestWorkloads*2 >= workloadCount {
		findings = append(findings, Finding{
			Type:     FindingTypeMissingRequestsWide,
			Severity: SeverityMedium,
			Message:  "missing resource requests are widespread across the namespace",
			Details: map[string]string{
				"affectedWorkloads": fmt.Sprintf("%d", missingRequestWorkloads),
				"workloadCount":     fmt.Sprintf("%d", workloadCount),
			},
		})
	}
	if metricsMissingWorkloads == workloadCount {
		findings = append(findings, Finding{
			Type:     FindingTypeMetricsMissingWide,
			Severity: SeverityLow,
			Message:  "metrics are unavailable for all workloads in the namespace",
			Details: map[string]string{
				"affectedWorkloads": fmt.Sprintf("%d", metricsMissingWorkloads),
				"workloadCount":     fmt.Sprintf("%d", workloadCount),
			},
		})
	} else if metricsMissingWorkloads > 0 {
		findings = append(findings, Finding{
			Type:     FindingTypeMetricsMissingWide,
			Severity: SeverityLow,
			Message:  "metrics are unavailable for some workloads in the namespace",
			Details: map[string]string{
				"affectedWorkloads": fmt.Sprintf("%d", metricsMissingWorkloads),
				"workloadCount":     fmt.Sprintf("%d", workloadCount),
			},
		})
	}
	if overprovisionedWorkloads*2 >= workloadCount {
		findings = append(findings, Finding{
			Type:     FindingTypeHighOverprovisioning,
			Severity: SeverityMedium,
			Message:  "overprovisioning appears common across the namespace",
			Details: map[string]string{
				"affectedWorkloads": fmt.Sprintf("%d", overprovisionedWorkloads),
				"workloadCount":     fmt.Sprintf("%d", workloadCount),
			},
		})
	}
	return findings
}

func hasAnyFinding(findings []Finding, findingTypes ...FindingType) bool {
	for _, finding := range findings {
		for _, findingType := range findingTypes {
			if finding.Type == findingType {
				return true
			}
		}
	}
	return false
}
