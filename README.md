---
title: 'configx'
linkTitle: 'configx'
description: 'Hierarchical Configuration Loading and Validation'
weight: 3
---

## configx

`configx` is a hierarchical configuration loader built on `koanf` and `go-playground/validator`.

It provides two main usage styles:

- **Typed load** — `LoadT[T]` / `LoadTErr[T]` unmarshals into a struct and (optionally) validates it.
- **Dynamic config** — `LoadConfig` returns `*configx.Config` for path-based access (`GetString`, `Exists`, `All`, `Unmarshal`).

Sources are merged by priority. Later sources override earlier ones. The default order is `dotenv → file → env → args`.

## Current capabilities

- `.env` loading (`WithDotenv`, `WithIgnoreDotenvError`)
- File loading (YAML/JSON/TOML) (`WithFiles`)
- Environment variables (`WithEnvPrefix`, `WithEnvSeparator`)
- Command-line args and flags (`WithArgs`, `WithOSArgs`, `WithFlagSet`, `WithCommandLineFlags`, `WithArgsNameFunc`)
- Explicit source priority (`WithPriority`)
- Defaults (`WithDefaults`, `WithDefaultsTyped`, `WithTypedDefaults`)
- Optional validation (`WithValidateLevel`, `WithValidator`)
- Optional observability (`WithObservability`)
- Hot reload support via `Watcher` (`NewWatcher` / `Watch`)

## Documentation map

- Release notes: [configx v0.3.0](./release-v0.3.0)
- Minimal typed load with validation: [Getting Started](./getting-started)
- Files, env vars, command-line inputs, merge order: [Sources and priority](./sources-and-priority)
- Custom validator and dynamic config: [Validation and dynamic config](./validation-and-dynamic)

## Install / Import

```bash
go get github.com/arcgolabs/configx@latest
```

## Key API surface (summary)

- `Load(out, opts...)` — load into an existing struct pointer
- `LoadT[T](opts...)` / `LoadTErr[T](opts...)` — typed load helpers
- `LoadConfig(opts...)` — return `*Config` for dynamic access
- `New(opts...)` / `NewT[T](opts...)` — build reusable loaders
- `NewWatcher(opts...)` / `Watch(ctx, ...)` — hot reload

## Integration guide

- **dix** — load once at startup, provide typed config via module providers.
- **httpx** — drive bind address, TLS, and middleware toggles from typed config.
- **dbx / kvx** — keep DSN/backend options centralized and environment-specific.
- **logx / observabilityx** — externalize runtime levels and instrumentation toggles.

## Examples (repository)

- [configx/examples/observability](https://github.com/arcgolabs/configx/tree/main/examples/observability)
