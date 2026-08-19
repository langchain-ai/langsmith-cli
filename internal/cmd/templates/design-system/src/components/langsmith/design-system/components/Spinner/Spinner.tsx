import { Loading01Icon } from '@langchain/untitled-ui-icons';

import { cn } from '../../utils/cn';

const Spinner = ({
  size = 'sm',
  className,
}: {
  size?: 'xs' | 'sm' | 'md' | 'lg';
  className?: string;
}) => {
  const sizeClass = {
    xs: 'size-3',
    sm: 'size-4',
    md: 'size-6',
    lg: 'size-8',
  }[size];
  return <Loading01Icon className={cn('animate-spin', sizeClass, className)} />;
};

export { Spinner };
