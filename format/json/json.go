// Package json provides file parser support for JSON configuration files.
package json

import (
	parserjson "github.com/knadh/koanf/parsers/json"

	"github.com/arcgolabs/configx"
)

// WithJSONSupport registers support for .json config files.
func WithJSONSupport() configx.Option {
	return configx.WithFileParser(".json", parserjson.Parser())
}
