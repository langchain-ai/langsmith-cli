package compile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
)

// The JSON the Rust CLI reads and writes is `serde_json` with `preserve_order`:
// objects keep their keys in document order, `remove` swaps the last key into
// the removed one's place, and `to_string_pretty` indents by two spaces. The
// generated manifests must match it byte for byte, so this file reproduces
// those semantics instead of using `encoding/json` maps.

// jsonObject is a JSON object that keeps its keys in insertion order, like
// `serde_json::Map` backed by an `IndexMap`.
//
// Values are nil, bool, string, [jsonNumber], []any or *jsonObject.
type jsonObject struct {
	keys   []string
	values map[string]any
}

func newJSONObject() *jsonObject {
	return &jsonObject{values: map[string]any{}}
}

// insert sets key to value, in place when key is present and last otherwise.
func (o *jsonObject) insert(key string, value any) {
	if _, ok := o.values[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
}

// entry returns the value of key, inserting value last when key is absent.
func (o *jsonObject) entry(key string, value any) any {
	if existing, ok := o.values[key]; ok {
		return existing
	}
	o.insert(key, value)
	return value
}

// remove removes key the way `serde_json::Map::remove` does with
// `preserve_order`: the last key takes the removed key's place.
func (o *jsonObject) remove(key string) bool {
	index := slices.Index(o.keys, key)
	if index < 0 {
		return false
	}
	last := len(o.keys) - 1
	o.keys[index] = o.keys[last]
	o.keys = o.keys[:last]
	delete(o.values, key)
	return true
}

// sortKeys orders the keys by their bytes, like `serde_json::Map::sort_keys`.
func (o *jsonObject) sortKeys() {
	slices.Sort(o.keys)
}

// object returns the object value of key.
func (o *jsonObject) object(key string) (*jsonObject, bool) {
	value, ok := o.values[key].(*jsonObject)
	return value, ok
}

// str returns the string value of key.
func (o *jsonObject) str(key string) (string, bool) {
	value, ok := o.values[key].(string)
	return value, ok
}

// jsonNumber is a JSON number as `serde_json` prints it after reading it: an
// integer when it fits in 64 bits, otherwise the shortest float.
type jsonNumber string

// parseJSON reads text into ordered values, rejecting trailing data.
func parseJSON(text []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(text))
	decoder.UseNumber()
	value, err := decodeJSONValue(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("trailing characters")
		}
		return nil, err
	}
	return value, nil
}

// parseJSONObject reads text that must hold a JSON object.
func parseJSONObject(text []byte) (*jsonObject, bool, error) {
	value, err := parseJSON(text)
	if err != nil {
		return nil, false, err
	}
	object, ok := value.(*jsonObject)
	return object, ok, nil
}

func decodeJSONValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, err
	}
	switch token := token.(type) {
	case json.Delim:
		switch token {
		case '{':
			object := newJSONObject()
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyToken.(string)
				if !ok {
					return nil, fmt.Errorf("expected an object key, got %v", keyToken)
				}
				value, err := decodeJSONValue(decoder)
				if err != nil {
					return nil, err
				}
				object.insert(key, value)
			}
			if _, err := decoder.Token(); err != nil {
				return nil, err
			}
			return object, nil
		case '[':
			array := []any{}
			for decoder.More() {
				value, err := decodeJSONValue(decoder)
				if err != nil {
					return nil, err
				}
				array = append(array, value)
			}
			if _, err := decoder.Token(); err != nil {
				return nil, err
			}
			return array, nil
		default:
			return nil, fmt.Errorf("unexpected %v", token)
		}
	case json.Number:
		return normalizeJSONNumber(string(token)), nil
	default:
		return token, nil
	}
}

