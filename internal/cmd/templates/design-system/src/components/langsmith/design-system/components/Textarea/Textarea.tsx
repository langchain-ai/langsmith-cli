import type { ReactNode } from 'react';
import {
  forwardRef,
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';

import { useDebouncedCallback } from 'use-debounce';

import { cn } from '../../utils/cn';
import { mergeRefs } from '../../utils/merge-refs';
import { Text } from '../Text';

const DEFAULT_AUTO_RESIZE_MAX_HEIGHT_PX = 200;

export interface TextareaProps {
  size?: 'sm' | 'md';
  /** Left side decorator/icon */
  leftDecorator?: ReactNode;
  /** Right side decorator/icon */
  rightDecorator?: ReactNode;
  /** Whether the textarea is disabled */
  disabled?: boolean;
  /** Whether the textarea has an error state */
  isError?: boolean;
  /** Label text displayed above the textarea */
  label?: string | ReactNode;
  /** Hint text displayed below the textarea */
  hintText?: string;
  /** Current textarea value */
  value?: string;
  /** Change handler that will be debounced */
  onChange: (value: string) => void;
  /** Placeholder text */
  placeholder?: string;
  /** Debounce delay in milliseconds (default: 300) */
  debounceMs?: number;
  /** Additional CSS classes */
  className?: string;
  /** Additional CSS classes applied to the inner <textarea> element */
  inputClassName?: string;
  /** Textarea ID for accessibility */
  id?: string;
  /** Whether the field is required */
  required?: boolean;
  /** Auto focus on mount */
  autoFocus?: boolean;
  /** Number of visible rows */
  rows?: number;
  /** Whether the textarea can be resized */
  resize?: 'none' | 'vertical' | 'horizontal' | 'both';
  /** When true, textarea height grows with content up to maxHeight (resize is forced to none) */
  autoResize?: boolean;
  /** Max height in px when autoResize is true (default: 200) */
  maxHeight?: number;
  /** Visual variant: default (bordered container) or plain (borderless, for embedding) */
  variant?: 'default' | 'plain';
}

export const Textarea = forwardRef<
  HTMLTextAreaElement,
  TextareaProps &
    Omit<React.TextareaHTMLAttributes<HTMLTextAreaElement>, keyof TextareaProps>
>(
  (
    {
      size = 'md',
      leftDecorator,
      rightDecorator,
      disabled = false,
      isError = false,
      label,
      hintText,
      value,
      onChange,
      placeholder,
      debounceMs = 0,
      className,
      inputClassName,
      id,
      required = false,
      autoFocus = false,
      rows = 3,
      resize = 'vertical',
      autoResize = false,
      maxHeight = DEFAULT_AUTO_RESIZE_MAX_HEIGHT_PX,
      variant = 'default',
      ...rest
    },
    ref
  ) => {
    const generatedId = useId();
    const textareaId = id ?? generatedId;
    const shouldDebounce = debounceMs > 0;
    const [internalValue, setInternalValue] = useState(value);
    const debouncedOnChange = useDebouncedCallback(onChange, debounceMs);
    const textareaRef = useRef<HTMLTextAreaElement>(null);

    // Update internal value when external value changes
    useEffect(() => {
      setInternalValue(value);
    }, [value]);

    // Flush pending debounced onChange on unmount so the last edit is not lost
    useEffect(() => {
      if (!shouldDebounce) return;
      return () => {
        debouncedOnChange.flush();
      };
    }, [shouldDebounce, debouncedOnChange]);

    // Auto-resize: grow with content up to maxHeight.
    // Run on value change and on element resize (e.g. container width change so text wraps).
    const runAutoResize = useCallback(() => {
      const el = textareaRef.current;
      if (!el || !autoResize) return;
      el.style.height = 'auto';
      const contentHeight = Math.min(el.scrollHeight, maxHeight);
      el.style.height = `${contentHeight}px`;
      el.style.overflowY = el.scrollHeight > maxHeight ? 'auto' : 'hidden';
    }, [autoResize, maxHeight]);

    useLayoutEffect(() => {
      const el = textareaRef.current;
      if (!el) return;
      if (!autoResize) {
        el.style.height = '';
        el.style.overflowY = '';
        return;
      }

      runAutoResize();

      const ro = new ResizeObserver(runAutoResize);
      ro.observe(el);
      return () => ro.disconnect();
    }, [autoResize, internalValue, runAutoResize]);

    const handleTextareaChange = (
      e: React.ChangeEvent<HTMLTextAreaElement>
    ) => {
      const newValue = e.target.value;
      setInternalValue(newValue);
      if (shouldDebounce) {
        debouncedOnChange(newValue);
      } else {
        onChange(newValue);
      }
    };

    // Styled container that holds the textarea and decorators
    const textareaContainerClasses = cn(
      'flex w-full gap-space-2',
      variant === 'default' && [
        'rounded-md border bg-transparent transition-[border-color,box-shadow,background-color]',
        {
          'py-space-1 pl-space-2 pr-1.5': size === 'sm',
          'py-space-2 pl-space-3 pr-2.5': size === 'md',
        },
        'focus-within:outline-none',
        'border-subtle',
        !disabled && !isError && '[&:hover:not(:focus-within)]:border-default',
        !isError &&
          'focus-within:border-focus focus-within:shadow-[0_0_0_2px_var(--border-focus)]',
        isError &&
          'border-error-strong focus-within:border-error-strong focus-within:shadow-[0_0_0_2px_var(--border-error-strong)]',
        disabled && 'cursor-not-allowed bg-disabled opacity-50',
      ],
      leftDecorator || rightDecorator ? 'items-start' : 'items-center'
    );

    // Unstyled textarea that fills the container
    const effectiveResize = autoResize ? 'none' : resize;
    const textareaClasses = cn(
      'max-w-full flex-1 border-none bg-transparent p-0 outline-none',
      {
        'text-xs': size === 'sm',
        'text-sm': size === 'md',
      },
      'placeholder:text-placeholder',
      disabled && 'cursor-not-allowed',
      // Resize handling (autoResize forces none)
      {
        'resize-none': effectiveResize === 'none',
        'resize-y': effectiveResize === 'vertical',
        'resize-x': effectiveResize === 'horizontal',
        resize: effectiveResize === 'both',
      },
      inputClassName
    );

    // Decorator styles
    const decoratorClasses = cn(
      'flex items-center text-tertiary',
      size === 'sm' ? 'mt-0.5' : 'mt-space-1' // Align with first line of text
    );
    const leftDecoratorClasses = cn(decoratorClasses);
    const rightDecoratorClasses = cn(decoratorClasses);

    const labelClasses = cn(
      'mb-space-1 block font-medium text-primary',
      disabled && 'text-disabled'
    );

    const hintTextClasses = cn(
      'mt-space-1 text-xs',
      isError ? 'text-error-secondary' : 'text-tertiary'
    );

    const containerClasses = cn('w-full', className);

    return (
      <div className={containerClasses}>
        {label && (
          <Text
            as="label"
            htmlFor={textareaId}
            className={labelClasses}
            variant="sm"
          >
            {label}
            {required && (
              <span className="ml-space-1 text-error-secondary">*</span>
            )}
          </Text>
        )}

        <div className={textareaContainerClasses}>
          {leftDecorator && (
            <div className={leftDecoratorClasses}>{leftDecorator}</div>
          )}

          {/* eslint-disable-next-line react/forbid-elements */}
          <textarea
            ref={mergeRefs([textareaRef, ref])}
            id={textareaId}
            value={internalValue ?? ''}
            onChange={handleTextareaChange}
            placeholder={placeholder}
            disabled={disabled}
            required={required}
            autoFocus={autoFocus}
            rows={rows}
            className={textareaClasses}
            {...rest}
          />

          {rightDecorator && (
            <div className={rightDecoratorClasses}>{rightDecorator}</div>
          )}
        </div>

        {hintText && (
          <Text className={hintTextClasses} variant="xs" weight="normal">
            {hintText}
          </Text>
        )}
      </div>
    );
  }
);

Textarea.displayName = 'Textarea';
