package configx_test

import (
	"net"
	"testing"
	"time"

	configx "github.com/arcgolabs/configx"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaBindFlags_StringSliceSupportsPFlagGetter(t *testing.T) {
	type config struct {
		Include []string `koanf:"include"`
	}

	schema, err := configx.SchemaOf(config{Include: []string{"src/**"}})
	require.NoError(t, err)
	fs := pflag.NewFlagSet("string-slice", pflag.ContinueOnError)
	require.NoError(t, schema.BindFlags(fs))
	defaults, err := fs.GetStringSlice("include")
	require.NoError(t, err)
	assert.Equal(t, []string{"src/**"}, defaults)
	require.NoError(t, fs.Set("include", "**/*.js,assets/**"))

	got, err := fs.GetStringSlice("include")
	require.NoError(t, err)
	assert.Equal(t, []string{"**/*.js", "assets/**"}, got)
}

func TestSchemaBindFlags_SliceValueSupportsAppendAndReplace(t *testing.T) {
	type config struct {
		Include []string `koanf:"include"`
	}

	schema, err := configx.SchemaOf(config{Include: []string{"src/**"}})
	require.NoError(t, err)
	fs := pflag.NewFlagSet("slice-value", pflag.ContinueOnError)
	require.NoError(t, schema.BindFlags(fs))
	flag := fs.Lookup("include")
	require.NotNil(t, flag)

	sliceValue, ok := flag.Value.(pflag.SliceValue)
	require.True(t, ok)
	assert.Equal(t, []string{"src/**"}, sliceValue.GetSlice())

	require.NoError(t, sliceValue.Append("assets/**"))
	assert.Equal(t, []string{"src/**", "assets/**"}, sliceValue.GetSlice())

	require.NoError(t, sliceValue.Replace([]string{"one,two", "three"}))
	assert.Equal(t, []string{"one,two", "three"}, sliceValue.GetSlice())
	assert.Equal(t, `["one,two",three]`, flag.Value.String())
}

func TestSchemaBindFlags_RepeatedPointerSliceFlagsAppend(t *testing.T) {
	type config struct {
		Include *[]string `koanf:"include"`
	}

	schema, err := configx.SchemaOf(config{})
	require.NoError(t, err)
	fs := pflag.NewFlagSet("pointer-slice", pflag.ContinueOnError)
	require.NoError(t, schema.BindFlags(fs))
	require.NoError(t, fs.Set("include", "src/**"))
	require.NoError(t, fs.Set("include", "assets/**"))

	loaded, err := configx.Load[config](configx.WithFlagSet(fs))
	require.NoError(t, err)
	require.NotNil(t, loaded.Include)
	assert.Equal(t, []string{"src/**", "assets/**"}, *loaded.Include)
}

func TestSchemaBindFlags_RedactedSliceValueDoesNotExposeValues(t *testing.T) {
	type config struct {
		Tokens []string `configx:"redact" koanf:"tokens"`
	}

	schema, err := configx.SchemaOf(config{Tokens: []string{"default-secret"}})
	require.NoError(t, err)
	fs := pflag.NewFlagSet("redacted-slice", pflag.ContinueOnError)
	require.NoError(t, schema.BindFlags(fs))
	require.NoError(t, fs.Set("tokens", "first-secret,second-secret"))
	flag := fs.Lookup("tokens")
	require.NotNil(t, flag)
	sliceValue, ok := flag.Value.(pflag.SliceValue)
	require.True(t, ok)

	assert.Equal(t, "[REDACTED]", flag.Value.String())
	assert.Equal(t, []string{"[REDACTED]", "[REDACTED]"}, sliceValue.GetSlice())
	assert.NotContains(t, flag.Value.String(), "secret")

	loaded, err := configx.Load[config](configx.WithFlagSet(fs))
	require.NoError(t, err)
	assert.Equal(t, []string{"first-secret", "second-secret"}, loaded.Tokens)
}

func TestSchemaBindFlags_SliceBackedTextValueRemainsScalar(t *testing.T) {
	type config struct {
		IP net.IP `koanf:"ip"`
	}

	schema, err := configx.SchemaOf(config{IP: net.ParseIP("192.0.2.1")})
	require.NoError(t, err)
	fs := pflag.NewFlagSet("slice-backed-text", pflag.ContinueOnError)
	require.NoError(t, schema.BindFlags(fs))
	flag := fs.Lookup("ip")
	require.NotNil(t, flag)
	_, isSliceValue := flag.Value.(pflag.SliceValue)
	assert.False(t, isSliceValue)
	assert.Equal(t, "192.0.2.1", flag.Value.String())

	require.NoError(t, fs.Set("ip", "192.0.2.2"))
	require.NoError(t, fs.Set("ip", "192.0.2.3"))
	loaded, err := configx.Load[config](configx.WithFlagSet(fs))
	require.NoError(t, err)
	assert.True(t, loaded.IP.Equal(net.ParseIP("192.0.2.3")))
}

func TestSchemaBindFlags_CustomSliceParserHandlesSliceValueItems(t *testing.T) {
	type config struct {
		Values []string `koanf:"values"`
	}
	parser := func(raw string) (any, error) {
		return []string{raw}, nil
	}

	schema, err := configx.SchemaOf(
		config{},
		configx.WithSchemaField(
			"values",
			configx.WithSchemaFieldParser(parser),
		),
	)
	require.NoError(t, err)
	fs := pflag.NewFlagSet("custom-slice-parser", pflag.ContinueOnError)
	require.NoError(t, schema.BindFlags(fs))
	flag := fs.Lookup("values")
	require.NotNil(t, flag)
	sliceValue, ok := flag.Value.(pflag.SliceValue)
	require.True(t, ok)

	require.NoError(t, sliceValue.Append("one,two"))
	assert.Equal(t, []string{"one,two"}, sliceValue.GetSlice())
	require.NoError(t, sliceValue.Replace([]string{"three,four", "five"}))
	assert.Equal(t, []string{"three,four", "five"}, sliceValue.GetSlice())
}

func TestSchemaBindFlags_AllSliceKindsImplementSliceValue(t *testing.T) {
	type config struct {
		Bools     []bool          `koanf:"bools"`
		Strings   []string        `koanf:"strings"`
		Ints      []int           `koanf:"ints"`
		Uints     []uint          `koanf:"uints"`
		Floats    []float64       `koanf:"floats"`
		Durations []time.Duration `koanf:"durations"`
		Modes     []schemaMode    `koanf:"modes"`
	}

	schema, err := configx.SchemaOf(config{})
	require.NoError(t, err)
	fs := pflag.NewFlagSet("all-slice-kinds", pflag.ContinueOnError)
	require.NoError(t, schema.BindFlags(fs))

	for _, name := range []string{
		"bools", "strings", "ints", "uints", "floats", "durations", "modes",
	} {
		flag := fs.Lookup(name)
		require.NotNil(t, flag)
		_, ok := flag.Value.(pflag.SliceValue)
		assert.True(t, ok, name)
	}
}

func TestSchemaBindFlags_SliceValueMutationDoesNotMarkSet(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(pflag.SliceValue) error
	}{
		{
			name: "append",
			mutate: func(value pflag.SliceValue) error {
				return value.Append("prepared")
			},
		},
		{
			name: "replace",
			mutate: func(value pflag.SliceValue) error {
				return value.Replace([]string{"prepared"})
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			type config struct {
				Values []string `koanf:"values"`
			}
			schema, err := configx.SchemaOf(config{Values: []string{"default"}})
			require.NoError(t, err)
			fs := pflag.NewFlagSet(test.name, pflag.ContinueOnError)
			require.NoError(t, schema.BindFlags(fs))
			value, ok := fs.Lookup("values").Value.(pflag.SliceValue)
			require.True(t, ok)
			require.NoError(t, test.mutate(value))

			require.NoError(t, fs.Set("values", "cli"))
			assert.Equal(t, []string{"cli"}, value.GetSlice())
		})
	}
}
