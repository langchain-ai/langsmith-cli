package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// readDashboardJSONInput accepts inline containers and file paths without reflecting input
// in errors: model configuration and prompt arguments can contain private data.
// An @ prefix forces file interpretation, including paths starting with { or [.
func readDashboardJSONInput(value string, maxBytes int) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		if len(value) > maxBytes {
			return nil, fmt.Errorf("JSON input exceeds %d bytes", maxBytes)
		}
		return []byte(value), nil
	}
	path := strings.TrimPrefix(value, "@")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read JSON input; provide inline JSON, a file path, or @file")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, int64(maxBytes)+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read JSON input")
	}
	if len(data) > maxBytes {
		return nil, fmt.Errorf("JSON input exceeds %d bytes", maxBytes)
	}
	return data, nil
}

func checkDashboardJSONKeys(d *json.Decoder, depth int) error {
	if depth > 128 {
		return fmt.Errorf("dashboard JSON exceeds 128 nesting levels")
	}
	token, err := d.Token()
	if err != nil {
		return fmt.Errorf("invalid dashboard JSON")
	}
	delim, nested := token.(json.Delim)
	if !nested {
		return nil
	}
	seen := map[string]bool{}
	for d.More() {
		if delim == '{' {
			key, err := d.Token()
			if err != nil {
				return fmt.Errorf("invalid dashboard JSON")
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return fmt.Errorf("dashboard JSON contains duplicate or invalid keys")
			}
			seen[name] = true
		}
		if err := checkDashboardJSONKeys(d, depth+1); err != nil {
			return err
		}
	}
	if _, err := d.Token(); err != nil {
		return fmt.Errorf("invalid dashboard JSON")
	}
	return nil
}
