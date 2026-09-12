package configx

import (
	"encoding"
	"encoding/csv"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func formatSchemaValue(value reflect.Value) string {
	if !value.IsValid() {
		return ""
	}
	return formatSchemaAny(value.Interface())
}

func formatSchemaAny(value any) string {
	if value != nil && supportsTextUnmarshal(reflect.TypeOf(value)) {
		if text, ok := formatSchemaText(value); ok {
			return text
		}
	}
	reflected, ok := schemaValueForFormatting(value)
	if !ok {
		return ""
	}
	if reflected.Type() == durationType {
		duration, ok := reflect.TypeAssert[time.Duration](reflected)
		if ok {
			return duration.String()
		}
	}
	if reflected.Kind() == reflect.Slice {
		return formatSchemaSlice(reflected)
	}
	return formatSchemaScalar(reflected, value)
}

func formatSchemaText(value any) (string, bool) {
	if value == nil {
		return "", false
	}
	reflected := reflect.ValueOf(value)
	if reflected.Kind() == reflect.Pointer && reflected.IsNil() {
		return "", false
	}
	if reflected.Kind() == reflect.Slice && reflected.IsNil() {
		return "", false
	}
	if text, ok := marshalSchemaText(reflected); ok {
		return text, true
	}
	if reflected.Kind() != reflect.Pointer {
		pointer := reflect.New(reflected.Type())
		pointer.Elem().Set(reflected)
		if text, ok := marshalSchemaText(pointer); ok {
			return text, true
		}
	}
	stringer, ok := reflect.TypeAssert[fmt.Stringer](reflected)
	if ok {
		return stringer.String(), true
	}
	return "", false
}

func marshalSchemaText(value reflect.Value) (string, bool) {
	marshaler, ok := reflect.TypeAssert[encoding.TextMarshaler](value)
	if !ok {
		return "", false
	}
	text, err := marshaler.MarshalText()
	if err != nil {
		return "", false
	}
	return string(text), true
}

func schemaValueForFormatting(value any) (reflect.Value, bool) {
	if value == nil {
		return reflect.Value{}, false
	}
	reflected := reflect.ValueOf(value)
	for reflected.Kind() == reflect.Pointer {
		if reflected.IsNil() {
			return reflect.Value{}, false
		}
		reflected = reflected.Elem()
	}
	return reflected, true
}

func formatSchemaSlice(reflected reflect.Value) string {
	items := make([]string, 0, reflected.Len())
	for index := range reflected.Len() {
		items = append(items, formatSchemaAny(reflected.Index(index).Interface()))
	}
	return formatCSV(items)
}

func formatSchemaScalar(reflected reflect.Value, original any) string {
	//nolint:exhaustive // Unsupported scalar kinds deliberately fall back to fmt.Sprint.
	switch reflected.Kind() {
	case reflect.Bool:
		return strconv.FormatBool(reflected.Bool())
	case reflect.String:
		return reflected.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(reflected.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(reflected.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(reflected.Float(), 'g', -1, reflected.Type().Bits())
	default:
		return fmt.Sprint(original)
	}
}

func formatCSV(items []string) string {
	var builder strings.Builder
	writer := csv.NewWriter(&builder)
	if err := writer.Write(items); err != nil {
		return strings.Join(items, ",")
	}
	writer.Flush()
	return strings.TrimSuffix(builder.String(), "\n")
}

func cloneSchemaValue(value reflect.Value) any {
	if !value.IsValid() {
		return nil
	}
	return cloneSchemaReflectValue(value).Interface()
}

func cloneSchemaReflectValue(value reflect.Value) reflect.Value {
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.New(value.Type().Elem())
		cloned.Elem().Set(cloneSchemaReflectValue(value.Elem()))
		return cloned
	}
	if value.Kind() == reflect.Slice {
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for index := range value.Len() {
			cloned.Index(index).Set(cloneSchemaReflectValue(value.Index(index)))
		}
		return cloned
	}
	return value
}
