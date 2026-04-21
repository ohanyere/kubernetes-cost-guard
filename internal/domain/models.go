package domain

import "time"

type WorkloadKind string

const (
	WorkloadKindDeployment WorkloadKind = "Deployment"
)

type WorkloadRef struct {
	Namespace string       `json:"namespace"`
	Name      string       `json:"name"`
	Kind      WorkloadKind `json:"kind"`
}

type ResourceQuantity struct {
	CPUMilli  int64 `json:"cpuMilli"`
	MemoryMiB int64 `json:"memoryMiB"`
}

type ContainerResources struct {
	Name     string           `json:"name"`
	Requests ResourceQuantity `json:"requests"`
	Limits   ResourceQuantity `json:"limits"`
}

type WorkloadResources struct {
	Ref               WorkloadRef          `json:"ref"`
	Replicas          int32                `json:"replicas"`
	DesiredReplicas   int32                `json:"desiredReplicas"`
	AvailableReplicas int32                `json:"availableReplicas"`
	Containers        []ContainerResources `json:"containers"`
	HasHPA            bool                 `json:"hasHPA"`
}

type ContainerUsage struct {
	Name           string `json:"name"`
	CPUUsedMilli   int64  `json:"cpuUsedMilli"`
	MemoryUsedMiB  int64  `json:"memoryUsedMiB"`
	MetricsWarning string `json:"metricsWarning,omitempty"`
}

type UsageSnapshot struct {
	Ref              WorkloadRef      `json:"ref"`
	CapturedAt       time.Time        `json:"capturedAt"`
	MetricsAvailable bool             `json:"metricsAvailable"`
	Containers       []ContainerUsage `json:"containers"`
	Warnings         []string         `json:"warnings,omitempty"`
}
