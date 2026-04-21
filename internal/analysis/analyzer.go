package analysis

import (
	"fmt"
	"time"

	"github.com/ohanyere/kubernetes-cost-guard/internal/domain"
)

const (
	overprovisionedThresholdPercent = int64(30)
	highReplicaCount                = int32(3)
)

func AnalyzeWorkload(resources domain.WorkloadResources, usage domain.UsageSnapshot) AnalysisResult {
	findings := make([]Finding, 0)
	findings = append(findings, findMissingResourceFindings(resources)...)

	if !usage.MetricsAvailable {
		findings = append(findings, metricsUnavailableFinding(usage))
	} else {
		findings = append(findings, findOverprovisionedResources(resources, usage)...)
		if finding, ok := findUnderutilizedReplicas(resources, usage); ok {
			findings = append(findings, finding)
		}
	}

	return AnalysisResult{
		Ref:        resources.Ref,
		Resources:  resources,
		Usage:      usage,
		Findings:   findings,
		AnalyzedAt: time.Now().UTC(),
	}
}

func findMissingResourceFindings(resources domain.WorkloadResources) []Finding {
	var findings []Finding

	for _, container := range resources.Containers {
		if container.Requests.CPUMilli == 0 {
			findings = append(findings, Finding{
				Type:     FindingTypeCPURequestMissing,
				Severity: SeverityMedium,
				Message:  fmt.Sprintf("container %q has no CPU request", container.Name),
				Details:  map[string]string{"container": container.Name, "resource": "cpu"},
			})
		}
		if container.Requests.MemoryMiB == 0 {
			findings = append(findings, Finding{
				Type:     FindingTypeMemoryRequestMissing,
				Severity: SeverityMedium,
				Message:  fmt.Sprintf("container %q has no memory request", container.Name),
				Details:  map[string]string{"container": container.Name, "resource": "memory"},
			})
		}
		if container.Limits.CPUMilli == 0 {
			findings = append(findings, Finding{
				Type:     FindingTypeCPULimitMissing,
				Severity: SeverityLow,
				Message:  fmt.Sprintf("container %q has no CPU limit", container.Name),
				Details:  map[string]string{"container": container.Name, "resource": "cpu"},
			})
		}
		if container.Limits.MemoryMiB == 0 {
			findings = append(findings, Finding{
				Type:     FindingTypeMemoryLimitMissing,
				Severity: SeverityLow,
				Message:  fmt.Sprintf("container %q has no memory limit", container.Name),
				Details:  map[string]string{"container": container.Name, "resource": "memory"},
			})
		}
	}

	return findings
}

func findOverprovisionedResources(resources domain.WorkloadResources, usage domain.UsageSnapshot) []Finding {
	usageByContainer := usageByContainerName(usage)
	replicas := effectiveReplicas(resources)
	var findings []Finding

	for _, container := range resources.Containers {
		containerUsage := usageByContainer[container.Name]
		requestedCPU := container.Requests.CPUMilli * int64(replicas)
		if requestedCPU > 0 && utilizationPercent(containerUsage.CPUUsedMilli, requestedCPU) < overprovisionedThresholdPercent {
			findings = append(findings, Finding{
				Type:     FindingTypeCPUOverprovisioned,
				Severity: SeverityMedium,
				Message:  fmt.Sprintf("container %q is using less than %d%% of requested CPU", container.Name, overprovisionedThresholdPercent),
				Details: map[string]string{
					"container":         container.Name,
					"requestedCpuMilli": fmt.Sprintf("%d", requestedCPU),
					"usedCpuMilli":      fmt.Sprintf("%d", containerUsage.CPUUsedMilli),
					"thresholdPercent":  fmt.Sprintf("%d", overprovisionedThresholdPercent),
				},
			})
		}

		requestedMemory := container.Requests.MemoryMiB * int64(replicas)
		if requestedMemory > 0 && utilizationPercent(containerUsage.MemoryUsedMiB, requestedMemory) < overprovisionedThresholdPercent {
			findings = append(findings, Finding{
				Type:     FindingTypeMemoryOverprovisioned,
				Severity: SeverityMedium,
				Message:  fmt.Sprintf("container %q is using less than %d%% of requested memory", container.Name, overprovisionedThresholdPercent),
				Details: map[string]string{
					"container":          container.Name,
					"requestedMemoryMiB": fmt.Sprintf("%d", requestedMemory),
					"usedMemoryMiB":      fmt.Sprintf("%d", containerUsage.MemoryUsedMiB),
					"thresholdPercent":   fmt.Sprintf("%d", overprovisionedThresholdPercent),
				},
			})
		}
	}

	return findings
}

