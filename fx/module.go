package fx

import (
	"context"
	"log/slog"

	pkgfx "github.com/arcgolabs/pkg/fx"
	"go.uber.org/fx"

	"github.com/arcgolabs/configx"
	"github.com/samber/oops"
)

// ── plain Config module ───────────────────────────────────────────────────────

// ConfigParams defines the inbound dependencies for the plain Config provider.
// Options are collected from the fxx value-group "configx_options", which lets
// any module in the application contribute additional Option values without
// having to know about the others.
type ConfigParams struct {
	fx.In

	// Options contributed by any fxx.Provide / fxx.Supply that annotates its
	// output with `group:"configx_options"`.  The field is optional so that
	// the module still works when no options are provided at all.
	Options []configx.Option `group:"configx_options,soft"`
}

// ConfigResult carries the *configx.Config that is provided to the rest of the
// application's dependency graph.
type ConfigResult struct {
	fx.Out

	Config *configx.Config
}

// NewConfig is the fxx constructor for a plain (non-watching) *configx.Config.
// It is wired up automatically by [NewConfigxModule].
func NewConfig(params ConfigParams) (ConfigResult, error) {
	cfg, err := configx.NewConfig(params.Options...)
	if err != nil {
		return ConfigResult{}, oops.In("configx/fx").
			With("op", "new_config", "option_count", len(params.Options)).
			Wrapf(err, "new config")
	}
	return ConfigResult{Config: cfg}, nil
}

// NewConfigxModule creates an fxx.Module named "configx" that provides a
// *configx.Config built from the supplied options.
//
// Any other module can contribute additional options through the fxx value-group
// "configx_options":
//
//	fxx.Provide(
//	    fxx.Annotate(
//	        func() configx.Option { return configx.WithEnvPrefix("APP") },
//	        fxx.ResultTags(`group:"configx_options"`),
//	    ),
//	)
func NewConfigxModule(opts ...configx.Option) fx.Option {
	return fx.Module("configx",
		// Seed the group with the options passed directly to this constructor.
		pkgfx.ProvideOptionGroup[configx.Options, configx.Option]("configx_options", opts...),
		fx.Provide(NewConfig),
	)
}

// NewConfigxModuleWithFiles is a convenience wrapper around [NewConfigxModule]
// that registers one or more config files as the sole option.
func NewConfigxModuleWithFiles(files ...string) fx.Option {
	return NewConfigxModule(configx.WithFiles(files...))
}

// NewConfigxModuleWithEnv is a convenience wrapper around [NewConfigxModule]
// that sets an environment-variable prefix as the sole option.
func NewConfigxModuleWithEnv(prefix string) fx.Option {
	return NewConfigxModule(configx.WithEnvPrefix(prefix))
}

// NewConfigxModuleWithDotenv is a convenience wrapper around [NewConfigxModule]
// that registers dotenv files as the sole option.
func NewConfigxModuleWithDotenv(files ...string) fx.Option {
	return NewConfigxModule(configx.WithDotenv(files...))
}

// ── watching / hot-reload module ──────────────────────────────────────────────

// WatcherParams mirrors ConfigParams but is used by the Watcher constructor so
// that the two can coexist in the same application without colliding.
type WatcherParams struct {
	fx.In

	Options []configx.Option `group:"configx_options,soft"`
}

// WatcherResult carries both the *configx.Watcher and a *configx.Config
// snapshot taken at application start.
//
// Services that only need config values available at startup should inject
// *configx.Config; services that must react to live changes should inject
// *configx.Watcher and call w.Config() to obtain the latest snapshot.
type WatcherResult struct {
	fx.Out

	Watcher *configx.Watcher
	Config  *configx.Config
}

// NewFxWatcher is the fxx constructor for a lifecycle-managed *configx.Watcher.
// It integrates with the fxx lifecycle so that:
//
//   - OnStart: the Watcher's watch loop is launched in a background goroutine.
//   - OnStop:  the context is canceled and Close is called, stopping all
//     fsnotify goroutines cleanly.
//
// It is wired up automatically by [NewConfigxWatcherModule].
func NewFxWatcher(lc fx.Lifecycle, params WatcherParams) (WatcherResult, error) {
	w, err := configx.NewWatcher(params.Options...)
	if err != nil {
		return WatcherResult{}, oops.In("configx/fx").
			With("op", "new_watcher", "option_count", len(params.Options)).
			Wrapf(err, "new watcher")
	}

	// A dedicated context lets OnStop cancel the watch loop independently of
	// the application's root context.
	ctx, cancel := context.WithCancel(context.Background())

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go func() {
				// Start blocks until ctx is canceled or w.Close() is called.
				// Errors are forwarded to the handler registered with
				// configx.WithWatchErrHandler; we do not surface them here
				// because they would crash the fxx app after startup.
				if err := w.Start(ctx); err != nil {
					slog.Error("configx watcher stopped with error", "error", err)
				}
			}()
			return nil
		},
		OnStop: func(_ context.Context) error {
			cancel()
			return w.Close()
		},
	})

	return WatcherResult{
		Watcher: w,
		// Snapshot taken at DI time – reflects the initial config load.
		// Call w.Config() at any later point to get the current values.
		Config: w.Config(),
	}, nil
}

// NewConfigxWatcherModule creates an fxx.Module named "configx" that provides
// both a *configx.Watcher (lifecycle-managed) and an initial *configx.Config
// snapshot.  Hot-reload via fsnotify is active for the lifetime of the fxx app.
//
// Like [NewConfigxModule], additional options can be contributed through the
// "configx_options" value-group.
func NewConfigxWatcherModule(opts ...configx.Option) fx.Option {
	return fx.Module("configx",
		pkgfx.ProvideOptionGroup[configx.Options, configx.Option]("configx_options", opts...),
		fx.Provide(NewFxWatcher),
	)
}
