package cmd

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

const projectContextFile = ".langsmith-context.json"

type projectContext struct {
	Version     int    `json:"version"`
	Profile     string `json:"profile,omitempty"`
	WorkspaceID string `json:"workspace_id"`
	ProjectID   string `json:"project_id"`
	APIURL      string `json:"api_url"`
}

var activeProjectContext *projectContext

func workflowFailure(code, message, next string) error {
	return commandDiagnostic{code: code, message: message, next: next}
}

func loadProjectContext(cmd *cobra.Command) error {
	// Only the current directory is consulted. File URLs never select a server.
	info, err := os.Lstat(projectContextFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 16384 {
		return workflowFailure("invalid_context", "context must be a regular file under 16 KiB", "use --no-context to bypass")
	}
	b, err := os.ReadFile(projectContextFile)
	if err != nil {
		return err
	}
	var pc projectContext
	if err = json.Unmarshal(b, &pc); err != nil {
		return workflowFailure("invalid_context", "cannot parse project context", "use --no-context and inspect the file")
	}
	if pc.Version != 1 {
		return workflowFailure("invalid_context", "unsupported context version", "use --no-context")
	}
	for _, id := range []string{pc.ProjectID, pc.WorkspaceID} {
		if _, err := uuid.Parse(id); err != nil {
			return workflowFailure("invalid_context", "project and workspace must be UUIDs", "use --no-context")
		}
	}
	if pc.Profile != "" && (os.Getenv("LANGSMITH_API_KEY") != "" || os.Getenv("LANGCHAIN_API_KEY") != "" || flagAPIKey != "") {
		return workflowFailure("credential_conflict", "an API key is set alongside the saved profile", "unset the conflicting key variables or use --no-context to choose a different target")
	}
	for _, v := range []string{flagProfile, os.Getenv("LANGSMITH_PROFILE")} {
		if v != "" && v != pc.Profile {
			return workflowFailure("context_conflict", "profile differs from project context", "use --no-context for an explicit override")
		}
	}
	for _, v := range []string{flagWorkspaceID, os.Getenv("LANGSMITH_WORKSPACE_ID"), os.Getenv("LANGSMITH_TENANT_ID")} {
		if v != "" && v != pc.WorkspaceID {
			return workflowFailure("context_conflict", "workspace differs from project context", "use --no-context for an explicit override")
		}
	}
	if flagProfile == "" {
		flagProfile = pc.Profile
	}
	if flagWorkspaceID == "" {
		flagWorkspaceID = pc.WorkspaceID
	}
	// Resolve without refreshing: an edited context must not redirect an OAuth refresh.
	opts, err := resolveClientOptions(false)
	if err != nil {
		return err
	}
	if client.NormalizeURL(opts.APIURL) != pc.APIURL {
		return workflowFailure("endpoint_conflict", "saved endpoint differs from the configured profile/environment", "use --no-context and verify the intended endpoint")
	}
	activeProjectContext = &pc
	return nil
}

func newOnboardingInitCmd() *cobra.Command {
	var project, projectID string
	cmd := &cobra.Command{Use: "init", Short: "Verify an existing project and save credential-free local context", Args: cobra.NoArgs,
		Long: "Bind this directory to an existing tracing project. Verifies project read access\nand writes a new .langsmith-context.json containing IDs and a profile label, never\ncredentials. Does not instrument an app or create traces. Use project create first\nif needed. Existing context files are not overwritten.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := os.Lstat(projectContextFile); err == nil {
				return workflowFailure("context_exists", "local context already exists", "inspect it or use --no-context for another target")
			} else if !os.IsNotExist(err) {
				return err
			}
			if flagProfile != "" && (os.Getenv("LANGSMITH_API_KEY") != "" || os.Getenv("LANGCHAIN_API_KEY") != "" || flagAPIKey != "") {
				return workflowFailure("credential_conflict", "explicit profile conflicts with a supplied API key", "unset conflicting key variables before binding the profile")
			}
			opts, err := resolveClientOptions(false)
			if err != nil {
				return err
			}
			if _, err := uuid.Parse(opts.WorkspaceID); err != nil {
				return workflowFailure("workspace_required", "choose an explicit workspace UUID", "run workspace list, then pass --workspace UUID")
			}
			c, err := getClient()
			if err != nil {
				return workflowFailure("authentication_required", "no usable authentication configuration", "run auth login or configure an API-key profile")
			}
			id, err := resolveSessionID(cmd.Context(), c, project, projectID, "init")
			if err != nil {
				return err
			}
			session, err := c.SDK.Sessions.Get(cmd.Context(), id, langsmith.SessionGetParams{})
			if err != nil {
				return workflowFailure("project_access_failed", "could not read the selected project", "verify profile, workspace, and project with --no-context")
			}
			pc := projectContext{Version: 1, Profile: opts.ProfileName, WorkspaceID: opts.WorkspaceID, ProjectID: session.ID, APIURL: c.APIURL()}
			if pc.Profile == "" {
				pc.Profile = flagProfile
			}
			if pc.Profile == "" {
				pc.Profile = strings.TrimSpace(os.Getenv("LANGSMITH_PROFILE"))
			}
			f, err := os.OpenFile(projectContextFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				return err
			}
			enc := json.NewEncoder(f)
			enc.SetIndent("", "  ")
			err = enc.Encode(pc)
			closeErr := f.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
			return output.OutputJSON(map[string]any{"status": "configured", "context": pc, "project_name": session.Name, "instrumented": false, "next_actions": []string{"langsmith doctor --format json", "Instrument your application using the LangSmith SDK; trace setup is for coding-agent tracing", "Run your application once, then langsmith trace verify --format json"}}, "")
		},
	}
	addProjectFlags(cmd, &project, &projectID)
	return cmd
}

func newDoctorCmd() *cobra.Command {
	var project, projectID string
	cmd := &cobra.Command{Use: "doctor", Short: "Check authentication and project read access without writing traces", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if (flagProfile != "" || os.Getenv("LANGSMITH_PROFILE") != "") && (flagAPIKey != "" || os.Getenv("LANGSMITH_API_KEY") != "" || os.Getenv("LANGCHAIN_API_KEY") != "") {
				return workflowFailure("credential_conflict", "profile and API-key authentication are both selected", "choose one credential source; no secrets were printed")
			}
			c, err := getClient()
			if err != nil {
				return workflowFailure("authentication_required", "authentication setup is unavailable or expired", "run auth login for the intended profile")
			}
			id, err := resolveSessionID(cmd.Context(), c, project, projectID, "doctor")
			if err != nil {
				return workflowFailure("project_required", "no unambiguous project selected", "run init or pass --project-id")
			}
			p, err := c.SDK.Sessions.Get(cmd.Context(), id, langsmith.SessionGetParams{})
			if err != nil {
				return workflowFailure("project_access_failed", "selected project is not readable", "verify credentials and workspace; read access is required")
			}
			return output.OutputJSON(map[string]any{"status": "ready", "project_id": p.ID, "project_name": p.Name, "api_url": c.APIURL(), "read_access": "verified", "write_access": "not_tested", "inference_access": "not_tested", "instrumentation": "not_tested", "next_action": "langsmith trace verify --format json"}, "")
		},
	}
	addProjectFlags(cmd, &project, &projectID)
	return cmd
}
