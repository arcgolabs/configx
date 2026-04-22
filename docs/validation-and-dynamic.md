---
title: 'configx Validation and Dynamic Config'
linkTitle: 'validate-dynamic'
description: 'Custom validator, validation levels, and path-based Config access'
weight: 4
---

## Validation and dynamic config

Use **`Load` / `LoadT` / `LoadTErr`** for typed structs, or **`LoadConfig`** when you need path-based getters (`GetString`, `Exists`, `All`) without a single struct.

## 1) Custom `validator.Validate` + `Load`

```go
package main

import (
	"fmt"
	"log"

	"github.com/DaiYuANg/arcgo/configx"
	"github.com/go-playground/validator/v10"
)

type AppConfig struct {
	Name string `validate:"required"`
	Port int    `validate:"required,min=1,max=65535"`
}

func main() {
	v := validator.New(validator.WithRequiredStructEnabled())

	var cfg AppConfig
	err := configx.Load(&cfg,
		configx.WithDefaults(map[string]any{
			"name": "demo",
			"port": 8080,
		}),
		configx.WithValidator(v),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", cfg)
}
```

## 2) `LoadConfig` for dynamic paths

```go
package main

import (
	"fmt"
	"log"

	"github.com/DaiYuANg/arcgo/configx"
)

func main() {
	c, err := configx.LoadConfig(
		configx.WithDefaults(map[string]any{
			"app.name": "demo",
			"app.port": 8080,
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	name := c.GetString("app.name")
	port := c.GetInt("app.port")
	exists := c.Exists("app.debug")
	all := c.All()
	fmt.Println(name, port, exists, len(all))
}
```

## Validation levels

- `ValidateLevelNone` — skip validation (default).
- `ValidateLevelStruct` — run struct tags via the configured `validator.Validate`.

## Observability hook-in

To emit load metrics through `observabilityx`, use `WithObservability`. A runnable sample lives in the repository: [configx/examples/observability](https://github.com/DaiYuANg/arcgo/tree/main/configx/examples/observability).

## Related

- [Getting Started](./getting-started)
- [Sources and priority](./sources-and-priority)
