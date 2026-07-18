// Canonical source, synced into every template that has a file at this same
// path by scripts/sync-template-shared.sh (run via `make sync-templates`,
// and automatically before `make build`/`make install`). Edit this copy —
// per-template copies get overwritten on next sync.
import { cn } from '../lib/utils';

interface SpinnerProps {
  size?: 'sm' | 'md';
  className?: string;
}

const SIZE_CLASSES: Record<NonNullable<SpinnerProps['size']>, string> = {
  sm: 'h-3 w-3 border',
  md: 'h-4 w-4 border-2',
};

export function Spinner({ size = 'md', className }: SpinnerProps) {
  return (
    <span
      className={cn(
        'inline-block shrink-0 animate-spin rounded-full border-tertiary border-t-brand',
        SIZE_CLASSES[size],
        className
      )}
    />
  );
}
