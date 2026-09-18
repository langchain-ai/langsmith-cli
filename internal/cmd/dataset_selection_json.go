package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Use raw SDK responses: generic SDK maps may have already rounded JSON numbers.
func preciseTraceIO(raw string) (struct{ Inputs, Outputs map[string]any }, error) {
	var payload struct{ Inputs, Outputs map[string]any }
	d := json.NewDecoder(bytes.NewBufferString(raw))
	d.UseNumber()
	if err := d.Decode(&payload); err != nil {
		return payload, fmt.Errorf("could not decode source inputs/outputs without loss of precision")
	}
	return payload, nil
}

func equalTraceJSON(a, b map[string]any) bool {
	left, leftErr := json.Marshal(a)
	right, rightErr := json.Marshal(b)
	return leftErr == nil && rightErr == nil && bytes.Equal(left, right)
}

func decodeTraceSelection(data []byte, target *traceSelection) error {
	keys := json.NewDecoder(bytes.NewReader(data))
	keys.UseNumber()
	if err := checkSelectionJSONKeys(keys, 0); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return fmt.Errorf("invalid selection JSON: check field names and value types")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("selection must contain exactly one JSON object")
	}
	return nil
}
