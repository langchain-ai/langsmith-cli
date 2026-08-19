// Canonical single source. apps_init.go copies this into a scaffolded app's
// src/ at generation time, and only if the template imports it — there are no
// standing per-template copies and no sync script.
//
// The design system's <Tooltip> is built on Radix, which throws
// "`Tooltip` must be used within `TooltipProvider`" unless a provider is
// mounted above it. The registry ships the tooltip but not the provider (the
// LangSmith app mounts one in its own root), so every app that renders a
// <Tooltip>, an <Icon label=…>, or an <IconButton> — which wraps its label in
// a tooltip — has to mount one itself. Wrap the whole app once, in entry.tsx.
import type { ReactNode } from 'react';
import * as TooltipPrimitive from '@radix-ui/react-tooltip';

export function DesignSystemProvider({ children }: { children: ReactNode }) {
  // 300ms matches the design system Tooltip's own default delay.
  return (
    <TooltipPrimitive.Provider delayDuration={300}>{children}</TooltipPrimitive.Provider>
  );
}
