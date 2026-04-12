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
func runRequest(c *client.Client, method, path, body string, headers []string, include bool, w io.Writer) (int, error) {
	apiURL := c.APIURL()
	fullURL := resolveEndpoint(apiURL, path)

	// RawDo prepends apiURL, so compute the relative path.
	// For full URLs with a different host, pass the full URL as the path
	// to a client constructed with an empty base URL and no API key
	// (don't leak credentials to external hosts).
	reqClient := c
	relPath := fullURL
	if isSameHost(fullURL, apiURL) {
		relPath = strings.TrimPrefix(fullURL, apiURL)
	} else if strings.HasPrefix(fullURL, "http://") || strings.HasPrefix(fullURL, "https://") {
		reqClient = client.New("", "")
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
		extraHeaders.Add(strings.TrimSpace(k), strings.TrimSpace(v))
	}

	statusCode, proto, respHeaders, respBody, err := reqClient.RawDo(context.Background(), method, relPath, bodyReader, extraHeaders)
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

// isSameHost returns true if fullURL starts with baseURL and they share
// the same host. A pure string-prefix check is insufficient because
// "https://api.example.com.evil.com" starts with "https://api.example.com"
// but is a different host.
func isSameHost(fullURL, baseURL string) bool {
	if !strings.HasPrefix(fullURL, baseURL) {
		return false
	}
	// If fullURL is exactly baseURL or continues with "/" or "?", it's the same host.
	// Any other continuation (e.g. ".evil.com") means a different host.
	rest := fullURL[len(baseURL):]
	return rest == "" || rest[0] == '/' || rest[0] == '?'
}

