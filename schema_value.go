package configx

import (
	"encoding"
	"encoding/csv"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var schemaKindsByReflectKind = map[reflect.Kind]SchemaKind{
	reflect.Invalid:       "",
	reflect.Bool:          SchemaKindBool,
	reflect.Int:           SchemaKindInt,
	reflect.Int8:          SchemaKindInt,
	reflect.Int16:         SchemaKindInt,
	reflect.Int32:         SchemaKindInt,
	reflect.Int64:         SchemaKindInt,
	reflect.Uint:          SchemaKindUint,
	reflect.Uint8:         SchemaKindUint,
	reflect.Uint16:        SchemaKindUint,
	reflect.Uint32:        SchemaKindUint,
	reflect.Uint64:        SchemaKindUint,
	reflect.Uintptr:       SchemaKindUint,
	reflect.Float32:       SchemaKindFloat,
	reflect.Float64:       SchemaKindFloat,
	reflect.Complex64:     "",
	reflect.Complex128:    "",
	reflect.Array:         "",
	reflect.Chan:          "",
	reflect.Func:          "",
	reflect.Interface:     "",
	reflect.Map:           "",
	reflect.Pointer:       "",
	reflect.Slice:         "",
	reflect.String:        SchemaKindString,
	reflect.Struct:        "",
	reflect.UnsafePointer: "",
}

func supportsTextUnmarshal(typeOf reflect.Type) bool {
	if typeOf.Kind() == reflect.Interface {
		return false
	}
	if typeOf.Implements(textUnmarshalerType) {
		return true
	}
	return typeOf.Kind() != reflect.Pointer && reflect.PointerTo(typeOf).Implements(textUnmarshalerType)
}

func schemaKind(typeOf reflect.Type) (SchemaKind, error) {
	if typeOf.Kind() == reflect.Pointer {
		return schemaKind(typeOf.Elem())
	}
	if typeOf == durationType {
		return SchemaKindDuration, nil
	}
	if supportsTextUnmarshal(typeOf) {
		return SchemaKindText, nil
	}
	if typeOf.Kind() == reflect.Slice {
		kind, err := schemaKind(typeOf.Elem())
		if err != nil {
			return "", err
		}
		return sliceSchemaKind(kind)
	}
	kind := schemaKindsByReflectKind[typeOf.Kind()]
	if kind == "" {
		return "", errors.New("unsupported type")
	}
	return kind, nil
}

func sliceSchemaKind(kind SchemaKind) (SchemaKind, error) {
	sliceKinds := map[SchemaKind]SchemaKind{
		SchemaKindBool:          SchemaKindBoolSlice,
		SchemaKindString:        SchemaKindStringSlice,
		SchemaKindInt:           SchemaKindIntSlice,
		SchemaKindUint:          SchemaKindUintSlice,
		SchemaKindFloat:         SchemaKindFloatSlice,
		SchemaKindDuration:      SchemaKindDurationSlice,
		SchemaKindText:          SchemaKindTextSlice,
		SchemaKindBoolSlice:     "",
		SchemaKindStringSlice:   "",
		SchemaKindIntSlice:      "",
		SchemaKindUintSlice:     "",
		SchemaKindFloatSlice:    "",
		SchemaKindDurationSlice: "",
		SchemaKindTextSlice:     "",
	}
	sliceKind, ok := sliceKinds[kind]
	if !ok {
		return "", errors.New("unsupported slice element type")
	}
	return sliceKind, nil
}

func parseSchemaValue(raw string, typeOf reflect.Type) (any, error) {
	if typeOf.Kind() == reflect.Pointer {
		return parseSchemaPointer(raw, typeOf)
	}
	if typeOf == durationType {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("parse duration: %w", err)
		}
		return parsed, nil
	}
	if supportsTextUnmarshal(typeOf) {
		return parseTextValue(raw, typeOf)
	}
	if typeOf.Kind() == reflect.Slice {
		return parseSchemaSlice(raw, typeOf)
	}

	return parseSchemaScalar(raw, typeOf)
}

func parseSchemaPointer(raw string, typeOf reflect.Type) (any, error) {
	parsed, err := parseSchemaValue(raw, typeOf.Elem())
	if err != nil {
		return nil, err
	}
	value, err := schemaReflectValue(parsed, typeOf.Elem())
	if err != nil {
		return nil, err
	}
	pointer := reflect.New(typeOf.Elem())
	pointer.Elem().Set(value)
	return pointer.Interface(), nil
}

