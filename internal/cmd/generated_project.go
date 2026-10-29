package cmd

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/urfave/cli/v3"
)

// generatedProjectFlag returns the flag through which op takes a single
// project UUID, --session-id or --project-id, or nil when it takes none. The
// flag is a *string when the field is nullable. A list of IDs, such as the
// experiments annotation-queues populate takes, is not a project.
func generatedProjectFlag(op *cli.Command) cli.Flag {
	for _, f := range op.Flags {
		name := f.Names()[0]
		if name != "session-id" && name != "project-id" {
			continue
		}
		switch f.Get().(type) {
		case string, *string:
			return f
		}
	}
	return nil
}

// addGeneratedProjectFlags documents --project, and --project-id beside a
// --session-id, on every generated operation that takes a project.
// resolveGeneratedProject removes them before the generated tree parses the
// arguments.
func addGeneratedProjectFlags(resources []*cli.Command) {
	for _, r := range resources {
		for _, op := range r.Commands {
			// The generated tree is global, and each root command adds to it.
			f := generatedProjectFlag(op)
			if f == nil || slices.ContainsFunc(op.Flags, func(f cli.Flag) bool { return f.Names()[0] == "project" }) {
				continue
			}
			op.Flags = append(op.Flags, &cli.StringFlag{Name: "project", Usage: projectFlagUsage})
			if f.Names()[0] == "session-id" {
				op.Flags = append(op.Flags, &cli.StringFlag{Name: "project-id", Usage: projectIDFlagUsage})
			}
		}
	}
}

// serverRequiresProject lists operations whose project_id the spec marks
// optional although the server rejects a request without it.
var serverRequiresProject = map[string]bool{
	"threads query":           true,
	"threads:streaming query": true,
	"traces query":            true,
	"traces:streaming query":  true,
}

// resolveGeneratedProject replaces --project NAME, and --project-id for an
// operation whose flag is --session-id, with the operation's own project
// flag. An operation that requires a project and was given none takes
// $LANGSMITH_PROJECT, as hand-written commands do.
func resolveGeneratedProject(ctx context.Context, resource *cli.Command, args []string, opts client.Options) ([]string, error) {
	valueFlags := generatedValueFlags()
	valueFlags["--project"], valueFlags["--project-id"] = true, true
	opName := operationName(resource, args, valueFlags)
	idx := slices.IndexFunc(resource.Commands, func(c *cli.Command) bool { return c.Name == opName })
	if idx < 0 {
		return args, nil
	}
	op := resource.Commands[idx]
	f := generatedProjectFlag(op)
	if f == nil {
		return args, nil
	}
	target := "--" + f.Names()[0]

	rest, project, err := extractValueFlag(args, "--project")
	if err != nil {
		return nil, err
	}
	projectID := ""
	if target == "--session-id" {
		if rest, projectID, err = extractValueFlag(rest, "--project-id"); err != nil {
			return nil, err
		}
	}
	given := hasValueFlag(rest, target)
	switch {
	case project != "" && projectID != "":
		return nil, fmt.Errorf("--project and --project-id are mutually exclusive")
	case (project != "" || projectID != "") && given:
		return nil, fmt.Errorf("--project and --project-id can't be combined with %s", target)
	case projectID != "":
		id, err := validateProjectID(projectID)
		if err != nil {
			return nil, err
		}
		return append(rest, target+"="+id), nil
	case project == "":
		required, ok := f.(interface{ IsRequiredAsFlagOrStdin() bool })
		if !ok || !required.IsRequiredAsFlagOrStdin() {
			ok = serverRequiresProject[resource.Name+" "+op.Name]
		}
		if given || !ok || positionalFillsFlag(op, f, rest, valueFlags) || stdinMayHoldRequest() {
			return rest, nil
		}
		if project = os.Getenv("LANGSMITH_PROJECT"); project == "" {
			return rest, nil
		}
	}
	id, err := client.NewWithOptions(opts).ResolveSessionID(ctx, project)
	if err != nil {
		return nil, err
	}
	return append(rest, target+"="+id), nil
}

// extractValueFlag removes every --name VALUE and --name=VALUE from args and
// returns the last value.
func extractValueFlag(args []string, name string) ([]string, string, error) {
	rest := make([]string, 0, len(args))
	value := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			rest = append(rest, args[i:]...)
			break
		}
		if v, ok := strings.CutPrefix(arg, name+"="); ok {
			value = v
			continue
		}
		if arg == name {
			if i+1 >= len(args) {
				return nil, "", fmt.Errorf("flag needs an argument: %s", name)
			}
			value = args[i+1]
			i++
			continue
		}
		rest = append(rest, arg)
	}
	return rest, strings.TrimSpace(value), nil
}

// positionalFillsFlag reports whether a positional argument will fill f. The
// generated handlers give positionals to the path parameters not set by flag,
// in order, so a positional run ID does not select a project.
func positionalFillsFlag(op *cli.Command, f cli.Flag, args []string, valueFlags map[string]bool) bool {
	slot := 0
	for _, pf := range op.Flags {
		p, ok := pf.(interface{ GetPathParam() string })
		if !ok || p.GetPathParam() == "" {
			continue
		}
		if pf == f {
			// countPositional includes the operation name.
			return countPositional(args, opValueFlags(op, valueFlags)) > slot+1
		}
		if !hasValueFlag(args, "--"+pf.Names()[0]) {
			slot++
		}
	}
	return false
}

// stdinMayHoldRequest reports whether stdin may carry the request, which can
// set the project itself. The generated parser lets a flag win over piped data,
// so the $LANGSMITH_PROJECT default must not be added then. It fails closed.
func stdinMayHoldRequest() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return true
	}
	mode := stat.Mode()
	return (mode.IsRegular() && stat.Size() > 0) || mode&(os.ModeNamedPipe|os.ModeSocket) != 0
}

func hasValueFlag(args []string, name string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		return arg == name || strings.HasPrefix(arg, name+"=")
	})
}

// opValueFlags adds op's flags that take a value to valueFlags.
func opValueFlags(op *cli.Command, valueFlags map[string]bool) map[string]bool {
	names := maps.Clone(valueFlags)
	for _, f := range op.Flags {
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

// countPositional counts the arguments that are neither flags nor flag values,
// including the operation name.
func countPositional(args []string, valueFlags map[string]bool) int {
	n := 0
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return n + len(args) - i - 1
		}
		if strings.HasPrefix(arg, "-") {
			if valueFlags[arg] {
				i++
			}
			continue
		}
		n++
	}
	return n
}
