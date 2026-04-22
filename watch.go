package configx

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/knadh/koanf/providers/file"
	"github.com/samber/lo"
	"github.com/samber/oops"
)

// ChangeHandler is the signature for callbacks registered with [Watcher.OnChange].
// cfg holds the freshly loaded config; err is non-nil when the reload failed.
// When err is non-nil, cfg is nil and the previous config remains active.
type ChangeHandler func(cfg *Config, err error)

type changeHandlers []ChangeHandler

// ChangeHandlerT is the callback signature for typed hot-reload handlers.
// cfg is the newly decoded typed config value when err is nil.
type ChangeHandlerT[T any] func(cfg T, err error)

// Watcher manages a live-reloading *Config.
//
// It sets up an fsnotify watcher for every file listed in the original option
// set. Whenever any of those files is written or recreated, the Watcher
// performs a *full* reload (defaults → files → env) so that every source is
// always in sync. Multiple rapid saves are collapsed into a single reload via
// a configurable debounce window (default 100 ms).
//
// Typical usage:
//
//	w, err := configx.NewWatcher(
//	    configx.WithFiles("config.yaml"),
//	    configx.WithEnvPrefix("APP"),
//	    configx.WithWatchDebounce(200*time.Millisecond),
//	    configx.WithWatchErrHandler(func(err error) {
//	        slog.Error("config watch error", "err", err)
//	    }),
//	)
//
//	w.OnChange(func(cfg *configx.Config, err error) {
//	    if err == nil {
//	        slog.Info("config reloaded", "port", cfg.GetInt("server.port"))
//	    }
//	})
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	go w.Start(ctx)
//
//	// Always use w.Config() to get the latest snapshot.
//	port := w.Config().GetInt("server.port")
type Watcher struct {
	// cfg is replaced atomically after each successful reload.
	cfg atomic.Pointer[Config]

	opts *Options

	// subsMu serializes subscriber registration; notify reads an immutable
	// snapshot through subs without taking a lock.
	subsMu sync.Mutex
	subs   atomic.Pointer[changeHandlers]

	// providers are used *only* for change detection – actual loading is
	// always done by a fresh call to loadConfigFromOptions. They are immutable
	// after construction, so a plain slice remains the cheapest representation.
	providers []*file.File

	// stopCh is closed by Close to signal the Start loop to exit.
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewWatcher builds a Watcher from the supplied options, performs the initial
// config load, and prepares fsnotify watchers for every supported config file.
//
// Call [Watcher.Start] (typically in a goroutine) to begin watching.
func NewWatcher(opts ...Option) (*Watcher, error) {
	return newWatcherFromOptions(context.Background(), buildOptions(opts...))
}

// newWatcherFromOptions is the internal constructor shared by NewWatcher and
// Loader.NewWatcher so that the options pointer is reused without re-applying
// functional options a second time.
func newWatcherFromOptions(ctx context.Context, opts *Options) (*Watcher, error) {
	cfg, err := loadConfigFromOptions(ctx, opts)
	if err != nil {
		logError(opts, "configx watcher initial load failed", "error", err)
		return nil, oops.In("configx").
			With("op", "watcher_initial_load").
			Wrapf(err, "configx: watcher initial load")
	}

	w := &Watcher{
		opts:      opts,
		providers: buildWatchProviders(opts.files),
		stopCh:    make(chan struct{}),
	}
	w.cfg.Store(cfg)
	logDebug(opts, "configx watcher created", "providers", len(w.providers))
	return w, nil
}

// Config returns the most recently successfully loaded config snapshot.
// It is safe to call from multiple goroutines.
func (w *Watcher) Config() *Config {
	return w.cfg.Load()
}

// OnChange registers fn to be called after every reload attempt.
//
//   - On success: cfg is the new config, err is nil.
//   - On failure: cfg is nil, err describes what went wrong; the previous
//     config remains active (w.Config() is unchanged).
//
// Handlers are invoked in registration order from a single goroutine, so they
// do not need to be goroutine-safe relative to each other.  Heavy work should
// be dispatched to a separate goroutine to avoid blocking the reload loop.
func (w *Watcher) OnChange(fn ChangeHandler) {
	if fn == nil {
		return
	}
	w.subsMu.Lock()
	defer w.subsMu.Unlock()

	current := w.loadSubscribers()
	w.subs.Store(new(changeHandlers(lo.Concat(current, []ChangeHandler{fn}))))
}

// Start begins watching config files for changes and blocks until ctx is
// canceled or [Watcher.Close] is called.
//
// If no files are configured Start simply waits for the context to be done, so
// it is always safe to run in a goroutine regardless of the option set.
//
// Errors from individual file watchers are forwarded to the handler registered
// with [WithWatchErrHandler]; Start itself only returns a non-nil error when
// it cannot set up an fsnotify watcher for a file.
func (w *Watcher) Start(ctx context.Context) error {
	ctx = normalizeWatcherContext(ctx)

	// Nothing to watch - block until signaled.
	if len(w.providers) == 0 {
		logDebug(w.opts, "configx watcher started without providers")
		return w.waitForStop(ctx)
	}

	debounce := normalizeWatchDebounce(w.opts.watchDebounce)
	reloadCh := make(chan struct{}, 1)
	if err := w.startProviders(func() {
		queueWatcherReload(reloadCh)
	}); err != nil {
		return err
	}

	logDebug(w.opts, "configx watcher started", "providers", len(w.providers), "debounce_ms", debounce.Milliseconds())
	return w.run(ctx, debounce, reloadCh)
}

// Close stops all file watchers and unblocks [Watcher.Start].
// It is idempotent and safe to call from multiple goroutines.
func (w *Watcher) Close() error {
	w.stopOnce.Do(func() { close(w.stopCh) })
	logDebug(w.opts, "configx watcher closing", "providers", len(w.providers))

	errs := lo.FilterMap(w.providers, func(fp *file.File, _ int) (error, bool) {
		err := fp.Unwatch()
		return oops.In("configx").
			With("op", "watcher_close").
			Wrapf(err, "configx: unwatch provider"), err != nil
	})
	if len(errs) > 0 {
		logError(w.opts, "configx watcher close completed with errors", "errors", len(errs))
	} else {
		logDebug(w.opts, "configx watcher closed")
	}
	return errors.Join(errs...)
}

// WatcherT provides typed hot-reload snapshots on top of Watcher.
type WatcherT[T any] struct {
	base    *Watcher
	current atomic.Pointer[T]
}

func newWatcherTFromOptions[T any](ctx context.Context, opts *Options) (*WatcherT[T], error) {
	base, err := newWatcherFromOptions(ctx, opts)
	if err != nil {
		return nil, err
	}

	var initial T
	if err := base.Config().Unmarshal("", &initial); err != nil {
		return nil, oops.In("configx").
			With("op", "typed_watcher_initial_unmarshal").
			Wrapf(errors.Join(ErrUnmarshal, err), "initial typed watcher unmarshal")
	}
	if err := base.Config().validateStruct(initial); err != nil {
		return nil, oops.In("configx").
			With("op", "typed_watcher_initial_validate").
			Wrapf(errors.Join(ErrValidate, err), "initial typed watcher value")
	}

	w := &WatcherT[T]{base: base}
	w.current.Store(&initial)
	return w, nil
}

// Config returns the latest successfully decoded typed snapshot.
func (w *WatcherT[T]) Config() T {
	ptr := w.current.Load()
	if ptr == nil {
		var zero T
		return zero
	}
	return *ptr
}

// RawConfig returns the underlying dynamic config snapshot.
func (w *WatcherT[T]) RawConfig() *Config {
	return w.base.Config()
}

// OnChange registers a typed callback. Decode/validate failures are surfaced
// via err and do not replace the current typed snapshot.
func (w *WatcherT[T]) OnChange(fn ChangeHandlerT[T]) {
	if fn == nil {
		return
	}
	w.base.OnChange(func(cfg *Config, err error) {
		var zero T
		if err != nil {
			fn(zero, err)
			return
		}
		var out T
		if err := cfg.Unmarshal("", &out); err != nil {
			wrapped := oops.In("configx").
				With("op", "typed_watcher_callback_unmarshal").
				Wrapf(errors.Join(ErrUnmarshal, err), "watcher typed callback decode")
			w.base.handleErr(wrapped)
			fn(zero, wrapped)
			return
		}
		if err := cfg.validateStruct(out); err != nil {
			wrapped := oops.In("configx").
				With("op", "typed_watcher_callback_validate").
				Wrapf(errors.Join(ErrValidate, err), "watcher typed callback value")
			w.base.handleErr(wrapped)
			fn(zero, wrapped)
			return
		}
		w.current.Store(&out)
		fn(out, nil)
	})
}

// Start starts the underlying watcher loop.
func (w *WatcherT[T]) Start(ctx context.Context) error {
	return w.base.Start(ctx)
}

// Close stops the underlying watcher.
func (w *WatcherT[T]) Close() error {
	return w.base.Close()
}
