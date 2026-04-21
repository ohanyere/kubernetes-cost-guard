package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ohanyere/kubernetes-cost-guard/internal/analysis"
	"github.com/ohanyere/kubernetes-cost-guard/internal/config"
	"github.com/ohanyere/kubernetes-cost-guard/internal/domain"
	"github.com/ohanyere/kubernetes-cost-guard/internal/kube"
	"github.com/ohanyere/kubernetes-cost-guard/internal/patch"
)

type fakeKubeReader struct {
	getDeployment   func(context.Context, string, string) (kube.DeploymentSnapshot, error)
	listDeployments func(context.Context, string) (kube.NamespaceSnapshot, error)
	applyPatch      func(context.Context, domain.WorkloadRef, []patch.PatchOperation) ([]patch.PatchOperation, error)
}

func (f fakeKubeReader) GetDeployment(ctx context.Context, namespace, name string) (kube.DeploymentSnapshot, error) {
	return f.getDeployment(ctx, namespace, name)
}

func (f fakeKubeReader) ListDeployments(ctx context.Context, namespace string) (kube.NamespaceSnapshot, error) {
	return f.listDeployments(ctx, namespace)
}

func (f fakeKubeReader) ApplyDeploymentPatch(ctx context.Context, target domain.WorkloadRef, operations []patch.PatchOperation) ([]patch.PatchOperation, error) {
	return f.applyPatch(ctx, target, operations)
}

func TestAnalyzeWorkloadReturnsNotFound(t *testing.T) {
	handler := NewServer(config.Config{}, slog.Default(), fakeKubeReader{
		getDeployment: func(context.Context, string, string) (kube.DeploymentSnapshot, error) {
			return kube.DeploymentSnapshot{}, kube.ErrNotFound
		},
	})

	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/analyze/workload", strings.NewReader(`{"target":{"namespace":"default","name":"missing","kind":"Deployment"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	var response ErrorResponse
	decodeResponse(t, rec, &response)
	if response.Error.Code != "not_found" {
		t.Fatalf("expected not_found error, got %#v", response.Error)
	}
}

func TestAnalyzeWorkloadReturnsBadRequestForMissingName(t *testing.T) {
	handler := NewServer(config.Config{}, slog.Default(), nil)

	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/analyze/workload", strings.NewReader(`{"target":{"namespace":"default","kind":"Deployment"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAnalyzeWorkloadReturnsAnalysisFindings(t *testing.T) {
	handler := NewServer(config.Config{}, slog.Default(), fakeKubeReader{
		getDeployment: func(context.Context, string, string) (kube.DeploymentSnapshot, error) {
			return kube.DeploymentSnapshot{
				Resources: domain.WorkloadResources{
					Ref:             domain.WorkloadRef{Namespace: "default", Name: "web", Kind: domain.WorkloadKindDeployment},
					DesiredReplicas: 1,
					Containers: []domain.ContainerResources{
						{Name: "app", Requests: domain.ResourceQuantity{CPUMilli: 1000, MemoryMiB: 512}, Limits: domain.ResourceQuantity{CPUMilli: 1500, MemoryMiB: 1024}},
					},
				},
				Usage: domain.UsageSnapshot{
					CapturedAt:       time.Now().UTC(),
					MetricsAvailable: true,
					Containers: []domain.ContainerUsage{
						{Name: "app", CPUUsedMilli: 100, MemoryUsedMiB: 400},
					},
				},
			}, nil
		},
	})

	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/analyze/workload", strings.NewReader(`{"target":{"namespace":"default","name":"web","kind":"Deployment"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response AnalyzeWorkloadResponse
	decodeResponse(t, rec, &response)
	if response.Error != nil {
		t.Fatalf("expected null error envelope, got %#v", response.Error)
	}
	if response.Data.Analysis.Ref.Name != "web" {
		t.Fatalf("expected analysis for web, got %#v", response.Data.Analysis.Ref)
	}
	if !hasAnalysisFinding(response.Data.Analysis.Findings, "cpu_overprovisioned") {
		t.Fatalf("expected cpu_overprovisioned finding, got %#v", response.Data.Analysis.Findings)
	}
}

