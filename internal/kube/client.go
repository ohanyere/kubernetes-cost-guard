package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/ohanyere/kubernetes-cost-guard/internal/change"
	"github.com/ohanyere/kubernetes-cost-guard/internal/config"
	"github.com/ohanyere/kubernetes-cost-guard/internal/domain"
)

type Client struct {
	core    kubernetes.Interface
	metrics metricsclientset.Interface
}

func NewClient(cfg config.Config) (*Client, error) {
	restConfig, err := loadRESTConfig(cfg.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrClientUnavailable, err)
	}

	core, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}

	metrics, err := metricsclientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create metrics client: %w", err)
	}

	return &Client{core: core, metrics: metrics}, nil
}

func loadRESTConfig(kubeconfig string) (*rest.Config, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}

	path := kubeconfig
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, ".kube", "config")
	}

	return clientcmd.BuildConfigFromFlags("", path)
}

func (c *Client) GetDeployment(ctx context.Context, namespace, name string) (DeploymentSnapshot, error) {
	deployment, err := c.core.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return DeploymentSnapshot{}, ErrNotFound
		}
		return DeploymentSnapshot{}, err
	}

	return c.snapshotDeployment(ctx, deployment)
}

func (c *Client) ListDeployments(ctx context.Context, namespace string) (NamespaceSnapshot, error) {
	deployments, err := c.core.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return NamespaceSnapshot{}, ErrNotFound
		}
		return NamespaceSnapshot{}, err
	}

	items := make([]DeploymentSnapshot, 0, len(deployments.Items))
	for i := range deployments.Items {
		snapshot, err := c.snapshotDeployment(ctx, &deployments.Items[i])
		if err != nil {
			return NamespaceSnapshot{}, err
		}
		items = append(items, snapshot)
	}

	return NamespaceSnapshot{Namespace: namespace, Items: items}, nil
}

func (c *Client) ApplyDeploymentPatch(ctx context.Context, target domain.WorkloadRef, operations []change.Operation) ([]change.Operation, error) {
	if len(operations) == 0 {
		return nil, nil
	}

	payload, err := deploymentPatchPayload(operations)
	if err != nil {
		return nil, err
	}

	if _, err := c.core.AppsV1().Deployments(target.Namespace).Patch(ctx, target.Name, types.StrategicMergePatchType, payload, metav1.PatchOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return operations, nil
}

func deploymentPatchPayload(operations []change.Operation) ([]byte, error) {
	patchBody := map[string]any{
		"spec": map[string]any{},
	}
	spec := patchBody["spec"].(map[string]any)
	containersByName := make(map[string]map[string]any)

	for _, operation := range operations {
		switch operation.FieldPath {
		case "spec.replicas":
			replicas, err := strconv.Atoi(operation.NewValue)
			if err != nil {
				return nil, fmt.Errorf("parse replicas %q: %w", operation.NewValue, err)
			}
			spec["replicas"] = replicas
		case "spec.template.spec.containers[].resources.requests.cpu":
			if _, err := resource.ParseQuantity(operation.NewValue); err != nil {
				return nil, fmt.Errorf("parse cpu request %q: %w", operation.NewValue, err)
			}
			requestsForContainer(containersByName, operation.TargetContainer)["cpu"] = operation.NewValue
		case "spec.template.spec.containers[].resources.requests.memory":
			if _, err := resource.ParseQuantity(operation.NewValue); err != nil {
				return nil, fmt.Errorf("parse memory request %q: %w", operation.NewValue, err)
			}
			requestsForContainer(containersByName, operation.TargetContainer)["memory"] = operation.NewValue
		default:
			return nil, fmt.Errorf("unsupported patch field path %q", operation.FieldPath)
		}
	}

	if len(containersByName) > 0 {
		containers := make([]map[string]any, 0, len(containersByName))
		names := make([]string, 0, len(containersByName))
		for name := range containersByName {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			containers = append(containers, map[string]any{
				"name": name,
				"resources": map[string]any{
					"requests": containersByName[name],
				},
			})
		}
		spec["template"] = map[string]any{
			"spec": map[string]any{
				"containers": containers,
			},
		}
	}

	return json.Marshal(patchBody)
}

func requestsForContainer(containersByName map[string]map[string]any, name string) map[string]any {
	if containersByName[name] == nil {
		containersByName[name] = make(map[string]any)
	}
	return containersByName[name]
}

func (c *Client) snapshotDeployment(ctx context.Context, deployment *appsv1.Deployment) (DeploymentSnapshot, error) {
	resources := MapDeploymentResources(deployment)
	usage, warnings := c.deploymentUsage(ctx, deployment)
	return DeploymentSnapshot{
		Resources: resources,
		Usage:     usage,
		Warnings:  warnings,
	}, nil
}
