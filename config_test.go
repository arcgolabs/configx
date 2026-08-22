//revive:disable:file-length-limit Config tests intentionally keep related behavior scenarios in one file.

package configx_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	configx "github.com/arcgolabs/configx"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type SimpleConfig struct {
	Name string `validate:"required"`
	Port int    `validate:"gte=1000,lte=65535"`
}

func TestNewConfig_Basic(t *testing.T) {
	cfg, err := configx.NewConfig(
		configx.WithDefaults(map[string]any{
			"name": "test",
			"port": 8080,
		}),
	)
	assert.NoError(t, err)
	assert.Equal(t, "test", cfg.GetString("name"))
	assert.Equal(t, 8080, cfg.GetInt("port"))
}

func TestWithDefaultsTyped(t *testing.T) {
	cfg, err := configx.LoadConfig(
		configx.WithDefaultsTyped(map[string]int{
			"port": 7001,
		}),
	)
	assert.NoError(t, err)
	assert.Equal(t, 7001, cfg.GetInt("port"))
}

func TestLoad_Generic(t *testing.T) {
	cfg, err := configx.Load[SimpleConfig](
		configx.WithDefaults(map[string]any{
			"name": "gen",
			"port": 9000,
		}),
	)
	assert.NoError(t, err)
	assert.Equal(t, "gen", cfg.Name)
	assert.Equal(t, 9000, cfg.Port)
}

func TestLoader_GenericMethodsSupportMultipleTypes(t *testing.T) {
	loader := configx.New(configx.WithDefaults(map[string]any{
		"name": "tuple",
		"port": 9100,
	}))

	cfg, err := loader.Load[SimpleConfig]()
	assert.NoError(t, err)
	assert.Equal(t, "tuple", cfg.Name)
	assert.Equal(t, 9100, cfg.Port)

	values, err := loader.Load[map[string]any]()
	assert.NoError(t, err)
	assert.Equal(t, "tuple", values["name"])
	assert.EqualValues(t, 9100, values["port"])
}

func TestLoad_GenericTuple(t *testing.T) {
	cfg, err := configx.Load[SimpleConfig](
		configx.WithDefaults(map[string]any{
			"name": "tuple",
			"port": 9100,
		}),
	)
	assert.NoError(t, err)
	assert.Equal(t, "tuple", cfg.Name)
	assert.Equal(t, 9100, cfg.Port)
}

func TestWithTypedDefaults_Generic(t *testing.T) {
	type AppConfig struct {
		Name string `validate:"required"`
		Port int    `validate:"gte=1"`
	}

	cfg, err := configx.Load[AppConfig](
		configx.WithTypedDefaults(AppConfig{Name: "typed-default", Port: 8081}),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	)
	assert.NoError(t, err)
	assert.Equal(t, "typed-default", cfg.Name)
	assert.Equal(t, 8081, cfg.Port)
}

func TestWithTypedDefaults_AllowsFormerReservedKey(t *testing.T) {
	type defaults struct {
		Value string `json:"__configx_invalid_typed_defaults__"`
	}

	cfg, err := configx.Load[map[string]any](
		configx.WithTypedDefaults(defaults{Value: "valid-user-value"}),
	)
	require.NoError(t, err)
	assert.Equal(t, "valid-user-value", cfg["__configx_invalid_typed_defaults__"])
}

func TestWithTypedDefaults_ReportsConversionError(t *testing.T) {
	_, err := configx.Load[map[string]any](configx.WithTypedDefaults(make(chan int)))
	require.Error(t, err)
	assert.ErrorIs(t, err, configx.ErrDefaults)
}

func TestSnapshot_ReturnsSortedKeys(t *testing.T) {
	cfg, err := configx.LoadConfig(
		configx.WithDefaults(map[string]any{
			"b.key": 2,
			"a.key": 1,
		}),
		configx.WithPriority(),
	)
	assert.NoError(t, err)
	snapshot := cfg.Snapshot()
	require.NotNil(t, snapshot.Keys)
	require.NotNil(t, snapshot.Values)
	assert.Equal(t, []string{"a.key", "b.key"}, snapshot.Keys)
	valueA, ok := snapshot.Values["a.key"]
	require.True(t, ok)
	assert.Equal(t, 1, valueA)
	valueB, ok := snapshot.Values["b.key"]
	require.True(t, ok)
	assert.Equal(t, 2, valueB)
}

