package configx

import "github.com/arcgolabs/pkg/option"

func buildOptions(opts ...Option) *Options {
	options := NewOptions()
	option.Apply(options, opts...)
	return options
}
