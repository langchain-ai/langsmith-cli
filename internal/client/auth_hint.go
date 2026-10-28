package client

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/langchain-ai/langsmith-go/option"
)

// authHint prints one hint on the first 401 or 403, so an agent fixes its
// credentials instead of retrying them on every command. A key from another
// region (an EU key sent to the US endpoint) is the most common cause, and the
// API answers it with 403 as well as 401.
type authHint struct {
	once sync.Once
	w    io.Writer
}

const regionAdvice = "EU keys need https://eu.api.smith.langchain.com (set LANGSMITH_ENDPOINT, --api-url, or the selected profile's API URL)"

func (h *authHint) check(req *http.Request, res *http.Response) {
	var msg string
	switch res.StatusCode {
	case http.StatusUnauthorized:
		msg = fmt.Sprintf("the API rejected these credentials (HTTP 401) at %s. Check the key hasn't been revoked and belongs to this region: %s. Or run `langsmith auth login`.", req.URL.Host, regionAdvice)
	case http.StatusForbidden:
		msg = fmt.Sprintf("the API refused this request (HTTP 403) at %s. Either the key lacks permission for this workspace or action (check its role and LANGSMITH_WORKSPACE_ID), or it belongs to another region: %s.", req.URL.Host, regionAdvice)
	default:
		return
	}
	h.once.Do(func() { fmt.Fprintf(h.w, "hint: %s\n", msg) })
}

func (h *authHint) option() option.RequestOption {
	return option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
		res, err := next(req)
		if err == nil {
			h.check(req, res)
		}
		return res, err
	})
}

type authHintTransport struct {
	hint *authHint
	base http.RoundTripper
}

func (t authHintTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	res, err := t.base.RoundTrip(req)
	if err == nil {
		t.hint.check(req, res)
	}
	return res, err
}

var processAuthHint = &authHint{w: os.Stderr}

// WrapDefaultTransportWithAuthHint adds the auth hint to requests sent through
// http.DefaultTransport, which generated commands use, until restore is called.
func WrapDefaultTransportWithAuthHint() (restore func()) {
	prev := http.DefaultTransport
	http.DefaultTransport = authHintTransport{hint: processAuthHint, base: prev}
	return func() { http.DefaultTransport = prev }
}
