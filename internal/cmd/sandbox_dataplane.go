package cmd

import (
	"fmt"
	"strings"
)

// dataplaneURL builds a full URL from a base dataplane URL and a path,
// ensuring no double slashes.
func dataplaneURL(base, path string) string {
	return strings.TrimRight(base, "/") + path
}

// dataplaneWSURL converts a dataplane HTTP URL to a WebSocket URL
// for the given path.
func dataplaneWSURL(baseURL, path string) string {
	wsScheme := "wss"
	if strings.HasPrefix(baseURL, "http://") {
		wsScheme = "ws"
	}
	hostPath := strings.TrimPrefix(strings.TrimPrefix(baseURL, "https://"), "http://")
	hostPath = strings.TrimRight(hostPath, "/")
	return fmt.Sprintf("%s://%s%s", wsScheme, hostPath, path)
}
