import { cn } from '../../utils/cn';

interface SkeletonProps {
  className?: string;
  as?: 'div' | 'span';
  style?: React.CSSProperties;
}

export const Skeleton: React.FC<SkeletonProps> = ({
  className,
  style,
  as = 'div',
}) => {
  const Component = as === 'div' ? 'div' : 'span';
  return (
    <Component
      className={cn(
        'h-4 w-full animate-pulse rounded bg-surface-level-3',
        className
      )}
      style={style}
    />
  );
};

export const SkeletonRows = ({
  rows,
  className,
  skeletonClassName,
}: {
  rows: number;
  className?: string;
  skeletonClassName?: string;
}) => {
  return (
    <div className={cn('flex flex-col gap-space-2', className)}>
      {Array.from({ length: rows }).map((_, index) => (
        <Skeleton key={index} className={skeletonClassName} />
      ))}
    </div>
  );
};

export const CircleSkeleton = ({ className, style }: SkeletonProps) => {
  return (
    <div
      className={cn(
        'h-5 w-5 animate-pulse rounded-full bg-surface-level-3',
        className
      )}
      style={style}
    />
  );
};
