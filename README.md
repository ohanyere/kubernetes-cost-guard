# Kubernetes Cost Guard

Kubernetes Cost Guard is a focused platform engineering service for Kubernetes workload efficiency and cost guardrails.

The project has one core job: analyze Kubernetes workloads, detect waste, recommend safer resource settings, preview safe changes in dry-run mode, and optionally apply approved changes with explicit guardrails.

## Current Scope

The current implementation provides the early API foundation:

- Go module and backend folder structure
- Core domain models for workloads, usage, analyses, recommendations, patches, apply results, namespace summaries, and guardrail violations
- Environment-based configuration loader
- HTTP API entrypoint with a health endpoint
- Versioned `/api/v1` placeholder endpoints
- Consistent JSON success and error responses
- Request logging middleware
- Read-only Kubernetes Deployment retrieval through a wrapped client
- Best-effort metrics-server usage snapshots
- Workload analysis findings for missing resources, overprovisioning, replica underutilization, and missing metrics
- Rule-based recommendation generation for workload findings
- Guarded safe apply support for approved Deployment patches
- Docker, Kubernetes, CI, DevSecOps, and ArgoCD deployment packaging
- Local run instructions

It intentionally does not include PostgreSQL, Slack, or multi-step approval workflows yet.

## Project Structure

```text
cmd/api              API process entrypoint
internal/config      Environment-based configuration
internal/domain      Core product domain models
internal/http        HTTP server wiring, DTOs, handlers, and JSON helpers
internal/kube        Wrapped Kubernetes client, Deployment mapping, and metrics reads
internal/analysis    Pure workload analysis logic and structured findings
internal/recommendation
internal/guardrails
internal/patch
internal/history
internal/store
internal/slack
migrations
```

Some directories are reserved for upcoming phases so package boundaries stay clear as the project grows.

## Configuration

The API reads configuration from environment variables:

| Variable | Default | Description |
| --- | --- | --- |
| `KCG_APP_NAME` | `kubernetes-cost-guard` | Service name returned by health checks |
| `KCG_ENV` | `local` | Runtime environment label |
| `KCG_HTTP_ADDR` | `:8080` | HTTP listen address |
| `KCG_VERSION` | `dev` | Version string returned by health checks |
| `KCG_KUBECONFIG` | value of `KUBECONFIG` | Optional kubeconfig path for local cluster access |

## Run Locally

```bash
go run ./cmd/api
```

Then check the health endpoint:

```bash
curl http://localhost:8080/healthz
```

Expected response shape:

```json
{
  "environment": "local",
  "service": "kubernetes-cost-guard",
  "status": "ok",
  "time": "2026-04-18T00:00:00Z",
  "version": "dev"
}
```

API examples:

```bash
curl -X POST http://localhost:8080/api/v1/analyze/workload \
  -H 'Content-Type: application/json' \
  -d '{"target":{"namespace":"default","name":"web","kind":"Deployment"}}'

curl -X POST http://localhost:8080/api/v1/analyze/namespace \
  -H 'Content-Type: application/json' \
  -d '{"namespace":"default"}'

curl -X POST http://localhost:8080/api/v1/recommend/dry-run \
  -H 'Content-Type: application/json' \
  -d '{"target":{"namespace":"default","name":"web","kind":"Deployment"}}'

curl -X POST http://localhost:8080/api/v1/recommend/apply \
  -H 'Content-Type: application/json' \
  -d '{"target":{"namespace":"default","name":"web","kind":"Deployment"}}'

curl 'http://localhost:8080/api/v1/history?namespace=default&workload=web'
```

## Docker

Build the production image:

```bash
docker build -t kubernetes-cost-guard:local .
```

Run it locally:

```bash
docker run --rm -p 8080:8080 \
  -e KCG_ENV=local \
  -e KCG_VERSION=local \
  kubernetes-cost-guard:local
```

The Dockerfile uses a multi-stage build. The first stage compiles the Go API binary, and the final image is a small distroless runtime image that runs only `/kubernetes-cost-guard` as a non-root user.

CI publishes release images to GitHub Container Registry:

```text
ghcr.io/<owner>/kubernetes-cost-guard:<short-sha>
ghcr.io/<owner>/kubernetes-cost-guard:latest
```

