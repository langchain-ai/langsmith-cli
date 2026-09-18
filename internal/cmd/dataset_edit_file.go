package cmd

import (
	"bytes"
	"encoding/json"
	"io"
)

// Bound local edit files and reject ambiguous JSON before making any requests.
func readDatasetEditFile(path string, target any) error {
	const maxBytes = 8 * 1024 * 1024
	b, err := readJSONInput(path, maxBytes)
	if err != nil || len(b) > maxBytes {
		return datasetInputError("dataset edit file must be readable and at most 8 MiB")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := checkSelectionJSONKeys(d, 0); err != nil {
		return datasetInputError("edit file contains invalid, duplicate, or excessively nested JSON")
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return datasetInputError("invalid edit file fields or value types")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return datasetInputError("edit file must contain exactly one JSON value")
	}
	return nil
}
