package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Deprecation describes a command, alias or flag that a later major version
// removes. See "Breaking changes" in AGENTS.md for when one is required.
type Deprecation struct {
	// RemovedIn is the major version that removes it, such as "1.0.0".
	RemovedIn string
	// Replacement is what to use instead, such as "langsmith runs list" or
	// "--project-id". Empty when nothing replaces it.
	Replacement string
}

// deprecationWarning is the JSON object written to stderr with --format json,
// so agents can detect deprecated usage without parsing prose.
type deprecationWarning struct {
	Deprecated  bool   `json:"deprecated"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	RemovedIn   string `json:"removed_in"`
	Replacement string `json:"replacement,omitempty"`
	Message     string `json:"message"`
}

const (
	deprecationAnnotation     = "langsmith.deprecated"
	deprecatedAliasAnnotation = "langsmith.deprecated-alias."
)

var majorVersion = regexp.MustCompile(`^[1-9][0-9]*\.0\.0$`)

// deprecateCommand hides cmd from help and warns whenever it runs.
func deprecateCommand(cmd *cobra.Command, d Deprecation) {
	setAnnotation(cmd, deprecationAnnotation, d)
	cmd.Hidden = true
}

// deprecateAlias warns when cmd is invoked by alias, which must already be one
// of its aliases. The alias stays listed in help until it is removed.
func deprecateAlias(cmd *cobra.Command, alias string, d Deprecation) {
	if !slices.Contains(cmd.Aliases, alias) {
		panic(fmt.Sprintf("deprecateAlias: %q is not an alias of %q", alias, cmd.Name()))
	}
	setAnnotation(cmd, deprecatedAliasAnnotation+alias, d)
}

// deprecateFlag hides the flag named name on cmd from help and warns whenever
// it is set. Deprecating a persistent flag covers every subcommand.
func deprecateFlag(cmd *cobra.Command, name string, d Deprecation) {
	flag := cmd.Flags().Lookup(name)
	if flag == nil {
		flag = cmd.PersistentFlags().Lookup(name)
	}
	if flag == nil {
		panic(fmt.Sprintf("deprecateFlag: %q has no flag --%s", cmd.Name(), name))
	}
	if flag.Annotations == nil {
		flag.Annotations = map[string][]string{}
	}
	flag.Annotations[deprecationAnnotation] = []string{encodeDeprecation(d)}
	flag.Hidden = true
}

func setAnnotation(cmd *cobra.Command, key string, d Deprecation) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[key] = encodeDeprecation(d)
}

func encodeDeprecation(d Deprecation) string {
	if !majorVersion.MatchString(d.RemovedIn) {
		panic(fmt.Sprintf("deprecation RemovedIn %q must be a major version such as 1.0.0", d.RemovedIn))
	}
	b, _ := json.Marshal(d)
	return string(b)
}

func decodeDeprecation(s string) Deprecation {
	var d Deprecation
	_ = json.Unmarshal([]byte(s), &d)
	return d
}

// warnDeprecated writes one warning to w for each deprecated command, alias
// and flag that running cmd uses: a JSON object per line when asJSON, otherwise
// a "warning:" line. Stdout is never written, so output stays parseable. args
// are the arguments cobra passed to cmd, and commandLine is the process's
// arguments after the program name.
func warnDeprecated(w io.Writer, cmd *cobra.Command, args, commandLine []string, asJSON bool) {
	// A command with flag parsing disabled has not merged its parents'
	// persistent flags into cmd.Flags(); both lookups below need them.
	_ = cmd.InheritedFlags()
	var warnings []deprecationWarning
	for c := cmd; c != nil; c = c.Parent() {
		if s, ok := c.Annotations[deprecationAnnotation]; ok {
			warnings = append(warnings, newDeprecationWarning("command", c.CommandPath(), decodeDeprecation(s)))
		}
	}
	for _, alias := range invokedAliases(cmd, commandLine) {
		if s, ok := alias.cmd.Annotations[deprecatedAliasAnnotation+alias.name]; ok {
			warnings = append(warnings, newDeprecationWarning("alias", alias.name, decodeDeprecation(s)))
		}
	}
	// A command with flag parsing disabled (the generated commands) receives
	// its flags, including global ones, as args, so Changed is never set.
	var unparsed map[string]bool
	if cmd.DisableFlagParsing {
		unparsed = unparsedFlags(cmd, args)
	}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		s, ok := f.Annotations[deprecationAnnotation]
		if !ok || len(s) != 1 {
			return
		}
		if f.Changed || unparsed["--"+f.Name] || (f.Shorthand != "" && unparsed["-"+f.Shorthand]) {
			warnings = append(warnings, newDeprecationWarning("flag", "--"+f.Name, decodeDeprecation(s[0])))
		}
	})
	if cmd.DisableFlagParsing && argsRequestJSON(args) {
		asJSON = true
	}
	for _, warning := range warnings {
		if asJSON {
			b, _ := json.Marshal(warning)
			fmt.Fprintln(w, string(b))
		} else {
			fmt.Fprintln(w, "warning: "+warning.Message)
		}
	}
}

// unparsedFlags returns the flag names set in unparsed args, as --name,
// --name=value, -n or -n=value, up to a -- terminator. The value of a flag
// that takes one, from cmd's flags or the generated commands' own, is skipped,
// so a value spelled like a flag is not counted.
func unparsedFlags(cmd *cobra.Command, args []string) map[string]bool {
	takesValue := generatedValueFlags()
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.NoOptDefVal == "" {
			takesValue["--"+f.Name] = true
			if f.Shorthand != "" {
				takesValue["-"+f.Shorthand] = true
			}
		}
	})
	set := map[string]bool{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		name, _, hasValue := strings.Cut(arg, "=")
		set[name] = true
		if !hasValue && takesValue[name] {
			i++
		}
	}
	return set
}

// argsRequestJSON reports whether unparsed args select JSON output: --jq, or
// a last --format of json.
func argsRequestJSON(args []string) bool {
	format := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		name, value, hasValue := strings.Cut(arg, "=")
		if name != "--jq" && name != "--format" {
			continue
		}
		if !hasValue && i+1 < len(args) {
			value = args[i+1]
			i++
		}
		if name == "--jq" {
			return true
		}
		format = value
	}
	return format == "json"
}

type invokedAlias struct {
	cmd  *cobra.Command
	name string
}

// invokedAliases returns each command on the path to cmd that was named by one
// of its aliases. Cobra records the called-as name only for cmd itself and
// does not expose the arguments it ran, so parents are found by walking the
// command words of commandLine, which is what cobra runs unless SetArgs was
// called. When commandLine does not lead to cmd, only cmd's own called-as
// name is used.
func invokedAliases(cmd *cobra.Command, commandLine []string) []invokedAlias {
	if aliases, ok := aliasesOnPath(cmd, commandLine); ok {
		return aliases
	}
	if name := cmd.CalledAs(); name != "" && name != cmd.Name() {
		return []invokedAlias{{cmd: cmd, name: name}}
	}
	return nil
}

// aliasesOnPath walks the command words in args from the root and reports
// whether they lead to cmd, with the commands named by an alias on the way.
func aliasesOnPath(cmd *cobra.Command, args []string) ([]invokedAlias, bool) {
	var aliases []invokedAlias
	c := cmd.Root()
	for i := 0; i < len(args) && c != cmd; i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "-") {
			// Skip the value of a flag given as "--name value" or "-n value".
			name := strings.TrimLeft(arg, "-")
			var flag *pflag.Flag
			if len(name) == 1 {
				flag = cmd.Flags().ShorthandLookup(name)
			} else {
				flag = cmd.Flags().Lookup(name)
			}
			if !strings.Contains(arg, "=") && flag != nil && flag.NoOptDefVal == "" {
				i++
			}
			continue
		}
		var next *cobra.Command
		for _, child := range c.Commands() {
			if child.Name() == arg || slices.Contains(child.Aliases, arg) {
				next = child
				break
			}
		}
		if next == nil {
			break
		}
		if next.Name() != arg {
			aliases = append(aliases, invokedAlias{cmd: next, name: arg})
		}
		c = next
	}
	return aliases, c == cmd
}

func newDeprecationWarning(kind, name string, d Deprecation) deprecationWarning {
	var msg strings.Builder
	fmt.Fprintf(&msg, "%s %s is deprecated and will be removed in %s", kind, name, d.RemovedIn)
	if d.Replacement != "" {
		fmt.Fprintf(&msg, "; use %s instead", d.Replacement)
	}
	msg.WriteString(".")
	return deprecationWarning{
		Deprecated:  true,
		Kind:        kind,
		Name:        name,
		RemovedIn:   d.RemovedIn,
		Replacement: d.Replacement,
		Message:     msg.String(),
	}
}
