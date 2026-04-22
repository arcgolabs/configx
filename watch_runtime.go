package configx

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/knadh/koanf/providers/file"
	"github.com/samber/lo"
	"github.com/samber/oops"
)

func normalizeWatcherContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func normalizeWatchDebounce(debounce time.Duration) time.Duration {
	if debounce <= 0 {
		return 100 * time.Millisecond
	}
	return debounce
}

func queueWatcherReload(reloadCh chan<- struct{}) {
	select {
	case reloadCh <- struct{}{}:
	default:
	}
}

func (w *Watcher) startProviders(trigger func()) error {
	started, err := lo.ReduceErr(w.providers, func(started int, fp *file.File, index int) (int, error) {
		if err := fp.Watch(w.watchProvider(index, trigger)); err != nil {
			return started, oops.In("configx").
				With("op", "watcher_start_provider", "provider_index", index, "path", w.providerPath(index)).
				Wrapf(err, "watch provider")
		}
		return started + 1, nil
	}, 0)
	if err != nil {
		w.cleanupStartedProviders(started)
		logError(w.opts, "configx watcher start failed", "started", started, "error", err)
		return oops.In("configx").
			With("op", "watcher_start", "started", started, "provider_count", len(w.providers)).
			Wrapf(err, "start file watcher")
	}
	return nil
}

func (w *Watcher) watchProvider(index int, trigger func()) func(_ any, err error) {
	return func(_ any, err error) {
		if err != nil {
			logError(w.opts, "configx watcher provider error", "index", index, "error", err)
			w.handleErr(oops.In("configx").
				With("op", "watcher_provider_event", "provider_index", index, "path", w.providerPath(index)).
				Wrapf(err, "fsnotify error"))
			return
		}

		logDebug(w.opts, "configx watcher change detected", "index", index)
		trigger()
	}
}

func (w *Watcher) cleanupStartedProviders(count int) {
	lo.ForEach(lo.Range(count), func(index int, _ int) {
		if err := w.providers[index].Unwatch(); err != nil {
			w.handleErr(oops.In("configx").
				With("op", "watcher_cleanup_provider", "provider_index", index, "path", w.providerPath(index)).
				Wrapf(err, "cleanup file watcher"))
		}
	})
}

func (w *Watcher) run(ctx context.Context, debounce time.Duration, reloadCh <-chan struct{}) error {
	resetTimer, stopTimer := newDebounceTimer(debounce, func() {
		w.reload(ctx)
	})
	defer stopTimer()

	for {
		select {
		case <-ctx.Done():
			if err := w.Close(); err != nil {
				w.handleErr(oops.In("configx").
					With("op", "watcher_close_on_context_done", "provider_count", len(w.providers)).
					Wrapf(err, "close watcher"))
			}
			return nil
		case <-w.stopCh:
			return nil
		case <-reloadCh:
			logDebug(w.opts, "configx watcher reload queued")
			resetTimer()
		}
	}
}

func newDebounceTimer(debounce time.Duration, fn func()) (reset, stop func()) {
	var (
		timer   *time.Timer
		timerMu sync.Mutex
	)

	reset = func() {
		timerMu.Lock()
		defer timerMu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(debounce, fn)
	}

	stop = func() {
		timerMu.Lock()
		defer timerMu.Unlock()
		if timer != nil {
			timer.Stop()
		}
	}

	return reset, stop
}

func (w *Watcher) waitForStop(ctx context.Context) error {
	select {
	case <-ctx.Done():
	case <-w.stopCh:
	}
	return nil
}

// reload performs a full config reload and notifies subscribers.
func (w *Watcher) reload(ctx context.Context) {
	logDebug(w.opts, "configx watcher reload started")
	newCfg, err := loadConfigFromOptions(ctx, w.opts)
	if err != nil {
		wrapped := oops.In("configx").
			With("op", "watcher_reload", "provider_count", len(w.providers)).
			Wrapf(err, "reload config")
		logError(w.opts, "configx watcher reload failed", "error", wrapped)
		w.handleErr(wrapped)
		w.notify(nil, wrapped)
		return
	}

	w.cfg.Store(newCfg)
	logDebug(w.opts, "configx watcher reload completed")
	w.notify(newCfg, nil)
}

// notify calls every registered ChangeHandler in order.
func (w *Watcher) notify(cfg *Config, err error) {
	logDebug(w.opts, "configx watcher notifying subscribers", "subscribers", len(w.loadSubscribers()), "has_error", err != nil)
	lo.ForEach(w.loadSubscribers(), func(fn ChangeHandler, _ int) {
		fn(cfg, err)
	})
}

// handleErr forwards err to the watchErrHandler when one is configured.
func (w *Watcher) handleErr(err error) {
	if err == nil || w.opts.watchErrHandler == nil {
		return
	}
	w.opts.watchErrHandler(err)
}

func (w *Watcher) loadSubscribers() []ChangeHandler {
	subs := w.subs.Load()
	if subs == nil {
		return nil
	}
	return *subs
}

// buildWatchProviders creates one *file.File provider per supported config
// file path. These providers are used exclusively for change detection;
// loadConfigFromOptions handles the actual reading and parsing.
func buildWatchProviders(paths []string) []*file.File {
	return lo.FilterMap(paths, func(path string, _ int) (*file.File, bool) {
		switch filepath.Ext(path) {
		case ".yaml", ".yml", ".json", ".toml":
			return file.Provider(path), true
		default:
			return nil, false
		}
	})
}

func (w *Watcher) providerPath(index int) string {
	if w == nil || w.opts == nil || index < 0 {
		return ""
	}

	current := 0
	for _, path := range w.opts.files {
		switch filepath.Ext(path) {
		case ".yaml", ".yml", ".json", ".toml":
			if current == index {
				return path
			}
			current++
		}
	}

	return ""
}
