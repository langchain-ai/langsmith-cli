package main

import (
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"io"
	"strings"
)

// Cobra can reject an unknown command before parsing persistent flags. Inspect
// only explicit output-format arguments so those failures still honor JSON mode.
func errorOutputFormat(args []string, fallback string) string {
	format := fallback
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if arg == "--format" && i+1 < len(args) {
			i++
			format = args[i]
		} else if value, ok := strings.CutPrefix(arg, "--format="); ok {
			format = value
		}
	}
	return format
}

func writeCommandError(w io.Writer, err error, format string) error {
	return output.WriteCommandError(w, err, format)
}

type commandErrorEnvelope struct {
	Error struct {
		Code      string   `json:"code"`
		Message   string   `json:"message"`
		NextSteps []string `json:"next_steps"`
	} `json:"error"`
}
