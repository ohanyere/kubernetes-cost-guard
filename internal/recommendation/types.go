package recommendation

import "github.com/ohanyere/kubernetes-cost-guard/internal/domain"

type RecommendationType string

const (
	RecommendationReduceCPURequest    RecommendationType = "reduce_cpu_request"
	RecommendationReduceMemoryRequest RecommendationType = "reduce_memory_request"
	RecommendationAddCPURequest       RecommendationType = "add_cpu_request"
	RecommendationAddMemoryRequest    RecommendationType = "add_memory_request"
	RecommendationReduceReplicas      RecommendationType = "reduce_replicas"
)

type SafetyLevel string

const (
	SafetyLevelSafe     SafetyLevel = "safe"
	SafetyLevelCautious SafetyLevel = "cautious"
)

type Recommendation struct {
	Type            RecommendationType `json:"type"`
	Target          domain.WorkloadRef `json:"target"`
	TargetContainer string             `json:"targetContainer,omitempty"`
	CurrentValue    string             `json:"currentValue"`
	SuggestedValue  string             `json:"suggestedValue"`
	Reason          string             `json:"reason"`
	SafetyLevel     SafetyLevel        `json:"safetyLevel"`
}

type RecommendationResult struct {
	Recommendations []Recommendation `json:"recommendations"`
}
