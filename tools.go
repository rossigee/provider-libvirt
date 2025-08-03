//go:build tools
// +build tools

/*
Copyright 2025 Ross Golder
*/

package tools

import (
	// code-generator tools
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)

//go:generate go run sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./apis/..."