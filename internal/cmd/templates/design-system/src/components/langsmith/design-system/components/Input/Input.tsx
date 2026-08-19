import type { ReactNode } from 'react';
import { forwardRef, useEffect, useId, useRef, useState } from 'react';

import { useDebouncedCallback } from 'use-debounce';

import {
  EyeIcon,
  EyeOffIcon,
  PaperclipIcon,
} from '@langchain/untitled-ui-icons';

import { cn } from '../../utils/cn';
import { mergeRefs } from '../../utils/merge-refs';
import { IconButton } from '../IconButton';
import { Text } from '../Text';
import {
  DECORATOR_CLASSES,
  getInputContainerClasses,
  getInputElementClasses,
} from './inputStyles';

export interface InputProps {
  size?: 'sm' | 'md';
  variant?: 'outlined' | 'plain';
  /** Left side decorator/icon */
  leftDecorator?: ReactNode;
  /** Right side decorator/icon */
  rightDecorator?: ReactNode;
  /** Whether the input is disabled */
  disabled?: boolean;
  /** Whether the input has an error state */
  isError?: boolean;
  /** Label text displayed above the input */
  label?: string | ReactNode;
  /** Hint text displayed below the input */
  hintText?: string;
  /** Current input value */
  value?: string;
  /** Change handler that will be debounced */
  onChange: (
    value: string,
    event?: React.ChangeEvent<HTMLInputElement>
  ) => void;
  /** Placeholder text */
  placeholder?: string;
  /**
   * Input type. Limited to the "text-like" native types that share this
   * component's styled-text rendering and string value contract. `password`
   * renders a masked secret field with a show/hide toggle. `date` uses the
   * browser's native date picker; its value is a `YYYY-MM-DD` string.
   * `datetime-local` uses the native date+time picker; its value is a
   * `YYYY-MM-DDTHH:mm` string in the viewer's local time. `file` is
   * uncontrolled: read the selection from `event.target.files` on the change
   * event, and scope it with `accept`/`multiple`.
   */
  type?:
    | 'text'
    | 'email'
    | 'password'
    | 'number'
    | 'url'
    | 'date'
    | 'datetime-local'
    | 'file';
  /** Debounce delay in milliseconds (default: 300). Ignored for `type="file"`. */
  debounceMs?: number;
  /** Additional CSS classes */
  className?: string;
  /** Input ID for accessibility */
  id?: string;
  /** Input name for forms */
  name?: string;
  /** Whether the field is required */
  required?: boolean;
  /** Auto focus on mount */
  autoFocus?: boolean;
  /** CSS class for the input container */
  inputContainerClassName?: string;
  /**
   * Which files the picker offers, as a comma-separated list of extensions or
   * MIME types (`'.csv,.json'`, `'image/*'`). `type="file"` only. The browser
   * treats this as a filter, not a guarantee — validate the selection too.
   */
  accept?: string;
  /** Whether more than one file can be picked. `type="file"` only. */
  multiple?: boolean;
}

function FileSelection({
  size,
  fileNames,
  placeholder,
}: {
  size: 'sm' | 'md';
  fileNames: string[];
  placeholder?: string;
}) {
  const summary =
    fileNames.length > 1 ? `${fileNames.length} files selected` : fileNames[0];
  const textClasses = cn(
    'leading-normal',
    size === 'sm' ? 'text-xs' : 'text-sm'
  );

  return (
    <>
      <Text
        aria-hidden
        as="span"
        className={cn(
          textClasses,
          'min-w-0 flex-1 truncate',
          summary ? 'text-primary' : 'text-placeholder'
        )}
      >
        {summary ?? placeholder ?? 'No file selected'}
      </Text>
      <Text
        aria-hidden
        as="span"
        weight="medium"
        className={cn(textClasses, 'shrink-0 text-secondary')}
      >
        Browse
      </Text>
    </>
  );
}

export const Input = forwardRef<
  HTMLInputElement,
  InputProps &
    Omit<React.InputHTMLAttributes<HTMLInputElement>, keyof InputProps>
