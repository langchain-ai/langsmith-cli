#!/usr/bin/env bash
#
# Syncs the canonical shared template files (internal/cmd/templates/_shared/)
# into every scaffold template that already has a file at the same relative
# path under its own src/. This is how templates share code without the
# *generated* apps having any cross-template (or cross-repo) dependency: each
# scaffolded app stays a fully standalone project, but the copies living in
# this repo are kept identical by this script instead of by hand.
#
# Existence-based, not a maintained list: a template "opts in" to a shared
# file simply by already having one at that path. To add a new template to
# sharing (or add a new shared file), copy it from _shared/ into that
# template once — this script takes care of every sync after that. To make a
# template's copy diverge intentionally, rename it there; anything still
# named the same as a _shared/ file is treated as wanting to stay identical
# and will be overwritten.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

SHARED_DIR="internal/cmd/templates/_shared"
TEMPLATES_DIR="internal/cmd/templates"

if [ ! -d "$SHARED_DIR" ]; then
  echo "no $SHARED_DIR directory — nothing to sync"
  exit 0
fi

synced=0
while IFS= read -r -d '' shared_file; do
  rel_path="${shared_file#"$SHARED_DIR"/}"
  for template_dir in "$TEMPLATES_DIR"/*/; do
    template_name="$(basename "$template_dir")"
    [ "$template_name" = "_shared" ] && continue
    [ "$template_name" = "agents-md" ] && continue

    target="${template_dir}src/${rel_path}"
    if [ -f "$target" ] && ! cmp -s "$shared_file" "$target"; then
      cp "$shared_file" "$target"
      echo "synced ${template_name}/src/${rel_path}"
      synced=$((synced + 1))
    fi
  done
done < <(find "$SHARED_DIR" -type f -print0)

if [ "$synced" -eq 0 ]; then
  echo "all template copies already up to date"
fi