// normalizeJSONNumber is the text `serde_json` prints for the number literal
// text: integers that fit `u64` (or, when negative, `i64`) stay integers; `-0`
// and everything else become an `f64`.
func normalizeJSONNumber(text string) jsonNumber {
	if !strings.ContainsAny(text, ".eE") {
		if negative, ok := strings.CutPrefix(text, "-"); ok {
			if value, err := strconv.ParseInt(text, 10, 64); err == nil && strings.Trim(negative, "0") != "" {
				return jsonNumber(strconv.FormatInt(value, 10))
			}
		} else if value, err := strconv.ParseUint(text, 10, 64); err == nil {
			return jsonNumber(strconv.FormatUint(value, 10))
		}
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil && !math.IsInf(value, 0) {
		return jsonNumber(text)
	}
	return jsonNumber(formatJSONFloat(value))
}

// formatJSONFloat prints a finite float the way `ryu` does for `serde_json`;
// infinities print as `null`, as `serde_json` writes them.
func formatJSONFloat(value float64) string {
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return "null"
	}
	sign := ""
	if math.Signbit(value) {
		sign = "-"
		value = -value
	}
	if value == 0 {
		return sign + "0.0"
	}
	// Shortest digits that round-trip, and the decimal exponent of the first.
	scientific := strconv.FormatFloat(value, 'e', -1, 64)
	mantissa, exponentText, _ := strings.Cut(scientific, "e")
	digits := strings.Replace(mantissa, ".", "", 1)
	exponent, _ := strconv.Atoi(exponentText)
	length := len(digits)
	// The decimal point sits after `point` digits.
	point := exponent + 1
	var out string
	switch {
	case length <= point && point <= 16:
		out = digits + strings.Repeat("0", point-length) + ".0"
	case 0 < point && point <= 16:
		out = digits[:point] + "." + digits[point:]
	case -5 < point && point <= 0:
		out = "0." + strings.Repeat("0", -point) + digits
	case length == 1:
		out = digits + "e" + strconv.Itoa(point-1)
	default:
		out = digits[:1] + "." + digits[1:] + "e" + strconv.Itoa(point-1)
	}
	return sign + out
}

// prettyJSON renders value like `serde_json::to_string_pretty`.
func prettyJSON(value any) string {
	var out strings.Builder
	writePrettyJSON(&out, value, 0)
	return out.String()
}

func writePrettyJSON(out *strings.Builder, value any, depth int) {
	indent := func(depth int) {
		out.WriteString(strings.Repeat("  ", depth))
	}
	switch value := value.(type) {
	case nil:
		out.WriteString("null")
	case bool:
		out.WriteString(strconv.FormatBool(value))
	case string:
		writeJSONString(out, value)
	case jsonNumber:
		out.WriteString(string(value))
	case []any:
		if len(value) == 0 {
			out.WriteString("[]")
			return
		}
		out.WriteString("[\n")
		for i, element := range value {
			if i > 0 {
				out.WriteString(",\n")
			}
			indent(depth + 1)
			writePrettyJSON(out, element, depth+1)
		}
		out.WriteString("\n")
		indent(depth)
		out.WriteString("]")
	case []string:
		elements := make([]any, len(value))
		for i, element := range value {
			elements[i] = element
		}
		writePrettyJSON(out, elements, depth)
	case *jsonObject:
		if len(value.keys) == 0 {
			out.WriteString("{}")
			return
		}
		out.WriteString("{\n")
		for i, key := range value.keys {
			if i > 0 {
				out.WriteString(",\n")
			}
			indent(depth + 1)
			writeJSONString(out, key)
			out.WriteString(": ")
			writePrettyJSON(out, value.values[key], depth+1)
		}
		out.WriteString("\n")
		indent(depth)
		out.WriteString("}")
	default:
		panic(fmt.Sprintf("unsupported JSON value %T", value))
	}
}

// writeJSONString escapes s like `serde_json`: quotes, backslashes and control
// characters only.
func writeJSONString(out *strings.Builder, s string) {
	out.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if c < 0x20 {
				fmt.Fprintf(out, `\u%04x`, c)
			} else {
				out.WriteByte(c)
			}
		}
	}
	out.WriteByte('"')
}

// jsonObjectOf builds an object from alternating keys and values, in order, the
// way `serde_json::json!` does.
func jsonObjectOf(pairs ...any) *jsonObject {
	object := newJSONObject()
	for i := 0; i < len(pairs); i += 2 {
		object.insert(pairs[i].(string), pairs[i+1])
	}
	return object
}
