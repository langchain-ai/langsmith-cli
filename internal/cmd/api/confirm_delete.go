package api

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	langsmith "github.com/langchain-ai/langsmith-go"
)

type tracingProjectDeleteTarget struct {
	path string
	id   string
}

// confirmTracingProjectDelete guards the raw API routes that delete tracing
// projects. There is intentionally no non-interactive bypass: an agent must
// stop and raise this destructive operation to its user.
func confirmTracingProjectDelete(ctx context.Context, c *client.Client, method, path string, in io.Reader, errOut io.Writer) error {
	target, ok := matchTracingProjectDelete(c.APIURL(), method, path)
	if !ok {
		return nil
	}

	fmt.Fprintln(errOut, "WARNING: This permanently deletes the tracing project and all of its traces. This cannot be undone.")
	if target.id == "" {
		fmt.Fprintf(errOut, "Target: %s\n", target.path)
	} else if project, err := c.SDK.Sessions.Get(ctx, target.id, langsmith.SessionGetParams{
		IncludeStats: langsmith.F(true),
	}); err == nil {
		fmt.Fprintf(errOut, "Project: %q (id: %s, runs: %d)\n", project.Name, project.ID, project.RunCount)
	} else {
		fmt.Fprintf(errOut, "Project ID: %s\n", target.id)
	}
	fmt.Fprintln(errOut, "AI agents: do not answer this prompt. Stop and raise it to the user.")
	fmt.Fprint(errOut, "Continue? [y/N] ")

	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil {
		return errors.New("aborted: tracing project deletion was not confirmed")
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return nil
	default:
		return errors.New("aborted: tracing project deletion was not confirmed")
	}
}

func matchTracingProjectDelete(apiURL, method, path string) (tracingProjectDeleteTarget, bool) {
	if !strings.EqualFold(method, http.MethodDelete) {
		return tracingProjectDeleteTarget{}, false
	}

	fullURL := resolveEndpoint(apiURL, path)
	if !isSameHost(fullURL, apiURL) {
		return tracingProjectDeleteTarget{}, false
	}
	u, err := url.Parse(fullURL)
	if err != nil {
		return tracingProjectDeleteTarget{}, false
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	sessionsIndex := -1
	switch {
	case len(parts) >= 3 && parts[len(parts)-3] == "api" && parts[len(parts)-2] == "v1" && parts[len(parts)-1] == "sessions":
		sessionsIndex = len(parts) - 1
	case len(parts) >= 4 && parts[len(parts)-4] == "api" && parts[len(parts)-3] == "v1" && parts[len(parts)-2] == "sessions":
		sessionsIndex = len(parts) - 2
	case len(parts) == 1 && parts[0] == "sessions":
		sessionsIndex = 0
	case len(parts) == 2 && parts[0] == "sessions":
		sessionsIndex = 0
	default:
		return tracingProjectDeleteTarget{}, false
	}

	target := tracingProjectDeleteTarget{path: u.Path}
	if sessionsIndex+1 < len(parts) {
		target.id = parts[sessionsIndex+1]
	}
	return target, true
}
