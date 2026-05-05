package cmd

import (
	"fmt"
	"regexp"
	"strings"
)

var hubRepoHandlePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// parseHubOwnerRepo splits "[owner/]repo[:ref]"; missing owner becomes "-" (API "current tenant").
func parseHubOwnerRepo(arg string) (string, string, string, error) {
	if arg == "" {
		return "", "", "", fmt.Errorf("empty repo identifier")
	}

	rest := arg
	ref := ""
	if i := strings.Index(rest, ":"); i >= 0 {
		ref = rest[i+1:]
		rest = rest[:i]
	}

	owner := "-"
	name := rest
	if i := strings.Index(rest, "/"); i >= 0 {
		owner = rest[:i]
		name = rest[i+1:]
	}

	if owner == "" || name == "" {
		return "", "", "", fmt.Errorf("invalid repo identifier %q (expected [OWNER/]REPO[:REF])", arg)
	}
	if strings.Contains(name, "/") {
		return "", "", "", fmt.Errorf("invalid repo identifier %q (too many '/' separators)", arg)
	}
	if owner == "-" && !hubRepoHandlePattern.MatchString(name) {
		return "", "", "", fmt.Errorf("invalid repo handle %q (must match %s)", name, hubRepoHandlePattern.String())
	}

	return owner, name, ref, nil
}
