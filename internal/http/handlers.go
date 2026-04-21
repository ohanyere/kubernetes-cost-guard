package http

import (
	"errors"
	"net/http"

	"github.com/ohanyere/kubernetes-cost-guard/internal/analysis"
	"github.com/ohanyere/kubernetes-cost-guard/internal/apply"
	"github.com/ohanyere/kubernetes-cost-guard/internal/kube"
	"github.com/ohanyere/kubernetes-cost-guard/internal/patch"
	"github.com/ohanyere/kubernetes-cost-guard/internal/recommendation"
)

func (s *Server) handleAnalyzeWorkload(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeWorkloadRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := validateTarget(req.Target); err != nil {
		writeBadRequest(w, err.Error())
		return
	}

	if s.kube == nil {
		writeError(w, http.StatusServiceUnavailable, "kubernetes_unavailable", "kubernetes client is not configured")
		return
	}

	snapshot, err := s.kube.GetDeployment(r.Context(), req.Target.Namespace, req.Target.Name)
	if err != nil {
		writeKubeError(w, err)
		return
	}

	result := analysis.AnalyzeWorkload(snapshot.Resources, snapshot.Usage)

	writeJSON(w, http.StatusOK, AnalyzeWorkloadResponse{
		Data: AnalyzeWorkloadData{
			Analysis: result,
		},
		Warnings: snapshot.Warnings,
		Error:    nil,
	})
}

func (s *Server) handleAnalyzeNamespace(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeNamespaceRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.Namespace == "" {
		writeBadRequest(w, "namespace is required")
		return
	}

	if s.kube == nil {
		writeError(w, http.StatusServiceUnavailable, "kubernetes_unavailable", "kubernetes client is not configured")
		return
	}

	snapshot, err := s.kube.ListDeployments(r.Context(), req.Namespace)
	if err != nil {
		writeKubeError(w, err)
		return
	}

	inputs := make([]analysis.WorkloadInput, 0, len(snapshot.Items))
	warnings := make([]string, 0)
	for _, item := range snapshot.Items {
		inputs = append(inputs, analysis.WorkloadInput{
			Resources: item.Resources,
			Usage:     item.Usage,
		})
		warnings = append(warnings, item.Warnings...)
	}

	result := analysis.AnalyzeNamespace(snapshot.Namespace, inputs)

	writeJSON(w, http.StatusOK, AnalyzeNamespaceResponse{
		Data: AnalyzeNamespaceData{
			NamespaceAnalysis: result,
		},
		Warnings: warnings,
		Error:    nil,
	})
}

func (s *Server) handleRecommendDryRun(w http.ResponseWriter, r *http.Request) {
	var req DryRunRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := validateTarget(req.Target); err != nil {
		writeBadRequest(w, err.Error())
		return
	}

	if s.kube == nil {
		writeError(w, http.StatusServiceUnavailable, "kubernetes_unavailable", "kubernetes client is not configured")
		return
	}

	snapshot, err := s.kube.GetDeployment(r.Context(), req.Target.Namespace, req.Target.Name)
	if err != nil {
		writeKubeError(w, err)
		return
	}

	analysisResult := analysis.AnalyzeWorkload(snapshot.Resources, snapshot.Usage)
	recommendationResult := recommendation.Generate(analysisResult)
	patchPreview := patch.Preview(recommendationResult.Recommendations)

	writeJSON(w, http.StatusOK, DryRunResponse{
		Data: DryRunData{
			Recommendations: recommendationResult.Recommendations,
			PatchPreview:    patchPreview.Operations,
		},
		Warnings: snapshot.Warnings,
		Error:    nil,
	})
}

func (s *Server) handleRecommendApply(w http.ResponseWriter, r *http.Request) {
	var req ApplyRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := validateTarget(req.Target); err != nil {
		writeBadRequest(w, err.Error())
		return
	}

	if s.kube == nil {
		writeError(w, http.StatusServiceUnavailable, "kubernetes_unavailable", "kubernetes client is not configured")
		return
	}

	snapshot, err := s.kube.GetDeployment(r.Context(), req.Target.Namespace, req.Target.Name)
	if err != nil {
		writeKubeError(w, err)
		return
	}

	analysisResult := analysis.AnalyzeWorkload(snapshot.Resources, snapshot.Usage)
	recommendationResult := recommendation.Generate(analysisResult)
	patchPreview := patch.Preview(recommendationResult.Recommendations)
	applyPlan := apply.Plan(recommendationResult.Recommendations, patchPreview, analysisResult.Usage.MetricsAvailable)

	applied := applyPlan.Applied
	if len(applyPlan.Operations) > 0 {
		appliedOperations, err := s.kube.ApplyDeploymentPatch(r.Context(), snapshot.Resources.Ref, applyPlan.Operations)
		if err != nil {
			writeKubeError(w, err)
			return
		}
		applied = apply.MarkApplied(applyPlan.Applied, appliedOperations)
	}

	writeJSON(w, http.StatusOK, ApplyResponse{
		Data: ApplyData{
			Applied: applied,
			Skipped: applyPlan.Skipped,
		},
		Warnings: snapshot.Warnings,
		Error:    nil,
	})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// TODO(Phase 9): replace this placeholder with persisted analyses, recommendations, dry-runs, and apply results.
	writeJSON(w, http.StatusOK, HistoryResponse{
		PlaceholderResponse: PlaceholderResponse{
			Status:  "ok",
			Message: "history endpoint is ready; PostgreSQL persistence will be implemented in Phase 9",
			Next:    "Phase 9 persists analyses, recommendations, dry-runs, and apply results",
		},
		Filters: HistoryRequestFilters{
			Namespace: query.Get("namespace"),
			Workload:  query.Get("workload"),
			Limit:     query.Get("limit"),
		},
		Items: []any{},
	})
}

func writeKubeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, kube.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "kubernetes resource not found")
	case errors.Is(err, kube.ErrClientUnavailable):
		writeError(w, http.StatusServiceUnavailable, "kubernetes_unavailable", "kubernetes client is not configured")
	default:
		writeError(w, http.StatusBadGateway, "kubernetes_error", err.Error())
	}
}
