package configx_test

import (
	"testing"

	configx "github.com/arcgolabs/configx"
)

type benchmarkServiceConfig struct {
	Name string
	Port int
}

var benchmarkDefaults = map[string]any{
	"service.name": "arcgo",
	"service.port": 8080,
	"feature.x":    true,
}

func benchmarkLoadedConfig(b *testing.B) *configx.Config {
	b.Helper()

	cfg, err := configx.LoadConfig(configx.WithDefaults(benchmarkDefaults))
	if err != nil {
		b.Fatalf("load config: %v", err)
	}
	return cfg
}

func BenchmarkLoadConfigDefaults(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		cfg, err := configx.LoadConfig(configx.WithDefaults(benchmarkDefaults))
		if err != nil {
			b.Fatalf("load config: %v", err)
		}
		if cfg.GetString("service.name") == "" {
			b.Fatal("service.name should not be empty")
		}
	}
}

func BenchmarkConfigGetters(b *testing.B) {
	cfg := benchmarkLoadedConfig(b)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_ = cfg.GetString("service.name")
		_ = cfg.GetInt("service.port")
		_ = cfg.GetBool("feature.x")
	}
}

func BenchmarkGetAsStruct(b *testing.B) {
	cfg := benchmarkLoadedConfig(b)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		value, err := configx.GetAs[benchmarkServiceConfig](cfg, "service")
		if err != nil {
			b.Fatalf("GetAs failed: %v", err)
		}
		if value.Name == "" || value.Port == 0 {
			b.Fatal("unexpected empty struct from GetAs")
		}
	}
}
