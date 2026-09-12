package configx

import (
	"reflect"

	"github.com/spf13/pflag"
)

var _ pflag.SliceValue = (*schemaSliceFlagValue)(nil)

type schemaSliceFlagValue struct {
	*schemaFlagValue
}

func (v *schemaSliceFlagValue) String() string {
	if v == nil || v.schemaFlagValue == nil {
		return ""
	}
	if v.redacted {
		return redactedDefault
	}
	return "[" + formatSchemaAny(v.current) + "]"
}

func (v *schemaSliceFlagValue) Append(raw string) error {
	if v.parser != nil {
		return v.assign(raw, true)
	}
	return v.assign(formatCSV([]string{raw}), true)
}

func (v *schemaSliceFlagValue) Replace(values []string) error {
	if v.parser != nil {
		return v.replaceParsedItems(values)
	}
	return v.assign(formatCSV(values), false)
}

func (v *schemaSliceFlagValue) GetSlice() []string {
	if v == nil || v.schemaFlagValue == nil {
		return nil
	}
	reflected := dereferenceSchemaSlice(reflect.ValueOf(v.current), v.typeOf)
	if v.redacted {
		return redactedSchemaSlice(reflected.Len())
	}
	values := make([]string, 0, reflected.Len())
	for index := range reflected.Len() {
		values = append(values, formatSchemaAny(reflected.Index(index).Interface()))
	}
	return values
}

func (v *schemaSliceFlagValue) replaceParsedItems(values []string) error {
	replacement := emptySchemaSliceValue(v.typeOf)
	for _, raw := range values {
		parsed, err := v.parseReflect(raw)
		if err != nil {
			return err
		}
		replacement = appendSchemaSliceValues(
			replacement.Interface(),
			parsed,
			v.typeOf,
		)
	}
	v.current = replacement.Interface()
	return nil
}

func isSchemaSliceKind(kind SchemaKind) bool {
	switch kind {
	case SchemaKindBoolSlice, SchemaKindStringSlice, SchemaKindIntSlice,
		SchemaKindUintSlice, SchemaKindFloatSlice, SchemaKindDurationSlice,
		SchemaKindTextSlice:
		return true
	case SchemaKindBool, SchemaKindString, SchemaKindInt, SchemaKindUint,
		SchemaKindFloat, SchemaKindDuration, SchemaKindText:
		return false
	}
	return false
}

func appendSchemaSliceValues(
	current any,
	next reflect.Value,
	typeOf reflect.Type,
) reflect.Value {
	currentSlice := dereferenceSchemaSlice(reflect.ValueOf(current), typeOf)
	nextSlice := dereferenceSchemaSlice(next, typeOf)
	combined := reflect.MakeSlice(
		currentSlice.Type(),
		0,
		currentSlice.Len()+nextSlice.Len(),
	)
	combined = reflect.AppendSlice(combined, currentSlice)
	combined = reflect.AppendSlice(combined, nextSlice)
	return wrapSchemaSlice(combined, typeOf)
}

func emptySchemaSliceValue(typeOf reflect.Type) reflect.Value {
	empty := reflect.MakeSlice(dereferenceType(typeOf), 0, 0)
	return wrapSchemaSlice(empty, typeOf)
}

func dereferenceSchemaSlice(value reflect.Value, typeOf reflect.Type) reflect.Value {
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.MakeSlice(dereferenceType(typeOf), 0, 0)
		}
		value = value.Elem()
	}
	return value
}

func wrapSchemaSlice(value reflect.Value, typeOf reflect.Type) reflect.Value {
	if typeOf.Kind() != reflect.Pointer {
		return value
	}
	wrapped := reflect.New(typeOf.Elem())
	wrapped.Elem().Set(wrapSchemaSlice(value, typeOf.Elem()))
	return wrapped
}

func redactedSchemaSlice(length int) []string {
	values := make([]string, length)
	for index := range values {
		values[index] = redactedDefault
	}
	return values
}
