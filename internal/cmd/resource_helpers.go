package cmd

import (
	"encoding/json"

	"github.com/google/uuid"
)

func sameResourceID(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	left, err := uuid.Parse(a)
	if err != nil {
		return false
	}
	right, err := uuid.Parse(b)
	return err == nil && left == right
}

func resourceUUID(value string) error {
	if _, err := uuid.Parse(value); err != nil {
		return commandDiagnostic{"invalid_resource_id", "resource ID must be a UUID", "List the resource and pass its id field, not its display name."}
	}
	return nil
}

func resourceObject(value string) (map[string]interface{}, error) {
	data, err := readJSONInput(value, 8*1024*1024)
	if err != nil {
		return nil, commandDiagnostic{"invalid_json_object", "expected a JSON object or readable file", "Provide inline JSON, a file path, or @file; maximum size is 8 MiB."}
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil || obj == nil {
		return nil, commandDiagnostic{"invalid_json_object", "expected a JSON object or @file containing one", "Provide a JSON object such as {} or @path/to/file.json; arrays and null are not accepted."}
	}
	return obj, nil
}
