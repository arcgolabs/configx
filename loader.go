package configx

import (
	"context"
	"errors"

	"github.com/arcgolabs/observabilityx"
	"github.com/samber/mo"
	"github.com/samber/oops"
)

const (
	metricConfigLoadTotal            = "configx_load_total"
	metricConfigLoadDurationMS       = "configx_load_duration_ms"
	metricConfigSourceLoadTotal      = "configx_source_load_total"
	metricConfigSourceLoadDurationMS = "configx_source_load_duration_ms"
)

var (
	configLoadTotalSpec = observabilityx.NewCounterSpec(
		metricConfigLoadTotal,
		observabilityx.WithDescription("Total number of config load operations."),
		observabilityx.WithLabelKeys("result"),
	)
	configLoadDurationSpec = observabilityx.NewHistogramSpec(
		metricConfigLoadDurationMS,
		observabilityx.WithDescription("Duration of config load operations in milliseconds."),
		observabilityx.WithUnit("ms"),
		observabilityx.WithLabelKeys("result"),
	)
	configSourceLoadTotalSpec = observabilityx.NewCounterSpec(
		metricConfigSourceLoadTotal,
		observabilityx.WithDescription("Total number of config source load operations."),
		observabilityx.WithLabelKeys("source", "result"),
	)
	configSourceLoadDurationSpec = observabilityx.NewHistogramSpec(
		metricConfigSourceLoadDurationMS,
		observabilityx.WithDescription("Duration of config source load operations in milliseconds."),
		observabilityx.WithUnit("ms"),
		observabilityx.WithLabelKeys("source", "result"),
	)
)

// ─── Loader ───────────────────────────────────────────────────────────────────

// Loader loads configuration from the sources defined in its Options and can
// optionally watch those sources for live changes.
//
// Build one with [New] and then call [Loader.Load], [Loader.LoadConfig], or
// [Loader.Watch] / [Loader.NewWatcher] for hot-reload support.
type Loader struct {
	opts *Options
}

// New creates a Loader from the supplied functional options.
//
//	loader := configx.New(
//	    configx.WithFiles("config.yaml"),
//	    configx.WithEnvPrefix("APP"),
//	)
func New(opts ...Option) *Loader {
	return &Loader{opts: buildOptions(opts...)}
}

// Load reads all configured sources, unmarshals the result into out, and runs
// struct validation according to the configured ValidateLevel.
func (l *Loader) Load(out any) error {
	return l.LoadContext(context.Background(), out)
}

// LoadContext is like [Loader.Load], but passes ctx to caller-provided custom
// sources.
func (l *Loader) LoadContext(ctx context.Context, out any) error {
	cfg, err := l.loadInternal(ctx)
	if err != nil {
		return oops.In("configx").
			With("op", "load").
			Wrapf(errors.Join(ErrLoad, err), "config")
	}
	if err := cfg.k.Unmarshal("", out); err != nil {
		return oops.In("configx").
			With("op", "unmarshal_output").
			Wrapf(errors.Join(ErrUnmarshal, err), "config output")
	}
	if err := cfg.validateStruct(out); err != nil {
		return oops.In("configx").
			With("op", "validate_output").
			Wrapf(errors.Join(ErrValidate, err), "config output")
	}
	return nil
}

// LoadConfig reads all configured sources and returns a *Config for ad-hoc
// path-based access (GetString, GetInt, Unmarshal, …).
func (l *Loader) LoadConfig() (*Config, error) {
	return l.LoadConfigContext(context.Background())
}

// LoadConfigContext is like [Loader.LoadConfig], but passes ctx to
// caller-provided custom sources.
func (l *Loader) LoadConfigContext(ctx context.Context) (*Config, error) {
	return l.loadInternal(ctx)
}

// NewWatcher performs the initial load and returns a *Watcher that will
// re-read all sources whenever a watched config file changes.
//
// Call [Watcher.Start] (typically in a goroutine) to begin watching.
func (l *Loader) NewWatcher() (*Watcher, error) {
	return newWatcherFromOptions(context.Background(), l.opts)
}

// Watch is a convenience wrapper around [Loader.NewWatcher] + [Watcher.Start].
// It registers onChange as a [ChangeHandler] and then blocks until ctx is
// canceled. onChange may be nil if the caller only needs the side-effect of
// keeping w.Config() up-to-date.
func (l *Loader) Watch(ctx context.Context, onChange ChangeHandler) error {
	w, err := newWatcherFromOptions(ctx, l.opts)
	if err != nil {
		return err
	}
	if onChange != nil {
		w.OnChange(onChange)
	}
	return w.Start(ctx)
}

