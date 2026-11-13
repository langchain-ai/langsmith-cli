package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"
)

const workflowProjectID = "018e4c7e-a9fb-7ef0-a5b6-6ea3a82e9327"

// fakeWorkflowAPI serves the endpoints the workflow commands read: run stats,
// the v2 runs query and the Issues Board. A request whose window starts at or
// after split is the "now"/"after" side; anything earlier is the baseline.
type fakeWorkflowAPI struct {
	t     *testing.T
	split time.Time

	healthBefore, healthAfter map[string]any
	failsBefore, failsAfter   []map[string]any
	// runCounts maps a run name to its [before, after] run_count, and
	// stepErrorRates to its [before, after] error_rate (0 when absent).
	runCounts      map[string][2]int64
	stepErrorRates map[string][2]float64
	issues         []map[string]any
	issuesStatus   int
	// issuesProbeStatus fails only the one-issue read past the issue cap.
	issuesProbeStatus int

	mu       sync.Mutex
	requests []fakeRequest
}

type fakeRequest struct {
	Method, Path string
	Body         map[string]any
	Query        string
}

var nameFilterRe = regexp.MustCompile(`eq\(name, "([^"]+)"\)`)

func (f *fakeWorkflowAPI) server() *httptest.Server {
	return newTestServer(f.t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		f.mu.Lock()
		f.requests = append(f.requests, fakeRequest{r.Method, r.URL.Path, body, r.URL.RawQuery})
		f.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/runs/stats":
			after := f.isAfter(body, "start_time")
			filter, _ := body["filter"].(string)
			if m := nameFilterRe.FindStringSubmatch(filter); m != nil {
				side := 0
				if after {
					side = 1
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"run_count":  f.runCounts[m[1]][side],
					"error_rate": f.stepErrorRates[m[1]][side],
				})
				return
			}
			h := f.healthBefore
			if after {
				h = f.healthAfter
			}
			_ = json.NewEncoder(w).Encode(h)
		case r.URL.Path == "/api/v2/runs/query":
			items := f.failsBefore
			if f.isAfter(body, "min_start_time") {
				items = f.failsAfter
			}
			if items == nil {
				items = []map[string]any{}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
		case r.URL.Path == "/api/v1/platform/issues":
			if f.issuesProbeStatus != 0 && r.URL.Query().Get("limit") == "1" {
				http.Error(w, `{"detail":"unavailable"}`, f.issuesProbeStatus)
				return
			}
			if f.issuesStatus != 0 {
				http.Error(w, `{"detail":"not found"}`, f.issuesStatus)
				return
			}
			// Page like the server: offset and limit from the query.
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			page := []map[string]any{}
			for i := offset; i < len(f.issues) && i < offset+limit; i++ {
				page = append(page, f.issues[i])
			}
			_ = json.NewEncoder(w).Encode(page)
		default:
			f.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
}

func (f *fakeWorkflowAPI) isAfter(body map[string]any, key string) bool {
	s, _ := body[key].(string)
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		f.t.Fatalf("request has no parseable %s: %v", key, body)
	}
	return !t.Before(f.split)
}

func (f *fakeWorkflowAPI) requestsTo(path string) []fakeRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []fakeRequest
	for _, r := range f.requests {
		if r.Path == path {
			out = append(out, r)
		}
	}
	return out
}

func failedRun(id, trace, name, runType, err string, start time.Time) map[string]any {
	return map[string]any{
		"id": id, "trace_id": trace, "name": name, "run_type": runType,
		"start_time": start.Format(time.RFC3339), "error": err,
	}
}