func TestValidate_Required(t *testing.T) {
	_, err := configx.Load[SimpleConfig](
		configx.WithDefaults(map[string]any{
			"name": "", // empty → required fails
			"port": 8080,
		}),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	)
	assert.Error(t, err)
}

func TestValidate_Range(t *testing.T) {
	_, err := configx.Load[SimpleConfig](
		configx.WithDefaults(map[string]any{
			"name": "ok",
			"port": 500, // < 1000 → gte fails
		}),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	)
	assert.Error(t, err)
}

func TestGetters(t *testing.T) {
	cfg, err := configx.LoadConfig(
		configx.WithDefaults(map[string]any{
			"app.name":    "getter-test",
			"app.port":    1234,
			"app.debug":   true,
			"app.timeout": "5s",
			"app.tags":    []string{"x", "y"},
			"app.ratio":   0.75,
			"app.ids":     []int{1, 2, 3},
		}),
	)
	assert.NoError(t, err)

	assert.Equal(t, "getter-test", cfg.GetString("app.name"))
	assert.Equal(t, 1234, cfg.GetInt("app.port"))
	assert.True(t, cfg.GetBool("app.debug"))
	assert.Equal(t, 5, int(cfg.GetDuration("app.timeout").Seconds()))
	assert.Equal(t, []string{"x", "y"}, cfg.GetStringSlice("app.tags"))
	assert.Equal(t, 0.75, cfg.GetFloat64("app.ratio"))
	assert.True(t, cfg.Exists("app.name"))
	assert.False(t, cfg.Exists("missing"))
	assert.Equal(t, int64(1234), cfg.GetInt64("app.port"))
	assert.Equal(t, []int{1, 2, 3}, cfg.GetIntSlice("app.ids"))
}

func TestSliceGetters_ReturnCopies(t *testing.T) {
	cfg, err := configx.LoadConfig(configx.WithDefaults(map[string]any{
		"strings": []string{"one", "two"},
		"ints":    []int{1, 2},
	}))
	require.NoError(t, err)

	stringsValue := cfg.GetStringSlice("strings")
	intsValue := cfg.GetIntSlice("ints")
	stringsValue[0] = "changed"
	intsValue[0] = 99

	assert.Equal(t, []string{"one", "two"}, cfg.GetStringSlice("strings"))
	assert.Equal(t, []int{1, 2}, cfg.GetIntSlice("ints"))
}

func TestWithIgnoreDotenvError(t *testing.T) {
	_, err := configx.Load[SimpleConfig](
		configx.WithDotenv("not-exists.env"),
		configx.WithIgnoreDotenvError(false),
		configx.WithPriority(configx.SourceDotenv),
	)
	assert.Error(t, err)
}

func TestDotenvDefaultModeIsOptional(t *testing.T) {
	_, err := configx.Load[SimpleConfig](
		configx.WithDotenv("not-exists.env"),
		configx.WithPriority(configx.SourceDotenv),
	)
	assert.NoError(t, err)
}

func TestWithIgnoreDotenvError_IgnoreParseError(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), ".env")
	writeErr := os.WriteFile(envFile, []byte("BROKEN='unclosed"), 0o600)
	assert.NoError(t, writeErr)

	_, err := configx.Load[SimpleConfig](
		configx.WithDotenv(envFile),
		configx.WithIgnoreDotenvError(true),
		configx.WithPriority(configx.SourceDotenv),
	)
	assert.NoError(t, err)
}

func TestWithFileParser_CustomExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config"+testConfigFileExtension)
	require.NoError(t, os.WriteFile(path, []byte("name: custom"), 0o600))

	cfg, err := configx.LoadConfig(
		configx.WithFileParser(testConfigFileExtension, kvFileParser{}),
		configx.WithFiles(path),
	)
	require.NoError(t, err)
	assert.Equal(t, "custom", cfg.GetString("name"))
}

