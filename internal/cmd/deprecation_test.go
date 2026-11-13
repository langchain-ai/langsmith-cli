package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// runWithDeprecations runs args as the process's command line, the way the
// langsmith binary runs, against the real root command with a few deprecated
// test commands mounted. It returns stdout and stderr.
func runWithDeprecations(t *testing.T, args ...string) (string, string) {
	t.Helper()
	return runWithDeprecationsAs(t, args, args)
}

// runWithDeprecationsAs runs args through SetArgs while the process's command
// line is commandLine.
func runWithDeprecationsAs(t *testing.T, commandLine, args []string) (string, string) {
	t.Helper()
	oldFormat, oldArgs := flagOutputFormat, os.Args
	os.Args = append([]string{"langsmith"}, commandLine...)
	t.Cleanup(func() {
		flagOutputFormat, os.Args = oldFormat, oldArgs
		output.SetJQ("")
	})

	root := NewRootCmd("test", "test")
	run := func(cmd *cobra.Command, _ []string) { cmd.Println("ok") }

	old := &cobra.Command{Use: "old-thing", Run: run}
	deprecateCommand(old, Deprecation{RemovedIn: "1.0.0", Replacement: "langsmith things list"})

	things := &cobra.Command{Use: "things", Aliases: []string{"thing"}}
	deprecateAlias(things, "thing", Deprecation{RemovedIn: "2.0.0", Replacement: "langsmith things"})
	things.PersistentFlags().String("legacy-global", "", "")
	deprecateFlag(things, "legacy-global", Deprecation{RemovedIn: "1.0.0"})
	list := &cobra.Command{Use: "list", Aliases: []string{"ls"}, Run: run}
	deprecateAlias(list, "ls", Deprecation{RemovedIn: "1.0.0", Replacement: "list"})
	list.Flags().String("legacy", "", "")
	list.Flags().String("current", "", "")
	deprecateFlag(list, "legacy", Deprecation{RemovedIn: "1.0.0", Replacement: "--current"})
	// Like the generated commands, raw gets its flags unparsed in args.
	raw := &cobra.Command{Use: "raw", DisableFlagParsing: true, Run: run}
	things.AddCommand(list, raw)
	root.AddCommand(old, things)
	cmdutil.AddJQFlags(root)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	require.NoError(t, root.Execute())
	return stdout.String(), stderr.String()
}

func TestDeprecatedCommandWarnsOnStderr(t *testing.T) {
	stdout, stderr := runWithDeprecations(t, "old-thing")

	require.Equal(t, "ok\n", stdout)
	require.Equal(t, "warning: command langsmith old-thing is deprecated and will be removed in 1.0.0; use langsmith things list instead.\n", stderr)
}

func TestDeprecationWarningIsJSONWithFormatJSON(t *testing.T) {
	for _, args := range [][]string{
		{"old-thing", "--format", "json"},
		{"old-thing", "--jq", "."},
	} {
		stdout, stderr := runWithDeprecations(t, args...)

		require.Equal(t, "ok\n", stdout, args)
		var warning map[string]any
		require.NoError(t, json.Unmarshal([]byte(stderr), &warning), args)
		require.Equal(t, map[string]any{
			"deprecated":  true,
			"kind":        "command",
			"name":        "langsmith old-thing",
			"removed_in":  "1.0.0",
			"replacement": "langsmith things list",
			"message":     "command langsmith old-thing is deprecated and will be removed in 1.0.0; use langsmith things list instead.",
		}, warning, args)
	}
}

func TestDeprecatedAliasWarnsOnlyWhenUsed(t *testing.T) {
	_, stderr := runWithDeprecations(t, "thing", "list")
	require.Equal(t, "warning: alias thing is deprecated and will be removed in 2.0.0; use langsmith things instead.\n", stderr)

	_, stderr = runWithDeprecations(t, "--format", "json", "thing", "list", "--current", "thing")
	require.Contains(t, stderr, `"name":"thing"`)

	_, stderr = runWithDeprecations(t, "things", "list", "--current", "thing")
	require.Empty(t, stderr)
}

func TestDeprecatedFlagWarnsOnlyWhenSet(t *testing.T) {
	_, stderr := runWithDeprecations(t, "things", "list", "--legacy", "x")
	require.Equal(t, "warning: flag --legacy is deprecated and will be removed in 1.0.0; use --current instead.\n", stderr)

	_, stderr = runWithDeprecations(t, "things", "list", "--current", "x")
	require.Empty(t, stderr)
}

func TestDeprecatedFlagWarnsWhenFlagParsingIsDisabled(t *testing.T) {
	_, stderr := runWithDeprecations(t, "things", "raw", "--legacy-global=x")
	require.Equal(t, "warning: flag --legacy-global is deprecated and will be removed in 1.0.0.\n", stderr)

	_, stderr = runWithDeprecations(t, "things", "raw", "--format", "json", "--legacy-global", "x")
	require.Contains(t, stderr, `{"deprecated":true,"kind":"flag","name":"--legacy-global"`)

	_, stderr = runWithDeprecations(t, "things", "raw", "--legacy-global", "x", "--", "y")
	require.Equal(t, "warning: flag --legacy-global is deprecated and will be removed in 1.0.0.\n", stderr)

	_, stderr = runWithDeprecations(t, "things", "raw", "--", "--legacy-global")
	require.Empty(t, stderr)

	// A value spelled like a deprecated flag is not that flag.
	_, stderr = runWithDeprecations(t, "things", "raw", "--transform", "--legacy-global")
	require.Empty(t, stderr)

	// The last --format wins, as in the generated command's own parser.
	_, stderr = runWithDeprecations(t, "things", "raw", "--format", "json", "--format", "pretty", "--legacy-global", "x")
	require.Equal(t, "warning: flag --legacy-global is deprecated and will be removed in 1.0.0.\n", stderr)
}

func TestDeprecatedParentAliasWarnsBeforeAnUnparsedCommand(t *testing.T) {
	_, stderr := runWithDeprecations(t, "thing", "--format", "json", "raw")
	require.Contains(t, stderr, `{"deprecated":true,"kind":"alias","name":"thing"`)
}

func TestDeprecatedAliasWithSetArgs(t *testing.T) {
	// The command line names a different command, so only the called-as name
	// of the command cobra ran is used.
	_, stderr := runWithDeprecationsAs(t, []string{"thing", "raw"}, []string{"things", "list"})
	require.Empty(t, stderr)

	_, stderr = runWithDeprecationsAs(t, nil, []string{"things", "ls"})
	require.Equal(t, "warning: alias ls is deprecated and will be removed in 1.0.0; use list instead.\n", stderr)
}

func TestDeprecatedCommandsAndFlagsAreHiddenFromHelp(t *testing.T) {
	stdout, _ := runWithDeprecations(t, "--help")
	require.NotContains(t, stdout, "old-thing")

	stdout, _ = runWithDeprecations(t, "things", "list", "--help")
	require.Contains(t, stdout, "--current")
	require.NotContains(t, stdout, "--legacy")
}

func TestDeprecationRequiresAMajorVersion(t *testing.T) {
	for _, removedIn := range []string{"", "1.2.0", "v1.0.0", "next"} {
		require.Panics(t, func() {
			deprecateCommand(&cobra.Command{Use: "x"}, Deprecation{RemovedIn: removedIn})
		}, removedIn)
	}
}
