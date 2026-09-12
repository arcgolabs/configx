package configx_test

import (
	"encoding"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	configx "github.com/arcgolabs/configx"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type schemaValueConfig struct {
	Count     uint16          `koanf:"count"`
	Ratio     float32         `koanf:"ratio"`
	Toggles   []bool          `koanf:"toggles"`
	Durations []time.Duration `koanf:"durations"`
	Modes     []schemaMode    `koanf:"modes"`
	Optional  *int            `koanf:"optional"`
}

type schemaMarshalOnlyInt int

type recursiveSchemaConfig struct {
	Next *recursiveSchemaConfig `koanf:"next"`
}

func (value schemaMarshalOnlyInt) MarshalText() ([]byte, error) {
	return []byte(strconv.FormatInt(int64(value), 16)), nil
}

func TestSchemaBindFlags_SupportsNumericSlicesTextSlicesAndPointers(t *testing.T) {
	schema, err := configx.SchemaOf(schemaValueConfig{})
	require.NoError(t, err)

	fs := pflag.NewFlagSet("values", pflag.ContinueOnError)
	require.NoError(t, schema.BindFlags(fs))
	require.NoError(t, fs.Parse([]string{
		"--count=16",
		"--ratio=1.5",
		"--toggles=true,false",
		"--toggles=true",
		"--durations=1s,2m",
		"--modes=safe,FAST",
		"--optional=7",
	}))

	loaded, err := configx.Load[schemaValueConfig](configx.WithFlagSet(fs))
	require.NoError(t, err)
	assert.Equal(t, uint16(16), loaded.Count)
	assert.Equal(t, float32(1.5), loaded.Ratio)
	assert.Equal(t, []bool{true, false, true}, loaded.Toggles)
	assert.Equal(t, []time.Duration{time.Second, 2 * time.Minute}, loaded.Durations)
	assert.Equal(t, []schemaMode{"safe", "fast"}, loaded.Modes)
	require.NotNil(t, loaded.Optional)
	assert.Equal(t, 7, *loaded.Optional)
}

func TestSchemaBindFlags_TextValuesUseRoundTripDefaults(t *testing.T) {
	type config struct {
		IP net.IP    `koanf:"ip"`
		At time.Time `koanf:"at"`
	}
	defaults := config{
		IP: net.ParseIP("192.0.2.1"),
		At: time.Date(2026, time.September, 13, 9, 30, 0, 0, time.UTC),
	}
	schema, err := configx.SchemaOf(defaults)
	require.NoError(t, err)
	ipDefault := schemaFieldByPath(t, schema.Fields(), "ip").Default
	atDefault := schemaFieldByPath(t, schema.Fields(), "at").Default
	assert.Equal(t, "192.0.2.1", ipDefault)
	assert.Equal(t, "2026-09-13T09:30:00Z", atDefault)

	fs := pflag.NewFlagSet("text-round-trip", pflag.ContinueOnError)
	require.NoError(t, schema.BindFlags(fs))
	require.NoError(t, fs.Parse([]string{"--ip=" + ipDefault, "--at=" + atDefault}))
	loaded, err := configx.Load[config](configx.WithFlagSet(fs))
	require.NoError(t, err)
	assert.True(t, defaults.IP.Equal(loaded.IP))
	assert.True(t, defaults.At.Equal(loaded.At))
}

func TestSchemaBindFlags_MarshalOnlyScalarUsesScalarDefault(t *testing.T) {
	type config struct {
		Value schemaMarshalOnlyInt `koanf:"value"`
	}
	schema, err := configx.SchemaOf(config{Value: 255})
	require.NoError(t, err)
	field := schemaFieldByPath(t, schema.Fields(), "value")
	assert.Equal(t, configx.SchemaKindInt, field.Kind)
	assert.Equal(t, "255", field.Default)

	fs := pflag.NewFlagSet("marshal-only", pflag.ContinueOnError)
	require.NoError(t, schema.BindFlags(fs))
	require.NoError(t, fs.Parse([]string{"--value=" + field.Default}))
	loaded, err := configx.Load[config](configx.WithFlagSet(fs))
	require.NoError(t, err)
	assert.Equal(t, schemaMarshalOnlyInt(255), loaded.Value)
}

func TestSchemaOf_AcceptsNilStructPointer(t *testing.T) {
	var defaults *schemaValueConfig
	schema, err := configx.SchemaOf(defaults)
	require.NoError(t, err)

	assert.Len(t, schema.Fields(), 6)
	assert.Equal(t, "", schemaFieldByPath(t, schema.Fields(), "optional").Default)
}

func TestSchemaOf_HonorsKoanfSquash(t *testing.T) {
	type nested struct {
		Port int `koanf:"port"`
	}
	type config struct {
		Server nested `koanf:",squash"`
	}

	schema, err := configx.SchemaOf(config{Server: nested{Port: 8080}})
	require.NoError(t, err)
	assert.Equal(t, []string{"port"}, schemaFieldPaths(schema.Fields()))
}

func TestSchemaOf_CustomMetadataTagAndProgrammaticOverrides(t *testing.T) {
	type config struct {
		Value string `cli:"hidden,nocli" koanf:"value"`
	}
	schema, err := configx.SchemaOf(
		config{Value: "visible-only-internally"},
		configx.WithSchemaMetadataTag("cli"),
		configx.WithSchemaField(
			"value",
			configx.WithSchemaFieldHidden(false),
			configx.WithSchemaFieldDeprecated("use replacement"),
			configx.WithSchemaFieldRedacted(true),
			configx.WithSchemaFieldCLIEnabled(true),
		),
	)
	require.NoError(t, err)

	field := schemaFieldByPath(t, schema.Fields(), "value")
	assert.False(t, field.Hidden)
	assert.Equal(t, "use replacement", field.Deprecated)
	assert.True(t, field.Redacted)
	assert.True(t, field.CLIEnabled)
	assert.Equal(t, "[REDACTED]", field.Default)
}

func TestSchemaOf_RejectsInvalidMetadataAndUnsupportedTypes(t *testing.T) {
	tests := map[string]any{
		"unknown metadata": struct {
			Value string `configx:"unknown"`
		}{},
		"unsupported map": struct {
			Value map[string]string
		}{},
		"uninstantiable text interface": struct {
			Value encoding.TextUnmarshaler
		}{},
		"duplicate path": struct {
			First  string `koanf:"same"`
			Second string `koanf:"same"`
		}{},
	}

	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := configx.SchemaOf(value)
			require.Error(t, err)
			assert.ErrorIs(t, err, configx.ErrSchema)
		})
	}
}

