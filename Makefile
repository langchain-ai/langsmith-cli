BINARY_NAME=langsmith
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: build clean test lint vet fmt install

build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/langsmith

install:
	CGO_ENABLED=0 go install $(LDFLAGS) ./cmd/langsmith

clean:
	rm -rf bin/

test:
	go test ./...

lint:
	golangci-lint run

vet:
	go vet ./...

fmt:
	gofmt -w .

all: fmt vet test build
