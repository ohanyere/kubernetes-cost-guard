package http

import (
	"github.com/ohanyere/kubernetes-cost-guard/internal/analysis"
	"github.com/ohanyere/kubernetes-cost-guard/internal/apply"
	"github.com/ohanyere/kubernetes-cost-guard/internal/domain"
	"github.com/ohanyere/kubernetes-cost-guard/internal/patch"
	"github.com/ohanyere/kubernetes-cost-guard/internal/recommendation"
)

type WorkloadTarget struct {
	Namespace string              `json:"namespace"`
	Name      string              `json:"name"`
	Kind      domain.WorkloadKind `json:"kind"`
}

type AnalyzeWorkloadRequest struct {
	Target WorkloadTarget `json:"target"`
}

type AnalyzeNamespaceRequest struct {
	Namespace string `json:"namespace"`
}

type DryRunRequest struct {
	Target WorkloadTarget `json:"target"`
}

type ApplyRequest struct {
	Target WorkloadTarget `json:"target"`
}

type HistoryRequestFilters struct {
	Namespace string `json:"namespace,omitempty"`
	Workload  string `json:"workload,omitempty"`
	Limit     string `json:"limit,omitempty"`
}

type PlaceholderResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Next    string `json:"next,omitempty"`
}

type AnalyzeWorkloadData struct {
	Analysis analysis.AnalysisResult `json:"analysis"`
}

type AnalyzeWorkloadResponse struct {
	Data     AnalyzeWorkloadData `json:"data"`
	Warnings []string            `json:"warnings"`
	Error    *ErrorBody          `json:"error"`
}

type AnalyzeNamespaceData struct {
	NamespaceAnalysis analysis.NamespaceAnalysisResult `json:"namespaceAnalysis"`
}

type AnalyzeNamespaceResponse struct {
	Data     AnalyzeNamespaceData `json:"data"`
	Warnings []string             `json:"warnings"`
	Error    *ErrorBody           `json:"error"`
}

type DryRunData struct {
	Recommendations []recommendation.Recommendation `json:"recommendations"`
	PatchPreview    []patch.PatchOperation          `json:"patchPreview"`
}

type DryRunResponse struct {
	Data     DryRunData `json:"data"`
	Warnings []string   `json:"warnings"`
	Error    *ErrorBody `json:"error"`
}

type ApplyData struct {
	Applied []apply.OperationResult `json:"applied"`
	Skipped []apply.OperationResult `json:"skipped"`
}

type ApplyResponse struct {
	Data     ApplyData  `json:"data"`
	Warnings []string   `json:"warnings"`
	Error    *ErrorBody `json:"error"`
}

type HistoryResponse struct {
	PlaceholderResponse
	Filters HistoryRequestFilters `json:"filters"`
	Items   []any                 `json:"items"`
}
