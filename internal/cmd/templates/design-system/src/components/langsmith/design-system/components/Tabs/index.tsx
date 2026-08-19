import type { ComponentType } from 'react';

// NOTE: Tabs are intentionally NOT re-exported from the design-system barrel
// (components/index.ts). Importing @headlessui/react through the barrel pulls
// ~297KB into every chunk that touches the barrel, bloating the initial load.
// Import directly from '@/design-system/components/Tabs' instead.
import {
  TabGroup as HeadlessTabGroup,
  TabList as HeadlessTabList,
  TabPanel as HeadlessTabPanel,
  TabPanels as HeadlessTabPanels,
  Tab,
} from '@headlessui/react';

import { cn } from '../../utils/cn';
import type { BadgeProps } from '../Badge';
import { Badge } from '../Badge';
import { Text } from '../Text';

export function TabList({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <HeadlessTabList
      className={cn(
        'flex gap-space-5 border-b border-secondary text-ls-black',
        className
      )}
    >
      {children}
    </HeadlessTabList>
  );
}

export function TabLabel({
  label,
  icon: Icon,
  badgeProps,
  className,
  disabled,
  dataTestId,
  onClick,
  onBlur,
}: {
  label: React.ReactNode;
  icon?: ComponentType<{ className?: string }>;
  badgeProps?: BadgeProps;
  className?: string;
  disabled?: boolean;
  dataTestId?: string;
  onClick?: () => void;
  onBlur?: () => void;
}) {
  return (
    <Tab
      onClick={onClick}
      onBlur={onBlur}
      className={cn(
        'group flex items-center gap-space-2 border-b-2 border-b-transparent pb-5 text-tertiary outline-none transition-colors',
        'text-primary data-[selected]:border-b-[var(--text-primary)]',
        'hover:border-b-brand-strong hover:text-brand-secondary data-[selected]:hover:border-b-brand-strong',
        'disabled:cursor-default disabled:text-disabled disabled:hover:border-b-transparent',
        className
      )}
      disabled={disabled}
      data-testid={dataTestId}
    >
      {Icon && <Icon className="size-3.5" />}
      {typeof label === 'string' ? (
        <Text
          variant="md"
          className="font-normal group-data-[selected]:font-medium"
        >
          {label}
        </Text>
      ) : label != null ? (
        label
      ) : null}
      {badgeProps && <Badge {...badgeProps} />}
    </Tab>
  );
}

export const TabPanels = HeadlessTabPanels;

export const TabGroup = HeadlessTabGroup;

export function TabPanel({
  children,
  className,
  panelRef,
  onScroll,
  unmount,
}: {
  children: React.ReactNode;
  className?: string;
  panelRef?: React.RefObject<HTMLDivElement | null>;
  onScroll?: (event: React.UIEvent<HTMLDivElement>) => void;
  unmount?: boolean;
}) {
  return (
    <HeadlessTabPanel
      className={cn('outline-none', className)}
      onScroll={onScroll}
      ref={panelRef}
      unmount={unmount}
    >
      {children}
    </HeadlessTabPanel>
  );
}
