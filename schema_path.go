package configx

import (
	"reflect"
	"strings"
)

func schemaRoot[T any](defaults T) (reflect.Type, reflect.Value, error) {
	value := reflect.ValueOf(defaults)
	typeOf := reflect.TypeFor[T]()
	if value.IsValid() {
		typeOf = value.Type()
	}
	for typeOf.Kind() == reflect.Pointer {
		typeOf = typeOf.Elem()
		if value.IsValid() && !value.IsNil() {
			value = value.Elem()
		} else {
			value = reflect.Zero(typeOf)
		}
	}
	if typeOf.Kind() != reflect.Struct {
		return nil, reflect.Value{}, schemaError(
			"schema root must be a struct or struct pointer",
			map[string]any{"type": typeOf.String()},
		)
	}
	return typeOf, value, nil
}

type schemaPathTag struct {
	name     string
	explicit bool
	skip     bool
	squash   bool
}

func parseSchemaPathTag(field reflect.StructField, tagName string) schemaPathTag {
	raw, explicit := field.Tag.Lookup(tagName)
	parts := strings.Split(raw, ",")
	name := strings.TrimSpace(parts[0])
	if explicit && name == "-" {
		return schemaPathTag{explicit: true, skip: true}
	}
	if name == "" {
		name = strings.ToLower(field.Name)
	}
	squash := false
	for _, option := range parts[1:] {
		if strings.TrimSpace(option) == "squash" {
			squash = true
		}
	}
	return schemaPathTag{
		name:     name,
		explicit: explicit && strings.TrimSpace(parts[0]) != "",
		squash:   squash,
	}
}

func schemaFieldValue(parent reflect.Value, index int, typeOf reflect.Type) reflect.Value {
	if parent.IsValid() && parent.Kind() == reflect.Struct {
		return parent.Field(index)
	}
	return reflect.Zero(typeOf)
}

func dereferenceType(typeOf reflect.Type) reflect.Type {
	for typeOf.Kind() == reflect.Pointer {
		typeOf = typeOf.Elem()
	}
	return typeOf
}

func dereferenceValue(value reflect.Value, typeOf reflect.Type) reflect.Value {
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Zero(typeOf)
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return reflect.Zero(typeOf)
	}
	return value
}

func isSchemaStruct(typeOf reflect.Type) bool {
	return typeOf.Kind() == reflect.Struct && typeOf != durationType && !supportsTextUnmarshal(typeOf)
}

func joinSchemaPath(prefix, name string) string {
	if prefix == "" {
		return normalizeArgsPath(name)
	}
	return normalizeArgsPath(prefix + "." + name)
}
