// Command langsmith is the LangSmith CLI. Its code lives in
// github.com/langchain-ai/langsmith-go/cli; this module builds and releases
// it, and keeps `go install github.com/langchain-ai/langsmith-cli/cmd/langsmith`
// working.
package main

import (
	"os"

	"github.com/langchain-ai/langsmith-go/cli"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	os.Exit(cli.Main(version, commit, date))
}
