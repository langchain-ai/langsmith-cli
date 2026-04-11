package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// dataplaneURL builds a full URL from a base dataplane URL and a path,
// ensuring no double slashes.
func dataplaneURL(base, path string) string {
	return strings.TrimRight(base, "/") + path
}

// dataplanePost sends a POST request to a sandbox dataplane endpoint.
// It sets auth headers, checks the HTTP status code, and decodes the
// JSON response into result (if non-nil).
func dataplanePost(baseURL, path string, body any, result any) error {
	url := dataplaneURL(baseURL, path)

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(http.MethodPost, url, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range sandboxAuthHeaders() {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}

// dataplanePostRaw sends a POST with a pre-built request body and custom
// content type. Used for multipart uploads.
func dataplanePostRaw(baseURL, path, contentType string, body io.Reader) error {
	url := dataplaneURL(baseURL, path)

	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	for k, v := range sandboxAuthHeaders() {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
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