func TestParseErrorSignature(t *testing.T) {
	cases := []struct {
		name, raw, wantType, wantDisplay string
		interrupt                        bool
	}{
		{"python repr", "FireworksModelNotFoundError('Model not found, inaccessible, and/or not deployed')",
			"FireworksModelNotFoundError", "FireworksModelNotFoundError: Model not found, inaccessible, and/or not deployed", false},
		{"module path and colon", "openai.RateLimitError: Error code: 429\n\nTraceback ...",
			"RateLimitError", "RateLimitError: Error code: 429", false},
		{"traceback first", "Traceback (most recent call last):\n  File \"x.py\", line 1\nValueError: bad input",
			"ValueError", "ValueError: bad input", false},
		{"plain message", "Something broke: no type here", "", "Something broke: no type here", false},
		{"interrupt", "GraphInterrupt((Interrupt(value='approve?'),))", "GraphInterrupt", "", true},
		{"empty", "", "", "(no error message)", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sig := parseErrorSignature(c.raw)
			if sig.errType != c.wantType {
				t.Errorf("type = %q, want %q", sig.errType, c.wantType)
			}
			if sig.interrupt != c.interrupt {
				t.Errorf("interrupt = %v, want %v", sig.interrupt, c.interrupt)
			}
			if c.wantDisplay != "" && sig.display != c.wantDisplay {
				t.Errorf("display = %q, want %q", sig.display, c.wantDisplay)
			}
		})
	}
}

// Errors that differ only in IDs and numbers are one failure, not many.
func TestErrorSignatureGroupsVolatileParts(t *testing.T) {
	a := parseErrorSignature("TimeoutError('request 6f1c2a3e-0000-4000-8000-000000000001 timed out after 30s')")
	b := parseErrorSignature("TimeoutError('request 9a8b7c6d-1111-4111-8111-111111111111 timed out after 31s')")
	c := parseErrorSignature("TimeoutError('connection pool exhausted')")
	if a.key != b.key {
		t.Errorf("keys differ for the same failure:\n%q\n%q", a.key, b.key)
	}
	if a.key == c.key {
		t.Errorf("different failures share key %q", a.key)
	}
}

func TestClassifyFailure(t *testing.T) {
	cases := []struct {
		raw, runType, cause, match string
	}{
		{"OpenAIModelNotFoundError('Not Found')", "llm", "Model unavailable", "exact"},
		{"anthropic.RateLimitError: Error code: 429", "llm", "Rate limited", "exact"},
		{"ValueError: Error code: 429 - rate limit reached", "llm", "Rate limited", "pattern"},
		{"httpx.ReadTimeout: timed out", "tool", "Timeout", "exact"},
		{"KeyError('foo')", "tool", "Tool error", "pattern"},
		{"KeyError('foo')", "chain", "Unclassified", "none"},
	}
	for _, c := range cases {
		cause, match := classifyFailure(parseErrorSignature(c.raw), c.runType)
		if cause != c.cause || match != c.match {
			t.Errorf("classifyFailure(%q, %s) = (%s, %s), want (%s, %s)", c.raw, c.runType, cause, match, c.cause, c.match)
		}
	}
}

func TestParseWindowDuration(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"90m": 90 * time.Minute, "24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "2w": 14 * 24 * time.Hour, "1.5d": 36 * time.Hour,
	} {
		got, err := parseWindowDuration(in)
		if err != nil || got != want {
			t.Errorf("parseWindowDuration(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "7", "-1d", "0h", "week"} {
		if _, err := parseWindowDuration(bad); err == nil {
			t.Errorf("parseWindowDuration(%q) accepted", bad)
		}
	}
}

func TestRunParallelBoundsConcurrency(t *testing.T) {
	var mu sync.Mutex
	inFlight, peak := 0, 0
	fns := make([]func() error, 50)
	for i := range fns {
		fns[i] = func() error {
			mu.Lock()
			inFlight++
			peak = max(peak, inFlight)
			mu.Unlock()
			time.Sleep(time.Millisecond)
			mu.Lock()
			inFlight--
			mu.Unlock()
			return nil
		}
	}
	if err := runParallel(fns...); err != nil {
		t.Fatal(err)
	}
	if peak > maxParallelReads {
		t.Errorf("peak concurrency = %d, want at most %d", peak, maxParallelReads)
	}
}
