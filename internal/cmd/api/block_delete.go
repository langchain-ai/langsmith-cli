package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type rawDeleteBlock struct {
	resource    string
	replacement string
}

var rawDeleteBlocks = map[string]rawDeleteBlock{
	"sessions": {
		resource:    "tracing projects",
		replacement: "`langsmith project delete --project-id PROJECT_ID`",
	},
}

func blockRawDelete(apiURL, method, path string) error {
	if !strings.EqualFold(method, http.MethodDelete) {
		return nil
	}

	fullURL := resolveEndpoint(apiURL, path)
	if !isSameHost(fullURL, apiURL) {
		return nil
	}
	u, err := url.Parse(fullURL)
	if err != nil {
		return nil
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "api" && parts[i+1] == "v1" {
			parts = parts[i+2:]
			break
		}
	}
	if len(parts) < 1 || len(parts) > 2 {
		return nil
	}

	block, ok := rawDeleteBlocks[parts[0]]
	if !ok {
		return nil
	}
	return fmt.Errorf("raw API deletion of %s is blocked; use %s instead", block.resource, block.replacement)
}
