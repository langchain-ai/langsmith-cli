#!/usr/bin/env bash
# Resolve current template dependency ranges into the committed npm lockfiles.
set -euo pipefail
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
make -C "$repo_root" build
scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
for variant in blank cn icons-cn; do
  mkdir "$scratch/$variant"
  (
    cd "$scratch/$variant"
    case "$variant" in
      blank) args=() ;;
      cn) args=(--template coding-agent-dashboard) ;;
      icons-cn) args=(--template annotation-queue) ;;
    esac
    "$repo_root/bin/langsmith" apps init --name custom-app-template --no-install "${args[@]}" >/dev/null
    cd custom-app-template
    rm package-lock.json
    npm install --package-lock-only --ignore-scripts --no-audit --no-fund
    cp package-lock.json "$repo_root/internal/cmd/templates/locks/$variant.json"
  )
done
