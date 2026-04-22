package configx

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
	envProvider "github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/v2"
	"github.com/samber/lo"
	"github.com/samber/oops"
)

// loadDotenv loads each dotenv file in order. If ignoreErr is true, missing
// files and parse errors are silently skipped; otherwise they are returned as
// errors.
func loadDotenv(files []string, ignoreErr bool) error {
	if err := lo.Reduce(files, func(result error, path string, _ int) error {
		if result != nil {
			return result
		}
		return loadDotenvFile(path, ignoreErr)
	}, error(nil)); err != nil {
		return oops.In("configx").
			With("op", "load_dotenv", "file_count", len(files), "ignore_error", ignoreErr).
			Wrapf(err, "load dotenv files")
	}
	return nil
}

func loadDotenvFile(path string, ignoreErr bool) error {
	if _, err := os.Stat(path); err != nil {
		if ignoreErr {
			return nil
		}
		if os.IsNotExist(err) {
			return oops.In("configx").
				With("op", "load_dotenv_file", "path", path, "ignore_error", ignoreErr).
				Wrapf(err, "dotenv file not found")
		}
		return oops.In("configx").
			With("op", "load_dotenv_file", "path", path, "ignore_error", ignoreErr).
			Wrapf(err, "stat dotenv file")
	}

	if err := godotenv.Load(path); err != nil {
		if ignoreErr {
			return nil
		}
		return oops.In("configx").
			With("op", "load_dotenv_file", "path", path, "ignore_error", ignoreErr).
			Wrapf(err, "load dotenv file")
	}

	return nil
}

// loadEnv loads environment variables into k. Only variables whose names begin
// with the given prefix (e.g. "APP_") are considered.
//
// The separator controls how the remainder of the key is translated into a
// koanf path:
//
//   - separator "_"  (default): every underscore becomes ".", so APP_DB_HOST
//     becomes the path "db.host".
//   - separator "__" (double-underscore convention): only double underscores
//     become ".", so APP_DB__HOST → "db.host" while APP_MAX_RETRY → "max_retry".
//
// Keys and values are always lowercased before insertion.
func loadEnv(k *koanf.Koanf, prefix, separator string) error {
	if separator == "" {
		separator = defaultEnvSeparator
	}

	normalizedPrefix := normalizeEnvPrefix(prefix)

	p := envProvider.Provider(".", envProvider.Opt{
		Prefix: normalizedPrefix,
		TransformFunc: func(k, v string) (string, any) {
			keyWithoutPrefix := strings.TrimPrefix(k, normalizedPrefix)
			keyWithoutPrefix = strings.TrimPrefix(keyWithoutPrefix, "_")

			// Replace the chosen separator with "." to form a koanf path.
			// The key is lowercased so that APP_DB_HOST and app_db_host are
			// treated identically.
			key := strings.ReplaceAll(
				strings.ToLower(keyWithoutPrefix),
				strings.ToLower(separator),
				".",
			)
			return key, v
		},
		EnvironFunc: os.Environ,
	})

	if err := k.Load(p, nil); err != nil {
		return oops.In("configx").
			With("op", "load_env", "prefix", normalizedPrefix, "separator", separator).
			Wrapf(err, "load environment variables")
	}
	return nil
}

// normalizeEnvPrefix ensures the prefix ends with exactly one trailing
// underscore. An empty prefix is returned as-is so that all env vars are
// considered.
func normalizeEnvPrefix(prefix string) string {
	clean := strings.TrimSpace(prefix)
	if clean == "" {
		return ""
	}
	return strings.TrimSuffix(clean, "_") + "_"
}
