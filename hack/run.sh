#!/usr/bin/env bash
# Run the langsmith CLI with .env loaded.
# Usage: ./hack/run.sh sandbox box list
set -euo pipefail
cd "$(dirname "$0")/.."
set -a && source .env && set +a
exec go run ./cmd/langsmith "$@"
