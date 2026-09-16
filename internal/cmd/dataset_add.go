package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// dataset add shares preview validation and the existing deterministic import path.
func newDatasetAddCmd() *cobra.Command {
	preview := newDatasetSelectionPreviewCmd()
	preview.Use = "add"
	preview.Short = "Preview or apply trace/run examples to a dataset"
	preview.Long = "Select a trace, run, root-run filter, or thread, bounded by --limit. Thread import\ncreates one example per selected root turn (not a merged conversation or necessarily\nthe entire thread). With --dry-run --output selection.json, freeze inputs for review;\napply with --selection selection.json. Observed outputs become references only with\n--reference-mode observed. Selection replay reconciles existing examples without\nrerunning discovery. For conversation-level evaluation, curate a combined example."
	preview.Example = `  langsmith dataset add --dataset DATASET_ID --project-id PROJECT_ID --trace-id TRACE_ID --dry-run --output selection.json
  langsmith dataset add --dataset DATASET_ID --project-id PROJECT_ID --error --limit 20 --dry-run --output selection.json
  langsmith dataset add --dataset DATASET_ID --project-id PROJECT_ID --thread-id THREAD_ID --dry-run --output thread-selection.json
  langsmith dataset add --dataset DATASET_ID --project-id PROJECT_ID --selection selection.json`
	preview.Long += "\nPreview selection_info reports the effective limit, selected count and whether more matching roots existed during discovery. Thread selection is turn-level; has_more does not refer to a merged conversation. Import returns resource IDs, per-item results and counts."
	var dryRun bool
	var selection, runID, threadID string
	preview.Flags().BoolVar(&dryRun, "dry-run", false, "Read-only preview; use --output to freeze a selection")
	preview.Flags().StringVar(&selection, "selection", "", "Apply a frozen selection from a prior dry run")
	preview.Flags().StringVar(&runID, "run-id", "", "Individual run UUID (alias of --run-ids for one run)")
	preview.Flags().StringVar(&threadID, "thread-id", "", "Import the thread's root turns as separate examples")
	preview.RunE = func(cmd *cobra.Command, args []string) error {
		for _, name := range []string{"run-id", "thread-id", "selection"} {
			value, _ := cmd.Flags().GetString(name)
			if cmd.Flags().Changed(name) && strings.TrimSpace(value) == "" {
				return commandDiagnostic{"invalid_selector", "--" + name + " must not be blank", "Provide an explicit selector value or omit the flag."}
			}
		}
		dataset, _ := cmd.Flags().GetString("dataset")
		project, _ := cmd.Flags().GetString("project")
		projectID, _ := cmd.Flags().GetString("project-id")
		target := []string{"--dataset", dataset}
		if projectID != "" {
			target = append(target, "--project-id", projectID)
		} else if project != "" {
			target = append(target, "--project", project)
		}
		if selection != "" {
			if dryRun {
				return commandDiagnostic{"invalid_selection_mode", "--selection cannot be combined with --dry-run", "Review the frozen selection file first, then apply with --selection without discovery or dry-run flags."}
			}
			var conflict string
			cmd.Flags().Visit(func(f *pflag.Flag) {
				switch f.Name {
				case "dataset", "project", "project-id", "selection":
				default:
					if cmd.LocalNonPersistentFlags().Lookup(f.Name) != nil {
						conflict = f.Name
					}
				}
			})
			if conflict != "" {
				return commandDiagnostic{"invalid_selection_mode", "--selection cannot be combined with --" + conflict, "Apply the reviewed file using only dataset/project selectors and --selection; do not rerun discovery or override reviewed values."}
			}
			apply := newDatasetSelectionApplyCmd()
			return executeDatasetSubcommand(cmd, apply, append(target, "--selection", selection))
		}
		if !dryRun {
			return commandDiagnostic{"selection_required", "use --dry-run --output selection.json first, then --selection selection.json", "Preview with the intended source and dataset, review the saved inputs and reference outputs, then apply that file with --selection."}
		}
		forwarded := []string{}
		fresh := newDatasetSelectionPreviewCmd()
		cmd.Flags().Visit(func(f *pflag.Flag) {
			if f.Name != "dry-run" && f.Name != "selection" && f.Name != "run-id" && f.Name != "thread-id" && fresh.Flags().Lookup(f.Name) != nil {
				forwarded = append(forwarded, "--"+f.Name+"="+f.Value.String())
			}
		})
		if runID != "" {
			if cmd.Flags().Changed("run-ids") || cmd.Flags().Changed("trace-id") {
				return fmt.Errorf("use only one of --run-id, --run-ids, and --trace-id")
			}
			forwarded = append(forwarded, "--run-ids", runID)
		}
		if threadID != "" {
			for _, f := range []string{"run-id", "run-ids", "trace-id", "trace-ids", "filter", "since", "before", "last-n-minutes"} {
				if cmd.Flags().Changed(f) {
					return fmt.Errorf("--thread-id cannot be combined with --%s", f)
				}
			}
			// Explicit thread selection must not inherit the default seven-day window.
			forwarded = append(forwarded, "--filter", fmt.Sprintf("eq(thread_id, %q)", threadID), "--since", "1970-01-01")
		}
		return executeDatasetSubcommand(cmd, fresh, forwarded)
	}
	return preview
}

// Only the outer command renders diagnostics. Nested commands reuse its context
// and streams without printing an additional Cobra error or usage block.
func executeDatasetSubcommand(parent, child *cobra.Command, args []string) error {
	child.SilenceErrors = true
	child.SilenceUsage = true
	child.SetContext(parent.Context())
	child.SetIn(parent.InOrStdin())
	child.SetOut(parent.OutOrStdout())
	child.SetErr(parent.ErrOrStderr())
	child.SetArgs(args)
	return child.Execute()
}
