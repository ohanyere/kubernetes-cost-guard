package kube

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/ohanyere/kubernetes-cost-guard/internal/domain"
)

func (c *Client) deploymentUsage(ctx context.Context, deployment *appsv1.Deployment) (domain.UsageSnapshot, []string) {
	ref := domain.WorkloadRef{
		Namespace: deployment.Namespace,
		Name:      deployment.Name,
		Kind:      domain.WorkloadKindDeployment,
	}
	usage := domain.UsageSnapshot{
		Ref:        ref,
		CapturedAt: time.Now().UTC(),
	}

	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		warning := fmt.Sprintf("deployment selector could not be parsed: %v", err)
		usage.Warnings = []string{warning}
		return usage, usage.Warnings
	}
	if selector.Empty() {
		warning := "deployment selector is empty; metrics lookup skipped"
		usage.Warnings = []string{warning}
		return usage, usage.Warnings
	}

	podMetrics, err := c.metrics.MetricsV1beta1().PodMetricses(deployment.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		warning := fmt.Sprintf("metrics-server data unavailable: %v", err)
		usage.Warnings = []string{warning}
		return usage, usage.Warnings
	}

	containerUsage := make(map[string]domain.ContainerUsage)
	for _, pod := range podMetrics.Items {
		if !selector.Matches(labels.Set(pod.Labels)) {
			continue
		}
		for _, container := range pod.Containers {
			current := containerUsage[container.Name]
			current.Name = container.Name
			current.CPUUsedMilli += container.Usage.Cpu().MilliValue()
			current.MemoryUsedMiB += container.Usage.Memory().Value() / (1024 * 1024)
			containerUsage[container.Name] = current
		}
	}

	usage.Containers = make([]domain.ContainerUsage, 0, len(containerUsage))
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if value, ok := containerUsage[container.Name]; ok {
			usage.Containers = append(usage.Containers, value)
		}
	}

	usage.MetricsAvailable = len(usage.Containers) > 0
	if !usage.MetricsAvailable {
		usage.Warnings = []string{"metrics-server returned no usage for this deployment"}
		return usage, usage.Warnings
	}

	return usage, nil
}
