package configx

import (
	"encoding"
	"reflect"
	"slices"
	"strings"
	"time"
)

const (
	defaultSchemaTag         = "koanf"
	defaultSchemaMetadataTag = "configx"
	redactedDefault          = "[REDACTED]"
)

var (
	durationType        = reflect.TypeFor[time.Duration]()
	textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
)

// SchemaKind identifies the command-line representation of a config field.
type SchemaKind string

const (
	SchemaKindBool          SchemaKind = "bool"
	SchemaKindString        SchemaKind = "string"
	SchemaKindInt           SchemaKind = "int"
	SchemaKindUint          SchemaKind = "uint"
	SchemaKindFloat         SchemaKind = "float"
	SchemaKindDuration      SchemaKind = "duration"
	SchemaKindText          SchemaKind = "text"
	SchemaKindBoolSlice     SchemaKind = "boolSlice"
	SchemaKindStringSlice   SchemaKind = "stringSlice"
	SchemaKindIntSlice      SchemaKind = "intSlice"
	SchemaKindUintSlice     SchemaKind = "uintSlice"
	SchemaKindFloatSlice    SchemaKind = "floatSlice"
	SchemaKindDurationSlice SchemaKind = "durationSlice"
	SchemaKindTextSlice     SchemaKind = "textSlice"
)

// SchemaField is the public, immutable-by-copy description of one config leaf.
type SchemaField struct {
	Path       string
	FlagName   string
	Usage      string
	Type       string
	Default    string
	Kind       SchemaKind
	Hidden     bool
	Deprecated string
	Redacted   bool
	CLIEnabled bool
}

// Schema is a reflection-derived description of a typed configuration.
type Schema struct {
	bindings []schemaBinding
}

type schemaBinding struct {
	field        SchemaField
	loadPath     string
	typeOf       reflect.Type
	defaultValue any
	parser       SchemaValueParser
	explicitFlag bool
}

type schemaOptions struct {
	tag                  string
	metadataTag          string
	overrides            map[string][]SchemaFieldOption
	invalidFieldOverride bool
}

// SchemaOption customizes schema derivation.
type SchemaOption func(*schemaOptions)

// SchemaFieldOption overrides metadata or parsing for one schema field.
type SchemaFieldOption func(*schemaFieldOptions)

// SchemaValueParser parses one flag value. It must return a value assignable
// or convertible to the corresponding config field type.
type SchemaValueParser func(string) (any, error)

type schemaFieldOptions struct {
	flagName   *string
	usage      *string
	hidden     *bool
	deprecated *string
	redacted   *bool
	cliEnabled *bool
	parser     SchemaValueParser
}

// SchemaOf derives a configuration schema from a struct value or pointer.
func SchemaOf[T any](defaults T, opts ...SchemaOption) (*Schema, error) {
	settings, err := buildSchemaOptions(opts)
	if err != nil {
		return nil, err
	}

	typeOf, value, err := schemaRoot(defaults)
	if err != nil {
		return nil, err
	}
	builder := schemaBuilder{
		settings: settings,
		seen:     make(map[string]struct{}),
		seenLoad: make(map[string]string),
		matched:  make(map[string]struct{}),
		visiting: make(map[reflect.Type]bool),
		bindings: make([]schemaBinding, 0, typeOf.NumField()),
	}
	if err := builder.walk(schemaWalkContext{typeOf: typeOf, value: value, cliEnabled: true}); err != nil {
		return nil, err
	}
	if err := builder.validateOverrides(); err != nil {
		return nil, err
	}
	return &Schema{bindings: builder.bindings}, nil
}

func buildSchemaOptions(options []SchemaOption) (schemaOptions, error) {
	settings := schemaOptions{
		tag:         defaultSchemaTag,
		metadataTag: defaultSchemaMetadataTag,
		overrides:   make(map[string][]SchemaFieldOption),
	}
	for _, option := range options {
		if option != nil {
			option(&settings)
		}
	}
	if settings.invalidFieldOverride {
		return schemaOptions{}, schemaError("field override path must not be empty", nil)
	}
	if strings.TrimSpace(settings.tag) == "" || strings.TrimSpace(settings.metadataTag) == "" {
		return schemaOptions{}, schemaError("schema tags must not be empty", nil)
	}
	return settings, nil
}

// Fields returns a defensive copy of the schema field descriptions.
func (s *Schema) Fields() []SchemaField {
	if s == nil {
		return []SchemaField{}
	}
	fields := make([]SchemaField, 0, len(s.bindings))
	for index := range s.bindings {
		fields = append(fields, s.bindings[index].field)
	}
	return fields
}

// WithSchemaTag selects the struct tag used for config paths. The default is koanf.
func WithSchemaTag(name string) SchemaOption {
	return func(options *schemaOptions) { options.tag = strings.TrimSpace(name) }
}

// WithSchemaMetadataTag selects the metadata tag. The default is configx.
func WithSchemaMetadataTag(name string) SchemaOption {
	return func(options *schemaOptions) { options.metadataTag = strings.TrimSpace(name) }
}

// WithSchemaField applies overrides to one fully-qualified config path.
func WithSchemaField(path string, opts ...SchemaFieldOption) SchemaOption {
	normalizedPath := normalizeArgsPath(path)
	fieldOptions := slices.Clone(opts)
	return func(options *schemaOptions) {
		if normalizedPath == "" {
			options.invalidFieldOverride = true
			return
		}
		options.overrides[normalizedPath] = append(options.overrides[normalizedPath], fieldOptions...)
	}
}

// WithSchemaFieldFlag overrides the generated flag name.
func WithSchemaFieldFlag(name string) SchemaFieldOption {
	return func(options *schemaFieldOptions) { options.flagName = new(strings.TrimSpace(name)) }
}

// WithSchemaFieldUsage overrides the generated help text.
func WithSchemaFieldUsage(usage string) SchemaFieldOption {
	return func(options *schemaFieldOptions) { options.usage = new(usage) }
}

// WithSchemaFieldHidden controls whether the generated flag is hidden.
func WithSchemaFieldHidden(hidden bool) SchemaFieldOption {
	return func(options *schemaFieldOptions) { options.hidden = new(hidden) }
}

// WithSchemaFieldDeprecated marks a generated flag as deprecated.
func WithSchemaFieldDeprecated(message string) SchemaFieldOption {
	return func(options *schemaFieldOptions) { options.deprecated = new(message) }
}

// WithSchemaFieldRedacted controls default-value redaction in schema and help output.
func WithSchemaFieldRedacted(redacted bool) SchemaFieldOption {
	return func(options *schemaFieldOptions) { options.redacted = new(redacted) }
}

// WithSchemaFieldCLIEnabled controls whether BindFlags registers the field.
func WithSchemaFieldCLIEnabled(enabled bool) SchemaFieldOption {
	return func(options *schemaFieldOptions) { options.cliEnabled = new(enabled) }
}

// WithSchemaFieldParser overrides flag parsing for one field.
func WithSchemaFieldParser(parser SchemaValueParser) SchemaFieldOption {
	return func(options *schemaFieldOptions) { options.parser = parser }
}
