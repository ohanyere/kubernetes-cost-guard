package http

import (
	"errors"

	"github.com/ohanyere/kubernetes-cost-guard/internal/domain"
)

func validateTarget(target WorkloadTarget) error {
	if target.Namespace == "" {
		return errors.New("target.namespace is required")
	}
	if target.Name == "" {
		return errors.New("target.name is required")
	}
	if target.Kind == "" {
		return errors.New("target.kind is required")
	}
	if target.Kind != domain.WorkloadKindDeployment {
		return errors.New("target.kind must be Deployment in v1")
	}
	return nil
}
