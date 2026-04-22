package configx

import (
	"errors"
	"fmt"

	"github.com/samber/oops"
)

var errNilConfig = errors.New("config is nil")

// GetAs converts related values.
func GetAs[T any](cfg *Config, path string) (T, error) {
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

// GetAsOr returns related data.
func GetAsOr[T any](cfg *Config, path string, fallback T) T {
	if cfg == nil {
		return fallback
	}
	if path != "" && !cfg.Exists(path) {
		return fallback
	}

	out, err := GetAs[T](cfg, path)
	if err != nil {
		return fallback
	}
	return out
}

// MustGetAs converts related values.
func MustGetAs[T any](cfg *Config, path string) T {
	out, err := GetAs[T](cfg, path)
	if err != nil {
		panic(oops.In("configx").
			With("op", "must_get_as", "path", path).
			Wrapf(err, "get config value"))
	}
	return out
}
