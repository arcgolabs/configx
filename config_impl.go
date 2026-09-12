package configx

import (
	"errors"
	"maps"
	"slices"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/knadh/koanf/v2"
	"github.com/samber/oops"
)

// Config documents related behavior.
type Config struct {
	k        *koanf.Koanf
	validate *validator.Validate
	level    ValidateLevel
}

func newConfig(k *koanf.Koanf, opts *Options) *Config {
	v := opts.validate
	if v == nil {
		v = validator.New()
	}
	return &Config{
		k:        k,
		validate: v,
		level:    opts.validateLevel,
	}
}

// validateStruct documents related behavior.
func (cfg *Config) validateStruct(out any) error {
	if cfg == nil {
		return oops.In("configx").
			With("op", "validate_struct").
			Wrapf(errNilConfig, "validate config")
	}
	switch cfg.level {
	case ValidateLevelNone:
		return nil
	case ValidateLevelStruct:
		if err := cfg.validate.Struct(out); err != nil {
			return oops.In("configx").
				With("op", "validate_struct", "level", cfg.level).
				Wrapf(err, "validate struct")
		}
		return nil
	default:
		return nil
	}
}

// Get retrieves related data.
func (cfg *Config) Get(path string) any {
	return cfg.k.Get(path)
}

// GetString retrieves related data.
func (cfg *Config) GetString(path string) string {
	return cfg.k.String(path)
}

// GetInt retrieves related data.
func (cfg *Config) GetInt(path string) int {
	return cfg.k.Int(path)
}

// GetInt64 retrieves related data.
func (cfg *Config) GetInt64(path string) int64 {
	return cfg.k.Int64(path)
}

// GetFloat64 retrieves related data.
func (cfg *Config) GetFloat64(path string) float64 {
	return cfg.k.Float64(path)
}

// GetBool retrieves related data.
func (cfg *Config) GetBool(path string) bool {
	return cfg.k.Bool(path)
}

// GetDuration retrieves related data.
func (cfg *Config) GetDuration(path string) time.Duration {
	return cfg.k.Duration(path)
}

// GetStringSlice retrieves related data.
func (cfg *Config) GetStringSlice(path string) []string {
	return slices.Clone(cfg.k.Strings(path))
}

// GetIntSlice retrieves related data.
func (cfg *Config) GetIntSlice(path string) []int {
	return slices.Clone(cfg.k.Ints(path))
}

// Unmarshal documents related behavior.
// path documents related behavior.
func (cfg *Config) Unmarshal(path string, out any) error {
	if cfg == nil {
		return oops.In("configx").
			With("op", "unmarshal", "path", path).
			Wrapf(errNilConfig, "validate config")
	}
	if err := cfg.k.Unmarshal(path, out); err != nil {
		return oops.In("configx").
			With("op", "unmarshal", "path", path).
			Wrapf(errors.Join(ErrUnmarshal, err), "unmarshal config path")
	}
	return nil
}

// UnmarshalWithValidate documents related behavior.
// path documents related behavior.
func (cfg *Config) UnmarshalWithValidate(path string, out any) error {
	if cfg == nil {
		return oops.In("configx").
			With("op", "unmarshal_with_validate", "path", path).
			Wrapf(errNilConfig, "validate config")
	}
	if err := cfg.k.Unmarshal(path, out); err != nil {
		return oops.In("configx").
			With("op", "unmarshal_with_validate", "path", path).
			Wrapf(errors.Join(ErrUnmarshal, err), "unmarshal config path")
	}
	if err := cfg.validateStruct(out); err != nil {
		return oops.In("configx").
			With("op", "unmarshal_with_validate", "path", path).
			Wrapf(errors.Join(ErrValidate, err), "validate config path")
	}
	return nil
}

// Exists checks related state.
func (cfg *Config) Exists(path string) bool {
	return cfg.k.Exists(path)
}

// All retrieves related data.
func (cfg *Config) All() map[string]any {
	return maps.Clone(cfg.k.All())
}

// Validate documents related behavior.
func (cfg *Config) Validate(out any) error {
	if cfg == nil {
		return oops.In("configx").
			With("op", "validate").
			Wrapf(errNilConfig, "validate config")
	}
	if err := cfg.validateStruct(out); err != nil {
		return oops.In("configx").
			With("op", "validate").
			Wrapf(errors.Join(ErrValidate, err), "validate config value")
	}
	return nil
}

// ConfigSnapshot provides a deterministic, inspectable view of loaded values.
type ConfigSnapshot struct {
	Values map[string]any
	Keys   []string
}

// Snapshot returns a copy-like diagnostic view of config values and sorted keys.
func (cfg *Config) Snapshot() ConfigSnapshot {
	values := cfg.All()
	keys := slices.Sorted(maps.Keys(values))
	return ConfigSnapshot{
		Values: values,
		Keys:   keys,
	}
}
