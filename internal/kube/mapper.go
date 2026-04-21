package kube

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/ohanyere/kubernetes-cost-guard/internal/domain"
)

func MapDeploymentResources(deployment *appsv1.Deployment) domain.WorkloadResources {
	desired := int32(1)
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}

	containers := make([]domain.ContainerResources, 0, len(deployment.Spec.Template.Spec.Containers))
	for _, container := range deployment.Spec.Template.Spec.Containers {
		containers = append(containers, domain.ContainerResources{
			Name:     container.Name,
			Requests: mapResourceQuantity(container.Resources.Requests),
			Limits:   mapResourceQuantity(container.Resources.Limits),
		})
	}

	return domain.WorkloadResources{
		Ref: domain.WorkloadRef{
			Namespace: deployment.Namespace,
			Name:      deployment.Name,
			Kind:      domain.WorkloadKindDeployment,
		},
		Replicas:          desired,
		DesiredReplicas:   desired,
		AvailableReplicas: deployment.Status.AvailableReplicas,
		Containers:        containers,
		// TODO: detect associated HorizontalPodAutoscalers once the kube reader owns HPA lookups.
		HasHPA: false,
	}
}

func mapResourceQuantity(resources corev1.ResourceList) domain.ResourceQuantity {
	return domain.ResourceQuantity{
		CPUMilli:  quantityMilliValue(resources, corev1.ResourceCPU),
		MemoryMiB: quantityMiBValue(resources, corev1.ResourceMemory),
	}
}

func quantityMilliValue(resources corev1.ResourceList, name corev1.ResourceName) int64 {
	quantity, ok := resources[name]
	if !ok {
		return 0
	}
	return quantity.MilliValue()
}

func quantityMiBValue(resources corev1.ResourceList, name corev1.ResourceName) int64 {
	quantity, ok := resources[name]
	if !ok {
		return 0
	}
	return quantity.Value() / (1024 * 1024)
}
