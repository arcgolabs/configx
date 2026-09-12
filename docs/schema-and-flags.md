---
title: 'configx Schema and Flags'
linkTitle: 'schema-flags'
description: 'Derive inspectable metadata and typed pflag definitions from configuration structs'
weight: 4
---

## Schema and generated flags

`SchemaOf` turns a typed default config into inspectable field metadata. `BindFlags`
uses the same schema to register typed `pflag` values, so applications do not need
to maintain a second manual flag registry.

```go
package main

import (
	"log"
	"os"
	"time"

	"github.com/arcgolabs/configx"
	"github.com/spf13/pflag"
)

type AppConfig struct {
	Server struct {
		Port int `koanf:"port" configx:"usage=HTTP listen port"`
	} `koanf:"server"`
	Timeout time.Duration `koanf:"timeout" configx:"usage=request timeout"`
	Token   string        `koanf:"token" configx:"redact,nocli"`
}

func main() {
	defaults := AppConfig{Timeout: 5 * time.Second}
	defaults.Server.Port = 8080

	schema, err := configx.SchemaOf(defaults)
	if err != nil {
		log.Fatal(err)
	}

	fs := pflag.NewFlagSet("app", pflag.ContinueOnError)
	if err := schema.BindFlags(fs, configx.WithFlagNameSeparator("-")); err != nil {
		log.Fatal(err)
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	cfg, err := configx.Load[AppConfig](
		configx.WithTypedDefaults(defaults),
		configx.WithFlagSet(fs),
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listen on %d", cfg.Server.Port)
}
```

Only flags explicitly changed by the user enter `SourceArgs`; generated default
values therefore do not override files, custom sources, or environment variables.
The schema records the config path on every generated flag, so separators and
explicit flag names do not require a matching `WithArgsNameFunc`.

## Tags and metadata

Config paths use the `koanf` tag by default. Untagged exported fields use their
lowercase Go name, `koanf:"-"` skips a field, tagged nested structs add a path
segment, and untagged embedded structs or structs tagged with `koanf:",squash"`
are flattened.

The default metadata tag is `configx` and accepts comma-separated options:

- `usage=text` sets flag help text.
- `flag=name` provides an exact flag name; separator replacement does not change it.
- `hidden` hides the flag.
- `deprecated=message` marks the flag as deprecated.
- `redact` replaces the default in schema/help output and `Value.String()` with
  `[REDACTED]`. Command-line arguments are not a secret transport: shells, process
  listings, and pflag parse errors may still expose user-supplied arguments.
- `nocli` keeps the field in the schema but prevents flag registration. On a nested
  struct it disables CLI binding for all descendants.

Use `WithSchemaTag` or `WithSchemaMetadataTag` to select other tag names. A
custom schema tag controls exposed paths and generated flag names; generated
flags still carry the corresponding `koanf` load path internally, so renamed
schema fields continue to decode through `WithFlagSet`.

## Supported values

Generated flags support booleans, strings, signed and unsigned integers, floats,
`time.Duration`, values implementing `encoding.TextUnmarshaler`, pointers to those
types, and slices of those types. Slice input uses CSV syntax; repeated slice flags
append values. Generated slice flags implement `pflag.SliceValue`, and their
non-redacted string representation follows pflag's bracketed slice format. This
allows matching native getters such as `FlagSet.GetStringSlice` to read generated
flags without losing the first and last characters.

Redacted slice flags keep returning `[REDACTED]` from `Value.String()` and return
only redaction placeholders from `SliceValue.GetSlice()`. `configx.Load` still
receives values explicitly supplied through the flag set.

For a slice field with a custom schema parser, `Set` passes the complete raw flag
value to the parser. `SliceValue.Append` and each item supplied to
`SliceValue.Replace` are parsed individually before their results are combined.

Maps, interfaces, arrays, complex numbers, functions, channels, and recursive
struct graphs are rejected with `ErrSchema` instead of being silently omitted.
Existing or duplicate generated flags fail before any new flags are registered and
can be detected with `ErrSchemaFlagConflict`.

## Programmatic overrides

Field overrides are keyed by the fully-qualified config path:

```go
schema, err := configx.SchemaOf(
	defaults,
	configx.WithSchemaField(
		"server.port",
		configx.WithSchemaFieldFlag("listen"),
		configx.WithSchemaFieldUsage("public HTTP port"),
		configx.WithSchemaFieldHidden(false),
		configx.WithSchemaFieldDeprecated("use server.bind instead"),
		configx.WithSchemaFieldRedacted(false),
		configx.WithSchemaFieldCLIEnabled(true),
		configx.WithSchemaFieldParser(customPortParser),
	),
)
```

An override for a path that does not exist returns `ErrSchema`. Custom parsers must
return a value assignable or convertible to the field type.
