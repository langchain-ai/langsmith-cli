package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/urfave/cli/v3"
)

// addGeneratedJQFlags documents --jq on every generated operation.
// runGenerated removes it before the generated tree parses the arguments.
func addGeneratedJQFlags(resources []*cli.Command) {
	for _, r := range resources {
		for _, op := range r.Commands {
			// The generated tree is global, and each root command adds to it.
			if slices.ContainsFunc(op.Flags, func(f cli.Flag) bool { return f.Names()[0] == "jq" }) {
				continue
			}
			op.Flags = append(op.Flags, &cli.StringFlag{Name: "jq", Usage: cmdutil.JQFlagUsage})
		}
	}
}

// extractGeneratedJQ removes --jq from args and checks that its expression
// compiles, so a bad expression fails before any request is sent.
func extractGeneratedJQ(args []string) ([]string, string, error) {
	rest, expr, err := extractValueFlag(args, "--jq")
	if err != nil || expr == "" {
		return rest, "", err
	}
	if _, err := output.CompileJQ(expr); err != nil {
		return nil, "", err
	}
	return rest, expr, nil
}

// withJQOutputFlags makes the generated tree print one compact JSON value per
// line, the form runGeneratedJQ reads back. The flags go before any "--" so
// they stay flags, and after the user's own so they win.
func withJQOutputFlags(args []string) []string {
	end := slices.Index(args, "--")
	if end < 0 {
		end = len(args)
	}
	return slices.Concat(args[:end], []string{"--format=jsonl", "--raw-output=false"}, args[end:])
}

// isGeneratedListOperation reports whether args run an operation that pages
// through results, which the generated tree prints as one item per line.
func isGeneratedListOperation(resource *cli.Command, args []string) bool {
	name := operationName(resource, args, generatedValueFlags())
	for _, op := range resource.Commands {
		if op.Name == name {
			return slices.ContainsFunc(op.Flags, func(f cli.Flag) bool { return f.Names()[0] == "max-items" })
		}
	}
	return false
}

// captureGeneratedStdout runs fn with os.Stdout redirected and returns what fn wrote.
// The generated tree writes to os.Stdout directly.
func captureGeneratedStdout(fn func() error) ([]byte, error) {
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	os.Stdout = w
	var buf bytes.Buffer
	copied := make(chan error, 1)
	go func() {
		_, err := io.Copy(&buf, r)
		copied <- err
	}()
	runErr := fn()
	os.Stdout = orig
	_ = w.Close()
	copyErr := <-copied
	_ = r.Close()
	return buf.Bytes(), errors.Join(runErr, copyErr)
}

// jqModel decodes the generated tree's JSONL output into the value --jq
// filters: an array for a list operation, as hand-written list commands
// print, and otherwise the single response.
func jqModel(out []byte, list bool) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(out))
	dec.UseNumber()
	values := []any{}
	for {
		var v any
		err := dec.Decode(&v)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("--jq needs JSON output, but the response is not JSON: %w", err)
		}
		values = append(values, v)
	}
	switch {
	case list || len(values) > 1:
		return values, nil
	case len(values) == 1:
		return values[0], nil
	default:
		return nil, nil
	}
}
