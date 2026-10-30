package hostapi

// Where the platform API is, from the mda CLI's `login.rs`. Its device login,
// PAT cache and `Login` are dropped: authentication is the LangSmith CLI's
// (see the package doc).

import "strings"

// langsmithEndpointEnv is the LangSmith SDK env var for the platform API
// origin (regional / self-hosted).
const langsmithEndpointEnv = "LANGSMITH_ENDPOINT"

// DefaultAPIURL is the default LangSmith platform API (GCP US prod). OAuth and
// Fleet live here.
const DefaultAPIURL = "https://api.smith.langchain.com"

// NormalizePlatformAPIURL strips trailing slashes and an optional `/api/v1`
// REST suffix.
//
// OAuth lives at the platform origin; SDK env vars often include the
// versioned REST base. Returns false when the value is blank after trimming.
func NormalizePlatformAPIURL(value string) (string, bool) {
	trimmed := strings.TrimRight(strings.TrimSpace(value), "/")
	origin := strings.TrimSuffix(trimmed, "/api/v1")
	return origin, origin != ""
}

// PlatformAPIURLForHost resolves the platform API origin that pairs with the
// host backend at hostURL.
//
// Order:
//  1. `LANGSMITH_ENDPOINT`
//  2. Derive from the host-backend URL (`api.host.*` → `api.smith.*`,
//     including regional / staging / dev prefixes)
//  3. The host URL itself (local/custom single-origin installs)
//  4. GCP US prod default
//
// env should prefer process env over project `.env` when both exist.
func PlatformAPIURLForHost(hostURL string, env func(string) (string, bool)) string {
	if value, ok := env(langsmithEndpointEnv); ok {
		if explicit, ok := NormalizePlatformAPIURL(value); ok {
			return explicit
		}
	}
	if derived, ok := LangsmithAPIBaseURL(hostURL); ok {
		return derived
	}
	if normalized, ok := NormalizePlatformAPIURL(hostURL); ok {
		return normalized
	}
	return DefaultAPIURL
}
