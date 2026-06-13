package configx

import (
	"context"
	"errors"
	"time"

	"github.com/arcgolabs/observabilityx"
	"github.com/knadh/koanf/v2"
	"github.com/samber/oops"
)

func loadConfiguredSource(
	ctx context.Context,
	obs observabilityx.Observability,
	k *koanf.Koanf,
	opts *Options,
	src Source,
) error {
	switch src {
	case SourceDotenv:
		return loadConfiguredDotenvSource(ctx, obs, opts, src)
	case SourceFile:
		return loadConfiguredFileSource(ctx, obs, k, opts, src)
	case SourceEnv:
		return loadConfiguredEnvSource(ctx, obs, k, opts, src)
	case SourceCustom:
		return loadConfiguredCustomSource(ctx, obs, k, opts, src)
	case SourceArgs:
		return loadConfiguredArgsSource(ctx, obs, k, opts, src)
	}

	return nil
}

func loadConfiguredDotenvSource(
	ctx context.Context,
	obs observabilityx.Observability,
	opts *Options,
	src Source,
) error {
	logDebug(opts, "configx source loading", "source", src.String())
	if err := loadSourceWithObservability(ctx, obs, src, func() error {
		return loadDotenv(opts.dotenvFiles, opts.ignoreDotenvErr)
	}); err != nil {
		return oops.In("configx").
			With("op", "load_source", "source", src.String()).
			Wrapf(errors.Join(ErrLoad, err), "dotenv source")
	}
	logDebug(opts, "configx source loaded", "source", src.String())
	return nil
}

func loadConfiguredFileSource(
	ctx context.Context,
	obs observabilityx.Observability,
	k *koanf.Koanf,
	opts *Options,
	src Source,
) error {
	logDebug(opts, "configx source loading", "source", src.String(), "files", len(opts.files))
	if err := loadSourceWithObservability(ctx, obs, src, func() error {
		return loadFiles(k, opts.files, opts.fileParsers)
	}); err != nil {
		return oops.In("configx").
			With("op", "load_source", "source", src.String(), "file_count", len(opts.files)).
			Wrapf(errors.Join(ErrLoad, err), "file source")
	}
	logDebug(opts, "configx source loaded", "source", src.String())
	return nil
}

func loadConfiguredEnvSource(
	ctx context.Context,
	obs observabilityx.Observability,
	k *koanf.Koanf,
	opts *Options,
	src Source,
) error {
	logDebug(opts, "configx source loading", "source", src.String(), "env_prefix", opts.envPrefix)
	if err := loadSourceWithObservability(ctx, obs, src, func() error {
		return loadEnv(k, opts.envPrefix, opts.envSeparator)
	}); err != nil {
		return oops.In("configx").
			With("op", "load_source", "source", src.String(), "env_prefix", opts.envPrefix).
			Wrapf(errors.Join(ErrLoad, err), "env source")
	}
	logDebug(opts, "configx source loaded", "source", src.String())
	return nil
}

func loadConfiguredCustomSource(
	ctx context.Context,
	obs observabilityx.Observability,
	k *koanf.Koanf,
	opts *Options,
	src Source,
) error {
	logDebug(opts, "configx source loading", "source", src.String(), "custom_sources", len(opts.customSources))
	if err := loadCustomSources(ctx, obs, k, opts); err != nil {
		return oops.In("configx").
			With("op", "load_source", "source", src.String(), "custom_source_count", len(opts.customSources)).
			Wrapf(errors.Join(ErrLoad, err), "custom source")
	}
	logDebug(opts, "configx source loaded", "source", src.String())
	return nil
}

func loadConfiguredArgsSource(
	ctx context.Context,
	obs observabilityx.Observability,
	k *koanf.Koanf,
	opts *Options,
	src Source,
) error {
	logDebug(opts,
		"configx source loading",
		"source", src.String(),
		"raw_args", len(opts.args),
		"changed_flags", changedFlagCount(opts.argsFlagSet),
	)
	if err := loadSourceWithObservability(ctx, obs, src, func() error {
		return loadArgs(k, opts.args, opts.argsFlagSet, opts.argsNameFunc)
	}); err != nil {
		return oops.In("configx").
			With("op", "load_source", "source", src.String(), "arg_count", len(opts.args)).
			Wrapf(errors.Join(ErrLoad, err), "args source")
	}
	logDebug(opts, "configx source loaded", "source", src.String())
	return nil
}

// loadSourceWithObservability wraps fn with a child span and per-source metrics
// so that every load operation is independently observable.
func loadSourceWithObservability(
	ctx context.Context,
	obs observabilityx.Observability,
	source Source,
	fn func() error,
) error {
	return loadNamedSourceWithObservability(ctx, obs, source.String(), fn)
}

func loadNamedSourceWithObservability(
	ctx context.Context,
	obs observabilityx.Observability,
	sourceName string,
	fn func() error,
) error {
	if fn == nil {
		return nil
	}

	sourceCtx, sourceSpan := obs.StartSpan(ctx, "configx.load."+sourceName,
		observabilityx.String("source", sourceName),
	)
	defer sourceSpan.End()

	start := time.Now()
	result := "success"
	defer func() {
		obs.Counter(configSourceLoadTotalSpec).Add(sourceCtx, 1,
			observabilityx.String("source", sourceName),
			observabilityx.String("result", result),
		)
		obs.Histogram(configSourceLoadDurationSpec).Record(sourceCtx, float64(time.Since(start).Milliseconds()),
			observabilityx.String("source", sourceName),
			observabilityx.String("result", result),
		)
	}()

	if err := fn(); err != nil {
		result = "error"
		sourceSpan.RecordError(err)
		return err
	}

	return nil
}
