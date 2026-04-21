package kube

import (
	"context"

	"github.com/ohanyere/kubernetes-cost-guard/internal/change"
	"github.com/ohanyere/kubernetes-cost-guard/internal/domain"
)

type Reader interface {
	GetDeployment(ctx context.Context, namespace, name string) (DeploymentSnapshot, error)
	ListDeployments(ctx context.Context, namespace string) (NamespaceSnapshot, error)
	ApplyDeploymentPatch(ctx context.Context, target domain.WorkloadRef, operations []change.Operation) ([]change.Operation, error)
}
