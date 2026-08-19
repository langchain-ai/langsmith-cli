import type { ReactNode } from 'react';

import { CheckIcon, PlusIcon, XIcon } from '@langchain/untitled-ui-icons';

import { cn } from '../../utils/cn';
import { Badge } from '../Badge';
import { Icon } from '../Icon';
import { IconButton } from '../IconButton';
import { Text } from '../Text';
import type {
  TypeaheadRenderOptionState,
  TypeaheadSelectedValue,
} from './Typeahead.types';
import { isTypeaheadOption } from './Typeahead.utils';

export function TypeaheadClearButton({
  label,
  onClick,
}: {
  label: string;
  onClick: (event: React.MouseEvent<HTMLButtonElement>) => void;
}) {
  return (
    <IconButton
      type="button"
      icon={XIcon}
      label={label}
      variant="plain"
      color="secondary"
      size="xs"
      tooltipProps={{ title: null }}
      className="size-4 bg-transparent p-0 text-icon-tertiary shadow-none hover:bg-elevated-hover hover:text-icon-primary"
      onMouseDown={(event) => event.preventDefault()}
      onClick={onClick}
    />
  );
}

export function TypeaheadDefaultTag<TOption>({
  selected,
  index,
  disabled,
  getLabel,
  getValue,
  onRemove,
}: {
  selected: TypeaheadSelectedValue<TOption>;
  index: number;
  disabled: boolean;
  getLabel: (option: TypeaheadSelectedValue<TOption>) => string;
  getValue: (option: TypeaheadSelectedValue<TOption>) => string;
  onRemove: () => void;
}) {
  const label = getLabel(selected);

  return (
    // TODO: Replace this clickable badge with a keyboard-accessible remove control.
    <Badge
      key={`${getValue(selected)}-${index}`}
      color="plain"
      rounded="xs"
      size="xs"
      textWeight="normal"
      rightDecorator={!disabled ? XIcon : undefined}
      aria-label={!disabled ? `Remove ${label}` : undefined}
      onMouseDown={(event) => event.preventDefault()}
      onClick={
        !disabled
          ? (event) => {
              event.stopPropagation();
              onRemove();
            }
          : undefined
      }
      className={cn(
        'min-w-0 max-w-full justify-start border-default bg-surface-level-2 px-space-1 text-primary [&>span]:min-w-0 [&>span]:truncate',
        !disabled && 'cursor-pointer',
        disabled && 'cursor-not-allowed'
      )}
    >
      {label}
    </Badge>
  );
}

export function TypeaheadDefaultOption<TOption>({
  option,
  state,
  getLabel,
}: {
  option: TypeaheadSelectedValue<TOption>;
  state: TypeaheadRenderOptionState;
  getLabel: (option: TypeaheadSelectedValue<TOption>) => string;
}) {
  const label = getLabel(option);
  const description = isTypeaheadOption(option)
    ? option.description
    : undefined;
  const rightDecorator = isTypeaheadOption(option)
    ? option.rightDecorator
    : undefined;

  return (
    <>
      <Icon
        aria-hidden="true"
        icon={CheckIcon}
        size="md"
        className={cn(
          'shrink-0 text-icon-primary',
          state.selected ? 'opacity-100' : 'opacity-0'
        )}
      />
      <OptionText label={label} description={description} />
      {rightDecorator && (
        <div className="ml-auto shrink-0">{rightDecorator}</div>
      )}
    </>
  );
}

function OptionText({
  label,
  description,
}: {
  label: string;
  description: ReactNode;
}) {
  return (
    <div className="flex min-w-0 flex-1 flex-col">
      <Text as="span" variant="sm" className="truncate text-primary">
        {label}
      </Text>
      {description && (
        <Text as="span" variant="xs" className="truncate text-tertiary">
          {description}
        </Text>
      )}
    </div>
  );
}

export function TypeaheadCreateOption({
  inputValue,
  label,
}: {
  inputValue: string;
  label?: ReactNode;
}) {
  if (label != null) {
    return label;
  }

  return (
    <>
      <PlusIcon
        aria-hidden="true"
        className="size-4 shrink-0 text-icon-secondary"
      />
      <span className="min-w-0 truncate">{inputValue || 'Add new'}</span>
    </>
  );
}
