package configx

import (
	"context"

	"github.com/samber/mo"
	"github.com/samber/oops"
)

// Load is a one-shot helper: it creates a temporary Loader, loads all sources,
// and unmarshals the result into out.
//
//	var cfg AppConfig
//	if err := configx.Load(&cfg,
//	    configx.WithFiles("config.yaml"),
//	    configx.WithEnvPrefix("APP"),
//	); err != nil { ... }
func Load(out any, opts ...Option) error {
	return LoadContext(context.Background(), out, opts...)
}

// LoadContext is a one-shot helper like [Load], but passes ctx to
// caller-provided custom sources.
func LoadContext(ctx context.Context, out any, opts ...Option) error {
	return New(opts...).LoadContext(ctx, out)
}

// LoadT is a one-shot helper that returns the typed config wrapped in a
// [mo.Result].
func LoadT[T any](opts ...Option) mo.Result[T] {
	return LoadTContext[T](context.Background(), opts...)
}

// LoadTContext is a one-shot helper like [LoadT], but passes ctx to
// caller-provided custom sources.
func LoadTContext[T any](ctx context.Context, opts ...Option) mo.Result[T] {
	return NewT[T](opts...).LoadContext(ctx)
}

// LoadTErr is a one-shot helper that returns the typed config as a plain
// (value, error) pair.
func LoadTErr[T any](opts ...Option) (T, error) {
	return LoadTErrContext[T](context.Background(), opts...)
}

// LoadTErrContext is a one-shot helper like [LoadTErr], but passes ctx to
// caller-provided custom sources.
func LoadTErrContext[T any](ctx context.Context, opts ...Option) (T, error) {
	result := LoadTContext[T](ctx, opts...)
	value, err := result.Get()
	if err != nil {
		return value, oops.In("configx").
			With("op", "load_typed").
			Wrapf(err, "load typed config")
	}

	return value, nil
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

// LoadConfigT is a one-shot helper that returns a raw *Config (the type
// parameter T is used only for option inference; it is not unmarshalled here).
func LoadConfigT[T any](opts ...Option) (*Config, error) {
	return LoadConfigTContext[T](context.Background(), opts...)
}

// LoadConfigTContext is a one-shot helper like [LoadConfigT], but passes ctx to
// caller-provided custom sources.
func LoadConfigTContext[T any](ctx context.Context, opts ...Option) (*Config, error) {
	return NewT[T](opts...).LoadConfigContext(ctx)
}

// NewWatcherT creates a one-shot typed watcher.
func NewWatcherT[T any](opts ...Option) (*WatcherT[T], error) {
	return NewT[T](opts...).NewWatcherT()
}
