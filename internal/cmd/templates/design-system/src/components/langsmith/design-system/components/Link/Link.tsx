import { forwardRef } from 'react';

import {
  Link as RouterLink,
  type LinkProps as RouterLinkProps,
} from 'react-router-dom';

import { cn } from '../../utils/cn';
import type { IconComponent } from '../Icon';
import { textVariantClasses } from '../Text';
import type { TextProps } from '../Text';

type LinkProps = (
  | (RouterLinkProps & { href?: never })
  | (React.AnchorHTMLAttributes<HTMLAnchorElement> & {
      href: string;
      to?: never;
    })
) & {
  /** Typography size — mirrors <Text> styles */
  variant?: TextProps['variant'];
  leftDecorator?: IconComponent;
  rightDecorator?: IconComponent;
};

const TextContent = (
  children: React.ReactNode,
  leftDecorator: IconComponent | undefined,
  rightDecorator: IconComponent | undefined
) => {
  const LeftIcon = leftDecorator;
  const RightIcon = rightDecorator;
  return (
    <>
      {LeftIcon && (
        <LeftIcon className="h-[1em] w-[1em] flex-shrink-0 text-link dark:text-brand-secondary dark:group-hover:text-brand-primary" />
      )}
      <span className="text-link underline-offset-2 hover:text-link-hover hover:underline hover:decoration-ls-neutral-300 hover:decoration-1 dark:text-brand-secondary dark:hover:text-brand-primary dark:hover:decoration-ls-neutral-400">
        {children}
      </span>
      {RightIcon && (
        <RightIcon className="h-[1em] w-[1em] flex-shrink-0 text-link dark:text-brand-secondary dark:group-hover:text-brand-primary" />
      )}
    </>
  );
};

const Link = forwardRef<HTMLAnchorElement, LinkProps>(
  (
    {
      variant = 'body',
      leftDecorator,
      rightDecorator,
      className,
      children,
      ...props
    },
    ref
  ) => {
    const containerClass = cn(
      'group inline-flex items-center gap-[2px]',
      textVariantClasses[variant],
      className
    );

    if ('href' in props && props.href !== undefined) {
      const { href, ...anchorProps } = props;
      return (
        // eslint-disable-next-line react/forbid-elements
        <a ref={ref} href={href} className={containerClass} {...anchorProps}>
          {TextContent(children, leftDecorator, rightDecorator)}
        </a>
      );
    }

    const { to, ...routerProps } = props as RouterLinkProps;
    return (
      <RouterLink ref={ref} to={to} className={containerClass} {...routerProps}>
        {TextContent(children, leftDecorator, rightDecorator)}
      </RouterLink>
    );
  }
);

Link.displayName = 'Link';

export { Link };
export type { LinkProps };
