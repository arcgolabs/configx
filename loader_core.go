package configx

import (
	"context"
	"errors"
	"time"

	"github.com/arcgolabs/observabilityx"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
	"github.com/samber/oops"
)

// loadConfigFromOptions is the single authoritative code path that builds a
// koanf instance from an *Options and wraps it in a *Config. All exported load
// functions ultimately call this.
func loadConfigFromOptions(ctx context.Context, opts *Options) (_ *Config, err error) {
	opts = normalizeLoadOptions(opts)
	logDebug(opts,
		"configx load started",
		"files", len(opts.files),
		"dotenv_files", len(opts.dotenvFiles),
		"custom_sources", len(opts.customSources),
		"priority", len(opts.priority),
		"env_prefix", opts.envPrefix,
		"raw_args", len(opts.args),
		"args_flags", changedFlagCount(opts.argsFlagSet),
	)

	ctx, obs, finish := beginConfigLoad(ctx, opts)
	defer func() {
		finish(err)
	}()

	k := koanf.New(".")
	if err := loadConfiguredDefaults(k, opts); err != nil {
		return nil, err
	}
	if err := loadConfiguredSources(ctx, obs, k, opts); err != nil {
		return nil, err
	}

	return newConfig(k, opts), nil
}

func normalizeLoadOptions(opts *Options) *Options {
	if opts == nil {
		return NewOptions()
	}
	return opts
}

func beginConfigLoad(
	ctx context.Context,
	opts *Options,
) (context.Context, observabilityx.Observability, func(error)) {
	if ctx == nil {
		ctx = context.Background()
	}

	obs := observabilityx.Normalize(opts.observability, nil)
	ctx, span := obs.StartSpan(ctx, "configx.load")
	start := time.Now()

	return ctx, obs, func(err error) {
		result := "success"
		if err != nil {
			result = "error"
			span.RecordError(err)
			logError(opts, "configx load failed", "error", err)
		} else {
			logDebug(opts, "configx load completed", "result", result)
		}

		obs.Counter(configLoadTotalSpec).Add(ctx, 1,
			observabilityx.String("result", result),
		)
		obs.Histogram(configLoadDurationSpec).Record(ctx, float64(time.Since(start).Milliseconds()),
			observabilityx.String("result", result),
		)
		span.End()
	}
}

func loadConfiguredDefaults(k *koanf.Koanf, opts *Options) error {
	if err := loadTypedDefaults(k, opts); err != nil {
		return err
	}

	if !opts.hasDefaults {
		return nil
	}

	if err := k.Load(confmap.Provider(opts.defaults, "."), nil); err != nil {
		return oops.In("configx").
			With("op", "load_defaults").
			Wrapf(errors.Join(ErrDefaults, err), "defaults map")
	}

	logDebug(opts, "configx defaults loaded")
	return nil
}

func loadTypedDefaults(k *koanf.Koanf, opts *Options) error {
	if opts.typedDefaultsErr != nil {
		return oops.In("configx").
			With("op", "load_typed_defaults").
			Wrapf(errors.Join(ErrDefaults, opts.typedDefaultsErr), "typed defaults")
	}
	if opts.typedDefaults == nil {
		return nil
	}
	if err := k.Load(confmap.Provider(opts.typedDefaults, "."), nil); err != nil {
		return oops.In("configx").
			With("op", "load_typed_defaults").
			Wrapf(errors.Join(ErrDefaults, err), "typed defaults map")
	}

	logDebug(opts, "configx typed defaults loaded")
	return nil
}

func loadConfiguredSources(
	ctx context.Context,
	obs observabilityx.Observability,
	k *koanf.Koanf,
	opts *Options,
) error {
	for _, src := range opts.priority {
		if err := loadConfiguredSource(ctx, obs, k, opts, src); err != nil {
			return oops.In("configx").
				With("op", "load_sources").
				Wrapf(err, "configx: load configured sources")
		}
	}
	return nil
}
