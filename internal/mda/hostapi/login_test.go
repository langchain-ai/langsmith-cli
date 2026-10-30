package hostapi

import "testing"

func envFrom(pairs map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := pairs[key]
		return value, ok
	}
}

func TestLangSmithSaaSIsTheDefaultSignInTarget(t *testing.T) {
	if got := ResolveLoginAPIURL("https://unknown.example", envFrom(nil)); got != "https://unknown.example" {
		t.Errorf("got %q", got)
	}
	if got := ResolveLoginAPIURL("", envFrom(nil)); got != DefaultAPIURL {
		t.Errorf("got %q", got)
	}
}

func TestLangSmithEndpointSelectsTheSignInTarget(t *testing.T) {
	for endpoint, want := range map[string]string{
		"https://mine.example":                       "https://mine.example",
		"https://eu.api.smith.langchain.com/api/v1/": "https://eu.api.smith.langchain.com",
		"https://eu.api.smith.langchain.com":         "https://eu.api.smith.langchain.com",
		"   ":                                        "https://api.smith.langchain.com",
	} {
		if got := ResolveLoginAPIURL("https://api.host.langchain.com", envFrom(map[string]string{"LANGSMITH_ENDPOINT": endpoint})); got != want {
			t.Errorf("%q: got %q, want %q", endpoint, got, want)
		}
	}
}

func TestLoginAPIURLDerivesFromHostWhenEnvIsUnset(t *testing.T) {
	for host, want := range map[string]string{
		"https://eu.api.host.langchain.com":   "https://eu.api.smith.langchain.com",
		"https://beta.api.host.langchain.com": "https://beta.api.smith.langchain.com",
		"https://dev.api.host.langchain.com":  "https://dev.api.smith.langchain.com",
	} {
		if got := ResolveLoginAPIURL(host, envFrom(nil)); got != want {
			t.Errorf("%s: got %q, want %q", host, got, want)
		}
	}
}
