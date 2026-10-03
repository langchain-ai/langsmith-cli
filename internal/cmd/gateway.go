package cmd

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

func newGatewayCmd() *cobra.Command {
	group := &cobra.Command{Use: "gateway", Short: "Run clients through LangSmith Gateway"}
	var endpoint string
	run := &cobra.Command{
		Use:   "exec [--gateway-url URL] -- command [args...]",
		Short: "Run a command with a local, automatically authenticated gateway proxy",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			if endpoint == "" {
				endpoint, err = gatewayURL(GetAPIURL())
			}
			if err != nil {
				return err
			}
			target, err := url.Parse(endpoint)
			if err != nil || target.Host == "" || target.User != nil || target.RawQuery != "" || target.Fragment != "" || (target.Scheme != "https" && !(target.Scheme == "http" && (target.Hostname() == "localhost" || net.ParseIP(target.Hostname()).IsLoopback()))) {
				return fmt.Errorf("gateway URL must be HTTPS (or HTTP on loopback), without credentials, query, or fragment")
			}
			if _, err := c.SDK.AuthHeaders(cmd.Context()); err != nil {
				return err
			}
			secret := make([]byte, 32)
			if _, err := rand.Read(secret); err != nil {
				return err
			}
			token := hex.EncodeToString(secret)
			proxy := gatewayProxy(target, token, func(ctx context.Context) (http.Header, error) { return c.SDK.AuthHeaders(ctx) })
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				return err
			}
			server := &http.Server{Handler: proxy, ReadHeaderTimeout: 10 * time.Second}
			defer server.Close()
			go func() { _ = server.Serve(listener) }()
			base := "http://" + listener.Addr().String() + "/v1"
			childArgs := gatewayChildArgs(args, base)
			child := exec.CommandContext(cmd.Context(), childArgs[0], childArgs[1:]...)
			child.Env = gatewayChildEnv(os.Environ(), base, token)
			child.Stdin, child.Stdout, child.Stderr = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
			signals := make(chan os.Signal, 1)
			signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(signals)
			if err := child.Start(); err != nil {
				return err
			}
			done := make(chan error, 1)
			go func() { done <- child.Wait() }()
			for {
				select {
				case err := <-done:
					return err
				case sig := <-signals:
					_ = child.Process.Signal(sig)
				}
			}
		},
	}
	run.Flags().StringVar(&endpoint, "gateway-url", "", "Gateway API base URL, including /v1; inferred from the LangSmith endpoint when omitted")
	group.AddCommand(run)
	return group
}

func gatewayURL(apiURL string) (string, error) {
	u, err := url.Parse(apiURL)
	if err != nil {
		return "", err
	}
	hosts := map[string]string{
		"api.smith.langchain.com":      "gateway.smith.langchain.com",
		"eu.api.smith.langchain.com":   "eu.gateway.smith.langchain.com",
		"apac.api.smith.langchain.com": "apac.gateway.smith.langchain.com",
		"aws.api.smith.langchain.com":  "aws.gateway.smith.langchain.com",
		"dev.api.smith.langchain.com":  "dev.gateway.smith.langchain.com",
		"beta.api.smith.langchain.com": "beta.gateway.smith.langchain.com",
	}
	if host, ok := hosts[u.Host]; ok && u.Scheme == "https" {
		return "https://" + host + "/v1", nil
	}
	return "", fmt.Errorf("pass --gateway-url for this LangSmith endpoint")
}

func gatewayProxy(target *url.URL, token string, auth func(context.Context) (http.Header, error)) http.Handler {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.URL.Path = strings.TrimRight(target.Path, "/") + strings.TrimPrefix(r.In.URL.Path, "/v1")
			r.Out.URL.RawPath = ""
			r.Out.Header = r.Out.Header.Clone()
			for _, key := range []string{"Authorization", "X-Api-Key", "Api-Key", "Cookie", "X-Service-Key", "X-Tenant-Id", "X-Organization-Id", "X-User-Id", "Origin", "Referer"} {
				r.Out.Header.Del(key)
			}
			headers := r.In.Context().Value(gatewayAuthKey{}).(http.Header)
			for key, values := range headers {
				r.Out.Header[key] = append([]string(nil), values...)
			}
		},
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "gateway request failed", http.StatusBadGateway)
		},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/v1/") || r.Method == http.MethodConnect || strings.Contains(r.URL.Path, "..") {
			http.Error(w, "invalid gateway path", http.StatusBadRequest)
			return
		}
		headers, err := auth(r.Context())
		if err != nil {
			http.Error(w, "gateway authentication failed; run langsmith auth login", http.StatusBadGateway)
			return
		}
		proxy.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), gatewayAuthKey{}, headers)))
	})
}

type gatewayAuthKey struct{}

func gatewayChildArgs(args []string, base string) []string {
	if strings.TrimSuffix(filepath.Base(args[0]), ".exe") != "codex" {
		return args
	}
	config := []string{
		`model_provider="langsmith_gateway"`,
		`model_providers.langsmith_gateway={name="LangSmith Gateway",base_url=` + strconv.Quote(base) + `,env_key="LANGSMITH_GATEWAY_TOKEN",wire_api="responses",requires_openai_auth=false,supports_websockets=false}`,
	}
	result := []string{args[0]}
	for _, value := range config {
		result = append(result, "-c", value)
	}
	return append(result, args[1:]...)
}

func gatewayChildEnv(env []string, base, token string) []string {
	result := make([]string, 0, len(env)+3)
	for _, value := range env {
		key, _, _ := strings.Cut(value, "=")
		if key != "OPENAI_BASE_URL" && key != "OPENAI_API_KEY" && key != "LANGSMITH_GATEWAY_TOKEN" && key != "LANGSMITH_API_KEY" {
			result = append(result, value)
		}
	}
	return append(result, "OPENAI_BASE_URL="+base, "OPENAI_API_KEY="+token, "LANGSMITH_GATEWAY_TOKEN="+token)
}
