package channels

// Decoding the channel manifests the runtime emits, as strictly as the mda
// CLI's serde derives do: a field without a default must be present, only an
// `Option` may be null, and a struct marked `deny_unknown_fields` refuses keys
// it does not name. A field's `strict` tag says which:
//
//	strict:"required"   must be present and not null
//	strict:"nullable"   may be missing or null (an `Option`)
//	(no tag)            may be missing, but not null (`#[serde(default)]`)
//
// Keys are matched exactly, unlike encoding/json's case-insensitive match.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
)

// decodeManifest decodes value, a JSON value, into out, a pointer to a
// struct. denyUnknown refuses keys out does not name.
func decodeManifest(value any, out any, denyUnknown bool) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return decodeStrict(data, out, denyUnknown)
}

// decodeStrict is [decodeManifest] for JSON text. out must not have an
// UnmarshalJSON method of its own that calls this, or it recurses; such a
// type passes a pointer to a plain copy of itself.
func decodeStrict(data []byte, out any, denyUnknown bool) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		return fmt.Errorf("invalid type: %s, expected an object", describe(data))
	}
	typ := reflect.TypeOf(out).Elem()
	known := map[string]bool{}
	for i := range typ.NumField() {
		field := typ.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		known[name] = true
		raw, present := object[name]
		rule := field.Tag.Get("strict")
		if !present {
			if rule == "required" {
				return fmt.Errorf("missing field `%s`", name)
			}
			continue
		}
		if rule != "nullable" && string(raw) == "null" {
			return fmt.Errorf("invalid type: null, expected a value for field `%s`", name)
		}
	}
	if denyUnknown {
		for _, key := range slices.Sorted(maps.Keys(object)) {
			if !known[key] {
				return fmt.Errorf("unknown field `%s`", key)
			}
		}
	}
	return json.Unmarshal(data, out)
}

func describe(data []byte) string {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return "invalid JSON"
	}
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "sequence"
	default:
		return "map"
	}
}

// sortedKeys are the requirements' channel names in order, as the mda CLI's
// `BTreeMap` iterates them.
func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}

// stringField is the string at key of a JSON object, if it is one.
func stringField(value any, key string) (string, bool) {
	object, ok := value.(map[string]any)
	if !ok {
		return "", false
	}
	text, ok := object[key].(string)
	return text, ok
}

// marshalUnescaped is value as compact JSON with object keys sorted, as
// `serde_json` prints a `Value`: without encoding/json's HTML escaping.
func marshalUnescaped(value any) (string, error) {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSuffix(encoded.String(), "\n"), nil
}