func TestWithFileGlobs_LoadsMatchesInSortedOrder(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "00-base"+testConfigFileExtension)
	overridePath := filepath.Join(dir, "10-override"+testConfigFileExtension)
	writeConfigFile(t, overridePath, "name: override\n")
	writeConfigFile(t, basePath, "name: base\nport: 8080\n")

	cfg, err := configx.LoadConfig(
		configx.WithFileParser(testConfigFileExtension, kvFileParser{}),
		configx.WithFileGlobs(filepath.Join(dir, "*"+testConfigFileExtension)),
	)
	require.NoError(t, err)
	assert.Equal(t, "override", cfg.GetString("name"))
	assert.Equal(t, 8080, cfg.GetInt("port"))
}

func TestWithFileGlobs_AppendsToExplicitFiles(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base"+testConfigFileExtension)
	overridePath := filepath.Join(dir, "override"+testConfigFileExtension)
	writeConfigFile(t, basePath, "name: base\nport: 8080\n")
	writeConfigFile(t, overridePath, "name: override\n")

	cfg, err := configx.LoadConfig(
		configx.WithFileParser(testConfigFileExtension, kvFileParser{}),
		configx.WithFiles(basePath),
		configx.WithFileGlobs(filepath.Join(dir, "override"+testConfigFileExtension)),
	)
	require.NoError(t, err)
	assert.Equal(t, "override", cfg.GetString("name"))
	assert.Equal(t, 8080, cfg.GetInt("port"))
}

func TestWithFileGlobs_SupportsRecursiveDoubleStar(t *testing.T) {
	dir := t.TempDir()
	nestedDir := filepath.Join(dir, "profiles", "prod")
	require.NoError(t, os.MkdirAll(nestedDir, 0o700))

	basePath := filepath.Join(dir, "00-base"+testConfigFileExtension)
	nestedPath := filepath.Join(nestedDir, "10-prod"+testConfigFileExtension)
	writeConfigFile(t, basePath, "name: base\nport: 8080\n")
	writeConfigFile(t, nestedPath, "name: prod\n")

	cfg, err := configx.LoadConfig(
		configx.WithFileParser(testConfigFileExtension, kvFileParser{}),
		configx.WithFileGlobs(filepath.Join(dir, "**", "*"+testConfigFileExtension)),
	)
	require.NoError(t, err)
	assert.Equal(t, "prod", cfg.GetString("name"))
	assert.Equal(t, 8080, cfg.GetInt("port"))
}

func TestWithFileGlobs_NoMatchIsIgnored(t *testing.T) {
	cfg, err := configx.LoadConfig(
		configx.WithDefaults(map[string]any{
			"name": "defaults",
		}),
		configx.WithFileParser(testConfigFileExtension, kvFileParser{}),
		configx.WithFileGlobs(filepath.Join(t.TempDir(), "*"+testConfigFileExtension)),
	)
	require.NoError(t, err)
	assert.Equal(t, "defaults", cfg.GetString("name"))
}

func TestWithIgnoreDotenvError_StrictParseError(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), ".env")
	writeErr := os.WriteFile(envFile, []byte("BROKEN='unclosed"), 0o600)
	assert.NoError(t, writeErr)

	_, err := configx.Load[SimpleConfig](
		configx.WithDotenv(envFile),
		configx.WithIgnoreDotenvError(false),
		configx.WithPriority(configx.SourceDotenv),
	)
	assert.Error(t, err)
}

func TestEnvPrefixWithoutTrailingUnderscore(t *testing.T) {
	t.Setenv("APP_NAME", "env-app")
	t.Setenv("APP_PORT", "8088")

	cfg, err := configx.Load[SimpleConfig](
		configx.WithEnvPrefix("APP"),
		configx.WithPriority(configx.SourceEnv),
	)
	assert.NoError(t, err)
	assert.Equal(t, "env-app", cfg.Name)
	assert.Equal(t, 8088, cfg.Port)
}