func TestAnalyzeNamespaceAggregatesDeployments(t *testing.T) {
	handler := NewServer(config.Config{}, slog.Default(), fakeKubeReader{
		listDeployments: func(context.Context, string) (kube.NamespaceSnapshot, error) {
			return kube.NamespaceSnapshot{
				Namespace: "default",
				Items: []kube.DeploymentSnapshot{
					{
						Resources: domain.WorkloadResources{
							Ref:             domain.WorkloadRef{Namespace: "default", Name: "web", Kind: domain.WorkloadKindDeployment},
							DesiredReplicas: 2,
							Containers: []domain.ContainerResources{
								{Name: "app", Requests: domain.ResourceQuantity{CPUMilli: 100, MemoryMiB: 256}},
							},
						},
						Usage: domain.UsageSnapshot{
							CapturedAt:       time.Now().UTC(),
							MetricsAvailable: true,
							Containers: []domain.ContainerUsage{
								{Name: "app", CPUUsedMilli: 80, MemoryUsedMiB: 128},
							},
						},
					},
				},
			}, nil
		},
	})

	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/analyze/namespace", strings.NewReader(`{"namespace":"default"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response AnalyzeNamespaceResponse
	decodeResponse(t, rec, &response)
	if response.Error != nil {
		t.Fatalf("expected null error envelope, got %#v", response.Error)
	}
	analysis := response.Data.NamespaceAnalysis
	if analysis.WorkloadCount != 1 {
		t.Fatalf("expected workload count 1, got %d", analysis.WorkloadCount)
	}
	if analysis.TotalRequested.CPUMilli != 200 {
		t.Fatalf("expected aggregated requested cpu 200, got %#v", analysis.TotalRequested)
	}
}

func TestRecommendDryRunReturnsRecommendations(t *testing.T) {
	handler := NewServer(config.Config{}, slog.Default(), fakeKubeReader{
		getDeployment: func(context.Context, string, string) (kube.DeploymentSnapshot, error) {
			return kube.DeploymentSnapshot{
				Resources: domain.WorkloadResources{
					Ref:             domain.WorkloadRef{Namespace: "default", Name: "web", Kind: domain.WorkloadKindDeployment},
					DesiredReplicas: 1,
					Containers: []domain.ContainerResources{
						{Name: "app", Requests: domain.ResourceQuantity{CPUMilli: 1000, MemoryMiB: 512}},
					},
				},
				Usage: domain.UsageSnapshot{
					CapturedAt:       time.Now().UTC(),
					MetricsAvailable: true,
					Containers: []domain.ContainerUsage{
						{Name: "app", CPUUsedMilli: 100, MemoryUsedMiB: 400},
					},
				},
			}, nil
		},
	})

	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/recommend/dry-run", strings.NewReader(`{"target":{"namespace":"default","name":"web","kind":"Deployment"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response DryRunResponse
	decodeResponse(t, rec, &response)
	if response.Error != nil {
		t.Fatalf("expected null error envelope, got %#v", response.Error)
	}
	if len(response.Data.Recommendations) != 1 {
		t.Fatalf("expected one recommendation, got %#v", response.Data.Recommendations)
	}
	if response.Data.Recommendations[0].Type != "reduce_cpu_request" {
		t.Fatalf("expected reduce_cpu_request recommendation, got %#v", response.Data.Recommendations[0])
	}
	if len(response.Data.PatchPreview) != 1 {
		t.Fatalf("expected one patch preview operation, got %#v", response.Data.PatchPreview)
	}
	operation := response.Data.PatchPreview[0]
	if operation.FieldPath != "spec.template.spec.containers[].resources.requests.cpu" {
		t.Fatalf("expected cpu request patch field path, got %#v", operation)
	}
	if operation.NewValue != "750m" {
		t.Fatalf("expected cpu patch new value 750m, got %#v", operation)
	}
}

