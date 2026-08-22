package configx

func buildOptions(opts ...Option) *Options {
	options := NewOptions()
	for _, apply := range opts {
		if apply != nil {
			apply(options)
		}
	}
	return options
}
