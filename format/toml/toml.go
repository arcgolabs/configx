// Package toml provides file parser support for TOML configuration files.
package toml

import (
	parsertoml "github.com/knadh/koanf/parsers/toml/v2"

	"github.com/arcgolabs/configx"
)

// WithTomlSupport registers support for .toml config files.
func WithTomlSupport() configx.Option {
	return configx.WithFileParser(".toml", parsertoml.Parser())
}
