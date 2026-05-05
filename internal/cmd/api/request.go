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
	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/langchain-ai/langsmith-cli/internal/structured"
	"github.com/spf13/cobra"
)

type requestInput struct {
	Body    string
	Headers []string
	Include bool
}

type apiResponse struct {
	StatusCode int
	Proto      string
	Headers    http.Header
	Body       any
	RawBody    []byte
	IsJSON     bool
	Include    bool
}

func requestCommand(method string) structured.Command[*requestInput] {
	return structured.Command[*requestInput]{
		Use:   method + " PATH",
		Short: fmt.Sprintf("Make an authenticated %s request", method),
		Args:  cobra.ExactArgs(1),
		Input: func(cmd *cobra.Command) *requestInput {
			in := &requestInput{}
			cmd.Flags().StringVar(&in.Body, "body", "", `Request body (JSON string, @file, or @- for stdin)`)
			cmd.Flags().StringArrayVarP(&in.Headers, "header", "H", nil, "Additional headers (Key:Value, repeatable)")
			cmd.Flags().BoolVarP(&in.Include, "include", "i", false, "Include HTTP response headers in output")
			return in
		},
		Action: func(_ context.Context, cmd *cobra.Command, in *requestInput, args []string) (any, error) {
			c, err := cmdutil.GetClient(cmd)
			if err != nil {
				return nil, err
			}
			resp, err := runRequest(c, method, args[0], in.Body, in.Headers, in.Include)
			if err != nil {
				return nil, err
			}
			if cmdutil.ResolveJQ(cmd) != "" && !resp.IsJSON {
				return nil, fmt.Errorf("response body is not JSON")
			}
			var afterRender error
			if resp.StatusCode >= 400 {
				afterRender = fmt.Errorf("HTTP %d", resp.StatusCode)
			}
			return structured.Result{
				Model:          resp.Body,
				TextModel:      resp,
				ErrAfterRender: afterRender,
			}, nil
		},
		Render: apiResponseRenderer{},
	}
}

// runRequest executes an HTTP request and returns the response model.
func runRequest(c *client.Client, method, path, body string, headers []string, include bool) (apiResponse, error) {
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
		reqClient = client.NewWithOptions(client.Options{})
	}

	// Resolve body
	bodyReader, err := resolveBody(body)
	if err != nil {
		return apiResponse{}, err
	}

	// Parse extra headers
	extraHeaders := make(http.Header)
	for _, h := range headers {
		k, v, ok := strings.Cut(h, ":")
		if !ok {
			return apiResponse{}, fmt.Errorf("invalid header format %q (expected Key:Value)", h)
		}
		extraHeaders.Add(strings.TrimSpace(k), strings.TrimSpace(v))
	}

	statusCode, proto, respHeaders, respBody, err := reqClient.RawDo(context.Background(), method, relPath, bodyReader, extraHeaders)
	if err != nil {
		return apiResponse{}, err
	}

	resp := apiResponse{
		StatusCode: statusCode,
		Proto:      proto,
		Headers:    respHeaders,
		RawBody:    respBody,
		Body:       string(respBody),
		Include:    include,
	}
	var decodedBody any
	if err := json.Unmarshal(respBody, &decodedBody); err == nil {
		resp.Body = decodedBody
		resp.IsJSON = true
	}
	return resp, nil
}

type apiResponseRenderer struct{}

func (apiResponseRenderer) RenderText(w io.Writer, model any) error {
	resp, ok := model.(apiResponse)
	if !ok {
		return fmt.Errorf("expected apiResponse, got %T", model)
	}
	if resp.Include {
		fmt.Fprintf(w, "%s %d %s\n", resp.Proto, resp.StatusCode, http.StatusText(resp.StatusCode))
		for k, vals := range resp.Headers {
			for _, v := range vals {
				fmt.Fprintf(w, "%s: %s\n", k, v)
			}
		}
		fmt.Fprintln(w)
	}
	if resp.IsJSON {
		var prettyBuf bytes.Buffer
		if err := json.Indent(&prettyBuf, resp.RawBody, "", "  "); err != nil {
			return err
		}
		fmt.Fprintln(w, prettyBuf.String())
		return nil
	}
	if _, err := w.Write(resp.RawBody); err != nil {
		return fmt.Errorf("writing response: %w", err)
	}
	fmt.Fprintln(w)
	return nil
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
