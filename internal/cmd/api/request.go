package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
)

// runRequest executes an HTTP request and writes the response to w.
// Returns the HTTP status code and any transport-level error.
func runRequest(apiURL, apiKey, method, path, body string, headers []string, include bool, w io.Writer) (int, error) {
	c := client.New(apiKey, apiURL)

	fullURL := resolveEndpoint(apiURL, path)
	// RawDo prepends apiURL, so compute the relative path.
	// For full URLs with a different host, pass the full URL as the path
	// to a client constructed with an empty base URL.
	relPath := fullURL
	if strings.HasPrefix(fullURL, apiURL) {
		relPath = strings.TrimPrefix(fullURL, apiURL)
	} else if strings.HasPrefix(fullURL, "http://") || strings.HasPrefix(fullURL, "https://") {
		// Full URL to a different host — use empty-base client so RawDo
		// doesn't prepend apiURL.
		c = client.New(apiKey, "")
	}

	// Resolve body
	bodyReader, err := resolveBody(body)
	if err != nil {
		return 0, err
	}

	// Parse extra headers
	extraHeaders := make(http.Header)
	for _, h := range headers {
		k, v, ok := strings.Cut(h, ":")
		if !ok {
			return 0, fmt.Errorf("invalid header format %q (expected Key:Value)", h)
		}
		extraHeaders.Set(strings.TrimSpace(k), strings.TrimSpace(v))
	}

	statusCode, proto, respHeaders, respBody, err := c.RawDo(context.Background(), method, relPath, bodyReader, extraHeaders)
	if err != nil {
		return 0, err
	}

	// Print response headers if --include
	if include {
		fmt.Fprintf(w, "%s %d %s\n", proto, statusCode, http.StatusText(statusCode))
		for k, vals := range respHeaders {
			for _, v := range vals {
				fmt.Fprintf(w, "%s: %s\n", k, v)
			}
		}
		fmt.Fprintln(w)
	}

	// Pretty-print JSON if possible, otherwise print raw
	var prettyBuf bytes.Buffer
	if json.Indent(&prettyBuf, respBody, "", "  ") == nil {
		fmt.Fprintln(w, prettyBuf.String())
	} else {
		if _, err := w.Write(respBody); err != nil {
			return statusCode, fmt.Errorf("writing response: %w", err)
		}
		fmt.Fprintln(w)
	}

	return statusCode, nil
}

// resolveBody resolves a --body value to an io.Reader.
//   - Empty string → nil (no body).
//   - "@-" → stdin.
//   - "@path" → file contents.
//   - Otherwise → treated as inline JSON string.
func resolveBody(body string) (io.Reader, error) {
	if body == "" {
		return nil, nil
	}
	if body == "@-" {
		return os.Stdin, nil
	}
	if strings.HasPrefix(body, "@") {
		filePath := body[1:]
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("reading body file %q: %w", filePath, err)
		}
		return bytes.NewReader(data), nil
	}
	return strings.NewReader(body), nil
}
