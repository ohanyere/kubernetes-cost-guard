package kube

import "github.com/ohanyere/kubernetes-cost-guard/internal/domain"

type DeploymentSnapshot struct {
	Resources domain.WorkloadResources `json:"resources"`
	Usage     domain.UsageSnapshot     `json:"usage"`
	Warnings  []string                 `json:"warnings,omitempty"`
}

type NamespaceSnapshot struct {
	Namespace string               `json:"namespace"`
	Items     []DeploymentSnapshot `json:"items"`
}
