package cmd

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAppsDevCmd() *cobra.Command {
	var (
		dir    string
		devURL string
		webURL string
		noOpen bool
	)

	cmd := &cobra.Command{
		Use:   "dev --url http://localhost:PORT",
		Short: "Preview a locally running custom app inside LangSmith",
		Long: `Open a LangSmith preview pointed at your local dev server, so you get
live data and live-reload together instead of push-per-change.

--url must point at localhost/127.0.0.1 — this is never allowed to be an
arbitrary remote URL.

Note: this requires the LangSmith web app to support rendering a custom app
from a live dev-server URL rather than uploaded files. If you get an error
opening the preview, that support may not have shipped yet — use
"langsmith apps push" as the fallback loop in the meantime.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if devURL == "" {
				return fmt.Errorf("--url is required (e.g. --url http://localhost:5173)")
			}
			parsed, err := url.Parse(devURL)
			if err != nil {
				return fmt.Errorf("parsing --url %q: %w", devURL, err)
			}
			host := parsed.Hostname()
			if host != "localhost" && host != "127.0.0.1" {
				return fmt.Errorf("--url must point at localhost or 127.0.0.1 (got %q) — dev preview never allows arbitrary remote URLs", host)
			}

			var link *appLink
			if dir != "" {
				link, err = readAppLink(dir)
				if err != nil {
					return err
				}
			}

			base := webURL
			if base == "" {
				base = webAppURLFromAPIURL(GetAPIURL())
			}

			previewURL, err := buildAppsDevPreviewURL(base, GetWorkspaceID(), devURL, link)
			if err != nil {
				return err
			}

			if !noOpen {
				_ = openBrowser(previewURL)
			}
			output.OutputJSON(map[string]any{
				"status": "opened",
				"url":    previewURL,
			}, "")
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Local app directory (reads .langsmith/app.json for context, if linked)")
	cmd.Flags().StringVar(&devURL, "url", "", "Local dev server URL to preview (required, localhost/127.0.0.1 only)")
	cmd.Flags().StringVar(&webURL, "web-url", "", "Override the LangSmith web app origin (defaults to inferring from --api-url)")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Print the preview URL instead of opening a browser")
	return cmd
}

// webAppURLFromAPIURL derives the LangSmith web app origin from the API
// origin. SaaS serves them from different subdomains (api.smith... vs
// smith...); self-hosted/local-dev deployments are single-origin, so API
// and web share the same host — see root CLAUDE.md's deployment_topology.
func webAppURLFromAPIURL(apiURL string) string {
	u, err := url.Parse(apiURL)
	if err != nil || u.Host == "" {
		return apiURL
	}
	if strings.HasPrefix(u.Host, "api.") {
		u.Host = strings.TrimPrefix(u.Host, "api.")
	}
	u.Path = ""
	return u.String()
}

func buildAppsDevPreviewURL(webBase, workspaceID, devURL string, link *appLink) (string, error) {
	base, err := url.Parse(strings.TrimRight(webBase, "/"))
	if err != nil {
		return "", fmt.Errorf("parsing web URL %q: %w", webBase, err)
	}
	if workspaceID != "" {
		base.Path = "/o/" + workspaceID + "/custom-apps/dev"
	} else {
		base.Path = "/custom-apps/dev"
	}

	q := url.Values{}
	q.Set("dev_url", devURL)
	if link != nil {
		q.Set("app_id", link.AppID)
		if link.ContextType != "" {
			q.Set("context_type", link.ContextType)
		}
	}
	base.RawQuery = q.Encode()
	return base.String(), nil
}
