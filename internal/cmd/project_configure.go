package cmd

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/spf13/cobra"
)

func newProjectConfigureCmd() *cobra.Command {
	var project, projectID, name, description, dataset, tier string
	var seconds int64
	var dryRun, apply bool
	cmd := &cobra.Command{Use: "configure", Short: "Read, preview, or update project settings", Args: cobra.NoArgs}
	cmd.Long = `Without edit flags, read stored project settings. Null means no explicit value;
effective inherited defaults are not resolved. Changes require --dry-run or --apply.
Only supplied fields are changed. An empty --description clears its text.

Renaming may require updating applications that trace by project name. The default
dataset is a selection preference, not automatic trace ingestion. Trace tiers can
affect retention and billing and require appropriate permissions. Changing a tier
does not promise to rewrite existing traces' retention. Thread idle time affects
every thread evaluator. Concurrent edits can race the read-modify-write of extra
when changing idle time. Preview is not a frozen plan or authorization check.`
	cmd.Example = "  langsmith project configure --project-id PROJECT_ID\n  langsmith project configure --project-id PROJECT_ID --description 'Support agent' --default-dataset DATASET_ID --dry-run\n  langsmith project configure --project-id PROJECT_ID --trace-tier longlived --apply"
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		changed := cmd.Flags().Changed
		idleChanged := changed("thread-idle-seconds")
		changing := idleChanged || changed("name") || changed("description") || changed("default-dataset") || changed("trace-tier")
		if changing && dryRun == apply || !changing && (dryRun || apply) {
			return invalidProjectConfiguration("provide edit flags and exactly one of --dry-run or --apply, or omit both to read")
		}
		if idleChanged && seconds < 120 {
			return invalidProjectConfiguration("thread idle time must be at least 120 seconds")
		}
		if changed("name") && strings.TrimSpace(name) == "" {
			return invalidProjectConfiguration("--name must not be blank")
		}
		if changed("default-dataset") && strings.TrimSpace(dataset) == "" {
			return invalidProjectConfiguration("--default-dataset must be a name or UUID; clearing is not supported")
		}
		if changed("trace-tier") && tier != "shortlived" && tier != "longlived" {
			return invalidProjectConfiguration("--trace-tier must be shortlived or longlived; resetting inheritance is not supported")
		}
		c, err := getClient()
		if err != nil {
			return err
		}
		id, err := resolveSessionID(cmd.Context(), c, project, projectID, "project configure")
		if err != nil {
			return err
		}
		session, err := c.SDK.Sessions.Get(cmd.Context(), id, langsmith.SessionGetParams{})
		if err != nil {
			return err
		}
		if session == nil {
			return invalidProjectConfiguration("service returned no project settings")
		}
		result := map[string]any{"project_id": id, "workspace_id": resultWorkspaceID(), "thread_idle_seconds": session.Extra["thread_idle_seconds"], "scope": "project"}
		before := projectSettingsView(session)
		result["settings"] = before
		result["effective_defaults_resolved"] = false
		if !changing {
			if session.Extra["thread_idle_seconds"] == nil {
				result["message"] = "No explicit thread idle time is stored. Null does not mean zero seconds or disabled evaluation."
				result["next_steps"] = []string{"Use --thread-idle-seconds with --dry-run to preview an explicit project-wide value; this affects all thread evaluators."}
			}
			return output.OutputJSON(result, "")
		}
		params := langsmith.SessionUpdateParams{}
		requested := map[string]any{}
		warnings := []string{}
		if changed("name") {
			params.Name = langsmith.F(name)
			requested["name"] = name
			warnings = append(warnings, "Update applications that trace by project name after renaming; otherwise they may send traces to a different project.")
		}
		if changed("description") {
			params.Description = langsmith.F(description)
			requested["description"] = description
		}
		if changed("default-dataset") {
			ds, err := resolveDataset(cmd.Context(), c, dataset)
			if err != nil {
				return err
			}
			params.DefaultDatasetID = langsmith.F(ds.ID)
			requested["default_dataset_id"] = ds.ID
			warnings = append(warnings, "The default dataset is a selection preference; no traces are imported and no evaluations are started.")
		}
		if changed("trace-tier") {
			params.TraceTier = langsmith.F(langsmith.SessionUpdateParamsTraceTier(tier))
			requested["trace_tier"] = tier
			warnings = append(warnings, "Trace tier changes may affect retention and billing. Permissions are checked on apply; existing traces' retention is not verified by this operation.")
		}
		if idleChanged {
			extra := make(map[string]any, len(session.Extra)+1)
			for key, value := range session.Extra {
				extra[key] = value
			}
			extra["thread_idle_seconds"] = seconds
			params.Extra = langsmith.F(extra)
			requested["thread_idle_seconds"] = seconds
			result["previous_thread_idle_seconds"] = session.Extra["thread_idle_seconds"]
			result["thread_idle_seconds"] = seconds
			warnings = append(warnings, "Idle time affects all thread evaluators; concurrent edits to project extra settings can race this update.")
		}
		after := make(map[string]any, len(before))
		for key, value := range before {
			after[key] = value
		}
		for key, value := range requested {
			after[key] = value
		}
		result["previous_settings"], result["settings"], result["changes"] = before, after, requested
		result["warnings"] = warnings
		result["status"] = "dry_run"
		result["next_steps"] = []string{"No project settings changed. Review the project-wide impact, then replace --dry-run with --apply if approved."}
		if apply {
			if idleChanged {
				fresh, err := c.SDK.Sessions.Get(cmd.Context(), id, langsmith.SessionGetParams{})
				if err != nil || fresh == nil {
					return commandDiagnostic{"project_preflight_failed", "Could not recheck project metadata before writing; no update was sent", "Read project configure and preview the change again before applying."}
				}
				if !reflect.DeepEqual(session.Extra, fresh.Extra) {
					return commandDiagnostic{"project_configuration_conflict", "Project metadata changed during preparation; no update was sent", "Read project configure and review a fresh preview. The client-side check is not an atomic concurrency guarantee."}
				}
			}
			_, err := c.SDK.Sessions.Update(cmd.Context(), id, params, option.WithMaxRetries(0))
			if err != nil {
				return projectConfigurationUnverified()
			}
			verified, err := c.SDK.Sessions.Get(cmd.Context(), id, langsmith.SessionGetParams{})
			if err != nil || verified == nil {
				return projectConfigurationUnverified()
			}
			actual := projectSettingsView(verified)
			for key, want := range requested {
				matches := actual[key] == want
				if key == "thread_idle_seconds" {
					matches = fmt.Sprint(actual[key]) == fmt.Sprint(want)
				}
				if !matches {
					return projectConfigurationUnverified()
				}
			}
			result["settings"] = actual
			result["thread_idle_seconds"] = actual["thread_idle_seconds"]
			result["status"] = "updated"
			result["verification"] = "read_back"
			result["next_steps"] = []string{"Requested settings were read back and verified. This operation does not instrument applications, import examples, backfill evaluators, or verify existing traces' retention. Use the project ID for subsequent commands, especially after a rename."}
		}
		return output.OutputJSON(result, "")
	}
	addProjectFlags(cmd, &project, &projectID)
	cmd.Flags().StringVar(&name, "name", "", "New project name; update applications tracing by name")
	cmd.Flags().StringVar(&description, "description", "", "Replacement description; an empty string clears the text")
	cmd.Flags().StringVar(&dataset, "default-dataset", "", "Default dataset name or UUID; does not import traces")
	cmd.Flags().StringVar(&tier, "trace-tier", "", "Project trace tier: shortlived or longlived; may affect retention/billing")
	cmd.Flags().Int64Var(&seconds, "thread-idle-seconds", 0, "Project-wide wait after thread activity, at least 120 seconds")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without changing project settings")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply only explicitly supplied settings and verify by reading them back")
	return cmd
}

// Decode only allowlisted fields, preserving explicit empty strings and nulls.
// Never print arbitrary extra settings, which can contain sensitive metadata.
func projectSettingsView(session *langsmith.TracerSession) map[string]any {
	var raw map[string]any
	_ = json.Unmarshal([]byte(session.JSON.RawJSON()), &raw)
	return map[string]any{"name": raw["name"], "description": raw["description"], "default_dataset_id": raw["default_dataset_id"], "trace_tier": raw["trace_tier"], "thread_idle_seconds": session.Extra["thread_idle_seconds"]}
}

func invalidProjectConfiguration(message string) error {
	return commandDiagnostic{"invalid_project_configuration", message, "Run project configure --help. Preview supported fields with --dry-run before approving --apply; server limits and permissions are checked on apply."}
}

func projectConfigurationUnverified() error {
	return commandDiagnostic{"project_configuration_unverified", "project settings write could not be verified", "Read project configure before retrying; the change may have applied. No automatic write retry was performed."}
}