The SHA-based tag is the deployment tag. It is immutable for practical GitOps use because it points at the exact source revision that built the image.

## Kubernetes Access

Phase 3 adds read-only Kubernetes access for Deployments.

For local development, point the API at a kubeconfig:

```bash
KCG_KUBECONFIG="$HOME/.kube/config" go run ./cmd/api
```

If `KCG_KUBECONFIG` is not set, the API uses `KUBECONFIG` and then falls back to `$HOME/.kube/config`. In a cluster, it first attempts in-cluster configuration.

The analysis endpoints currently read:

- Deployment name and namespace
- Desired and available replicas
- Container CPU and memory requests
- Container CPU and memory limits
- Best-effort pod usage from metrics-server

Metrics are optional in Phase 3. If metrics-server is unavailable or returns no data, the API still returns workload configuration and includes a warning instead of failing the request.

## In-Cluster Deployment

Minimal Kubernetes manifests live in `deploy/k8s`:

- `serviceaccount.yaml` creates the runtime identity used by the API Pod.
- `rbac.yaml` grants the API narrow Kubernetes permissions.
- `deployment.yaml` runs one API replica on port `8080`.
- `service.yaml` exposes the API inside the cluster as a `ClusterIP` Service.

Deploy manually:

```bash
kubectl create namespace kubernetes-cost-guard
kubectl apply -f deploy/k8s/
```

Check the rollout:

```bash
kubectl -n kubernetes-cost-guard rollout status deploy/kubernetes-cost-guard
kubectl -n kubernetes-cost-guard get pods,svc
```

Test the in-cluster Service with port-forwarding:

```bash
kubectl -n kubernetes-cost-guard port-forward svc/kubernetes-cost-guard 8080:80
curl http://localhost:8080/healthz
```

The deployed container uses these environment variables:

| Variable | Value in manifest | Purpose |
| --- | --- | --- |
| `KCG_APP_NAME` | `kubernetes-cost-guard` | Health response service name |
| `KCG_ENV` | `cluster` | Runtime environment label |
| `KCG_HTTP_ADDR` | `:8080` | HTTP bind address |
| `KCG_VERSION` | `0.1.0` | Health response version |

For manual testing, update `deploy/k8s/deployment.yaml` with your published image tag before deploying to a real cluster:

```yaml
image: ghcr.io/ohanyere/kubernetes-cost-guard:0.1.0
```

In the normal CI/CD flow, GitHub Actions updates this image line automatically after it builds, scans, and pushes the new image.

In-cluster mode works through Kubernetes service account credentials mounted into the Pod. The client first attempts `rest.InClusterConfig()`, so no kubeconfig file is needed once the API runs inside Kubernetes.

### ServiceAccount and RBAC

The ServiceAccount is bound to a minimal ClusterRole because the API accepts namespaces in requests and may analyze workloads outside its own namespace.

Granted access:

- read `apps/deployments` for workload and namespace analysis
- patch `apps/deployments` for the guarded `/api/v1/recommend/apply` endpoint
- read `pods` so Deployment usage can be correlated with matching Pods
- read `metrics.k8s.io/pods` for metrics-server CPU and memory usage snapshots

The role does not grant broad admin verbs such as `create`, `delete`, or `update`.

## Workload Analysis

Phase 4 changes `POST /api/v1/analyze/workload` from a raw Kubernetes snapshot response to a structured analysis response:

```json
{
  "status": "ok",
  "analysis": {
    "ref": {
      "namespace": "default",
      "name": "web",
      "kind": "Deployment"
    },
    "resources": {
      "ref": {
        "namespace": "default",
        "name": "web",
        "kind": "Deployment"
      },
      "replicas": 2,
      "desiredReplicas": 2,
      "availableReplicas": 2,
      "containers": []
    },
    "usage": {
      "ref": {
        "namespace": "default",
        "name": "web",
        "kind": "Deployment"
      },
      "capturedAt": "2026-04-19T00:00:00Z",
      "metricsAvailable": true,
      "containers": []
    },
    "findings": [
      {
        "type": "cpu_overprovisioned",
        "severity": "medium",
        "message": "container \"app\" is using less than 30% of requested CPU",
        "details": {
          "container": "app",
          "thresholdPercent": "30"
        }
      }
    ],
    "analyzedAt": "2026-04-19T00:00:00Z"
  }
}
```

