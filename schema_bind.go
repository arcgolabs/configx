package configx

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/pflag"
)

type bindFlagsOptions struct {
	separator string
}

// BindFlagsOption customizes pflag registration.
type BindFlagsOption func(*bindFlagsOptions)

// WithFlagNameSeparator replaces dots in derived flag names. Explicit flag
// names from metadata or field overrides are left unchanged.
func WithFlagNameSeparator(separator string) BindFlagsOption {
	return func(options *bindFlagsOptions) { options.separator = separator }
}

// BindFlags registers every CLI-enabled schema field on fs.
func (s *Schema) BindFlags(fs *pflag.FlagSet, opts ...BindFlagsOption) error {
	if s == nil {
		return schemaError("bind flags on nil schema", nil)
	}
	if fs == nil {
		return schemaError("flag set must not be nil", nil)
	}
	settings, err := buildBindFlagsOptions(opts)
	if err != nil {
		return err
	}
	names, err := s.preflightFlagNames(fs, settings)
	if err != nil {
		return err
	}
	return s.bindPreparedFlags(fs, names)
}

func buildBindFlagsOptions(options []BindFlagsOption) (bindFlagsOptions, error) {
	settings := bindFlagsOptions{separator: "."}
	for _, option := range options {
		if option != nil {
			option(&settings)
		}
	}
	if settings.separator == "" {
		return bindFlagsOptions{}, schemaError("flag name separator must not be empty", nil)
	}
	return settings, nil
}

func (s *Schema) bindPreparedFlags(fs *pflag.FlagSet, names map[int]string) error {
	for index := range s.bindings {
		binding := s.bindings[index]
		if !binding.field.CLIEnabled {
			continue
		}
		if err := bindSchemaFlag(fs, names[index], binding); err != nil {
			return err
		}
	}
	return nil
}

func (s *Schema) preflightFlagNames(
	fs *pflag.FlagSet,
	settings bindFlagsOptions,
) (map[int]string, error) {
	names := make(map[int]string, len(s.bindings))
	seen := make(map[string]string, len(s.bindings))
	for index := range s.bindings {
		binding := &s.bindings[index]
		if !binding.field.CLIEnabled {
			continue
		}
		name, err := schemaFlagName(binding, settings.separator)
		if err != nil {
			return nil, err
		}
		normalizedName := string(fs.GetNormalizeFunc()(fs, name))
		if existingPath, ok := seen[normalizedName]; ok {
			return nil, schemaFlagConflict(name, binding.field.Path, existingPath)
		}
		if fs.Lookup(name) != nil {
			return nil, schemaFlagConflict(name, binding.field.Path, "existing flag set")
		}
		seen[normalizedName] = binding.field.Path
		names[index] = name
	}
	return names, nil
}

func schemaFlagName(binding *schemaBinding, separator string) (string, error) {
	name := binding.field.FlagName
	if !binding.explicitFlag {
		name = strings.ReplaceAll(binding.field.Path, ".", separator)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", schemaError("generated flag name is empty", map[string]any{"path": binding.field.Path})
	}
	return name, nil
}

func bindSchemaFlag(fs *pflag.FlagSet, name string, binding schemaBinding) error {
	value := &schemaFlagValue{
		typeOf:   binding.typeOf,
		current:  cloneSchemaAny(binding.defaultValue),
		parser:   binding.parser,
		kind:     binding.field.Kind,
		redacted: binding.field.Redacted,
	}
	fs.Var(value, name, binding.field.Usage)
	flag := fs.Lookup(name)
	if flag == nil {
		return schemaError("registered flag cannot be found", map[string]any{"path": binding.field.Path})
	}
	flag.Annotations = map[string][]string{schemaPathAnnotation: {binding.loadPath}}
	if binding.field.Redacted {
		flag.DefValue = redactedDefault
	}
	if binding.field.Kind == SchemaKindBool {
		flag.NoOptDefVal = "true"
	}
	if binding.field.Hidden {
		if err := fs.MarkHidden(name); err != nil {
			return schemaError("mark flag hidden", map[string]any{"path": binding.field.Path, "cause": err})
		}
	}
	if binding.field.Deprecated != "" {
		if err := fs.MarkDeprecated(name, binding.field.Deprecated); err != nil {
			return schemaError("mark flag deprecated", map[string]any{"path": binding.field.Path, "cause": err})
		}
	}
	return nil
}

type schemaFlagValue struct {
	typeOf   reflect.Type
	current  any
	parser   SchemaValueParser
	kind     SchemaKind
	redacted bool
	set      bool
}

func (v *schemaFlagValue) Set(raw string) error {
	parsed, err := v.parse(raw)
	if err != nil {
		if v.redacted {
			return fmt.Errorf("parse %s flag", v.kind)
		}
		return fmt.Errorf("parse %s flag: %w", v.kind, err)
	}
	reflected, err := schemaReflectValue(parsed, v.typeOf)
	if err != nil {
		return err
	}
	if v.set && v.typeOf.Kind() == reflect.Slice {
		current := reflect.ValueOf(v.current)
		reflected = reflect.AppendSlice(current, reflected)
	}
	v.current = reflected.Interface()
	v.set = true
	return nil
}

func (v *schemaFlagValue) String() string {
	if v == nil {
		return ""
	}
	if v.redacted {
		return redactedDefault
	}
	return formatSchemaAny(v.current)
}

func (v *schemaFlagValue) Type() string {
	if v == nil {
		return string(SchemaKindText)
	}
	return string(v.kind)
}

func (v *schemaFlagValue) Get() any {
	if v == nil {
		return nil
	}
	return cloneSchemaAny(v.current)
}

func (v *schemaFlagValue) parse(raw string) (any, error) {
	if v.parser != nil {
		return v.parser(raw)
	}
	return parseSchemaValue(raw, v.typeOf)
}

func cloneSchemaAny(value any) any {
	if value == nil {
		return nil
	}
	return cloneSchemaReflectValue(reflect.ValueOf(value)).Interface()
}

func schemaFlagConflict(name, path, existing string) error {
	cause := errors.Join(ErrSchema, ErrSchemaFlagConflict)
	return fmt.Errorf("flag=%s path=%s existing=%s: %w", name, path, existing, cause)
}