func (l *Loader) loadInternal(ctx context.Context) (*Config, error) {
	return loadConfigFromOptions(ctx, l.opts)
}

// ─── LoaderT ──────────────────────────────────────────────────────────────────

// LoaderT is the generic, type-safe counterpart of [Loader]. It unmarshals the
// full config into T and returns the result wrapped in a [mo.Result].
//
// Build one with [NewT] and then call [LoaderT.Load], [LoaderT.LoadConfig], or
// [LoaderT.Watch] / [LoaderT.NewWatcher] for hot-reload support.
type LoaderT[T any] struct {
	opts *Options
}

// NewT creates a LoaderT[T] from the supplied functional options.
//
//	loader := configx.NewT[AppConfig](
//	    configx.WithFiles("config.yaml"),
//	    configx.WithEnvPrefix("APP"),
//	    configx.WithValidateLevel(configx.ValidateLevelStruct),
//	)
func NewT[T any](opts ...Option) *LoaderT[T] {
	return &LoaderT[T]{opts: buildOptions(opts...)}
}

// Load reads all configured sources, unmarshals the result into a new T, runs
// struct validation, and returns the value wrapped in a [mo.Result].
func (l *LoaderT[T]) Load() mo.Result[T] {
	return l.LoadContext(context.Background())
}

// LoadContext is like [LoaderT.Load], but passes ctx to caller-provided custom
// sources.
func (l *LoaderT[T]) LoadContext(ctx context.Context) mo.Result[T] {
	cfg, err := l.loadInternal(ctx)
	if err != nil {
		return mo.Err[T](err)
	}

	var out T
	if err := cfg.k.Unmarshal("", &out); err != nil {
		return mo.Err[T](oops.In("configx").
			With("op", "unmarshal_typed_output").
			Wrapf(errors.Join(ErrUnmarshal, err), "typed output"))
	}
	if err := cfg.validateStruct(out); err != nil {
		return mo.Err[T](oops.In("configx").
			With("op", "validate_typed_output").
			Wrapf(errors.Join(ErrValidate, err), "typed output"))
	}
	return mo.Ok(out)
}

// LoadConfig reads all configured sources and returns a raw *Config for
// path-based access.
func (l *LoaderT[T]) LoadConfig() (*Config, error) {
	return l.LoadConfigContext(context.Background())
}

// LoadConfigContext is like [LoaderT.LoadConfig], but passes ctx to
// caller-provided custom sources.
func (l *LoaderT[T]) LoadConfigContext(ctx context.Context) (*Config, error) {
	return l.loadInternal(ctx)
}

// NewWatcher performs the initial load and returns a *Watcher that will
// re-read all sources whenever a watched config file changes.
//
// Call [Watcher.Start] (typically in a goroutine) to begin watching.
func (l *LoaderT[T]) NewWatcher() (*Watcher, error) {
	return newWatcherFromOptions(context.Background(), l.opts)
}

// NewWatcherT performs the initial load and returns a typed watcher that
// publishes validated T snapshots on every successful reload.
func (l *LoaderT[T]) NewWatcherT() (*WatcherT[T], error) {
	return newWatcherTFromOptions[T](context.Background(), l.opts)
}

// Watch is a convenience wrapper around [LoaderT.NewWatcher] + [Watcher.Start].
// It registers onChange as a [ChangeHandler] and then blocks until ctx is
// canceled.
func (l *LoaderT[T]) Watch(ctx context.Context, onChange ChangeHandler) error {
	w, err := newWatcherFromOptions(ctx, l.opts)
	if err != nil {
		return err
	}
	if onChange != nil {
		w.OnChange(onChange)
	}
	return w.Start(ctx)
}

// WatchT is the typed convenience wrapper around NewWatcherT + Start.
func (l *LoaderT[T]) WatchT(ctx context.Context, onChange ChangeHandlerT[T]) error {
	w, err := newWatcherTFromOptions[T](ctx, l.opts)
	if err != nil {
		return err
	}
	if onChange != nil {
		w.OnChange(onChange)
	}
	return w.Start(ctx)
}

func (l *LoaderT[T]) loadInternal(ctx context.Context) (*Config, error) {
	return loadConfigFromOptions(ctx, l.opts)
}
