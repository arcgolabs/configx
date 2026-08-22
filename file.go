package configx

import (
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/samber/oops"
)

// parserFor returns the registered parser for a file path.
func parserFor(path string, parserRegistry map[string]koanf.Parser) koanf.Parser {
	ext := strings.ToLower(filepath.Ext(path))
	return parserRegistry[ext]
}

func wantExtensions(parserRegistry map[string]koanf.Parser) []string {
	extensions := supportedExtensions(parserRegistry)
	if len(extensions) == 0 {
		return nil
	}
	return extensions
}

// loadFiles loads each file in order into k, merging on top of any previously
// loaded values. Later files take precedence over earlier ones.
//
// Returns [ErrUnsupportedFileFormat] (wrapped) if any file has an extension
// with no registered parser.
func loadFiles(k *koanf.Koanf, files []string, parserRegistry map[string]koanf.Parser) error {
	for _, path := range files {
		ext := strings.ToLower(filepath.Ext(path))
		parser := parserFor(path, parserRegistry)
		if parser == nil {
			return oops.In("configx").
				With("op", "load_files", "file_count", len(files)).
				Wrapf(oops.In("configx").
					With("op", "load_file", "path", path, "extension", ext).
					Wrapf(ErrUnsupportedFileFormat, "%q (got %q, want one of %v)", path, ext, wantExtensions(parserRegistry)), "configx: load files")
		}

		if err := k.Load(file.Provider(path), parser); err != nil {
			return oops.In("configx").
				With("op", "load_files", "file_count", len(files)).
				Wrapf(oops.In("configx").
					With("op", "load_file", "path", path, "extension", ext).
					Wrapf(err, "configx: load config file %q", path), "configx: load files")
		}
	}
	return nil
}
