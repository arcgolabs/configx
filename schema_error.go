package configx

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

func schemaError(message string, attributes map[string]any) error {
	causes := []error{ErrSchema, errors.New(message)}
	parts := make([]string, 0, len(attributes))
	for _, key := range slices.Sorted(maps.Keys(attributes)) {
		value := attributes[key]
		if cause, ok := value.(error); ok {
			causes = append(causes, cause)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%v", key, value))
	}
	err := errors.Join(causes...)
	if len(parts) == 0 {
		return err
	}
	return fmt.Errorf("%s: %w", strings.Join(parts, " "), err)
}
