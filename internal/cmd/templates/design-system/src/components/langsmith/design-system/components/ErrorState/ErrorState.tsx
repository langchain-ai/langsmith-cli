import type { ReactNode } from 'react';

import { Link } from 'react-router-dom';

import { cn } from '../../utils/cn';
import { Button } from '../Button';
import { Text } from '../Text';
import NoMatchBackground from './NoMatchBackground.svg?react';

const WaveOverlay = () => (
  <svg
    className="pointer-events-none absolute left-0 top-0 h-[72vh] w-full -scale-y-100"
    viewBox="0 24 150 28"
    preserveAspectRatio="none"
    aria-hidden="true"
  >
    <defs>
      <style>{`
        @keyframes wave-move {
          0%   { transform: translateX(-90px); }
          100% { transform: translateX(85px); }
        }
      `}</style>
      <path
        id="gentle-wave"
        d="M-160 44c30 0 58-18 88-18s 58 18 88 18 58-18 88-18 58 18 88 18 v44h-352z"
      />
      <linearGradient
        id="wave-gradient"
        gradientUnits="userSpaceOnUse"
        x1="0"
        y1="14"
        x2="0"
        y2="50"
      >
        <stop
          offset="0%"
          stopColor="var(--bg-surface-level-1)"
          stopOpacity="0"
        />
        <stop
          offset="100%"
          stopColor="var(--bg-surface-level-1)"
          stopOpacity="1"
        />
      </linearGradient>
    </defs>
    <g>
      <use
        href="#gentle-wave"
        x="50"
        y="0"
        fill="url(#wave-gradient)"
        fillOpacity=".5"
        className="blur-md"
        style={{ animation: 'wave-move 16s linear -10s infinite' }}
      />
      <use
        href="#gentle-wave"
        x="50"
        y="2"
        fill="url(#wave-gradient)"
        fillOpacity=".60"
        className="blur-sm"
        style={{ animation: 'wave-move 10s linear -8s infinite' }}
      />
      <use
        href="#gentle-wave"
        x="50"
        y="8"
        fill="url(#wave-gradient)"
        fillOpacity=".72"
        className="blur-[2px]"
        style={{ animation: 'wave-move 4s linear -4s infinite' }}
      />
    </g>
  </svg>
);

interface ErrorStateProps {
  /** Primary heading. */
  title: string;
  /** Supporting copy below the title. */
  message?: string;
  /** When `404`, renders the illustrated not-found background. */
  status?: number | null;
  /** Route to navigate to via the default back button (ignored when `action` is set). */
  backTo?: string;
  /** Label for the default back button. Defaults to `'Return to home'`. */
  backLabel?: string;
  /** Custom action node rendered in place of the default back button. */
  action?: ReactNode;
  /** Extra classes for the outer container. */
  className?: string;
  /** Extra classes for the inner content wrapper (e.g. to override top padding). */
  contentClassName?: string;
  /** Suppress the animated wave under the 404 illustration. */
  withoutOverlay?: boolean;
}

export function ErrorState({
  title,
  message,
  status,
  backTo,
  backLabel,
  action,
  className,
  contentClassName,
  withoutOverlay,
}: ErrorStateProps) {
  return (
    <div className={cn('relative size-full overflow-hidden', className)}>
      {status === 404 && (
        <div className="absolute left-0 top-64 w-full transform">
          <NoMatchBackground className="size-full text-icon-brand [&_*]:fill-current" />
          {!withoutOverlay && <WaveOverlay />}
        </div>
      )}

      <div
        className={cn('relative w-full justify-center pt-64', contentClassName)}
      >
        <div className="flex flex-col items-center justify-center">
          <Text
            variant="h1"
            className="text-center text-3xl text-brand-primary"
          >
            {title}
          </Text>
          {message && (
            <Text
              className="pb-space-5 pt-space-3 text-center font-mono leading-5 text-quaternary"
              variant="body"
            >
              {message}
            </Text>
          )}

          {action}
          {!action && backTo && (
            <Link to={backTo}>
              <Button variant="normal" size="sm" color="secondary">
                {backLabel ?? 'Return to home'}
              </Button>
            </Link>
          )}
        </div>
      </div>
    </div>
  );
}

export type { ErrorStateProps };