func TestRecommendApplyAppliesSafeOperations(t *testing.T) {
	var applied []patch.PatchOperation
	handler := NewServer(config.Config{}, slog.Default(), fakeKubeReader{
		getDeployment: func(context.Context, string, string) (kube.DeploymentSnapshot, error) {
			return overprovisionedSnapshot("default"), nil
		},
		applyPatch: func(_ context.Context, _ domain.WorkloadRef, operations []patch.PatchOperation) ([]patch.PatchOperation, error) {
			applied = operations
			return operations, nil
		},
	})

	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/recommend/apply", strings.NewReader(`{"target":{"namespace":"default","name":"web","kind":"Deployment"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(applied) != 1 {
		t.Fatalf("expected one applied operation, got %d", len(applied))
	}
	var response ApplyResponse
	decodeResponse(t, rec, &response)
	if response.Error != nil {
		t.Fatalf("expected null error envelope, got %#v", response.Error)
	}
	if len(response.Data.Applied) != 1 {
		t.Fatalf("expected one applied operation, got %#v", response.Data.Applied)
	}
	if response.Data.Applied[0].FieldPath != "spec.template.spec.containers[].resources.requests.cpu" {
		t.Fatalf("expected cpu request apply field path, got %#v", response.Data.Applied[0])
	}
	if len(response.Data.Skipped) != 0 {
		t.Fatalf("expected no skipped operations, got %#v", response.Data.Skipped)
	}
}

func TestRecommendApplySkipsProtectedNamespace(t *testing.T) {
	handler := NewServer(config.Config{}, slog.Default(), fakeKubeReader{
		getDeployment: func(context.Context, string, string) (kube.DeploymentSnapshot, error) {
			return overprovisionedSnapshot("kube-system"), nil
		},
		applyPatch: func(context.Context, domain.WorkloadRef, []patch.PatchOperation) ([]patch.PatchOperation, error) {
			t.Fatal("protected namespace must not apply patches")
			return nil, nil
		},
	})

	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/recommend/apply", strings.NewReader(`{"target":{"namespace":"kube-system","name":"web","kind":"Deployment"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response ApplyResponse
	decodeResponse(t, rec, &response)
	if len(response.Data.Applied) != 0 {
		t.Fatalf("expected no applied operations, got %#v", response.Data.Applied)
	}
	if len(response.Data.Skipped) != 1 {
		t.Fatalf("expected one skipped operation, got %#v", response.Data.Skipped)
	}
	if response.Data.Skipped[0].SkipReason != "namespace is protected" {
		t.Fatalf("expected protected namespace skip reason, got %#v", response.Data.Skipped[0])
	}
}

func TestWriteKubeErrorDefaultsToBadGateway(t *testing.T) {
	rec := httptest.NewRecorder()

	writeKubeError(rec, errors.New("cluster read failed"))

	if rec.Code != nethttp.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func overprovisionedSnapshot(namespace string) kube.DeploymentSnapshot {
	ref := domain.WorkloadRef{Namespace: namespace, Name: "web", Kind: domain.WorkloadKindDeployment}
	return kube.DeploymentSnapshot{
		Resources: domain.WorkloadResources{
			Ref:             ref,
			DesiredReplicas: 1,
			Containers: []domain.ContainerResources{
				{Name: "app", Requests: domain.ResourceQuantity{CPUMilli: 1000, MemoryMiB: 512}},
			},
		},
		Usage: domain.UsageSnapshot{
			Ref:              ref,
			CapturedAt:       time.Now().UTC(),
			MetricsAvailable: true,
			Containers: []domain.ContainerUsage{
				{Name: "app", CPUUsedMilli: 100, MemoryUsedMiB: 400},
			},
		},
	}
}

func decodeResponse(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()

	if err := json.Unmarshal(rec.Body.Bytes(), dst); err != nil {
		t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
	}
}

func hasAnalysisFinding(findings []analysis.Finding, findingType analysis.FindingType) bool {
	for _, finding := range findings {
		if finding.Type == findingType {
			return true
		}
	}
	return false
}
