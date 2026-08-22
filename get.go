package configx

import (
	"errors"
	"fmt"

	"github.com/samber/oops"
)

var errNilConfig = errors.New("config is nil")

// GetAs unmarshals the value at path into T.
func (cfg *Config) GetAs[T any](path string) (T, error) {
	var zero T
	if cfg == nil {
		return zero, oops.In("configx").
			With("op", "get_as", "path", path, "target_type", fmt.Sprintf("%T", zero)).
			Wrapf(errNilConfig, "validate config")
	}

	var out T
	if err := cfg.Unmarshal(path, &out); err != nil {
		return zero, oops.In("configx").
			With("op", "get_as", "path", path, "target_type", fmt.Sprintf("%T", out)).
			Wrapf(err, "unmarshal config value")
	}
	return out, nil
}

// GetAsOr returns fallback when path is absent or cannot be decoded as T.
func (cfg *Config) GetAsOr[T any](path string, fallback T) T {
	if cfg == nil {
		return fallback
	}
	if path != "" && !cfg.Exists(path) {
		return fallback
	}

	out, err := cfg.GetAs[T](path)
	if err != nil {
		return fallback
	}
	return out
}

// MustGetAs unmarshals the value at path into T and panics on failure.
func (cfg *Config) MustGetAs[T any](path string) T {
	out, err := cfg.GetAs[T](path)
	if err != nil {
		panic(oops.In("configx").
			With("op", "must_get_as", "path", path).
			Wrapf(err, "get config value"))
	}
	return out
}

// GetAs is retained as a compatibility wrapper. Prefer [Config.GetAs].
func GetAs[T any](cfg *Config, path string) (T, error) {
	return cfg.GetAs[T](path)
}

// GetAsOr is retained as a compatibility wrapper. Prefer [Config.GetAsOr].
func GetAsOr[T any](cfg *Config, path string, fallback T) T {
	return cfg.GetAsOr(path, fallback)
}

// MustGetAs is retained as a compatibility wrapper. Prefer [Config.MustGetAs].
func MustGetAs[T any](cfg *Config, path string) T {
	return cfg.MustGetAs[T](path)
}
