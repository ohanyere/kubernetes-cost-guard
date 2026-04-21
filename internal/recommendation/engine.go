package recommendation

import (
	"fmt"

	"github.com/ohanyere/kubernetes-cost-guard/internal/analysis"
	"github.com/ohanyere/kubernetes-cost-guard/internal/domain"
)

const (
	baselineCPUMilli      = int64(100)
	baselineMemoryMiB     = int64(128)
	minCPUMilli           = int64(50)
	minMemoryMiB          = int64(64)
	maxReductionPercent   = int64(25)
	usageHeadroomFactor   = int64(2)
	minRecommendedReplica = int32(1)
)

func Generate(result analysis.AnalysisResult) RecommendationResult {
	recommendations := make([]Recommendation, 0)
	seen := make(map[string]bool)

	for _, finding := range result.Findings {
		switch finding.Type {
		case analysis.FindingTypeCPUOverprovisioned:
			if !result.Usage.MetricsAvailable {
				continue
			}
			if recommendation, ok := cpuReductionRecommendation(result, finding); ok && markSeen(seen, recommendation) {
				recommendations = append(recommendations, recommendation)
			}
		case analysis.FindingTypeMemoryOverprovisioned:
			if !result.Usage.MetricsAvailable {
				continue
			}
			if recommendation, ok := memoryReductionRecommendation(result, finding); ok && markSeen(seen, recommendation) {
				recommendations = append(recommendations, recommendation)
			}
		case analysis.FindingTypeCPURequestMissing:
			if recommendation, ok := addCPURequestRecommendation(result, finding); ok && markSeen(seen, recommendation) {
				recommendations = append(recommendations, recommendation)
			}
		case analysis.FindingTypeMemoryRequestMissing:
			if recommendation, ok := addMemoryRequestRecommendation(result, finding); ok && markSeen(seen, recommendation) {
				recommendations = append(recommendations, recommendation)
			}
		case analysis.FindingTypeUnderutilizedReplicas:
			if !result.Usage.MetricsAvailable {
				continue
			}
			if recommendation, ok := replicaReductionRecommendation(result); ok && markSeen(seen, recommendation) {
				recommendations = append(recommendations, recommendation)
			}
		}
	}

	return RecommendationResult{Recommendations: recommendations}
}

func cpuReductionRecommendation(result analysis.AnalysisResult, finding analysis.Finding) (Recommendation, bool) {
	containerName := finding.Details["container"]
	container, ok := findContainer(result.Resources, containerName)
	if !ok || container.Requests.CPUMilli <= 0 {
		return Recommendation{}, false
	}

	usage := findUsage(result.Usage, containerName)
	replicas := effectiveReplicas(result.Resources)
	usedPerReplica := ceilDiv(usage.CPUUsedMilli, int64(replicas))
	floor := maxInt64(minCPUMilli, usedPerReplica*usageHeadroomFactor)
	suggested := conservativeReduction(container.Requests.CPUMilli, floor)
	if suggested >= container.Requests.CPUMilli {
		return Recommendation{}, false
	}

	return Recommendation{
		Type:            RecommendationReduceCPURequest,
		Target:          result.Ref,
		TargetContainer: containerName,
		CurrentValue:    formatCPU(container.Requests.CPUMilli),
		SuggestedValue:  formatCPU(suggested),
		Reason:          fmt.Sprintf("CPU usage for container %q is below the analysis threshold; reduce the request by no more than %d%% while keeping headroom above observed usage.", containerName, maxReductionPercent),
		SafetyLevel:     SafetyLevelSafe,
	}, true
}

func memoryReductionRecommendation(result analysis.AnalysisResult, finding analysis.Finding) (Recommendation, bool) {
	containerName := finding.Details["container"]
	container, ok := findContainer(result.Resources, containerName)
	if !ok || container.Requests.MemoryMiB <= 0 {
		return Recommendation{}, false
	}

	usage := findUsage(result.Usage, containerName)
	replicas := effectiveReplicas(result.Resources)
	usedPerReplica := ceilDiv(usage.MemoryUsedMiB, int64(replicas))
	floor := maxInt64(minMemoryMiB, usedPerReplica*usageHeadroomFactor)
	suggested := conservativeReduction(container.Requests.MemoryMiB, floor)
	if suggested >= container.Requests.MemoryMiB {
		return Recommendation{}, false
	}

	return Recommendation{
		Type:            RecommendationReduceMemoryRequest,
		Target:          result.Ref,
		TargetContainer: containerName,
		CurrentValue:    formatMemory(container.Requests.MemoryMiB),
		SuggestedValue:  formatMemory(suggested),
		Reason:          fmt.Sprintf("Memory usage for container %q is below the analysis threshold; reduce the request by no more than %d%% while keeping headroom above observed usage.", containerName, maxReductionPercent),
		SafetyLevel:     SafetyLevelSafe,
	}, true
}

