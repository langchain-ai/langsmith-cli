package api

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
)

func parseFields(rawFields, typedFields []string) (map[string]any, error) {
	params := make(map[string]any)
	for _, field := range rawFields {
		if err := parseField(params, field, false); err != nil {
			return nil, err
		}
	}
	for _, field := range typedFields {
		if err := parseField(params, field, true); err != nil {
			return nil, err
		}
	}
	return params, nil
}

func parseField(params map[string]any, field string, typed bool) error {
	var valueIndex int
	var keys []string
	keyStartAt := 0
parseLoop:
	for i, r := range field {
		switch r {
		case '[':
			if keyStartAt == 0 {
				keys = append(keys, field[0:i])
			}
			keyStartAt = i + 1
		case ']':
			keys = append(keys, field[keyStartAt:i])
		case '=':
			if keyStartAt == 0 {
				keys = append(keys, field[0:i])
			}
			valueIndex = i + 1
			break parseLoop
		}
	}
	if len(keys) == 0 || keys[0] == "" {
		if valueIndex == 0 && !strings.ContainsAny(field, "[]") {
			return fmt.Errorf("field %q requires a value separated by '='", field)
		}
		return fmt.Errorf("invalid field key: %q", field)
	}

	key := field
	var value any
	if valueIndex == 0 {
		if keys[len(keys)-1] != "" {
			return fmt.Errorf("field %q requires a value separated by '='", key)
		}
	} else {
		key = field[:valueIndex-1]
		value = field[valueIndex:]
	}

	if typed && value != nil {
		typedValue, err := parseTypedFieldValue(value.(string))
		if err != nil {
			return fmt.Errorf("error parsing %q value: %w", key, err)
		}
		value = typedValue
	}

	return setFieldValue(params, keys, value)
}

func parseTypedFieldValue(value string) (any, error) {
	if strings.HasPrefix(value, "@") {
		data, err := readFieldFile(value[1:])
		if err != nil {
			return nil, err
		}
		return string(data), nil
	}
	if value == "true" {
		return true, nil
	}
	if value == "false" {
		return false, nil
	}
	if value == "null" {
		return nil, nil
	}
	if n, err := strconv.Atoi(value); err == nil {
		return n, nil
	}
	return value, nil
}

func readFieldFile(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

func setFieldValue(params map[string]any, keys []string, value any) error {
	current := params
	isArray := false
	var subkey string
	for _, key := range keys {
		if key == "" {
			isArray = true
			continue
		}
		if subkey != "" {
			var err error
			if isArray {
				current, err = addFieldSlice(current, subkey, key)
				isArray = false
			} else {
				current, err = addFieldMap(current, subkey)
			}
			if err != nil {
				return err
			}
		}
		subkey = key
	}

	if isArray {
		if value == nil {
			current[subkey] = []any{}
			return nil
		}
		if existing, ok := current[subkey]; ok {
			slice, ok := existing.([]any)
			if !ok {
				return fmt.Errorf("expected array type under %q, got %T", subkey, existing)
			}
			current[subkey] = append(slice, value)
			return nil
		}
		current[subkey] = []any{value}
		return nil
	}
	if _, exists := current[subkey]; exists {
		return fmt.Errorf("unexpected override existing field under %q", subkey)
	}
	current[subkey] = value
	return nil
}

func addFieldMap(m map[string]any, key string) (map[string]any, error) {
	if existing, ok := m[key]; ok {
		next, ok := existing.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected object under %q, got %T", key, existing)
		}
		return next, nil
	}
	next := make(map[string]any)
	m[key] = next
	return next, nil
}

func addFieldSlice(m map[string]any, prevKey, newKey string) (map[string]any, error) {
	if existing, ok := m[prevKey]; ok {
		slice, ok := existing.([]any)
		if !ok {
			return nil, fmt.Errorf("expected array type under %q, got %T", prevKey, existing)
		}
		if len(slice) > 0 {
			last := slice[len(slice)-1]
			if lastMap, ok := last.(map[string]any); ok {
				if _, exists := lastMap[newKey]; !exists || reflect.TypeOf(lastMap[newKey]).Kind() == reflect.Slice {
					return lastMap, nil
				}
			}
		}
		next := make(map[string]any)
		m[prevKey] = append(slice, next)
		return next, nil
	}
	next := make(map[string]any)
	m[prevKey] = []any{next}
	return next, nil
}
