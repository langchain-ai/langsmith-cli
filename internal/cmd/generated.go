package cmd

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	generated "github.com/langchain-ai/langsmith-cli/internal/generated/pkg/cmd"
	"github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
	"github.com/urfave/cli/v3"
)

// exitGenerated ends the process with the generated command's exit code after
// its error has been printed. Tests replace it.
var exitGenerated = os.Exit

// generatedResources returns the resource commands in the generated tree,
// which contains only the resources exposed by the generator's configuration.
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
				rest, flags, err := translateGlobalFlags(args)
				if err != nil {
					return err
				}
				// --yes belongs to delete operations only; elsewhere the generated
				// tree rejects it as an unknown flag.
				if isDeleteInvocation(resource, rest, generatedValueFlags()) {
					var yes bool
					rest, yes = extractFlag(rest, "--yes")
					args, _ = extractFlag(args, "--yes")
					if !yes && !hasHelpFlag(rest) {
						if err := confirmDelete(cmd, deleteConfirmation{
							target:   "the " + strings.ReplaceAll(name, "-", " ") + " below",
							identity: "Command: langsmith " + name + " " + strings.Join(redactCredentials(args), " "),
						}); err != nil {
							return err
						}
					}
				}
				return runGenerated(cmd, resource, rest, flags)
			},
		})
	}
}

// redactCredentials returns args with the value of every --api-key replaced by
// ***, so the command can be echoed without leaking the key.
func redactCredentials(args []string) []string {
	redacted := make([]string, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		redacted[i] = arg
		if arg == "--" {
			copy(redacted[i:], args[i:])
			break
		}
		if strings.HasPrefix(arg, "--api-key=") {
			redacted[i] = "--api-key=***"
		} else if arg == "--api-key" && i+1 < len(args) {
			redacted[i+1] = "***"
			i++
		}
	}
	return redacted
}

// generatedValueFlags returns every spelling of the generated root's flags
// that takes a separate value argument. langsmith's own global flags are
// removed by translateGlobalFlags before operations are identified.
func generatedValueFlags() map[string]bool {
	names := map[string]bool{}
	for _, f := range generated.Command.Flags {
		// A flag that cannot say whether it takes a value is treated as taking
		// one, which at worst hides the operation and keeps the delete prompt.
		if v, ok := f.(interface{ TakesValue() bool }); ok && !v.TakesValue() {
			continue
		}
		for _, n := range f.Names() {
			if len(n) == 1 {
				names["-"+n] = true
			} else {
				names["--"+n] = true
			}
		}
	}
	return names
}

// operationName returns the generated operation named in args, for example
// "delete" in `prompt-webhooks delete --webhook-id x`: the first positional
// argument, skipping flags in valueFlags and their values.
func operationName(resource *cli.Command, args []string, valueFlags map[string]bool) string {
	arg, ok := firstPositional(args, valueFlags)
	if !ok {
		return ""
	}
	for _, op := range resource.Commands {
		if arg == op.Name {
			return arg
		}
	}
	return ""
}

// firstPositional returns the first argument that is neither a flag nor the
// value of a flag in valueFlags.
func firstPositional(args []string, valueFlags map[string]bool) (string, bool) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return "", false
		}
		if strings.HasPrefix(arg, "-") {
			if valueFlags[arg] {
				i++
			}
			continue
		}
		return arg, true
	}
	return "", false
}

// isDeleteInvocation reports whether args may run a delete operation. When the
// operation cannot be identified, any argument naming a delete operation counts,
// so an unrecognized flag can never skip the confirmation.
func isDeleteInvocation(resource *cli.Command, args []string, valueFlags map[string]bool) bool {
	if op := operationName(resource, args, valueFlags); op != "" {
		return strings.HasPrefix(op, "delete")
	}
	for _, arg := range args {
		for _, op := range resource.Commands {
			if arg == op.Name && strings.HasPrefix(op.Name, "delete") {
				return true
			}
		}
	}
	return false
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

// runGenerated hands the arguments to the generated command tree. The mounted
// command disables Cobra's flag parsing, so translateGlobalFlags extracts
// langsmith's global flags from the arguments. They are resolved exactly as
// for hand-written commands and passed to the generated root as explicit
// --base-url, --tenant-id, and --api-key flags, which take precedence over
// anything the Go SDK reads from the environment or the config file.
//
// The generated root cannot take a bearer token, so a profile that
// authenticates with OAuth is selected through LANGSMITH_PROFILE and the SDK
// loads its token; the explicit --base-url still pins the host the token is
// sent to.
func runGenerated(cmd *cobra.Command, resource *cli.Command, rest []string, flags globalFlags) error {
	argv := []string{"langsmith"}
	if !isHelpInvocation(rest) {
		opts, err := resolveAuthenticatedOptions(flags)
		if err != nil {
			return err
		}
		argv = append(argv, "--base-url="+opts.APIURL)
		if opts.WorkspaceID != "" {
			argv = append(argv, "--tenant-id="+opts.WorkspaceID)
		}
		if opts.APIKey != "" {
			argv = append(argv, "--api-key="+opts.APIKey)
		}
		if opts.ProfileName != "" {
			if err := os.Setenv("LANGSMITH_PROFILE", opts.ProfileName); err != nil {
				return err
			}
		}
	}
	argv = append(argv, resource.Name)
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
	return nil
}

// isHelpInvocation reports whether args only ask for help, which needs no
// credentials.
func isHelpInvocation(args []string) bool {
	if hasHelpFlag(args) {
		return true
	}
	arg, ok := firstPositional(args, generatedValueFlags())
	return !ok || arg == "help"
}

// translateGlobalFlags removes langsmith's global auth and routing flags from
// args and returns their values. The generated root's own spellings,
// --base-url and --tenant-id, are accepted as aliases of --api-url and
// --workspace.
func translateGlobalFlags(args []string) (rest []string, flags globalFlags, err error) {
	targets := map[string]*string{
		"--api-key":      &flags.APIKey,
		"--api-url":      &flags.APIURL,
		"--base-url":     &flags.APIURL,
		"--profile":      &flags.Profile,
		"--workspace":    &flags.WorkspaceID,
		"--workspace-id": &flags.WorkspaceID,
		"--tenant-id":    &flags.WorkspaceID,
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			rest = append(rest, args[i:]...)
			break
		}
		flag, value, hasValue := strings.Cut(arg, "=")
		target, ok := targets[flag]
		if !ok {
			rest = append(rest, arg)
			continue
		}
		if !hasValue {
			if i+1 >= len(args) {
				return nil, flags, fmt.Errorf("flag needs an argument: %s", flag)
			}
			value = args[i+1]
			i++
		}
		*target = value
	}
	return rest, flags, nil
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
