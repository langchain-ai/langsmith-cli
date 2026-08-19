import type { ReactNode } from 'react';
import { forwardRef } from 'react';

import { sanitizeUrl } from '@braintree/sanitize-url';

import { cn } from '../../utils/cn';
import { getColorByString } from '../../utils/get-color-by-string';
import { Tooltip } from '../Tooltip';

type AvatarSize = 'xs' | 'sm' | 'md' | 'lg' | 'xl';
type AvatarShape = 'circle' | 'square';

interface AvatarProps {
  /** Box size, matching Icon's padded box scale: xs=16px, sm=20px, md=24px, lg=36px, xl=48px. */
  size?: AvatarSize;
  shape?: AvatarShape;
  /** Show a small notification dot in the top-right corner. */
  badge?: boolean;
  /** Wrap the avatar in a tooltip with this title. */
  tooltip?: string;
  /** Extra classes on the avatar box. Can override size/text/color via `twMerge`. */
  className?: string;
  /** Text used for the initial and, unless `color` is set, the color hash. */
  label: string;
  /** Renders an image instead of the initial when present. */
  imageUrl?: string;
  /** Overrides the color-from-string hash with an explicit gradient start color. */
  color?: string;
  /** Rendered instead of the initial when `label` is empty. */
  fallbackIcon?: ReactNode;
  /** Highlighted brand-fill state (e.g. the current user's own entry). */
  active?: boolean;
}

// Box sizes mirror Icon's padded box scale (glyph + padding) so the two
// primitives line up at each size name: 16 / 20 / 24 / 36 / 48px.
const BOX_SIZE: Record<AvatarSize, string> = {
  xs: 'size-4',
  sm: 'size-5',
  md: 'size-6',
  lg: 'size-9',
  xl: 'size-12',
};

const TEXT_SIZE: Record<AvatarSize, string> = {
  xs: 'text-xxs',
  sm: 'text-xs',
  md: 'text-sm',
  lg: 'text-lg',
  xl: 'text-xl',
};

const Avatar = forwardRef<HTMLDivElement, AvatarProps>(
  (
    {
      size = 'md',
      shape = 'square',
      badge,
      tooltip,
      className,
      label,
      imageUrl,
      color,
      fallbackIcon,
      active,
    },
    ref
  ) => {
    const startColor = color ?? getColorByString(label);
    const rounding = shape === 'circle' ? 'rounded-full' : 'rounded-sm';

    const inner = (
      // The box is intentionally not `overflow-hidden` so the badge dot can
      // overflow the top-right corner. The image child clips itself instead.
      <div
        ref={ref}
        className={cn(
          'relative inline-flex shrink-0 cursor-default items-center justify-center text-center font-medium uppercase text-white/90 transition-all',
          rounding,
          BOX_SIZE[size],
          TEXT_SIZE[size],
          active && 'bg-brand',
          className
        )}
        style={
          active
            ? undefined
            : {
                backgroundImage: `linear-gradient(to bottom right, ${startColor}, #aaaaaa)`,
              }
        }
      >
        {imageUrl ? (
          <div
            className={cn(
              'absolute inset-0 size-full bg-cover bg-center',
              rounding
            )}
            style={{ backgroundImage: `url(${sanitizeUrl(imageUrl)})` }}
          />
        ) : label ? (
          <span>{label.charAt(0)}</span>
        ) : (
          (fallbackIcon ?? null)
        )}

        {badge && (
          <div className="absolute -right-1 -top-1 size-3 rounded-full border-2 border-current bg-error-strong outline-transparent" />
        )}
      </div>
    );

    if (tooltip) {
      return <Tooltip title={tooltip}>{inner}</Tooltip>;
    }

    return inner;
  }
);

Avatar.displayName = 'Avatar';

export { Avatar };
export type { AvatarProps, AvatarSize };
