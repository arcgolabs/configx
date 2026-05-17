package configx

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/arcgolabs/observabilityx"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
	"github.com/samber/oops"
)

func loadCustomSources(
	ctx context.Context,
	obs observabilityx.Observability,
	k *koanf.Koanf,
	opts *Options,
) error {
	for index, source := range opts.customSources {
		if source == nil {
			continue
		}

		name := customSourceName(source.Name(), index)
		logDebug(opts, "configx custom source loading", "custom_source", name)
		if err := loadNamedSourceWithObservability(ctx, obs, customSourceMetricName(name), func() error {
			return loadCustomSource(ctx, k, source, name)
		}); err != nil {
			return err
		}
		logDebug(opts, "configx custom source loaded", "custom_source", name)
	}
	return nil
}

func loadCustomSource(ctx context.Context, k *koanf.Koanf, source ConfigSource, name string) error {
	if err := ctx.Err(); err != nil {
		return oops.In("configx").
			With("op", "load_custom_source", "source", name).
			Wrapf(errors.Join(ErrSource, err), "custom source context")
	}

	values, err := source.Load(ctx)
	if err != nil {
		return oops.In("configx").
			With("op", "load_custom_source", "source", name).
			Wrapf(errors.Join(ErrSource, err), "load custom source")
	}
	if len(values) == 0 {
		return nil
	}

	if err := k.Load(confmap.Provider(values, "."), nil); err != nil {
		return oops.In("configx").
			With("op", "merge_custom_source", "source", name, "key_count", len(values)).
			Wrapf(errors.Join(ErrSource, err), "merge custom source")
	}
	return nil
}

func customSourceName(name string, index int) string {
	clean := strings.TrimSpace(name)
	if clean != "" {
		return clean
	}
	return fmt.Sprintf("custom[%d]", index)
}

func customSourceMetricName(name string) string {
	return "custom." + name
}
