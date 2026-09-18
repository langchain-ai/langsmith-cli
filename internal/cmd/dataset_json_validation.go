package cmd

import (
	"encoding/json"
	"fmt"
)

// Map decoding silently overwrites duplicate keys, including reviewed IO values.
func checkSelectionJSONKeys(d *json.Decoder, depth int) error {
	if depth > 128 {
		return fmt.Errorf("selection JSON exceeds 128 nesting levels")
	}
	token, err := d.Token()
	if err != nil {
		return fmt.Errorf("invalid selection JSON")
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
				return fmt.Errorf("invalid selection JSON")
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return fmt.Errorf("selection JSON contains duplicate or invalid keys")
			}
			seen[name] = true
		}
		if err := checkSelectionJSONKeys(d, depth+1); err != nil {
			return err
		}
	}
	if _, err := d.Token(); err != nil {
		return fmt.Errorf("invalid selection JSON")
	}
	return nil
}
