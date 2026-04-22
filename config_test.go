//revive:disable:file-length-limit Config tests intentionally keep related behavior scenarios in one file.

package configx_test

import (
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

func TestLoadT_Generic(t *testing.T) {
	result := configx.LoadT[SimpleConfig](
		configx.WithDefaults(map[string]any{
			"name": "gen",
			"port": 9000,
		}),
	)
	assert.True(t, result.IsOk())
	cfg, err := result.Get()
	assert.NoError(t, err)
	assert.Equal(t, "gen", cfg.Name)
	assert.Equal(t, 9000, cfg.Port)
}

func TestLoadTErr_Generic(t *testing.T) {
	cfg, err := configx.LoadTErr[SimpleConfig](
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

	cfg, err := configx.LoadTErr[AppConfig](
		configx.WithTypedDefaults(AppConfig{Name: "typed-default", Port: 8081}),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	)
	assert.NoError(t, err)
	assert.Equal(t, "typed-default", cfg.Name)
	assert.Equal(t, 8081, cfg.Port)
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
	assert.Equal(t, []string{"a.key", "b.key"}, snapshot.Keys.Values())
	valueA, ok := snapshot.Values.Get("a.key")
	require.True(t, ok)
	assert.Equal(t, 1, valueA)
	valueB, ok := snapshot.Values.Get("b.key")
	require.True(t, ok)
	assert.Equal(t, 2, valueB)
}

func TestValidate_Required(t *testing.T) {
	result := configx.LoadT[SimpleConfig](
		configx.WithDefaults(map[string]any{
			"name": "", // empty → required fails
			"port": 8080,
		}),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	)
	assert.True(t, result.IsError())
	err := result.Error()
	assert.Error(t, err)
}

func TestValidate_Range(t *testing.T) {
	result := configx.LoadT[SimpleConfig](
		configx.WithDefaults(map[string]any{
			"name": "ok",
			"port": 500, // < 1000 → gte fails
		}),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	)
	assert.True(t, result.IsError())
	assert.Error(t, result.Error())
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
	assert.Equal(t, []string{"x", "y"}, cfg.GetStringSlice("app.tags").Values())
	assert.Equal(t, 0.75, cfg.GetFloat64("app.ratio"))
	assert.True(t, cfg.Exists("app.name"))
	assert.False(t, cfg.Exists("missing"))
	assert.Equal(t, int64(1234), cfg.GetInt64("app.port"))
	assert.Equal(t, []int{1, 2, 3}, cfg.GetIntSlice("app.ids").Values())
}

func TestWithIgnoreDotenvError(t *testing.T) {
	var cfg SimpleConfig
	err := configx.Load(&cfg,
		configx.WithDotenv("not-exists.env"),
		configx.WithIgnoreDotenvError(false),
		configx.WithPriority(configx.SourceDotenv),
	)
	assert.Error(t, err)
}

func TestDotenvDefaultModeIsOptional(t *testing.T) {
	var cfg SimpleConfig
	err := configx.Load(&cfg,
		configx.WithDotenv("not-exists.env"),
		configx.WithPriority(configx.SourceDotenv),
	)
	assert.NoError(t, err)
}

func TestWithIgnoreDotenvError_IgnoreParseError(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), ".env")
	writeErr := os.WriteFile(envFile, []byte("BROKEN='unclosed"), 0o600)
	assert.NoError(t, writeErr)

	var cfg SimpleConfig
	err := configx.Load(&cfg,
		configx.WithDotenv(envFile),
		configx.WithIgnoreDotenvError(true),
		configx.WithPriority(configx.SourceDotenv),
	)
	assert.NoError(t, err)
}

func TestWithIgnoreDotenvError_StrictParseError(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), ".env")
	writeErr := os.WriteFile(envFile, []byte("BROKEN='unclosed"), 0o600)
	assert.NoError(t, writeErr)

	var cfg SimpleConfig
	err := configx.Load(&cfg,
		configx.WithDotenv(envFile),
		configx.WithIgnoreDotenvError(false),
		configx.WithPriority(configx.SourceDotenv),
	)
	assert.Error(t, err)
}

func TestEnvPrefixWithoutTrailingUnderscore(t *testing.T) {
	t.Setenv("APP_NAME", "env-app")
	t.Setenv("APP_PORT", "8088")

	result := configx.LoadT[SimpleConfig](
		configx.WithEnvPrefix("APP"),
		configx.WithPriority(configx.SourceEnv),
	)
	assert.True(t, result.IsOk())

	cfg, err := result.Get()
	assert.NoError(t, err)
	assert.Equal(t, "env-app", cfg.Name)
	assert.Equal(t, 8088, cfg.Port)
}

func TestFlagSetSource_ChangedFlagsOnly(t *testing.T) {
	fs := pflag.NewFlagSet("configx-test", pflag.ContinueOnError)
	fs.String("name", "flag-default", "")
	fs.Int("port", 7001, "")
	require.NoError(t, fs.Parse([]string{"--name=cli-name"}))

	cfg, err := configx.LoadTErr[SimpleConfig](
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

func TestFlagSetSource_DefaultNameMapping(t *testing.T) {
	type nestedConfig struct {
		Server struct {
			Port int `validate:"gte=1"`
		}
	}

	fs := pflag.NewFlagSet("configx-test", pflag.ContinueOnError)
	fs.Int("server-port", 0, "")
	require.NoError(t, fs.Parse([]string{"--server-port=9090"}))

	cfg, err := configx.LoadTErr[nestedConfig](configx.WithFlagSet(fs))
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

	cfg, err := configx.LoadTErr[nestedConfig](
		configx.WithFlagSet(fs),
		configx.WithArgsNameFunc(func(name string) string {
			return name
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, 0, cfg.Server.Port)

	cfg, err = configx.LoadTErr[nestedConfig](
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

	cfg, err := configx.LoadTErr[cliConfig](
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

	cfg, err := configx.LoadTErr[cliConfig](
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

	cfg, err := configx.LoadTErr[nestedConfig](
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

	cfg, err := configx.LoadTErr[SimpleConfig](
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

	port, err := configx.GetAs[int](cfg, "service.port")
	assert.NoError(t, err)
	assert.Equal(t, 9090, port)

	name, err := configx.GetAs[string](cfg, "service.name")
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

	got := configx.GetAsOr[int](cfg, "service.missing", 8080)
	assert.Equal(t, 8080, got)

	assert.Equal(t, 9090, configx.MustGetAs[int](cfg, "service.port"))
}
