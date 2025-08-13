//go:build tools
// +build tools

package tools

import (
	_ "github.com/maxbrunsfeld/counterfeiter/v6"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
