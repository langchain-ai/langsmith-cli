// Canonical source, synced into every template that has a file at this same
// path by scripts/sync-template-shared.sh (run via `make sync-templates`,
// and automatically before `make build`/`make install`). Edit this copy —
// per-template copies get overwritten on next sync.
export function cn(...classes: (string | false | null | undefined)[]): string {
  return classes.filter(Boolean).join(' ');
}
