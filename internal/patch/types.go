package patch

import "github.com/ohanyere/kubernetes-cost-guard/internal/change"

type PatchOperation = change.Operation

type PatchPreview struct {
	Operations []PatchOperation `json:"operations"`
}
