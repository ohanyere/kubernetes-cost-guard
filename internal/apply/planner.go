package apply

import (
	"strconv"

	"github.com/ohanyere/kubernetes-cost-guard/internal/domain"
	"github.com/ohanyere/kubernetes-cost-guard/internal/patch"
	"github.com/ohanyere/kubernetes-cost-guard/internal/recommendation"
)

type OperationResult struct {
	Target          domain.WorkloadRef `json:"target"`
	TargetContainer string             `json:"targetContainer,omitempty"`
	FieldPath       string             `json:"fieldPath"`
	CurrentValue    string             `json:"currentValue"`
	NewValue        string             `json:"newValue"`
	Reason          string             `json:"reason,omitempty"`
	SkipReason      string             `json:"skipReason,omitempty"`
}

type PlanResult struct {
	Operations []patch.PatchOperation
	Applied    []OperationResult
	Skipped    []OperationResult
}

type operationIdentity struct {
	Kind            domain.WorkloadKind
	Namespace       string
	Name            string
	TargetContainer string
	FieldPath       string
	CurrentValue    string
	NewValue        string
}

func Plan(recommendations []recommendation.Recommendation, preview patch.PatchPreview, metricsAvailable bool) PlanResult {
	result := PlanResult{
		Operations: make([]patch.PatchOperation, 0, len(preview.Operations)),
		Applied:    make([]OperationResult, 0, len(preview.Operations)),
		Skipped:    make([]OperationResult, 0),
	}
	recommendationsByOperation := recommendationsByOperation(recommendations)

	for _, operation := range preview.Operations {
		item, ok := recommendationsByOperation[identityForOperation(operation)]
		if !ok {
			result.Skipped = append(result.Skipped, skippedOperation(operation, "recommendation was not found for patch preview"))
			continue
		}

		if ProtectedNamespace(operation.Target.Namespace) {
			result.Skipped = append(result.Skipped, skippedOperation(operation, "namespace is protected"))
			continue
		}
		if item.SafetyLevel != recommendation.SafetyLevelSafe {
			result.Skipped = append(result.Skipped, skippedOperation(operation, "recommendation is not marked safe"))
			continue
		}
		if usageBasedRecommendation(item.Type) && !metricsAvailable {
			result.Skipped = append(result.Skipped, skippedOperation(operation, "metrics are required for usage-based changes"))
			continue
		}
		if operation.FieldPath == "spec.replicas" && replicaBelowMinimum(operation.NewValue) {
			result.Skipped = append(result.Skipped, skippedOperation(operation, "replicas cannot be reduced below 1"))
			continue
		}

		result.Operations = append(result.Operations, operation)
		result.Applied = append(result.Applied, appliedOperation(operation, item.Reason))
	}

	return result
}

func MarkApplied(planned []OperationResult, applied []patch.PatchOperation) []OperationResult {
	appliedByOperation := make(map[operationIdentity]bool, len(applied))
	for _, operation := range applied {
		appliedByOperation[identityForOperation(operation)] = true
	}

	results := make([]OperationResult, 0, len(planned))
	for _, item := range planned {
		if appliedByOperation[identityForResult(item)] {
			results = append(results, item)
		}
	}
	return results
}

func ProtectedNamespace(namespace string) bool {
	switch namespace {
	case "kube-system", "kube-public", "kube-node-lease":
		return true
	default:
		return false
	}
}

func recommendationsByOperation(recommendations []recommendation.Recommendation) map[operationIdentity]recommendation.Recommendation {
	byOperation := make(map[operationIdentity]recommendation.Recommendation, len(recommendations))
	for _, item := range recommendations {
		operation, ok := patch.OperationFor(item)
		if !ok {
			continue
		}
		byOperation[identityForOperation(operation)] = item
	}
	return byOperation
}

func usageBasedRecommendation(recommendationType recommendation.RecommendationType) bool {
	switch recommendationType {
	case recommendation.RecommendationReduceCPURequest, recommendation.RecommendationReduceMemoryRequest, recommendation.RecommendationReduceReplicas:
		return true
	default:
		return false
	}
}

func replicaBelowMinimum(value string) bool {
	replicas, err := strconv.Atoi(value)
	return err != nil || replicas < 1
}

func appliedOperation(operation patch.PatchOperation, reason string) OperationResult {
	result := operationResult(operation)
	result.Reason = reason
	return result
}

func skippedOperation(operation patch.PatchOperation, reason string) OperationResult {
	result := operationResult(operation)
	result.SkipReason = reason
	return result
}

func operationResult(operation patch.PatchOperation) OperationResult {
	return OperationResult{
		Target:          operation.Target,
		TargetContainer: operation.TargetContainer,
		FieldPath:       operation.FieldPath,
		CurrentValue:    operation.CurrentValue,
		NewValue:        operation.NewValue,
	}
}

func identityForOperation(operation patch.PatchOperation) operationIdentity {
	return operationIdentity{
		Kind:            operation.Target.Kind,
		Namespace:       operation.Target.Namespace,
		Name:            operation.Target.Name,
		TargetContainer: operation.TargetContainer,
		FieldPath:       operation.FieldPath,
		CurrentValue:    operation.CurrentValue,
		NewValue:        operation.NewValue,
	}
}

func identityForResult(result OperationResult) operationIdentity {
	return operationIdentity{
		Kind:            result.Target.Kind,
		Namespace:       result.Target.Namespace,
		Name:            result.Target.Name,
		TargetContainer: result.TargetContainer,
		FieldPath:       result.FieldPath,
		CurrentValue:    result.CurrentValue,
		NewValue:        result.NewValue,
	}
}
