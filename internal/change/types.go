package change

import "github.com/ohanyere/kubernetes-cost-guard/internal/domain"

type Operation struct {
	Target          domain.WorkloadRef `json:"target"`
	TargetContainer string             `json:"targetContainer,omitempty"`
	FieldPath       string             `json:"fieldPath"`
	CurrentValue    string             `json:"currentValue"`
	NewValue        string             `json:"newValue"`
}