func addCPURequestRecommendation(result analysis.AnalysisResult, finding analysis.Finding) (Recommendation, bool) {
	containerName := finding.Details["container"]
	container, ok := findContainer(result.Resources, containerName)
	if !ok || container.Requests.CPUMilli != 0 {
		return Recommendation{}, false
	}

	return Recommendation{
		Type:            RecommendationAddCPURequest,
		Target:          result.Ref,
		TargetContainer: containerName,
		CurrentValue:    formatCPU(container.Requests.CPUMilli),
		SuggestedValue:  formatCPU(baselineCPUMilli),
		Reason:          fmt.Sprintf("Container %q has no CPU request; add a small baseline request so Kubernetes can schedule it with an explicit CPU reservation.", containerName),
		SafetyLevel:     SafetyLevelCautious,
	}, true
}

func addMemoryRequestRecommendation(result analysis.AnalysisResult, finding analysis.Finding) (Recommendation, bool) {
	containerName := finding.Details["container"]
	container, ok := findContainer(result.Resources, containerName)
	if !ok || container.Requests.MemoryMiB != 0 {
		return Recommendation{}, false
	}

	return Recommendation{
		Type:            RecommendationAddMemoryRequest,
		Target:          result.Ref,
		TargetContainer: containerName,
		CurrentValue:    formatMemory(container.Requests.MemoryMiB),
		SuggestedValue:  formatMemory(baselineMemoryMiB),
		Reason:          fmt.Sprintf("Container %q has no memory request; add a small baseline request so Kubernetes can schedule it with an explicit memory reservation.", containerName),
		SafetyLevel:     SafetyLevelCautious,
	}, true
}

func replicaReductionRecommendation(result analysis.AnalysisResult) (Recommendation, bool) {
	replicas := effectiveReplicas(result.Resources)
	suggested := replicas - 1
	if suggested < minRecommendedReplica {
		suggested = minRecommendedReplica
	}
	if suggested >= replicas {
		return Recommendation{}, false
	}

	return Recommendation{
		Type:           RecommendationReduceReplicas,
		Target:         result.Ref,
		CurrentValue:   fmt.Sprintf("%d", replicas),
		SuggestedValue: fmt.Sprintf("%d", suggested),
		Reason:         "CPU and memory utilization are both below the analysis threshold; reduce replicas by one and review before making further changes.",
		SafetyLevel:    SafetyLevelCautious,
	}, true
}

func conservativeReduction(current, floor int64) int64 {
	reduced := current * (100 - maxReductionPercent) / 100
	if reduced < floor {
		return floor
	}
	return reduced
}

func findContainer(resources domain.WorkloadResources, name string) (domain.ContainerResources, bool) {
	for _, container := range resources.Containers {
		if container.Name == name {
			return container, true
		}
	}
	return domain.ContainerResources{}, false
}

func findUsage(usage domain.UsageSnapshot, name string) domain.ContainerUsage {
	for _, container := range usage.Containers {
		if container.Name == name {
			return container
		}
	}
	return domain.ContainerUsage{Name: name}
}

func effectiveReplicas(resources domain.WorkloadResources) int32 {
	if resources.DesiredReplicas > 0 {
		return resources.DesiredReplicas
	}
	if resources.Replicas > 0 {
		return resources.Replicas
	}
	return minRecommendedReplica
}

func markSeen(seen map[string]bool, recommendation Recommendation) bool {
	key := fmt.Sprintf("%s/%s/%s", recommendation.Type, recommendation.Target.Name, recommendation.TargetContainer)
	if seen[key] {
		return false
	}
	seen[key] = true
	return true
}

func ceilDiv(value, divisor int64) int64 {
	if divisor <= 0 {
		return value
	}
	return (value + divisor - 1) / divisor
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func formatCPU(cpuMilli int64) string {
	return fmt.Sprintf("%dm", cpuMilli)
}

func formatMemory(memoryMiB int64) string {
	return fmt.Sprintf("%dMi", memoryMiB)
}