func TestFlagSetSource_ChangedFlagsOnly(t *testing.T) {
	fs := pflag.NewFlagSet("configx-test", pflag.ContinueOnError)
	fs.String("name", "flag-default", "")
	fs.Int("port", 7001, "")
	require.NoError(t, fs.Parse([]string{"--name=cli-name"}))

	cfg, err := configx.Load[SimpleConfig](
		configx.WithDefaults(map[string]any{
			"name": "defaults-name",
			"port": 8080,
		}),
		configx.WithFlagSet(fs),
	)
	require.NoError(t, err)
	assert.Equal(t, "cli-name", cfg.Name)
	assert.Equal(t, 8080, cfg.Port)
}

func TestCustomSource_DefaultPriority(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config"+testConfigFileExtension)
	writeConfigFile(t, path, "name: file-name\nport: 5000\n")
	t.Setenv("APP_PORT", "7000")

	cfg, err := configx.Load[SimpleConfig](
		configx.WithFileParser(testConfigFileExtension, kvFileParser{}),
		configx.WithFiles(path),
		configx.WithSources(configx.NewSource("remote", func(_ context.Context) (map[string]any, error) {
			return map[string]any{
				"name": "custom-name",
				"port": 6000,
			}, nil
		})),
		configx.WithEnvPrefix("APP"),
	)
	require.NoError(t, err)
	assert.Equal(t, "custom-name", cfg.Name)
	assert.Equal(t, 7000, cfg.Port)
}

func TestCustomSource_WithPriority(t *testing.T) {
	t.Setenv("APP_PORT", "7000")

	cfg, err := configx.Load[SimpleConfig](
		configx.WithSource("remote", func(_ context.Context) (map[string]any, error) {
			return map[string]any{
				"name": "custom-name",
				"port": 6000,
			}, nil
		}),
		configx.WithEnvPrefix("APP"),
		configx.WithPriority(configx.SourceEnv, configx.SourceCustom),
	)
	require.NoError(t, err)
	assert.Equal(t, "custom-name", cfg.Name)
	assert.Equal(t, 6000, cfg.Port)
}

func TestCustomSource_Error(t *testing.T) {
	boom := errors.New("boom")

	_, err := configx.LoadConfig(
		configx.WithSource("broken", func(_ context.Context) (map[string]any, error) {
			return nil, boom
		}),
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, configx.ErrSource)
	assert.ErrorIs(t, err, boom)
}

func TestCustomSource_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	called := false

	_, err := configx.LoadConfigContext(ctx,
		configx.WithSource("remote", func(_ context.Context) (map[string]any, error) {
			called = true
			return map[string]any{}, nil
		}),
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, configx.ErrSource)
	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, called)
}

func TestFlagSetSource_DefaultNameMapping(t *testing.T) {
	type nestedConfig struct {
		Server struct {
			Port int `validate:"gte=1"`
		}
	}

	fs := pflag.NewFlagSet("configx-test", pflag.ContinueOnError)
	fs.Int("server-port", 0, "")
	require.NoError(t, fs.Parse([]string{"--server-port=9090"}))

	cfg, err := configx.Load[nestedConfig](configx.WithFlagSet(fs))
	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.Server.Port)
}

