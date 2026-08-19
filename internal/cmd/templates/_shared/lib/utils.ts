// Canonical single source. apps_init.go copies this into a scaffolded app's
// src/ at generation time, and only if the template imports it — there are no
// standing per-template copies and no sync script.
//
// Re-exported from the design system rather than reimplemented, so template
// code and design-system components merge Tailwind classes the same way: this
// cn() is tailwind-merge aware, so a className passed into a component
// genuinely overrides the component's own conflicting class.
export { cn } from '@/components/langsmith/design-system/utils/cn';
