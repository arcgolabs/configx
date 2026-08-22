package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/arcgolabs/configx"
	"github.com/arcgolabs/observabilityx"
	otelobs "github.com/arcgolabs/observabilityx/otel"
	promobs "github.com/arcgolabs/observabilityx/prometheus"
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

	cfg, err := configx.Load[appConfig](
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

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", prom.Handler())
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("metrics route registered: http://localhost:%d/metrics", cfg.Port)
	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("serve metrics: %w", err)
	}
	return nil
}
