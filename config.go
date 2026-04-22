package configx

// NewConfig creates related functionality.
// Note.
// Note.
//
// Note.
//
//	cfg, err := configx.NewConfig(
//	    configx.WithFiles("config.yaml"),
//	    configx.WithEnvPrefix("APP_"),
//	    configx.WithDefaults(map[string]any{"port": 8080}),
//	)
func NewConfig(opts ...Option) (*Config, error) {
	return LoadConfig(opts...)
}