func TestSchemaOf_RejectsRecursiveStructs(t *testing.T) {
	_, err := configx.SchemaOf(recursiveSchemaConfig{})
	require.Error(t, err)
	assert.ErrorIs(t, err, configx.ErrSchema)
}

func TestSchemaBindFlags_NestedNoCLIIsInherited(t *testing.T) {
	type nested struct {
		Port int `koanf:"port"`
	}
	type config struct {
		Server nested `configx:"nocli" koanf:"server"`
	}
	schema, err := configx.SchemaOf(config{})
	require.NoError(t, err)
	field := schemaFieldByPath(t, schema.Fields(), "server.port")
	assert.False(t, field.CLIEnabled)

	fs := pflag.NewFlagSet("nested-nocli", pflag.ContinueOnError)
	require.NoError(t, schema.BindFlags(fs))
	assert.Nil(t, fs.Lookup("server.port"))
}

func TestSchemaBindFlags_PreflightsNormalizedNameConflicts(t *testing.T) {
	type config struct {
		First  string `koanf:"same-name"`
		Second string `koanf:"same_name"`
	}
	schema, err := configx.SchemaOf(config{})
	require.NoError(t, err)
	fs := pflag.NewFlagSet("normalized-conflict", pflag.ContinueOnError)
	fs.SetNormalizeFunc(func(_ *pflag.FlagSet, name string) pflag.NormalizedName {
		return pflag.NormalizedName(strings.ReplaceAll(name, "_", "-"))
	})

	assert.NotPanics(t, func() {
		err = schema.BindFlags(fs)
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, configx.ErrSchemaFlagConflict)
	assert.Nil(t, fs.Lookup("same-name"))
}

func TestSchemaBindFlags_RedactionDoesNotExposeValues(t *testing.T) {
	type redactedConfig struct {
		Token int `configx:"redact" koanf:"token"`
	}

	schema, err := configx.SchemaOf(redactedConfig{Token: 123456})
	require.NoError(t, err)
	fs := pflag.NewFlagSet("redacted", pflag.ContinueOnError)
	require.NoError(t, schema.BindFlags(fs))

	flag := fs.Lookup("token")
	require.NotNil(t, flag)
	assert.Equal(t, "[REDACTED]", flag.DefValue)
	assert.Equal(t, "[REDACTED]", flag.Value.String())
	getter, ok := flag.Value.(interface{ Get() any })
	require.True(t, ok)
	assert.Equal(t, 0, getter.Get())

	err = fs.Parse([]string{"--token=sensitive-input"})
	require.Error(t, err)
	assert.False(t, strings.Contains(err.Error(), "123456"))
}

func TestSchemaBindFlags_PreflightsAllConflicts(t *testing.T) {
	schema, err := configx.SchemaOf(schemaDefaults())
	require.NoError(t, err)
	fs := pflag.NewFlagSet("conflict", pflag.ContinueOnError)
	fs.String("name", "", "existing")

	err = schema.BindFlags(fs)
	require.Error(t, err)
	assert.True(t, errors.Is(err, configx.ErrSchemaFlagConflict))
	assert.Nil(t, fs.Lookup("region"))
}

func TestSchemaBindFlags_RejectsNilAndEmptySeparator(t *testing.T) {
	var nilSchema *configx.Schema
	err := nilSchema.BindFlags(pflag.NewFlagSet("nil-schema", pflag.ContinueOnError))
	require.ErrorIs(t, err, configx.ErrSchema)

	schema, err := configx.SchemaOf(schemaValueConfig{})
	require.NoError(t, err)
	require.ErrorIs(t, schema.BindFlags(nil), configx.ErrSchema)
	require.ErrorIs(t,
		schema.BindFlags(
			pflag.NewFlagSet("separator", pflag.ContinueOnError),
			configx.WithFlagNameSeparator(""),
		),
		configx.ErrSchema,
	)
}

func TestSchemaBindFlags_CanBindIndependentFlagSets(t *testing.T) {
	type config struct {
		Name string `koanf:"name"`
	}
	schema, err := configx.SchemaOf(config{Name: "default"})
	require.NoError(t, err)

	first := pflag.NewFlagSet("first", pflag.ContinueOnError)
	second := pflag.NewFlagSet("second", pflag.ContinueOnError)
	require.NoError(t, schema.BindFlags(first))
	require.NoError(t, schema.BindFlags(second))
	require.NoError(t, first.Parse([]string{"--name=first"}))
	require.NoError(t, second.Parse([]string{"--name=second"}))

	firstConfig, err := configx.Load[config](configx.WithFlagSet(first))
	require.NoError(t, err)
	secondConfig, err := configx.Load[config](configx.WithFlagSet(second))
	require.NoError(t, err)
	assert.Equal(t, "first", firstConfig.Name)
	assert.Equal(t, "second", secondConfig.Name)
}
