package configx

import (
	"errors"
	"time"

	"github.com/arcgolabs/collectionx"
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
func (c *Config) validateStruct(out any) error {
	if c == nil {
		return oops.In("configx").
			With("op", "validate_struct").
			Wrapf(errNilConfig, "validate config")
	}
	switch c.level {
	case ValidateLevelNone:
		return nil
	case ValidateLevelStruct:
		if err := c.validate.Struct(out); err != nil {
			return oops.In("configx").
				With("op", "validate_struct", "level", c.level).
				Wrapf(err, "validate struct")
		}
		return nil
	default:
		return nil
	}
}

// Get retrieves related data.
func (c *Config) Get(path string) any {
	return c.k.Get(path)
}

// GetString retrieves related data.
func (c *Config) GetString(path string) string {
	return c.k.String(path)
}

// GetInt retrieves related data.
func (c *Config) GetInt(path string) int {
	return c.k.Int(path)
}

// GetInt64 retrieves related data.
func (c *Config) GetInt64(path string) int64 {
	return c.k.Int64(path)
}

// GetFloat64 retrieves related data.
func (c *Config) GetFloat64(path string) float64 {
	return c.k.Float64(path)
}

// GetBool retrieves related data.
func (c *Config) GetBool(path string) bool {
	return c.k.Bool(path)
}

// GetDuration retrieves related data.
func (c *Config) GetDuration(path string) time.Duration {
	return c.k.Duration(path)
}

// GetStringSlice retrieves related data.
func (c *Config) GetStringSlice(path string) collectionx.List[string] {
	items := c.k.Strings(path)
	return collectionx.NewListWithCapacity(len(items), items...)
}

// GetIntSlice retrieves related data.
func (c *Config) GetIntSlice(path string) collectionx.List[int] {
	items := c.k.Ints(path)
	return collectionx.NewListWithCapacity(len(items), items...)
}

// Unmarshal documents related behavior.
// path documents related behavior.
func (c *Config) Unmarshal(path string, out any) error {
	if c == nil {
		return oops.In("configx").
			With("op", "unmarshal", "path", path).
			Wrapf(errNilConfig, "validate config")
	}
	if err := c.k.Unmarshal(path, out); err != nil {
		return oops.In("configx").
			With("op", "unmarshal", "path", path).
			Wrapf(errors.Join(ErrUnmarshal, err), "unmarshal config path")
	}
	return nil
}

// UnmarshalWithValidate documents related behavior.
// path documents related behavior.
func (c *Config) UnmarshalWithValidate(path string, out any) error {
	if c == nil {
		return oops.In("configx").
			With("op", "unmarshal_with_validate", "path", path).
			Wrapf(errNilConfig, "validate config")
	}
	if err := c.k.Unmarshal(path, out); err != nil {
		return oops.In("configx").
			With("op", "unmarshal_with_validate", "path", path).
			Wrapf(errors.Join(ErrUnmarshal, err), "unmarshal config path")
	}
	if err := c.validateStruct(out); err != nil {
		return oops.In("configx").
			With("op", "unmarshal_with_validate", "path", path).
			Wrapf(errors.Join(ErrValidate, err), "validate config path")
	}
	return nil
}

// Exists checks related state.
func (c *Config) Exists(path string) bool {
	return c.k.Exists(path)
}

// All retrieves related data.
func (c *Config) All() collectionx.Map[string, any] {
	return collectionx.NewMapFrom(c.k.All())
}

// Validate documents related behavior.
func (c *Config) Validate(out any) error {
	if c == nil {
		return oops.In("configx").
			With("op", "validate").
			Wrapf(errNilConfig, "validate config")
	}
	if err := c.validateStruct(out); err != nil {
		return oops.In("configx").
			With("op", "validate").
			Wrapf(errors.Join(ErrValidate, err), "validate config value")
	}
	return nil
}

// ConfigSnapshot provides a deterministic, inspectable view of loaded values.
type ConfigSnapshot struct {
	Values collectionx.Map[string, any]
	Keys   collectionx.List[string]
}

// Snapshot returns a copy-like diagnostic view of config values and sorted keys.
func (c *Config) Snapshot() ConfigSnapshot {
	values := c.All()
	keys := collectionx.NewListWithCapacity[string](values.Len())
	values.Range(func(key string, _ any) bool {
		keys.Add(key)
		return true
	})
	keys.Sort(func(left, right string) int {
		switch {
		case left < right:
			return -1
		case left > right:
			return 1
		default:
			return 0
		}
	})
	return ConfigSnapshot{
		Values: values,
		Keys:   keys,
	}
}
