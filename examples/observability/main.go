package main

import (
	"fmt"
	"log"

	"github.com/DaiYuANg/arcgo/httpx"
	"github.com/DaiYuANg/arcgo/httpx/adapter"
	"github.com/DaiYuANg/arcgo/httpx/adapter/std"
	"github.com/DaiYuANg/arcgo/observabilityx"
	otelobs "github.com/DaiYuANg/arcgo/observabilityx/otel"
	promobs "github.com/DaiYuANg/arcgo/observabilityx/prometheus"
	"github.com/arcgolabs/configx"
)

type appConfig struct {
	Name string `validate:"required"`
	Port int    `validate:"required,min=1,max=65535"`
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	prom := promobs.New(promobs.WithNamespace("configx_example"))
	obs := observabilityx.Multi(otelobs.New(), prom)

	cfg, err := configx.LoadTErr[appConfig](
		configx.WithObservability(obs),
		configx.WithDefaults(map[string]any{
			"name": "arcgo",
			"port": 8080,
		}),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log.Printf("loaded config: %+v", cfg)

	stdAdapter := std.New(nil, adapter.HumaOptions{DisableDocsRoutes: true})
	metricsServer := httpx.New(
		httpx.WithAdapter(stdAdapter),
	)
	stdAdapter.Router().Handle("/metrics", prom.Handler())

	log.Println("httpx metrics route registered: GET /metrics")
	if err := metricsServer.ListenAndServe(":8080"); err != nil {
		return fmt.Errorf("serve metrics: %w", err)
	}
	return nil
}
