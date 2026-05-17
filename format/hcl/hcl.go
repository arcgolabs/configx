// Package hcl provides file parser support for HashiCorp HCL configuration files.
package hcl

import (
	parserhcl "github.com/knadh/koanf/parsers/hcl"

	"github.com/arcgolabs/configx"
)

// WithHCLSupport registers support for .hcl, .tf, and .tfvars config files.
func WithHCLSupport() configx.Option {
	return func(o *configx.Options) {
		parser := parserhcl.Parser(true)
		configx.WithFileParser(".hcl", parser)(o)
		configx.WithFileParser(".tf", parser)(o)
		configx.WithFileParser(".tfvars", parser)(o)
	}
}
