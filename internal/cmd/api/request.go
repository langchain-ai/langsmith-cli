package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-go/option"
)

// runRequest executes an HTTP request and writes the response to w.
// Returns the HTTP status code and any transport-level error.
func runRequest(c *client.Client, method, path, body, input string, params map[string]any, headers []string, include bool, w io.Writer) (int, error) {
	apiURL := c.APIURL()
	fullURL := resolveEndpoint(apiURL, path)
	if len(params) > 0 && strings.EqualFold(method, "GET") {
		fullURL = addQueryParams(fullURL, params)
	}

	// RawDo prepends apiURL, so compute the relative path.
	// For full URLs with a different host, pass the full URL as the path
	// to a client constructed with an empty base URL and no API key
	// (don't leak credentials to external hosts).
	reqClient := c
	relPath := fullURL
	if isSameHost(fullURL, apiURL) {
		relPath = strings.TrimPrefix(fullURL, apiURL)
	} else if strings.HasPrefix(fullURL, "http://") || strings.HasPrefix(fullURL, "https://") {
		target, err := url.Parse(fullURL)
		if err != nil {
			return 0, fmt.Errorf("parsing request URL: %w", err)
		}
		reqClient = client.NewWithOptions(client.Options{APIURL: target.Scheme + "://" + target.Host})
		relPath = target.RequestURI()
	}

	bodyReader, err := resolveRequestBody(method, body, input, params)
	if err != nil {
		return 0, err
	}
	// Buffered so the request can be replayed on a retry, and so the same bytes
	// the user supplied go on the wire rather than a re-marshalled copy.
	var bodyBytes []byte
	if bodyReader != nil {
		if bodyBytes, err = io.ReadAll(bodyReader); err != nil {
			return 0, fmt.Errorf("reading request body: %w", err)
		}
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

	var (
		statusCode  int
		proto       string
		respHeaders http.Header
		respBody    []byte
	)
	if reqClient == c {
		// Same host: go through the SDK so the request gets its retry policy
		// (408/409/429/5xx with backoff, honouring Retry-After) instead of the
		// single bare attempt RawDo makes.
		statusCode, proto, respHeaders, respBody, err = executeViaSDK(
			context.Background(), c, method, relPath, bodyBytes, extraHeaders)
	} else {
		// Cross-host: deliberately a credential-free client, so it must not go
		// through the SDK, which carries auth.
		statusCode, proto, respHeaders, respBody, err = reqClient.RawDo(
			context.Background(), method, relPath, bytes.NewReader(bodyBytes), extraHeaders)
	}
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

func resolveRequestBody(method, body, input string, params map[string]any) (io.Reader, error) {
	if input != "" && body != "" {
		return nil, fmt.Errorf("only one of --input or --body may be used")
	}
	if input != "" && len(params) > 0 && !strings.EqualFold(method, "GET") {
		return nil, fmt.Errorf("--input cannot be combined with field parameters unless using GET")
	}
	if strings.EqualFold(method, "GET") {
		if input != "" {
			return resolveInput(input)
		}
		if body != "" {
			return resolveBody(body)
		}
		return nil, nil
	}
	if input != "" {
		return resolveInput(input)
	}
	if body != "" {
		return resolveBody(body)
	}
	if len(params) == 0 {
		return nil, nil
	}
	data, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshaling fields: %w", err)
	}
	return bytes.NewReader(data), nil
}

func resolveInput(input string) (io.Reader, error) {
	if input == "-" {
		return os.Stdin, nil
	}
	data, err := os.ReadFile(input)
	if err != nil {
		return nil, fmt.Errorf("reading input file %q: %w", input, err)
	}
	return bytes.NewReader(data), nil
}

func addQueryParams(rawURL string, params map[string]any) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	for key, value := range params {
		addQueryValue(q, key, value)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func addQueryValue(q url.Values, key string, value any) {
	switch v := value.(type) {
	case map[string]any:
		data, err := json.Marshal(v)
		if err != nil {
			q.Add(key, fmt.Sprint(v))
			return
		}
		q.Add(key, string(data))
	case []any:
		for _, item := range v {
			addQueryValue(q, key, item)
		}
	case nil:
		q.Add(key, "")
	default:
		q.Add(key, fmt.Sprint(v))
	}
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

// executeViaSDK issues the request through the SDK client so it inherits the
// SDK's retry policy, then reports the response verbatim.
//
// A non-2xx is not an error here: `api` is a passthrough and prints whatever
// came back, as RawDo did. The middleware buffers each response body because
// the SDK consumes it to build its error type before the caller sees it, and
// only re-exposes it when it parsed as JSON — an HTML error page (what the
// edge returns for 429) would otherwise reach the user empty.
func executeViaSDK(
	ctx context.Context,
	c *client.Client,
	method, relPath string,
	body []byte,
	extra http.Header,
) (int, string, http.Header, []byte, error) {
	var (
		last     *http.Response
		lastBody []byte
	)
	opts := []option.RequestOption{
		option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
			res, err := next(req)
			if res != nil {
				if b, readErr := io.ReadAll(res.Body); readErr == nil {
					_ = res.Body.Close()
					res.Body = io.NopCloser(bytes.NewReader(b))
					lastBody = b
				}
				last = res
			}
			return res, err
		}),
	}
	for k, vals := range extra {
		for _, v := range vals {
			opts = append(opts, option.WithHeaderAdd(k, v))
		}
	}

	// The SDK joins paths onto its base URL the way its generated methods do:
	// "api/v1/runs", no leading slash.
	var payload any
	if len(body) > 0 {
		payload = body
	}
	err := c.SDK.Execute(ctx, method, strings.TrimPrefix(relPath, "/"), payload, nil, opts...)
	if last == nil {
		// Never got a response: a genuine transport or setup failure.
		return 0, "", nil, nil, err
	}
	return last.StatusCode, last.Proto, last.Header, lastBody, nil
}
