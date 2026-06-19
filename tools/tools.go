//go:build tools

// Package tools pins developer tooling used via `go generate` so it is tracked
// in go.mod. It is never compiled into the provider binary.
package tools

import (
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)
