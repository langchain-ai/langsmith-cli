package client

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
)

func TestAuthHintOption(t *testing.T) {
	tests := []struct {
		status int
		want   string
	}{
		{status: http.StatusUnauthorized, want: "rejected these credentials (HTTP 401)"},
		{status: http.StatusForbidden, want: "lacks permission for this workspace"},
		{status: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"detail":"Invalid token"}`))
			}))
			defer ts.Close()
			var stderr bytes.Buffer
			hint := &authHint{w: &stderr}
			sdk := langsmith.NewClient(option.WithBaseURL(ts.URL), option.WithAPIKey("k"), option.WithMaxRetries(0), hint.option())

			for range 2 {
				_, _ = sdk.Sessions.List(context.Background(), langsmith.SessionListParams{})
			}

			wantCount := 0
			if tt.want != "" {
				wantCount = 1
			}
			if got := strings.Count(stderr.String(), "hint:"); got != wantCount || !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("stderr = %q, want %d hint containing %q", stderr.String(), wantCount, tt.want)
			}
		})
	}
}

func TestAuthHintTransport(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()
	var stderr bytes.Buffer
	c := &http.Client{Transport: authHintTransport{hint: &authHint{w: &stderr}, base: http.DefaultTransport}}

	res, err := c.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	if !strings.Contains(stderr.String(), "HTTP 401") {
		t.Fatalf("stderr = %q, want the 401 hint", stderr.String())
	}
}
