package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// DiagnoseCommandError preserves operational errors and annotates Cobra's
// required-flag errors using registered flag names, never argument values.
func DiagnoseCommandError(cmd *cobra.Command, err error) error {
	if cmd == nil || err == nil || !cmd.Flags().Parsed() {
		return err
	}
	validation := cmd.ValidateRequiredFlags()
	if validation == nil || validation.Error() != err.Error() {
		if groupErr := cmd.ValidateFlagGroups(); groupErr != nil && groupErr.Error() == err.Error() {
			return commandDiagnostic{"invalid_flag_combination", groupErr.Error(), "See " + cmd.CommandPath() + " --help for supported flag combinations."}
		}
		return err
	}
	var missing []string
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if required := flag.Annotations[cobra.BashCompOneRequiredFlag]; len(required) > 0 && required[0] == "true" && !flag.Changed {
			missing = append(missing, "--"+flag.Name)
		}
	})
	return commandDiagnostic{"missing_required_flag", "Missing required flags: " + strings.Join(missing, ", "), "Supply the required flags. See " + cmd.CommandPath() + " --help for usage."}
}

// attachArgumentDiagnostics wraps validation before command execution. Only
// registered usage text is exposed; supplied arguments may contain secrets.
func attachArgumentDiagnostics(cmd *cobra.Command) {
	if validate := cmd.Args; validate != nil {
		cmd.Args = func(c *cobra.Command, args []string) error {
			if err := validate(c, args); err != nil {
				if GetFormat() != "json" {
					return err
				}
				return commandDiagnostic{"invalid_arguments", "Invalid positional arguments for " + c.CommandPath(), "Usage: " + c.UseLine() + ". See " + c.CommandPath() + " --help."}
			}
			return nil
		}
	}
	for _, child := range cmd.Commands() {
		attachArgumentDiagnostics(child)
	}
}

// commandDiagnostic contains only fixed, safe text. Keep argument values and
// upstream error bodies out of diagnostics rendered for machine consumers.
type commandDiagnostic struct {
	code    string
	message string
	next    string
}

func (e commandDiagnostic) Error() string { return e.message }

func (e commandDiagnostic) CLIDiagnostic() (string, string, string) {
	return e.code, e.message, e.next
}
