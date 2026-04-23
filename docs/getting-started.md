---
title: 'configx Getting Started'
linkTitle: 'getting-started'
description: 'Load a typed config with defaults and struct validation'
weight: 2
---

## Getting Started

`configx` merges configuration sources by priority and can run **go-playground/validator** on the result. This page uses **defaults only** (no files or environment variables) so the program is easy to copy into a fresh module.

## 1) Install

```bash
go get github.com/arcgolabs/configx@latest
```

## 2) Create `main.go`

Flat default keys (`name`, `port`) map to the struct fields. `LoadTErr[T]` returns `(T, error)` after unmarshal + validation.

```go
package main

import (
	"fmt"
	"log"

	"github.com/arcgolabs/configx"
)

type AppConfig struct {
	Name string `validate:"required"`
	Port int    `validate:"required,min=1,max=65535"`
}

func main() {
	cfg, err := configx.LoadTErr[AppConfig](
		configx.WithDefaults(map[string]any{
			"name": "demo",
			"port": 8080,
		}),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", cfg)
}
```

## 3) Run

```bash
go mod init example.com/configx-hello
go get github.com/arcgolabs/configx@latest
go run .
```

## Next

- Files, environment variables, and source order: [Sources and priority](./sources-and-priority)
- Custom validators and dynamic `*configx.Config` access: [Validation and dynamic config](./validation-and-dynamic)
