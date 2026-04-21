package kube

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMapDeploymentResources(t *testing.T) {
	replicas := int32(3)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "app",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("250m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas: 2,
		},
	}

	got := MapDeploymentResources(deployment)

	if got.Ref.Namespace != "default" || got.Ref.Name != "web" {
		t.Fatalf("unexpected ref: %#v", got.Ref)
	}
	if got.DesiredReplicas != 3 || got.Replicas != 3 || got.AvailableReplicas != 2 {
		t.Fatalf("unexpected replicas: %#v", got)
	}
	if len(got.Containers) != 1 {
		t.Fatalf("expected one container, got %d", len(got.Containers))
	}
	container := got.Containers[0]
	if container.Requests.CPUMilli != 250 || container.Requests.MemoryMiB != 512 {
		t.Fatalf("unexpected requests: %#v", container.Requests)
	}
	if container.Limits.CPUMilli != 1000 || container.Limits.MemoryMiB != 1024 {
		t.Fatalf("unexpected limits: %#v", container.Limits)
	}
}
