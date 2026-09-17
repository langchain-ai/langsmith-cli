package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// readJSONInput accepts inline containers and file paths without reflecting input
// in errors: model configuration and prompt arguments can contain private data.
// An @ prefix forces file interpretation, including paths starting with { or [.
func readJSONInput(value string, maxBytes int) ([]byte, error) {
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