func findUnderutilizedReplicas(resources domain.WorkloadResources, usage domain.UsageSnapshot) (Finding, bool) {
	replicas := effectiveReplicas(resources)
	if replicas < highReplicaCount {
		return Finding{}, false
	}

	requestedCPU, requestedMemory := totalRequested(resources, replicas)
	usedCPU, usedMemory := totalUsed(usage)
	if requestedCPU == 0 || requestedMemory == 0 {
		return Finding{}, false
	}

	cpuPercent := utilizationPercent(usedCPU, requestedCPU)
	memoryPercent := utilizationPercent(usedMemory, requestedMemory)
	if cpuPercent >= overprovisionedThresholdPercent || memoryPercent >= overprovisionedThresholdPercent {
		return Finding{}, false
	}

	return Finding{
		Type:     FindingTypeUnderutilizedReplicas,
		Severity: SeverityLow,
		Message:  fmt.Sprintf("workload has %d replicas with low CPU and memory usage", replicas),
		Details: map[string]string{
			"replicas":             fmt.Sprintf("%d", replicas),
			"cpuUtilizationPct":    fmt.Sprintf("%d", cpuPercent),
			"memoryUtilizationPct": fmt.Sprintf("%d", memoryPercent),
			"thresholdPercent":     fmt.Sprintf("%d", overprovisionedThresholdPercent),
		},
	}, true
}

func metricsUnavailableFinding(usage domain.UsageSnapshot) Finding {
	details := map[string]string{"metricsAvailable": "false"}
	if len(usage.Warnings) > 0 {
		details["warning"] = usage.Warnings[0]
	}

	return Finding{
		Type:     FindingTypeMetricsUnavailable,
		Severity: SeverityLow,
		Message:  "metrics are unavailable, so usage-based findings are limited",
		Details:  details,
	}
}

func usageByContainerName(usage domain.UsageSnapshot) map[string]domain.ContainerUsage {
	containers := make(map[string]domain.ContainerUsage, len(usage.Containers))
	for _, container := range usage.Containers {
		containers[container.Name] = container
	}
	return containers
}

func effectiveReplicas(resources domain.WorkloadResources) int32 {
	if resources.DesiredReplicas > 0 {
		return resources.DesiredReplicas
	}
	if resources.Replicas > 0 {
		return resources.Replicas
	}
	return 1
}

func totalRequested(resources domain.WorkloadResources, replicas int32) (int64, int64) {
	var cpuMilli int64
	var memoryMiB int64
	for _, container := range resources.Containers {
		cpuMilli += container.Requests.CPUMilli * int64(replicas)
		memoryMiB += container.Requests.MemoryMiB * int64(replicas)
	}
	return cpuMilli, memoryMiB
}

func totalUsed(usage domain.UsageSnapshot) (int64, int64) {
	var cpuMilli int64
	var memoryMiB int64
	for _, container := range usage.Containers {
		cpuMilli += container.CPUUsedMilli
		memoryMiB += container.MemoryUsedMiB
	}
	return cpuMilli, memoryMiB
}

func utilizationPercent(used, requested int64) int64 {
	if requested <= 0 {
		return 0
	}
	return used * 100 / requested
}
