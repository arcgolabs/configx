---
title: 'configx Sources and Priority'
linkTitle: 'sources-priority'
description: 'Load from files, environment variables, and command-line inputs with explicit merge order'
weight: 3
---

## Sources and priority

Later sources **override** earlier ones. The default order is **dotenv → file → env → args**.

These examples use a **temporary YAML file**, `os.Setenv`, and `pflag` so they stay self-contained.

## 1) Load from a YAML file

```go
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/DaiYuANg/arcgo/configx"
)

type AppConfig struct {
	Name string `validate:"required"`
	Port int    `validate:"required,min=1,max=65535"`
}

func main() {
	dir, err := os.MkdirTemp("", "configx-doc-*")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("name: from-yaml\nport: 3000\n"), 0o644); err != nil {
		log.Fatal(err)
	}

	cfg, err := configx.LoadTErr[AppConfig](
		configx.WithFiles(path),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%+v", cfg)
}
```

## 2) Environment variables override file values

With `WithEnvPrefix("APP")`, env vars like `APP_PORT` map to the `port` key (underscores become dots after the prefix).

```go
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/DaiYuANg/arcgo/configx"
)

type AppConfig struct {
	Name string `validate:"required"`
	Port int    `validate:"required,min=1,max=65535"`
}

func main() {
	dir, err := os.MkdirTemp("", "configx-doc-*")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("name: from-yaml\nport: 3000\n"), 0o644); err != nil {
		log.Fatal(err)
	}

	if err := os.Setenv("APP_PORT", "4000"); err != nil {
		log.Fatal(err)
	}
	defer func() { _ = os.Unsetenv("APP_PORT") }()

	cfg, err := configx.LoadTErr[AppConfig](
		configx.WithFiles(path),
		configx.WithEnvPrefix("APP"),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%+v", cfg)
}
```

## 3) Explicit `WithPriority`

When you only care about **file** and **env**, list them in merge order (env last wins).

```go
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/DaiYuANg/arcgo/configx"
)

type AppConfig struct {
	Name string `validate:"required"`
	Port int    `validate:"required,min=1,max=65535"`
}

func main() {
	dir, err := os.MkdirTemp("", "configx-doc-*")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("name: from-yaml\nport: 3000\n"), 0o644); err != nil {
		log.Fatal(err)
	}

	if err := os.Setenv("APP_PORT", "5000"); err != nil {
		log.Fatal(err)
	}
	defer func() { _ = os.Unsetenv("APP_PORT") }()

	cfg, err := configx.LoadTErr[AppConfig](
		configx.WithFiles(path),
		configx.WithEnvPrefix("APP"),
		configx.WithPriority(configx.SourceFile, configx.SourceEnv),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%+v", cfg)
}
```

## 4) Command-line args override env and file values

`SourceArgs` now supports two entry styles:

- `WithArgs(...)` / `WithOSArgs()` for raw long-form argv input
- `WithFlagSet(fs)` / `WithCommandLineFlags()` for changed `pflag` values

In the example below, the CLI value `6000` overrides both the file value `3000` and the env value `4000`.

```go
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/DaiYuANg/arcgo/configx"
)

type AppConfig struct {
	Name string `validate:"required"`
	Port int    `validate:"required,min=1,max=65535"`
}

func main() {
	dir, err := os.MkdirTemp("", "configx-doc-*")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("name: from-yaml\nport: 3000\n"), 0o644); err != nil {
		log.Fatal(err)
	}

	if err := os.Setenv("APP_PORT", "4000"); err != nil {
		log.Fatal(err)
	}
	defer func() { _ = os.Unsetenv("APP_PORT") }()

	cfg, err := configx.LoadTErr[AppConfig](
		configx.WithFiles(path),
		configx.WithEnvPrefix("APP"),
		configx.WithArgs("--name=from-cli", "--port", "6000"),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%+v", cfg)
}
```

Raw args support:

- `--key=value`
- `--key value`
- `--flag`
- `--no-flag`

Positional args are ignored, and parsing stops after a standalone `--`.

## 5) Use a `pflag.FlagSet`

If your application already uses `pflag`, hand the parsed `FlagSet` directly to `configx`.

```go
package main

import (
	"log"

	"github.com/DaiYuANg/arcgo/configx"
	"github.com/spf13/pflag"
)

type AppConfig struct {
	Server struct {
		Port int `validate:"required,min=1,max=65535"`
	}
	Debug bool
}

func main() {
	fs := pflag.NewFlagSet("app", pflag.ContinueOnError)
	fs.Int("server-port", 0, "")
	fs.Bool("debug", false, "")

	if err := fs.Parse([]string{"--server-port=7000", "--debug"}); err != nil {
		log.Fatal(err)
	}

	cfg, err := configx.LoadTErr[AppConfig](
		configx.WithFlagSet(fs),
		configx.WithValidateLevel(configx.ValidateLevelStruct),
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%+v", cfg)
}
```

`WithFlagSet` only reads flags explicitly marked as changed. Default `pflag` values do not override file/env/default values.

## Environment and command-line key mapping

With `WithEnvPrefix("APP")` and the default separator `_`:

- `APP_PORT` → `port`
- `APP_DATABASE_HOST` → `database.host`

With the default `WithArgsNameFunc`:

- `--server-port` → `server.port`
- `--db-read-timeout` → `db.read.timeout`
- `--no-debug` → `debug = false`

If you do not want the default `-` to `.` mapping, provide a custom `WithArgsNameFunc`.

## Related

- [Getting Started](./getting-started)
- [Validation and dynamic config](./validation-and-dynamic)