The analyzer is intentionally heuristic-based. It detects missing CPU and memory requests, missing CPU and memory limits, CPU and memory overprovisioning when usage is below 30% of requested resources, underutilized replica sets, and missing metrics.

## Namespace Analysis

Phase 5 changes `POST /api/v1/analyze/namespace` to return an envelope with a namespace analysis summary:

```json
{
  "data": {
    "namespaceAnalysis": {
      "namespace": "default",
      "workloadCount": 2,
      "totalRequested": {
        "cpuMilli": 2500,
        "memoryMiB": 1280
      },
      "totalObserved": {
        "cpuMilli": 600,
        "memoryMiB": 928
      },
      "metricsAvailable": true,
      "workloads": [
        {
          "name": "web",
          "namespace": "default",
          "kind": "Deployment",
          "replicas": 2,
          "findingsCount": 1,
          "mediumCount": 1,
          "keyFindings": ["cpu_overprovisioned"],
          "severityScore": 2
        }
      ],
      "topWastefulWorkloads": [],
      "findingsByType": {
        "cpu_overprovisioned": 1
      },
      "findings": [],
      "analyzedAt": "2026-04-19T00:00:00Z"
    }
  },
  "warnings": [],
  "error": null
}
```

Namespace analysis reuses workload analysis for every Deployment, totals requested and observed resources, summarizes findings per workload, ranks the top wasteful workloads by a simple severity score, and adds namespace-level findings for widespread missing requests, missing metrics, and common overprovisioning.

## Recommendations

Phase 6 changes `POST /api/v1/recommend/dry-run` from a placeholder into a read-only recommendation endpoint. It fetches the target Deployment snapshot, runs workload analysis, and turns supported findings into explainable recommendations:

- reduce overprovisioned CPU requests conservatively
- reduce overprovisioned memory requests conservatively
- add baseline CPU requests when missing
- add baseline memory requests when missing
- reduce underutilized replicas by one at a time, never below one

Phase 7 adds patch previews to the same endpoint. The preview shows the target field, current value, and suggested new value for each recommendation. It is still read-only and does not apply changes to Kubernetes.

Response shape:

```json
{
  "data": {
    "recommendations": [
      {
        "type": "reduce_cpu_request",
        "target": {
          "namespace": "default",
          "name": "web",
          "kind": "Deployment"
        },
        "targetContainer": "app",
        "currentValue": "1000m",
        "suggestedValue": "750m",
        "reason": "CPU usage for container \"app\" is below the analysis threshold; reduce the request by no more than 25% while keeping headroom above observed usage.",
        "safetyLevel": "safe"
      }
    ],
    "patchPreview": [
      {
        "target": {
          "namespace": "default",
          "name": "web",
          "kind": "Deployment"
        },
        "targetContainer": "app",
        "fieldPath": "spec.template.spec.containers[].resources.requests.cpu",
        "currentValue": "1000m",
        "newValue": "750m"
      }
    ]
  },
  "warnings": [],
  "error": null
}
```

The endpoint does not apply changes, write to a database, or mutate Kubernetes resources in Phase 7. Apply behavior is reserved for a later phase.

## Safe Apply

Phase 8 changes `POST /api/v1/recommend/apply` from a placeholder into a guarded apply endpoint. It fetches the target Deployment, regenerates the current workload analysis, recommendations, and patch preview, then applies only operations whose recommendation is marked `safe`.

Guardrails:

- protected namespaces such as `kube-system`, `kube-public`, and `kube-node-lease` are skipped
- replica changes must never reduce replicas below 1
- usage-based changes require available metrics
- cautious recommendations are reported as skipped, not applied

Response shape:

```json
{
  "data": {
    "applied": [
      {
        "target": {
          "namespace": "default",
          "name": "web",
          "kind": "Deployment"
        },
        "targetContainer": "app",
        "fieldPath": "spec.template.spec.containers[].resources.requests.cpu",
        "currentValue": "1000m",
        "newValue": "750m",
        "reason": "CPU usage for container \"app\" is below the analysis threshold; reduce the request by no more than 25% while keeping headroom above observed usage."
      }
    ],
    "skipped": [
      {
        "target": {
          "namespace": "kube-system",
          "name": "coredns",
          "kind": "Deployment"
        },
        "targetContainer": "app",
        "fieldPath": "spec.template.spec.containers[].resources.requests.cpu",
        "currentValue": "1000m",
        "newValue": "750m",
        "skipReason": "namespace is protected"
      }
    ]
  },
  "warnings": [],
  "error": null
}
```

