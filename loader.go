package configx

import (
	"context"
	"errors"

	"github.com/arcgolabs/observabilityx"
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

// Load reads all configured sources, unmarshals the result into T, and runs
// struct validation according to the configured ValidateLevel.
func (l *Loader) Load[T any]() (T, error) {
	return l.LoadContext[T](context.Background())
}

// LoadContext is like [Loader.Load], but passes ctx to caller-provided custom
// sources.
func (l *Loader) LoadContext[T any](ctx context.Context) (T, error) {
	var zero T
	cfg, err := l.loadInternal(ctx)
	if err != nil {
		return zero, oops.In("configx").
			With("op", "load").
			Wrapf(errors.Join(ErrLoad, err), "config")
	}
	var out T
	if err := cfg.k.Unmarshal("", &out); err != nil {
		return zero, oops.In("configx").
			With("op", "unmarshal_output").
			Wrapf(errors.Join(ErrUnmarshal, err), "config output")
	}
	if err := cfg.validateStruct(out); err != nil {
		return zero, oops.In("configx").
			With("op", "validate_output").
			Wrapf(errors.Join(ErrValidate, err), "config output")
	}
	return out, nil
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

// NewWatcherT performs the initial load and returns a typed watcher that
// publishes validated T snapshots on every successful reload.
func (l *Loader) NewWatcherT[T any]() (*WatcherT[T], error) {
	return newWatcherTFromOptions[T](context.Background(), l.opts)
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

// WatchT is the typed convenience wrapper around NewWatcherT and Start.
func (l *Loader) WatchT[T any](ctx context.Context, onChange ChangeHandlerT[T]) error {
	w, err := newWatcherTFromOptions[T](ctx, l.opts)
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