func parseSchemaScalar(raw string, typeOf reflect.Type) (any, error) {
	value := reflect.New(typeOf).Elem()
	kind, err := schemaKind(typeOf)
	if err != nil {
		return nil, err
	}
	switch kind {
	case SchemaKindBool:
		return parseSchemaBool(raw, value)
	case SchemaKindString:
		value.SetString(raw)
		return value.Interface(), nil
	case SchemaKindInt:
		return parseSchemaInt(raw, value)
	case SchemaKindUint:
		return parseSchemaUint(raw, value)
	case SchemaKindFloat:
		return parseSchemaFloat(raw, value)
	case SchemaKindDuration, SchemaKindText,
		SchemaKindBoolSlice, SchemaKindStringSlice, SchemaKindIntSlice,
		SchemaKindUintSlice, SchemaKindFloatSlice, SchemaKindDurationSlice,
		SchemaKindTextSlice:
		return nil, errors.New("unsupported type")
	}
	return nil, errors.New("unsupported type")
}

func parseSchemaBool(raw string, value reflect.Value) (any, error) {
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, fmt.Errorf("parse bool: %w", err)
	}
	value.SetBool(parsed)
	return value.Interface(), nil
}

func parseSchemaInt(raw string, value reflect.Value) (any, error) {
	parsed, err := strconv.ParseInt(raw, 0, value.Type().Bits())
	if err != nil {
		return nil, fmt.Errorf("parse signed integer: %w", err)
	}
	value.SetInt(parsed)
	return value.Interface(), nil
}

func parseSchemaUint(raw string, value reflect.Value) (any, error) {
	parsed, err := strconv.ParseUint(raw, 0, value.Type().Bits())
	if err != nil {
		return nil, fmt.Errorf("parse unsigned integer: %w", err)
	}
	value.SetUint(parsed)
	return value.Interface(), nil
}

func parseSchemaFloat(raw string, value reflect.Value) (any, error) {
	parsed, err := strconv.ParseFloat(raw, value.Type().Bits())
	if err != nil {
		return nil, fmt.Errorf("parse float: %w", err)
	}
	value.SetFloat(parsed)
	return value.Interface(), nil
}

func parseTextValue(raw string, typeOf reflect.Type) (any, error) {
	var target reflect.Value
	var result reflect.Value
	if typeOf.Kind() == reflect.Pointer {
		target = reflect.New(typeOf.Elem())
		result = target
	} else {
		target = reflect.New(typeOf)
		result = target.Elem()
	}
	unmarshaler, ok := reflect.TypeAssert[encoding.TextUnmarshaler](target)
	if !ok {
		return nil, errors.New("type does not implement encoding.TextUnmarshaler")
	}
	if err := unmarshaler.UnmarshalText([]byte(raw)); err != nil {
		return nil, fmt.Errorf("unmarshal text: %w", err)
	}
	return result.Interface(), nil
}

func parseSchemaSlice(raw string, typeOf reflect.Type) (any, error) {
	if raw == "" {
		return reflect.MakeSlice(typeOf, 0, 0).Interface(), nil
	}
	items, err := csv.NewReader(strings.NewReader(raw)).Read()
	if err != nil {
		return nil, fmt.Errorf("parse CSV value: %w", err)
	}
	result := reflect.MakeSlice(typeOf, 0, len(items))
	for _, item := range items {
		parsed, err := parseSchemaValue(item, typeOf.Elem())
		if err != nil {
			return nil, err
		}
		value, err := schemaReflectValue(parsed, typeOf.Elem())
		if err != nil {
			return nil, err
		}
		result = reflect.Append(result, value)
	}
	return result.Interface(), nil
}

func schemaReflectValue(value any, typeOf reflect.Type) (reflect.Value, error) {
	if value == nil {
		return reflect.Value{}, errors.New("parser returned nil")
	}
	reflected := reflect.ValueOf(value)
	if reflected.Type().AssignableTo(typeOf) {
		return reflected, nil
	}
	if reflected.Type().ConvertibleTo(typeOf) {
		return reflected.Convert(typeOf), nil
	}
	return reflect.Value{}, fmt.Errorf("parser returned %s, expected %s", reflected.Type(), typeOf)
}
