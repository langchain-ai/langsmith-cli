package api

import (
	"net/http"
	"strings"
)

// commandHint names the CLI command that replaces a raw request, so agents
// stop reaching for `langsmith api` when a command already does the job.
func commandHint(apiURL, method, path string) string {
	parts := apiRoute(apiURL, path)
	switch {
	case strings.EqualFold(method, http.MethodPost) && len(parts) == 2 && parts[0] == "runs" && parts[1] == "query":
		return "use `langsmith runs list --project NAME --filter FILTER` (or `langsmith traces list` for root runs) instead of raw run queries"
	case strings.EqualFold(method, http.MethodGet) && len(parts) == 2 && parts[0] == "runs" && parts[1] != "rules" && parts[1] != "stats":
		return "use `langsmith runs get RUN_ID` instead of fetching a run through the raw API (add --since for runs older than 7 days)"
	}
	return ""
}
