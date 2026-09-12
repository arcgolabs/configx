package configx_test

import (
	"encoding"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	configx "github.com/arcgolabs/configx"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type schemaMode string

var _ encoding.TextUnmarshaler = (*schemaMode)(nil)

func (m *schemaMode) UnmarshalText(text []byte) error {
	value := strings.ToLower(string(text))
	if value != "safe" && value != "fast" {
		return errors.New("unsupported mode")
	}
	*m = schemaMode(value)
	return nil
}

type EmbeddedSchemaConfig struct {
	Region string `configx:"usage=deployment region" koanf:"region"`
}

type schemaServerConfig struct {
	Host string `configx:"usage=server host"                  koanf:"host"`
	Port int    `configx:"flag=listen-port,usage=listen port" koanf:"port"`
}

type schemaTestConfig struct {
	EmbeddedSchemaConfig
	Name     string             `configx:"usage=service name"  koanf:"name"`
	Debug    bool               `koanf:"debug"`
	Timeout  time.Duration      `koanf:"timeout"`
	IDs      []int              `koanf:"ids"`
	Mode     schemaMode         `koanf:"mode"`
	Server   schemaServerConfig `koanf:"server"`
	Secret   string             `configx:"redact"              koanf:"secret"`
	Hidden   string             `configx:"hidden"              koanf:"hidden"`
	Legacy   string             `configx:"deprecated=use name" koanf:"legacy"`
	Internal string             `configx:"nocli"               koanf:"internal"`
	Ignored  string             `koanf:"-"`
}

func schemaDefaults() schemaTestConfig {
	defaults := schemaTestConfig{
		Region:  "cn-east",
		Name:    "configx",
		Timeout: 2 * time.Second,
		IDs:     []int{1, 2},
		Mode:    "safe",
		Server: schemaServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Secret:   "do-not-expose",
		Hidden:   "hidden-default",
		Legacy:   "legacy-default",
		Internal: "internal-default",
		Ignored:  "ignored-default",
	}
	return defaults
}

func TestSchemaOf_DerivesNestedFieldsAndMetadata(t *testing.T) {
	schema, err := configx.SchemaOf(schemaDefaults())
	require.NoError(t, err)

	fields := schema.Fields()
	require.Len(t, fields, 12)
	assert.Equal(t, []string{
		"region",
		"name",
		"debug",
		"timeout",
		"ids",
		"mode",
		"server.host",
		"server.port",
		"secret",
		"hidden",
		"legacy",
		"internal",
	}, schemaFieldPaths(fields))

	port := schemaFieldByPath(t, fields, "server.port")
	assert.Equal(t, "listen-port", port.FlagName)
	assert.Equal(t, "listen port", port.Usage)
	assert.Equal(t, configx.SchemaKindInt, port.Kind)
	assert.Equal(t, "8080", port.Default)

	secret := schemaFieldByPath(t, fields, "secret")
	assert.True(t, secret.Redacted)
	assert.Equal(t, "[REDACTED]", secret.Default)
	assert.True(t, secret.CLIEnabled)
	assert.NotContains(t, fmt.Sprintf("%#v", schema), "do-not-expose")

	assert.True(t, schemaFieldByPath(t, fields, "hidden").Hidden)
	assert.Equal(t, "use name", schemaFieldByPath(t, fields, "legacy").Deprecated)
	assert.False(t, schemaFieldByPath(t, fields, "internal").CLIEnabled)

	fields[0].Path = "mutated"
	assert.Equal(t, "region", schema.Fields()[0].Path)
}

func TestSchemaOf_CustomTagAndFieldOverride(t *testing.T) {
	type taggedConfig struct {
		Port int `cfg:"listen"`
	}

	schema, err := configx.SchemaOf(
		taggedConfig{Port: 80},
		configx.WithSchemaTag("cfg"),
		configx.WithSchemaField(
			"listen",
			configx.WithSchemaFieldFlag("port"),
			configx.WithSchemaFieldUsage("HTTP port"),
			configx.WithSchemaFieldParser(func(raw string) (any, error) {
				value, err := strconv.Atoi(raw)
				if err != nil {
					return nil, fmt.Errorf("parse custom port: %w", err)
				}
				return value * 10, nil
			}),
		),
	)
	require.NoError(t, err)

	field := schemaFieldByPath(t, schema.Fields(), "listen")
	assert.Equal(t, "port", field.FlagName)
	assert.Equal(t, "HTTP port", field.Usage)

	fs := pflag.NewFlagSet("custom", pflag.ContinueOnError)
	require.NoError(t, schema.BindFlags(fs))
	require.NoError(t, fs.Parse([]string{"--port=9"}))

	loaded, err := configx.Load[taggedConfig](configx.WithFlagSet(fs))
	require.NoError(t, err)
	assert.Equal(t, 90, loaded.Port)
}

func TestSchemaBindFlags_ParsesSupportedValuesAndPreservesPaths(t *testing.T) {
	schema, err := configx.SchemaOf(schemaDefaults())
	require.NoError(t, err)

	fs := pflag.NewFlagSet("schema", pflag.ContinueOnError)
	require.NoError(t, schema.BindFlags(fs, configx.WithFlagNameSeparator("_")))
	require.NoError(t, fs.Parse([]string{
		"--name=updated",
		"--debug",
		"--timeout=3s",
		"--ids=3,4",
		"--mode=FAST",
		"--server_host=localhost",
		"--listen-port=9090",
		"--secret=changed-secret",
	}))

	loaded, err := configx.Load[schemaTestConfig](
		configx.WithTypedDefaults(schemaDefaults()),
		configx.WithFlagSet(fs),
	)
	require.NoError(t, err)
	assert.Equal(t, "updated", loaded.Name)
	assert.True(t, loaded.Debug)
	assert.Equal(t, 3*time.Second, loaded.Timeout)
	assert.Equal(t, []int{3, 4}, loaded.IDs)
	assert.Equal(t, schemaMode("fast"), loaded.Mode)
	assert.Equal(t, "localhost", loaded.Server.Host)
	assert.Equal(t, 9090, loaded.Server.Port)
	assert.Equal(t, "changed-secret", loaded.Secret)
	assert.Equal(t, "internal-default", loaded.Internal)

	assert.Nil(t, fs.Lookup("internal"))
	assert.True(t, fs.Lookup("hidden").Hidden)
	assert.Equal(t, "use name", fs.Lookup("legacy").Deprecated)
	assert.Equal(t, "[REDACTED]", fs.Lookup("secret").DefValue)
}

func TestSchemaBindFlags_DefaultsDoNotOverrideOtherSources(t *testing.T) {
	schema, err := configx.SchemaOf(schemaDefaults())
	require.NoError(t, err)

	fs := pflag.NewFlagSet("schema", pflag.ContinueOnError)
	require.NoError(t, schema.BindFlags(fs))

	loaded, err := configx.Load[schemaTestConfig](
		configx.WithDefaults(map[string]any{"name": "from-source"}),
		configx.WithFlagSet(fs),
	)
	require.NoError(t, err)
	assert.Equal(t, "from-source", loaded.Name)
}

func TestSchemaBindFlags_RejectsConflictsAndInvalidInput(t *testing.T) {
	_, err := configx.SchemaOf(42)
	require.Error(t, err)
	assert.ErrorIs(t, err, configx.ErrSchema)

	_, err = configx.SchemaOf(
		schemaDefaults(),
		configx.WithSchemaField("missing", configx.WithSchemaFieldUsage("missing")),
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, configx.ErrSchema)

	_, err = configx.SchemaOf(
		schemaDefaults(),
		configx.WithSchemaField("  ", configx.WithSchemaFieldRedacted(true)),
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, configx.ErrSchema)

	schema, err := configx.SchemaOf(schemaDefaults())
	require.NoError(t, err)
	fs := pflag.NewFlagSet("schema", pflag.ContinueOnError)
	fs.String("name", "", "existing")
	err = schema.BindFlags(fs)
	require.Error(t, err)
	assert.ErrorIs(t, err, configx.ErrSchemaFlagConflict)
}

func schemaFieldPaths(fields []configx.SchemaField) []string {
	paths := make([]string, 0, len(fields))
	for index := range fields {
		paths = append(paths, fields[index].Path)
	}
	return paths
}

func schemaFieldByPath(t *testing.T, fields []configx.SchemaField, path string) configx.SchemaField {
	t.Helper()
	for index := range fields {
		if fields[index].Path == path {
			return fields[index]
		}
	}
	require.FailNow(t, "schema field not found", path)
	return configx.SchemaField{}
}
