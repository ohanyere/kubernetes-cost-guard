package patch

import "github.com/ohanyere/kubernetes-cost-guard/internal/recommendation"

const (
	cpuRequestPath    = "spec.template.spec.containers[].resources.requests.cpu"
	memoryRequestPath = "spec.template.spec.containers[].resources.requests.memory"
	replicasPath      = "spec.replicas"
)

func Preview(recommendations []recommendation.Recommendation) PatchPreview {
	operations := make([]PatchOperation, 0, len(recommendations))

	for _, item := range recommendations {
		if operation, ok := OperationFor(item); ok {
			operations = append(operations, operation)
		}
	}

	return PatchPreview{Operations: operations}
}

func OperationFor(item recommendation.Recommendation) (PatchOperation, bool) {
	fieldPath, ok := fieldPathForRecommendation(item.Type)
	if !ok {
		return PatchOperation{}, false
	}

	return PatchOperation{
		Target:          item.Target,
		TargetContainer: item.TargetContainer,
		FieldPath:       fieldPath,
		CurrentValue:    item.CurrentValue,
		NewValue:        item.SuggestedValue,
	}, true
}

func fieldPathForRecommendation(recommendationType recommendation.RecommendationType) (string, bool) {
	switch recommendationType {
	case recommendation.RecommendationReduceCPURequest, recommendation.RecommendationAddCPURequest:
		return cpuRequestPath, true
	case recommendation.RecommendationReduceMemoryRequest, recommendation.RecommendationAddMemoryRequest:
		return memoryRequestPath, true
	case recommendation.RecommendationReduceReplicas:
		return replicasPath, true
	default:
		return "", false
	}
}
