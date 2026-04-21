package kube

import "errors"

var (
	ErrClientUnavailable = errors.New("kubernetes client unavailable")
	ErrNotFound          = errors.New("kubernetes resource not found")
)
