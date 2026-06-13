package configx_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const testConfigFileExtension = ".kv"

type kvFileParser struct{}

func (kvFileParser) Unmarshal(raw []byte) (map[string]any, error) {
	data, err := parseKeyValueConfig(raw)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (kvFileParser) Marshal(_ map[string]any) ([]byte, error) {
	return []byte("{}"), nil
}

func parseKeyValueConfig(raw []byte) (map[string]any, error) {
	lines := strings.Split(string(raw), "\n")
	out := make(map[string]any, len(lines))

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		index := strings.Index(trimmed, ":")
		if index <= 0 {
			return nil, fmt.Errorf("invalid key-value line at %d: %q", i+1, line)
		}

		key := strings.TrimSpace(trimmed[:index])
		if key == "" {
			return nil, fmt.Errorf("missing key at line %d", i+1)
		}

		rawValue := strings.TrimSpace(trimmed[index+1:])
		value := parseKeyValueScalar(rawValue)
		out[key] = value
	}

	return out, nil
}

func parseKeyValueScalar(value string) any {
	if value == "" {
		return ""
	}

	if boolValue, err := strconv.ParseBool(value); err == nil {
		return boolValue
	}

	if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
		return int(intValue)
	}

	return value
}

func writeConfigFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func tempConfigFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config"+testConfigFileExtension)
	writeConfigFile(t, path, content)
	return path
}
