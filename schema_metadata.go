package configx

import (
	"errors"
	"fmt"
	"strings"
)

type schemaMetadata struct {
	flagName   string
	usage      string
	hidden     bool
	deprecated string
	redacted   bool
	cliEnabled bool
}

func parseSchemaMetadata(raw string) (schemaMetadata, error) {
	metadata := schemaMetadata{cliEnabled: true}
	for part := range strings.SplitSeq(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, hasValue := strings.Cut(part, "=")
		if err := applySchemaMetadata(&metadata, strings.TrimSpace(key), value, hasValue); err != nil {
			return schemaMetadata{}, err
		}
	}
	return metadata, nil
}

func applySchemaMetadata(metadata *schemaMetadata, key, value string, hasValue bool) error {
	switch key {
	case "flag":
		parsed, err := requiredMetadataValue("flag", value, hasValue)
		if err != nil {
			return err
		}
		metadata.flagName = parsed
	case "usage":
		if !hasValue {
			return errors.New("usage metadata requires a value")
		}
		metadata.usage = strings.TrimSpace(value)
	case "hidden":
		metadata.hidden = true
	case "deprecated":
		metadata.deprecated = metadataValueOr(value, hasValue, "deprecated")
	case "redact":
		metadata.redacted = true
	case "nocli", "-":
		metadata.cliEnabled = false
	default:
		return fmt.Errorf("unknown metadata option %q", key)
	}
	return nil
}

func requiredMetadataValue(key, value string, hasValue bool) (string, error) {
	parsed := strings.TrimSpace(value)
	if !hasValue || parsed == "" {
		return "", fmt.Errorf("%s metadata requires a value", key)
	}
	return parsed, nil
}

func metadataValueOr(value string, hasValue bool, fallback string) string {
	parsed := strings.TrimSpace(value)
	if !hasValue || parsed == "" {
		return fallback
	}
	return parsed
}
