//go:build tools
// +build tools

// Package tools is used to track dependencies on binary tools.
package tools

import (
	_ "golang.org/x/lint/golint"         // Golint linter
	_ "golang.org/x/tools/cmd/goimports" // Goimports tool
)
