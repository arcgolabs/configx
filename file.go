package configx

import (
	"errors"
	"path/filepath"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/samber/lo"
	"github.com/samber/oops"
)

// ErrUnsupportedFileFormat is returned when a config file has an extension
// that configx does not know how to parse. Supported extensions are:
// .yaml, .yml, .json, .toml.
var ErrUnsupportedFileFormat = errors.New("configx: unsupported config file format")

// supportedExtensions lists every extension that loadFiles can parse.
var supportedExtensions = []string{".yaml", ".yml", ".json", ".toml"}

// parserFor returns the koanf.Parser for the given file extension, or nil if
// the extension is not supported. Callers must check for nil before use.
func parserFor(ext string) koanf.Parser {
	switch ext {
	case ".yaml", ".yml":
		return yaml.Parser()
	case ".json":
		return json.Parser()
	case ".toml":
		return toml.Parser()
	default:
		return nil
	}
}

// loadFiles loads each file in order into k, merging on top of any previously
// loaded values. Later files take precedence over earlier ones.
//
// Returns [ErrUnsupportedFileFormat] (wrapped) if any file has an extension
// that is not in [supportedExtensions]. Use errors.Is to detect it.
func loadFiles(k *koanf.Koanf, files []string) error {
	if err := lo.Reduce(files, func(result error, path string, _ int) error {
		if result != nil {
			return result
		}

		ext := filepath.Ext(path)
		parser := parserFor(ext)
		if parser == nil {
			return oops.In("configx").
				With("op", "load_file", "path", path, "extension", ext).
				Wrapf(ErrUnsupportedFileFormat, "%q (got %q, want one of %v)", path, ext, supportedExtensions)
		}

		if err := k.Load(file.Provider(path), parser); err != nil {
			return oops.In("configx").
				With("op", "load_file", "path", path, "extension", ext).
				Wrapf(err, "configx: load config file %q", path)
		}
		return nil
	}, error(nil)); err != nil {
		return oops.In("configx").
			With("op", "load_files", "file_count", len(files)).
			Wrapf(err, "configx: load files")
	}
	return nil
}
