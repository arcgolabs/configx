package configx

import "reflect"

type schemaBuilder struct {
	settings schemaOptions
	seen     map[string]struct{}
	seenLoad map[string]string
	matched  map[string]struct{}
	visiting map[reflect.Type]bool
	bindings []schemaBinding
}

type schemaWalkContext struct {
	typeOf     reflect.Type
	value      reflect.Value
	prefix     string
	loadPrefix string
	cliEnabled bool
}

type schemaFieldContext struct {
	typeOf     reflect.StructField
	value      reflect.Value
	prefix     string
	name       string
	squash     bool
	loadPrefix string
	loadName   string
	loadSquash bool
	cliEnabled bool
	metadata   schemaMetadata
}

func (b *schemaBuilder) validateOverrides() error {
	for path := range b.settings.overrides {
		if _, ok := b.matched[path]; !ok {
			return schemaError("field override does not match a schema path", map[string]any{"path": path})
		}
	}
	return nil
}

func (b *schemaBuilder) walk(context schemaWalkContext) error {
	if b.visiting[context.typeOf] {
		return schemaError("recursive struct types are unsupported", map[string]any{"type": context.typeOf.String()})
	}
	b.visiting[context.typeOf] = true
	defer delete(b.visiting, context.typeOf)

	for index := range context.typeOf.NumField() {
		fieldType := context.typeOf.Field(index)
		fieldValue := schemaFieldValue(context.value, index, fieldType.Type)
		if err := b.walkField(fieldType, fieldValue, context); err != nil {
			return err
		}
	}
	return nil
}

func (b *schemaBuilder) walkField(
	fieldType reflect.StructField,
	fieldValue reflect.Value,
	context schemaWalkContext,
) error {
	if !fieldType.IsExported() {
		return nil
	}
	pathTag := parseSchemaPathTag(fieldType, b.settings.tag)
	loadTag := parseSchemaPathTag(fieldType, defaultSchemaTag)
	if pathTag.skip || loadTag.skip {
		return nil
	}
	metadata, err := parseSchemaMetadata(fieldType.Tag.Get(b.settings.metadataTag))
	if err != nil {
		return schemaError("parse field metadata", map[string]any{"field": fieldType.Name, "cause": err})
	}
	fieldCLIEnabled := context.cliEnabled && metadata.cliEnabled
	field := schemaFieldContext{
		typeOf:     fieldType,
		value:      fieldValue,
		prefix:     context.prefix,
		name:       pathTag.name,
		squash:     pathTag.squash,
		loadPrefix: context.loadPrefix,
		loadName:   loadTag.name,
		loadSquash: loadTag.squash,
		cliEnabled: fieldCLIEnabled,
		metadata:   metadata,
	}
	baseType := dereferenceType(fieldType.Type)
	if isSchemaStruct(baseType) {
		return b.walkNestedField(field, baseType, pathTag.explicit, loadTag.explicit)
	}
	return b.addLeafField(field)
}

func (b *schemaBuilder) walkNestedField(
	field schemaFieldContext,
	baseType reflect.Type,
	explicit bool,
	loadExplicit bool,
) error {
	nestedPrefix := field.prefix
	if !field.squash && (!field.typeOf.Anonymous || explicit) {
		nestedPrefix = joinSchemaPath(field.prefix, field.name)
	}
	nestedLoadPrefix := field.loadPrefix
	if !field.loadSquash && (!field.typeOf.Anonymous || loadExplicit) {
		nestedLoadPrefix = joinSchemaPath(field.loadPrefix, field.loadName)
	}
	nestedValue := dereferenceValue(field.value, baseType)
	return b.walk(schemaWalkContext{
		typeOf:     baseType,
		value:      nestedValue,
		prefix:     nestedPrefix,
		loadPrefix: nestedLoadPrefix,
		cliEnabled: field.cliEnabled,
	})
}

func (b *schemaBuilder) addLeafField(field schemaFieldContext) error {
	path := joinSchemaPath(field.prefix, field.name)
	loadPath := joinSchemaPath(field.loadPrefix, field.loadName)
	if path == "" {
		return schemaError("field path is empty", map[string]any{"field": field.typeOf.Name})
	}
	if _, exists := b.seen[path]; exists {
		return schemaError("duplicate schema path", map[string]any{"path": path})
	}
	b.seen[path] = struct{}{}
	if existingPath, exists := b.seenLoad[loadPath]; exists {
		return schemaError(
			"duplicate koanf load path",
			map[string]any{"path": path, "load_path": loadPath, "existing_path": existingPath},
		)
	}
	b.seenLoad[loadPath] = path

	binding, err := newSchemaBinding(field, path, loadPath)
	if err != nil {
		return err
	}
	if err := b.applyOverrides(&binding); err != nil {
		return err
	}
	if binding.field.Redacted {
		binding.defaultValue = reflect.Zero(binding.typeOf).Interface()
	}
	b.bindings = append(b.bindings, binding)
	return nil
}

func newSchemaBinding(
	field schemaFieldContext,
	path, loadPath string,
) (schemaBinding, error) {
	kind, err := schemaKind(field.typeOf.Type)
	if err != nil {
		return schemaBinding{}, schemaError(
			"unsupported schema field type",
			map[string]any{"path": path, "type": field.typeOf.Type.String()},
		)
	}
	binding := schemaBinding{
		field: SchemaField{
			Path:       path,
			FlagName:   path,
			Usage:      field.metadata.usage,
			Type:       field.typeOf.Type.String(),
			Default:    formatSchemaValue(field.value),
			Kind:       kind,
			Hidden:     field.metadata.hidden,
			Deprecated: field.metadata.deprecated,
			Redacted:   field.metadata.redacted,
			CLIEnabled: field.cliEnabled,
		},
		loadPath:     loadPath,
		typeOf:       field.typeOf.Type,
		defaultValue: cloneSchemaValue(field.value),
	}
	if field.metadata.flagName != "" {
		binding.field.FlagName = field.metadata.flagName
		binding.explicitFlag = true
	}
	if binding.field.Redacted {
		binding.field.Default = redactedDefault
	}
	return binding, nil
}

func (b *schemaBuilder) applyOverrides(binding *schemaBinding) error {
	options, ok := b.settings.overrides[binding.field.Path]
	if !ok {
		return nil
	}
	b.matched[binding.field.Path] = struct{}{}
	override := collectSchemaFieldOptions(options)
	return applySchemaFieldOptions(binding, override)
}

func collectSchemaFieldOptions(options []SchemaFieldOption) schemaFieldOptions {
	override := schemaFieldOptions{}
	for _, option := range options {
		if option != nil {
			option(&override)
		}
	}
	return override
}

func applySchemaFieldOptions(binding *schemaBinding, override schemaFieldOptions) error {
	if override.flagName != nil {
		if *override.flagName == "" {
			return schemaError("flag name must not be empty", map[string]any{"path": binding.field.Path})
		}
		binding.field.FlagName = *override.flagName
		binding.explicitFlag = true
	}
	if override.usage != nil {
		binding.field.Usage = *override.usage
	}
	if override.hidden != nil {
		binding.field.Hidden = *override.hidden
	}
	if override.deprecated != nil {
		binding.field.Deprecated = *override.deprecated
	}
	if override.redacted != nil {
		binding.field.Redacted = *override.redacted
	}
	if override.cliEnabled != nil {
		binding.field.CLIEnabled = *override.cliEnabled
	}
	if override.parser != nil {
		binding.parser = override.parser
	}
	binding.field.Default = formatSchemaAny(binding.defaultValue)
	if binding.field.Redacted {
		binding.field.Default = redactedDefault
	}
	return nil
}