func TestFlagSetSource_CustomNameFunc(t *testing.T) {
	type nestedConfig struct {
		Server struct {
			Port int
		}
	}

	fs := pflag.NewFlagSet("configx-test", pflag.ContinueOnError)
	fs.Int("server_port", 0, "")
	require.NoError(t, fs.Parse([]string{"--server_port=9091"}))

	cfg, err := configx.Load[nestedConfig](
		configx.WithFlagSet(fs),
		configx.WithArgsNameFunc(func(name string) string {
			return name
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, 0, cfg.Server.Port)

	cfg, err = configx.Load[nestedConfig](
		configx.WithFlagSet(fs),
		configx.WithArgsNameFunc(func(name string) string {
			return "server.port"
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, 9091, cfg.Server.Port)
}

func TestFlagSetSource_DuplicateResolvedPath(t *testing.T) {
	fs := pflag.NewFlagSet("configx-test", pflag.ContinueOnError)
	fs.String("db-host", "", "")
	fs.String("db.host", "", "")
	require.NoError(t, fs.Parse([]string{"--db-host=a", "--db.host=b"}))

	_, err := configx.LoadConfig(configx.WithFlagSet(fs))
	require.Error(t, err)
	assert.ErrorIs(t, err, configx.ErrArgs)
}

func TestArgsSource_RawArgs_BasicForms(t *testing.T) {
	type cliConfig struct {
		Name  string
		Port  int
		Debug bool
	}

	cfg, err := configx.Load[cliConfig](
		configx.WithArgs(
			"serve",
			"--name", "cli-name",
			"--port=9092",
			"--debug",
		),
	)
	require.NoError(t, err)
	assert.Equal(t, "cli-name", cfg.Name)
	assert.Equal(t, 9092, cfg.Port)
	assert.True(t, cfg.Debug)
}

func TestArgsSource_RawArgs_NoFlagAndDoubleDash(t *testing.T) {
	type cliConfig struct {
		Name  string
		Debug bool
	}

	cfg, err := configx.Load[cliConfig](
		configx.WithDefaults(map[string]any{
			"name":  "defaults-name",
			"debug": true,
		}),
		configx.WithArgs(
			"--no-debug",
			"--",
			"--name=ignored",
		),
	)
	require.NoError(t, err)
	assert.Equal(t, "defaults-name", cfg.Name)
	assert.False(t, cfg.Debug)
}

func TestArgsSource_RawArgs_CustomNameFunc(t *testing.T) {
	type nestedConfig struct {
		Server struct {
			Port int
		}
	}

	cfg, err := configx.Load[nestedConfig](
		configx.WithArgs("--server_port=9091"),
		configx.WithArgsNameFunc(func(name string) string {
			if name == "server_port" {
				return "server.port"
			}
			return name
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, 9091, cfg.Server.Port)
}

func TestArgsSource_RawArgs_DuplicateResolvedPath(t *testing.T) {
	_, err := configx.LoadConfig(
		configx.WithArgs("--db-host=a", "--db.host=b"),
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, configx.ErrArgs)
}

func TestArgsSource_FlagSetOverridesRawArgs(t *testing.T) {
	fs := pflag.NewFlagSet("configx-test", pflag.ContinueOnError)
	fs.Int("port", 0, "")
	require.NoError(t, fs.Parse([]string{"--port=9091"}))

	cfg, err := configx.Load[SimpleConfig](
		configx.WithDefaults(map[string]any{
			"name": "defaults-name",
			"port": 8080,
		}),
		configx.WithArgs("--name=cli-name", "--port=8081"),
		configx.WithFlagSet(fs),
	)
	require.NoError(t, err)
	assert.Equal(t, "cli-name", cfg.Name)
	assert.Equal(t, 9091, cfg.Port)
}

func TestGetAs_GenericValue(t *testing.T) {
	cfg, err := configx.LoadConfig(
		configx.WithDefaults(map[string]any{
			"service.port": 9090,
			"service.name": "arcgo",
		}),
	)
	assert.NoError(t, err)

	port, err := cfg.GetAs[int]("service.port")
	assert.NoError(t, err)
	assert.Equal(t, 9090, port)

	name, err := cfg.GetAs[string]("service.name")
	assert.NoError(t, err)
	assert.Equal(t, "arcgo", name)
}

func TestGetAsOr_And_MustGetAs(t *testing.T) {
	cfg, err := configx.LoadConfig(
		configx.WithDefaults(map[string]any{
			"service.port": 9090,
		}),
	)
	assert.NoError(t, err)

	got := cfg.GetAsOr("service.missing", 8080)
	assert.Equal(t, 8080, got)

	assert.Equal(t, 9090, cfg.MustGetAs[int]("service.port"))
}

func TestGenericGetters_NilConfig(t *testing.T) {
	var cfg *configx.Config

	_, err := cfg.GetAs[int]("port")
	assert.Error(t, err)
	assert.Equal(t, 8080, cfg.GetAsOr("port", 8080))
	assert.Panics(t, func() { cfg.MustGetAs[int]("port") })
}
