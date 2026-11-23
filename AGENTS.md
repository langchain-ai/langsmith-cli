# AGENTS.md

This is the `github.com/langchain-ai/langsmith-cli` module, which releases the
`langsmith` CLI. The CLI's code is in the Go SDK module, `sdks/go/cli` in
langchainplus; read `sdks/go/cli/AGENTS.md` before changing commands.

- `cmd/langsmith/`: the released binary's `main` package. It calls
  `github.com/langchain-ai/langsmith-go/cli.Main`, and keeps
  `go install github.com/langchain-ai/langsmith-cli/cmd/langsmith@latest`
  working.
- `go.mod` requires `langsmith-go` main as a pseudo-version, and releases
  build against it. langsmith-cli owns its `go.mod` and `go.sum`: Copybara
  does not export them, and the export workflow's `pin-cli` job
  (`scripts/pin_cli_langsmith_go.sh`) moves the pin when `langsmith-go` gets
  CLI changes. The copy here is for building and linting `cmd/langsmith` in
  langchainplus. Never add a `replace`; CI fails it. In langchainplus the
  binary is built from `sdks/go/cmd/langsmith` instead, and the Makefile does
  that when it finds `../sdks/go`.
- `scripts/`: install scripts, and `build_wheels.py`, which repackages release
  archives as PyPI wheels (`make test-wheels` tests it).
- `evals/`: agent evals run through the built CLI (`evals/README.md`).
- `README.md`: the user-facing install, command and release documentation.
