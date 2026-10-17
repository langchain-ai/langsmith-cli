package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGatewayProxy(t *testing.T) {
	var calls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "/gateway/v1/responses", r.URL.Path)
		require.Equal(t, fmt.Sprintf("Bearer refreshed-%d", calls), r.Header.Get("Authorization"))
		require.Equal(t, "workspace", r.Header.Get("X-Tenant-Id"))
		require.Empty(t, r.Header.Get("Cookie"))
		require.Empty(t, r.Header.Get("X-Service-Key"))
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer upstream.Close()
	target, err := url.Parse(upstream.URL + "/gateway/v1")
	require.NoError(t, err)
	refreshes := 0
	proxy := httptest.NewServer(gatewayProxy(target, "local-secret", func(context.Context) (http.Header, error) {
		refreshes++
		return http.Header{"Authorization": {fmt.Sprintf("Bearer refreshed-%d", refreshes)}, "X-Tenant-Id": {"workspace"}}, nil
	}))
	defer proxy.Close()
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, proxy.URL+"/v1/responses", strings.NewReader(`{}`))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer local-secret")
		req.Header.Set("X-Tenant-Id", "attacker")
		req.Header.Set("Cookie", "secret")
		req.Header.Set("X-Service-Key", "secret")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		data := make([]byte, len("data: first\n\n"))
		_, err = io.ReadFull(resp.Body, data)
		require.NoError(t, err)
		require.Equal(t, "data: first\n\n", string(data))
		cancel()
		require.NoError(t, resp.Body.Close())
	}
	require.Equal(t, 2, refreshes)
}

func TestGatewayProxyRejectsUnauthorized(t *testing.T) {
	target, err := url.Parse("https://gateway.example/v1")
	require.NoError(t, err)
	handler := gatewayProxy(target, "secret", func(context.Context) (http.Header, error) {
		t.Fatal("unauthorized request reached authentication")
		return nil, nil
	})
	for _, origin := range []string{"", "https://attacker.example"} {
		req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		if origin != "" {
			req.Header.Set("Authorization", "Bearer secret")
			req.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		require.Equal(t, http.StatusUnauthorized, w.Code)
	}
}

func TestGatewayChildConfiguration(t *testing.T) {
	base := "http://127.0.0.1:1234/v1"
	args := gatewayChildArgs([]string{"codex", "exec", "hello"}, base)
	require.Equal(t, "codex", args[0])
	require.Contains(t, strings.Join(args, " "), `env_key="LANGSMITH_GATEWAY_TOKEN"`)
	require.Equal(t, []string{"exec", "hello"}, args[len(args)-2:])
	require.Equal(t, []string{"other", "--flag"}, gatewayChildArgs([]string{"other", "--flag"}, base))
	env := gatewayChildEnv([]string{"PATH=/bin", "OPENAI_API_KEY=real", "LANGSMITH_API_KEY=real"}, base, "local")
	require.Contains(t, env, "PATH=/bin")
	require.Contains(t, env, "OPENAI_API_KEY=local")
	require.NotContains(t, env, "LANGSMITH_API_KEY=real")
}
