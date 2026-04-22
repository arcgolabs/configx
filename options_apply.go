package configx

import "github.com/DaiYuANg/arcgo/pkg/option"

func buildOptions(opts ...Option) *Options {
	options := NewOptions()
	option.Apply(options, opts...)
	return options
}
