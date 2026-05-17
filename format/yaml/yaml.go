// Package yaml provides file parser support for YAML configuration files.
package yaml

import (
	parseryaml "github.com/knadh/koanf/parsers/yaml"

	"github.com/arcgolabs/configx"
)

// WithYAMLSupport registers support for .yaml and .yml config files.
func WithYAMLSupport() configx.Option {
	return func(o *configx.Options) {
		configx.WithFileParser(".yaml", parseryaml.Parser())(o)
		configx.WithFileParser(".yml", parseryaml.Parser())(o)
	}
}