The apply endpoint sends a minimal strategic merge patch to Kubernetes for the approved Deployment fields only. It does not write history, send Slack notifications, or perform multi-step approval workflows.

## Development

Format and test the code:

```bash
go fmt ./...
go test ./...
```

Vet the code:

```bash
go vet ./...
```

## CI/CD and DevSecOps

GitHub Actions workflow: `.github/workflows/ci.yaml`.

Pull requests to `main` perform validation only:

- checkout and Go setup
- Go formatting check with `gofmt`
- `go vet ./...`
- `go test ./...`
- Trivy filesystem scan for high and critical repo vulnerabilities
- Checkov scan over `deploy/k8s` Kubernetes manifests
- Docker image build using the production Dockerfile
- Trivy image scan for high and critical image vulnerabilities

Pushes to `main` run the same checks, then publish and promote through GitOps:

1. Build the image with `ghcr.io/<owner>/kubernetes-cost-guard:<short-sha>`.
2. Also tag the same image as `ghcr.io/<owner>/kubernetes-cost-guard:latest`.
3. Scan the built image with Trivy.
4. Authenticate to GHCR with the built-in `GITHUB_TOKEN`.
5. Push both image tags to GHCR.
6. Update only the `image:` line in `deploy/k8s/deployment.yaml`.
7. Commit the manifest update as `github-actions[bot]`.
8. Push the manifest commit back to `main`.

Trivy is configured to fail on high or critical findings. Checkov is currently configured with `soft_fail: true` so it reports Kubernetes hardening findings without blocking early portfolio iteration; tighten that later by changing it to `false`.

The manifest update is intentionally small and deterministic:

```bash
sed -i -E "s#^([[:space:]]*image: ).*kubernetes-cost-guard:.*#\1${image}#" deploy/k8s/deployment.yaml
```

The workflow ignores push events that only change `deploy/k8s/deployment.yaml`. That prevents the bot's GitOps image update commit from starting an endless image-build loop.

Required GitHub settings:

- `GITHUB_TOKEN` must have `contents: write` so the workflow can commit the manifest update.
- `GITHUB_TOKEN` must have `packages: write` so the workflow can push to GHCR.
- Repository Actions settings must allow GitHub Actions to create and approve workflow commits on the target branch.
- If `main` is protected, allow the bot push path or use a pull-request based promotion flow instead.

## ArgoCD GitOps

The minimal ArgoCD Application manifest lives at `deploy/argocd/application.yaml`.

It points ArgoCD at this repository and watches the `deploy/k8s` path. The destination namespace is `kubernetes-cost-guard`, and automated sync is enabled with prune and self-heal:

- a Git change to `deploy/k8s` becomes the desired cluster state
- ArgoCD compares that desired state with the live cluster
- ArgoCD syncs changes into the `kubernetes-cost-guard` namespace
- if someone manually drifts the live objects, self-heal restores the Git version

End-to-end deployment flow:

```text
code change -> CI checks -> GHCR image push -> deployment.yaml image update -> ArgoCD sync -> Kubernetes rollout
```

GitHub Actions does not deploy directly to the cluster. Its deployment responsibility stops at publishing the image and updating Git. ArgoCD remains the only actor that reconciles Kubernetes manifests into the cluster.

Apply the ArgoCD Application after ArgoCD is installed:

```bash
kubectl apply -f deploy/argocd/application.yaml
```

The Application uses `CreateNamespace=true`, so ArgoCD can create the target namespace during sync.

## Roadmap

The implementation will proceed in small, testable phases:

1. Project scaffolding and domain models
2. HTTP API skeleton with versioned endpoints
3. Kubernetes read-only integration
4. Workload analysis
5. Namespace summaries
6. Rule-based recommendations
7. Dry-run patch previews
8. Safe apply logic
9. PostgreSQL-backed history
10. Slack alerts
11. Backstage-ready API design notes
