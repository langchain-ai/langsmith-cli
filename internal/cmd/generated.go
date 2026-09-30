package cmd

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	generated "github.com/langchain-ai/langsmith-cli/internal/generated/pkg/cmd"
	"github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
	"github.com/urfave/cli/v3"
)

// exitGenerated ends the process with the generated command's exit code after
// its error has been printed. Tests replace it.
var exitGenerated = os.Exit

// globalFlagRenames maps langsmith's global flags to the generated tree's names.
var globalFlagRenames = map[string]string{
	"--api-url":      "--base-url",
	"--workspace":    "--tenant-id",
	"--workspace-id": "--tenant-id",
}

// generatedResources returns the resource commands in the generated tree.
// generated-resources.txt at the repository root selects which resources are
// generated into internal/generated.
func generatedResources() []*cli.Command {
	var resources []*cli.Command
	for _, c := range generated.Command.Commands {
		if c.Category == "API RESOURCE" {
			resources = append(resources, c)
		}
	}
	return resources
}

// addGeneratedCommands mounts every generated resource as a `langsmith`
// command. A name that clashes with a hand-written command is a programming
// error.
func addGeneratedCommands(root *cobra.Command) {
	existing := map[string]bool{}
	for _, c := range root.Commands() {
		existing[c.Name()] = true
	}
	for _, resource := range generatedResources() {
		name := resource.Name
		if existing[name] {
			panic(fmt.Sprintf("generated command %q clashes with a hand-written command", name))
		}
		root.AddCommand(&cobra.Command{
			Use:                name,
			Short:              "Manage " + strings.ReplaceAll(name, "-", " "),
			DisableFlagParsing: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				// --yes belongs to delete operations only; elsewhere the generated
				// tree rejects it as an unknown flag.
				if op := operationName(resource, args); strings.HasPrefix(op, "delete") {
					var yes bool
					args, yes = extractFlag(args, "--yes")
					if !yes && !hasHelpFlag(args) {
						target := fmt.Sprintf("the %s selected by 'langsmith %s %s'", strings.ReplaceAll(name, "-", " "), name, op)
						if err := confirmDelete(cmd, deleteConfirmation{target: target}); err != nil {
							return err
						}
					}
				}
				runGenerated(cmd, name, args)
				return nil
			},
		})
	}
}

// operationName returns the generated operation named in args, for example
// "delete" in `prompt-webhooks delete --webhook-id x`.
func operationName(resource *cli.Command, args []string) string {
	for _, arg := range args {
		for _, op := range resource.Commands {
			if arg == op.Name {
				return arg
			}
		}
	}
	return ""
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

// extractFlag removes a boolean flag the generated tree does not define and
// reports whether it was present.
func extractFlag(args []string, flag string) ([]string, bool) {
	rest := make([]string, 0, len(args))
	found := false
	for _, arg := range args {
		if arg == flag || arg == flag+"=true" {
			found = true
			continue
		}
		rest = append(rest, arg)
	}
	return rest, found
}

// runGenerated hands the arguments to the generated command tree. Auth comes
// from the Go SDK's defaults (the current profile in ~/.langsmith/config.json,
// then LANGSMITH_* environment variables), which match how langsmith resolves
// them; langsmith's global flags are forwarded on top.
func runGenerated(cmd *cobra.Command, name string, args []string) {
	argv := []string{"langsmith"}
	if flagAPIKey != "" {
		argv = append(argv, "--api-key", flagAPIKey)
	}
	if flagAPIURL != "" {
		argv = append(argv, "--base-url", client.NormalizeURL(flagAPIURL))
	}
	if flagWorkspaceID != "" {
		argv = append(argv, "--tenant-id", flagWorkspaceID)
	}
	rest, profile := translateGlobalFlags(args)
	if profile == "" {
		profile = flagProfile
	}
	if profile != "" {
		// The SDK selects the profile from this variable.
		_ = os.Setenv("LANGSMITH_PROFILE", profile)
	}
	argv = append(argv, name)
	argv = append(argv, rest...)

	generated.CommandErrorBuffer.Reset()
	if err := generated.Command.Run(cmd.Context(), argv); err != nil {
		printGeneratedError(cmd.ErrOrStderr(), err)
		code := 1
		var exitErr cli.ExitCoder
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		}
		exitGenerated(code)
	}
}

// translateGlobalFlags renames langsmith global flags that appear after the
// command name and extracts --profile, which the generated tree does not define.
func translateGlobalFlags(args []string) (rest []string, profile string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		flag, value, hasValue := strings.Cut(arg, "=")
		if flag == "--profile" {
			if hasValue {
				profile = value
			} else if i+1 < len(args) {
				profile = args[i+1]
				i++
			}
			continue
		}
		if renamed, ok := globalFlagRenames[flag]; ok {
			if hasValue {
				arg = renamed + "=" + value
			} else {
				arg = renamed
			}
		}
		rest = append(rest, arg)
	}
	return rest, profile
}

func printGeneratedError(w io.Writer, err error) {
	var apiErr *langsmith.Error
	if errors.As(err, &apiErr) {
		fmt.Fprintf(w, "%s %q: %d %s\n", apiErr.Request.Method, apiErr.Request.URL, apiErr.Response.StatusCode, http.StatusText(apiErr.Response.StatusCode))
		if body := gjson.Parse(apiErr.JSON.RawJSON()); body.Exists() {
			fmt.Fprintln(w, body.Raw)
		}
		return
	}
	if generated.CommandErrorBuffer.Len() > 0 {
		_, _ = w.Write(generated.CommandErrorBuffer.Bytes())
		return
	}
	fmt.Fprintln(w, err.Error())
}
