package configx

import (
	"context"
)

// Load is a one-shot helper that loads all sources and returns T.
//
//	cfg, err := configx.Load[AppConfig](
//	    configx.WithFiles("config.yaml"),
//	    configx.WithEnvPrefix("APP"),
//	)
func Load[T any](opts ...Option) (T, error) {
	return LoadContext[T](context.Background(), opts...)
}

// LoadContext is a one-shot helper like [Load], but passes ctx to
// caller-provided custom sources.
func LoadContext[T any](ctx context.Context, opts ...Option) (T, error) {
	return New(opts...).LoadContext[T](ctx)
}

// LoadConfig is a one-shot helper that returns a raw *Config.
func LoadConfig(opts ...Option) (*Config, error) {
	return LoadConfigContext(context.Background(), opts...)
}

// LoadConfigContext is a one-shot helper like [LoadConfig], but passes ctx to
// caller-provided custom sources.
func LoadConfigContext(ctx context.Context, opts ...Option) (*Config, error) {
	return New(opts...).LoadConfigContext(ctx)
}

// NewWatcherT creates a one-shot typed watcher.
func NewWatcherT[T any](opts ...Option) (*WatcherT[T], error) {
	return New(opts...).NewWatcherT[T]()
}
