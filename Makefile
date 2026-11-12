BINARY_NAME=langsmith
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: build clean test test-integration test-wheels lint vet fmt install agent-evals test-agent-evals

build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/langsmith

install:
	CGO_ENABLED=0 go install $(LDFLAGS) ./cmd/langsmith

clean:
	rm -rf bin/

test:
	go test -v ./...

test-integration:
	go test -tags=integration -v -run Integration ./internal/cmd/

# Builds the binary for this machine so the wheel test can install it and run
# `langsmith --version`; the rest of the test uses fake archives.
test-wheels: build
	LANGSMITH_WHEEL_TEST_BINARY=$(CURDIR)/bin/$(BINARY_NAME) uv run scripts/test_build_wheels.py -v

# Agent evals (evals/README.md): an agent runs the fixed task set through this
# build of the CLI. Needs ANTHROPIC_API_KEY and LANGSMITH_API_KEY; pass options
# with ARGS, e.g. make agent-evals ARGS="--model claude-haiku-5-5 --no-upload".
agent-evals: build
	cd evals && LANGSMITH_CLI=$(CURDIR)/bin/$(BINARY_NAME) uv run python -m agent_evals run $(ARGS)

# Offline tests of the eval harness (no API keys needed).
test-agent-evals: build
	cd evals && LANGSMITH_CLI=$(CURDIR)/bin/$(BINARY_NAME) uv run pytest -q

lint:
	golangci-lint run

vet:
	go vet ./...

fmt:
	gofmt -w .

all: fmt vet test build