>(
  (
    {
      size = 'md',
      variant = 'outlined',
      leftDecorator,
      rightDecorator,
      disabled = false,
      isError = false,
      label,
      hintText,
      value,
      onChange,
      placeholder,
      type = 'text',
      debounceMs = 0,
      className,
      inputContainerClassName,
      id,
      name,
      required = false,
      autoFocus = false,
      accept,
      multiple,
      'aria-describedby': ariaDescribedBy,
      ...rest
    },
    ref
  ) => {
    const generatedId = useId();
    const inputId = id || generatedId;
    const hintTextId = `${inputId}-hint`;
    const describedBy =
      [hintText ? hintTextId : undefined, ariaDescribedBy]
        .filter(Boolean)
        .join(' ') || undefined;
    const shouldDebounce = debounceMs > 0;
    const isFileInput = type === 'file';
    const isPasswordInput = type === 'password';
    const [internalValue, setInternalValue] = useState(value);
    const [isPasswordVisible, setIsPasswordVisible] = useState(false);
    const [selectedFileNames, setSelectedFileNames] = useState<string[]>([]);
    const debouncedOnChange = useDebouncedCallback(onChange, debounceMs);
    const fileInputRef = useRef<HTMLInputElement>(null);

    // Skip the click forwarded below, which would otherwise re-enter here.
    const openFilePicker = (event: React.MouseEvent<HTMLDivElement>) => {
      const target = event.target;
      if (
        disabled ||
        (target instanceof Element &&
          target.closest(
            'button, a, input, select, textarea, summary, [role="button"], [role="link"], [contenteditable="true"]'
          ))
      ) {
        return;
      }
      fileInputRef.current?.click();
    };

    // Update internal value when external value changes
    useEffect(() => {
      setInternalValue(value);
    }, [value]);

    useEffect(() => {
      if (!isPasswordInput) {
        setIsPasswordVisible(false);
      }
    }, [isPasswordInput]);

    useEffect(() => {
      const form = isFileInput ? fileInputRef.current?.form : null;
      if (!form) return;

      const clearFileSelection = () => setSelectedFileNames([]);
      form.addEventListener('reset', clearFileSelection);
      return () => form.removeEventListener('reset', clearFileSelection);
    }, [isFileInput]);

    const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
      const newValue = e.target.value;
      if (isFileInput) {
        onChange(newValue, e);
        setSelectedFileNames(
          Array.from(e.target.files ?? []).map((file) => file.name)
        );
        return;
      }

      setInternalValue(newValue);
      if (shouldDebounce) {
        debouncedOnChange(newValue);
      } else {
        onChange(newValue, e);
      }
    };

    const inputContainerClasses = getInputContainerClasses({
      size,
      variant,
      isError,
      disabled,
      className: inputContainerClassName,
    });

    const inputClasses = isFileInput
      ? 'sr-only'
      : getInputElementClasses({
          size,
          disabled,
          className: cn(
            isPasswordInput && !isPasswordVisible && 'text-security'
          ),
        });

    const labelClasses = cn(
      'mb-space-1 block text-sm font-medium text-primary',
      disabled && 'text-disabled'
    );

    const hintTextClasses = cn(
      'mt-space-1 text-xs',
      isError ? 'text-error-secondary' : 'text-tertiary'
    );

    const effectiveLeftDecorator =
      leftDecorator ??
      (isFileInput ? <PaperclipIcon className="size-4" /> : null);

    const showPasswordToggle = isPasswordInput;
    const effectiveRightDecorator =
      showPasswordToggle || rightDecorator ? (
        <>
          {rightDecorator}
          {showPasswordToggle && (
            <IconButton
              label="toggle password visibility"
              color="secondary"
              variant="plain"
              size="xs"
              tooltipProps={{ disabled: true }}
              disabled={disabled}
              onClick={() => setIsPasswordVisible((show) => !show)}
              onMouseDown={(event) => event.preventDefault()}
              icon={isPasswordVisible ? EyeOffIcon : EyeIcon}
            />
          )}
        </>
      ) : null;

    return (
      <div className={cn('w-full', className)}>
        {label && (
          <Text
            as="label"
            htmlFor={inputId}
            className={labelClasses}
            variant="xs"
          >
            {label}
            {required && (
              <span className="ml-space-1 text-error-secondary">*</span>
            )}
          </Text>
        )}

        <div
          className={cn(
            inputContainerClasses,
            isFileInput && !disabled && 'cursor-pointer'
          )}
          onClick={isFileInput ? openFilePicker : undefined}
        >
          {effectiveLeftDecorator && (
            <div className={DECORATOR_CLASSES}>{effectiveLeftDecorator}</div>
          )}

          {isFileInput && (
            <FileSelection
              size={size}
              fileNames={selectedFileNames}
              placeholder={placeholder}
            />
          )}

          {/* eslint-disable-next-line react/forbid-elements */}
          <input
            ref={isFileInput ? mergeRefs([fileInputRef, ref]) : ref}
            id={inputId}
            name={name ?? (typeof label === 'string' ? label : undefined)}
            type={
              isPasswordInput ? (isPasswordVisible ? 'text' : 'password') : type
            }
            value={isFileInput ? undefined : (internalValue ?? '')}
            onChange={handleInputChange}
            placeholder={placeholder}
            disabled={disabled}
            required={required}
            autoFocus={autoFocus}
            accept={accept}
            multiple={multiple}
            aria-describedby={describedBy}
            className={inputClasses}
            {...rest}
          />

          {effectiveRightDecorator && (
            <div className={cn(DECORATOR_CLASSES, 'gap-space-1')}>
              {effectiveRightDecorator}
            </div>
          )}
        </div>

        {hintText && (
          <Text
            id={hintTextId}
            className={hintTextClasses}
            variant="xs"
            weight="normal"
          >
            {hintText}
          </Text>
        )}
      </div>
    );
  }
);

Input.displayName = 'Input';
