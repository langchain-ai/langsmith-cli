package cmd

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	generated "github.com/langchain-ai/langsmith-cli/internal/generated/pkg/cmd"
	"github.com/langchain-ai/langsmith-cli/internal/usage"
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

// generatedAnnotation holds the name of the generated resource a command runs.
const generatedAnnotation = "langsmith/generated-resource"

// generatedOperationNames gives a command name to generated operations named
// after an SDK method that reads badly as a command, keyed by
// "<resource> <operation>". The SDK method keeps its name.
var generatedOperationNames = map[string]string{
	"annotation-queues annotation-queues":          "create",
	"annotation-queues retrieve-annotation-queues": "list",
}

// operationCommandName returns the command name of a generated operation.
func operationCommandName(resource, op string) string {
	if name, ok := generatedOperationNames[resource+" "+op]; ok {
		return name
	}
	return op
}

// renameGeneratedOperations renames operations in the generated tree itself,
// so its help, errors and dispatch all use the command name.
func renameGeneratedOperations(resources []*cli.Command) {
	for _, r := range resources {
		for _, op := range r.Commands {
			op.Name = operationCommandName(r.Name, op.Name)
		}
	}
}

func addGeneratedCommands(root *cobra.Command) {
	// The generated root validates every subcommand's flags on each run, so
	// drop its helpers (@manpages, __complete), which langsmith covers itself.
	resources := generatedResources()
	generated.Command.Commands = resources
	renameGeneratedOperations(resources)
	mountGeneratedResources(root, resources)
}

// mountGeneratedResources mounts sessions:insights as `sessions insights`.
func mountGeneratedResources(root *cobra.Command, resources []*cli.Command) {
	parents := map[string]bool{}
	for _, r := range resources {
		path := strings.Split(r.Name, ":")
		for i := 1; i < len(path); i++ {
			parents[strings.Join(path[:i], ":")] = true
		}
	}
	for _, r := range resources {
		mountGenerated(root, r, parents[r.Name])
	}
}

// mountGenerated adds a free leaf resource as one command, otherwise one command
// per operation. Hand-written subcommands win.
func mountGenerated(root *cobra.Command, resource *cli.Command, hasSubresources bool) {
	path := strings.Split(resource.Name, ":")
	parent := root
	for _, name := range path[:len(path)-1] {
		parent = mountPoint(parent, name, resource)
	}
	name := path[len(path)-1]
	if subcommand(parent, name) == nil && !hasSubresources {
		parent.AddCommand(generatedCommand(name, resource, nil))
		return
	}
	group := mountPoint(parent, name, resource)
	for _, op := range resource.Commands {
		switch c := subcommand(group, op.Name); {
		case c == nil:
			group.AddCommand(generatedCommand(op.Name, resource, op))
		case c.Annotations[generatedAnnotation] != "":
			panic(fmt.Sprintf("generated %s %s clashes with generated %s", resource.Name, op.Name, c.Annotations[generatedAnnotation]))
		}
	}
}

// subcommand finds a subcommand by name or alias.
func subcommand(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name || slices.Contains(c.Aliases, name) {
			return c
		}
	}
	return nil
}

// handWritten finds a hand-written subcommand by name or alias.
func handWritten(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Annotations[generatedAnnotation] == "" && (c.Name() == name || slices.Contains(c.Aliases, name)) {
			return c
		}
	}
	return nil
}

// mountPoint finds or adds the group resource mounts under. Nesting under a
// generated command is a clash.
func mountPoint(parent *cobra.Command, name string, resource *cli.Command) *cobra.Command {
	if c := subcommand(parent, name); c != nil {
		if other := c.Annotations[generatedAnnotation]; other != "" {
			panic(fmt.Sprintf("generated %s clashes with generated %s at %s", resource.Name, other, c.CommandPath()))
		}
		return c
	}
	group := &cobra.Command{
		Use:   name,
		Short: manageShort(name),
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	parent.AddCommand(group)
	return group
}

func manageShort(name string) string {
	return "Manage " + strings.ReplaceAll(name, "-", " ")
}

// generatedCommand runs the generated resource, or only op when set.
func generatedCommand(name string, resource, op *cli.Command) *cobra.Command {
	c := &cobra.Command{
		Use:                name,
		Short:              manageShort(name),
		DisableFlagParsing: true,
		Annotations:        map[string]string{generatedAnnotation: resource.Name},
	}
	if op != nil {
		c.Short, c.Aliases, c.Hidden = op.Usage, op.Aliases, op.Hidden
	}
	c.RunE = func(cmd *cobra.Command, args []string) error {
		path := cmd.CommandPath()
		if op != nil {
			path = cmd.Parent().CommandPath()
			args = append([]string{op.Name}, args...)
		}
		return forwardGenerated(cmd, resource, path, args)
	}
	return c
}

func forwardGenerated(cmd *cobra.Command, resource *cli.Command, path string, args []string) error {
	rest, flags, err := translateGlobalFlags(args)
	if err != nil {
		return err
	}
	// --yes belongs to delete operations only; elsewhere the generated tree
	// rejects it as an unknown flag.
	if isDeleteInvocation(resource, rest, generatedValueFlags()) {
		var yes bool
		rest, yes = extractFlag(rest, "--yes")
		args, _ = extractFlag(args, "--yes")
		if !yes && !hasHelpFlag(rest) {
			noun := strings.ReplaceAll(strings.TrimPrefix(path, cmd.Root().Name()+" "), "-", " ")
			if err := confirmDelete(cmd, deleteConfirmation{
				target:   "the " + noun + " below",
				identity: "Command: " + path + " " + strings.Join(redactCredentials(args), " "),
			}); err != nil {
				return err
			}
		}
	}
	return runGenerated(cmd, resource, rest, flags)
}

// shadowedGeneratedOperations lists generated operation names and aliases a
// hand-written subcommand hides; skip or rename them in stainless.yml.
func shadowedGeneratedOperations(root *cobra.Command) []string {
	var shadowed []string
	for _, resource := range generatedResources() {
		mount := root
		for _, name := range strings.Split(resource.Name, ":") {
			if mount = subcommand(mount, name); mount == nil {
				break
			}
		}
		if mount == nil || mount.Annotations[generatedAnnotation] == resource.Name {
			continue
		}
		for _, op := range resource.Commands {
			for _, name := range append([]string{op.Name}, op.Aliases...) {
				if handWritten(mount, name) == nil {
					continue
				}
				if name != op.Name {
					name += " (alias of " + op.Name + ")"
				}
				shadowed = append(shadowed, mount.CommandPath()+" "+name)
			}
		}
	}
	return shadowed
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
		return isDeleteOperation(op)
	}
	for _, arg := range args {
		for _, op := range resource.Commands {
			if arg == op.Name && isDeleteOperation(op.Name) {
				return true
			}
		}
	}
	return false
}

// isDeleteOperation reports whether "delete" is a word of the operation name,
// as in delete, delete-all and bulk-delete. Generation fails for a DELETE, or a
// POST to .../delete, whose command name lacks it, so every one is confirmed.
func isDeleteOperation(name string) bool {
	return slices.Contains(strings.Split(name, "-"), "delete")
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

	defer usage.WrapDefaultTransport()()
	defer client.WrapDefaultTransportWithAuthHint()()

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
