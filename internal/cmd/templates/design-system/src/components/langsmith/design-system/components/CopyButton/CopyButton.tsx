import { type ReactNode, useState } from 'react';

import { CheckIcon, Copy06Icon } from '@langchain/untitled-ui-icons';

import { Button } from '../Button';
import { IconButton, type IconButtonProps } from '../IconButton';
import { Text } from '../Text';
import { useCopy } from './useCopy';

export interface CopyButtonProps {
  copy: string;
  variant?: 'full' | 'icon';
  className?: string;
  copyText?: string;
  disabled?: boolean;
  onCopy?: () => void;
}

export function CopyButton(props: CopyButtonProps) {
  const { copied, onCopy } = useCopy({ copy: props.copy });

  if (props.variant === 'icon') {
    return (
      <CopyIconButton
        copy={props.copy}
        copyText={props.copyText}
        className={props.className}
        disabled={props.disabled}
        onCopy={props.onCopy}
      />
    );
  }

  const CopyIcon = copied ? CheckIcon : Copy06Icon;

  return (
    <Button
      size="sm"
      color="secondary"
      variant="outlined"
      aria-label={copied ? 'Copied' : (props.copyText ?? 'Copy')}
      onClick={(event) => {
        event.stopPropagation();
        onCopy();
        props.onCopy?.();
      }}
      className={props.className}
      disabled={props.disabled}
    >
      <span className="grid">
        {/* Both states rendered invisibly so the grid cell is always wide enough for either */}
        <Text
          as="span"
          variant="sm"
          weight="normal"
          className="invisible col-start-1 row-start-1 flex items-center gap-1 whitespace-nowrap"
          aria-hidden
        >
          <Copy06Icon className="size-4 shrink-0" />
          {props.copyText ?? 'Copy'}
        </Text>
        <Text
          as="span"
          variant="sm"
          weight="normal"
          className="invisible col-start-1 row-start-1 flex items-center gap-1 whitespace-nowrap"
          aria-hidden
        >
          <CheckIcon className="size-4 shrink-0" />
          Copied
        </Text>
        <Text
          as="span"
          variant="sm"
          weight="normal"
          className="col-start-1 row-start-1 flex items-center justify-center gap-1 whitespace-nowrap"
        >
          <CopyIcon className="size-4 shrink-0" />
          {copied ? 'Copied' : (props.copyText ?? 'Copy')}
        </Text>
      </span>
    </Button>
  );
}

export interface CopyIconButtonProps {
  copy: string;
  className?: string;
  iconClassName?: string;
  copyText?: ReactNode;
  label?: string;
  disabled?: boolean;
  color?: IconButtonProps['color'];
  size?: IconButtonProps['size'];
  variant?: IconButtonProps['variant'];
  onCopy?: () => void;
}

function getCopyLabel(copyText: ReactNode, label?: string) {
  if (label) return label;
  return typeof copyText === 'string' ? copyText : 'Copy';
}

export function CopyIconButton(props: CopyIconButtonProps) {
  const { copied, onCopy } = useCopy({ copy: props.copy });
  const [tooltipOpen, setTooltipOpen] = useState(false);
  const copyLabel = getCopyLabel(props.copyText, props.label);
  const tooltipTitle = props.copyText ?? copyLabel;

  return (
    <IconButton
      onClick={(e) => {
        e.stopPropagation();
        onCopy();
        props.onCopy?.();
      }}
      disabled={props.disabled}
      icon={copied ? CheckIcon : Copy06Icon}
      label={copied ? 'Copied' : copyLabel}
      tooltipProps={{
        title: copied ? 'Copied' : tooltipTitle,
        side: 'top',
        open: tooltipOpen,
        onOpenChange: (open) => {
          if (open) setTooltipOpen(true);
        },
      }}
      variant={props.variant ?? 'plain'}
      color={props.color ?? 'secondary'}
      size={props.size ?? 'sm'}
      className={props.className}
      iconClassName={props.iconClassName}
      onMouseLeave={() => setTooltipOpen(false)}
      onBlur={() => setTooltipOpen(false)}
    />
  );
}
