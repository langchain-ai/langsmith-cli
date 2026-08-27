package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

// overviewRepoOwner is the pseudo-owner Engine's Agent Overview repos are
// created under — they're internal, session-scoped hub repos with no real
// owner, mirroring the "-" convention already used for hub get/pull.
const overviewRepoOwner = "-"

// overviewRepoHandle mirrors the Python agent's own handle derivation
// (smith_issues_agent/graph.py: f"ao-{session_id[:8]}").
func overviewRepoHandle(sessionID string) string {
	n := 8
	if len(sessionID) < n {
		n = len(sessionID)
	}
	return "ao-" + sessionID[:n]
}

func newProjectIssuesOverviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "overview",
		Short: "Read a project's Agent Overview",
		Long: `Read a project's Agent Overview — Engine's per-project knowledge doc.

The Agent Overview is stored as a Prompt Hub commit under the hood; this
command hides that so callers get back plain markdown. Saving is done via
Engine's own save_ao tool, not this CLI.

Examples:
  langsmith project issues overview pull --project my-app`,
	}

	cmd.AddCommand(newProjectIssuesOverviewPullCmd())
	return cmd
}

func newProjectIssuesOverviewPullCmd() *cobra.Command {
	var (
		project    string
		projectID  string
		outputFile string
	)

	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Fetch the current Agent Overview as plain markdown",
		Long: `Fetch the current Agent Overview for a project and write it as plain markdown.

Extracts the template text out of whatever shape the backing Prompt Hub
commit happens to be in — an agent-authored PromptTemplate, or a
ChatPromptTemplate/RunnableSequence produced by editing it in the Hub UI —
so the caller always gets back plain markdown, never a manifest to unwrap.

Fails if the project has no Agent Overview yet (first scan).

Examples:
  langsmith project issues overview pull --project my-app
  langsmith project issues overview pull --project my-app -o /tmp/scan/agent_overview.md`,
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			sessionID, err := resolveSessionID(ctx, c, project, projectID, "project issues overview pull")
			if err != nil {
				ExitErrorf("%v", err)
			}
			projectLabel := ResolveProject(project)
			if projectLabel == "" {
				projectLabel = sessionID
			}

			repo := overviewRepoHandle(sessionID)
			resp, err := c.SDK.Commits.Get(ctx, overviewRepoOwner, repo, "latest", langsmith.CommitGetParams{})
			if err != nil {
				ExitErrorf("no Agent Overview exists yet for %q: %v", projectLabel, err)
			}

			manifest, _ := resp.Manifest.(map[string]interface{})
			template := extractOverviewTemplate(manifest)
			if template == "" {
				ExitErrorf("Agent Overview commit for %q has no readable template", projectLabel)
			}

			if outputFile == "" {
				fmt.Print(template)
				return
			}
			if err := os.WriteFile(outputFile, []byte(template), 0o644); err != nil {
				ExitErrorf("writing %s: %v", outputFile, err)
			}
			output.OutputJSON(map[string]any{
				"status":      "pulled",
				"project":     projectLabel,
				"commit_hash": resp.CommitHash,
				"file":        outputFile,
				"bytes":       len(template),
			}, "")
		},
	}

	addProjectFlags(cmd, &project, &projectID)
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write markdown to this file instead of stdout")
	return cmd
}

// extractOverviewTemplate extracts template text from a hub commit manifest.
// Mirrors smith_issues_agent/_prompt_hub.py::extract_template — keep the two
// in sync if either changes. Handles three formats the hub UI / SDK produce:
//   - PromptTemplate: kwargs.template (our canonical AO format)
//   - ChatPromptTemplate: kwargs.messages[0].kwargs.prompt.kwargs.template
//   - RunnableSequence (prompt+model): kwargs.first.kwargs.messages[0]...
func extractOverviewTemplate(manifest map[string]interface{}) string {
	if manifest == nil {
		return ""
	}
	kwargs, _ := manifest["kwargs"].(map[string]interface{})
	if kwargs == nil {
		return ""
	}

	if template, ok := kwargs["template"].(string); ok {
		return template
	}

	if messages, ok := kwargs["messages"].([]interface{}); ok && len(messages) > 0 {
		if msg, ok := messages[0].(map[string]interface{}); ok {
			return overviewTemplateFromMessage(msg)
		}
	}

	if first, ok := kwargs["first"].(map[string]interface{}); ok {
		return extractOverviewTemplate(first)
	}

	return ""
}

func overviewTemplateFromMessage(msg map[string]interface{}) string {
	kwargs, _ := msg["kwargs"].(map[string]interface{})
	if kwargs == nil {
		return ""
	}
	prompt, ok := kwargs["prompt"].(map[string]interface{})
	if !ok {
		return ""
	}
	promptKwargs, _ := prompt["kwargs"].(map[string]interface{})
	if promptKwargs == nil {
		return ""
	}
	template, _ := promptKwargs["template"].(string)
	return template
}
