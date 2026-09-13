// Canonical single source. apps_init.go copies this into a scaffolded app's
// src/ at generation time, and only if the template imports it — there are no
// standing per-template copies and no sync script.
export { cn } from 'cnfast';
