/**
 * Textarea component - reusable multi-line text input with variants, sizes, and proper accessibility.
 * Supports error states, resize control, auto-resize, character count, and all standard HTML textarea attributes.
 */

import {
	forwardRef,
	useId,
	useRef,
	useCallback,
	useEffect,
	useImperativeHandle,
} from 'react';
import './Textarea.css';

export type TextareaSize = 'sm' | 'md' | 'lg';
export type TextareaVariant = 'default' | 'error';
export type TextareaResize = 'none' | 'vertical' | 'horizontal' | 'both';

export interface TextareaProps
	extends Omit<React.TextareaHTMLAttributes<HTMLTextAreaElement>, 'size'> {
	variant?: TextareaVariant;
	size?: TextareaSize;
	resize?: TextareaResize;
	error?: string;
	/** Enable auto-resize behavior - textarea grows with content */
	autoResize?: boolean;
	/** Maximum height in pixels when auto-resize is enabled (default: 300) */
	maxHeight?: number;
	/** Show character count when maxLength is provided */
	showCount?: boolean;
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(
	(
		{
			variant = 'default',
			size = 'md',
			resize = 'vertical',
			error,
			className = '',
			disabled,
			id,
			autoResize = false,
			maxHeight = 300,
			showCount = false,
			maxLength,
			value,
			defaultValue,
			onChange,
			'aria-describedby': ariaDescribedBy,
			...props
		},
		ref
	) => {
		const generatedId = useId();
		const textareaId = id || generatedId;
		const errorId = `${textareaId}-error`;
		const countId = `${textareaId}-count`;
		const internalRef = useRef<HTMLTextAreaElement>(null);

		// Expose the internal ref to parent via forwarded ref
		useImperativeHandle(ref, () => internalRef.current!, []);

		// Determine effective variant - error prop takes precedence
		const effectiveVariant = error ? 'error' : variant;

		// Get current character count
		const currentValue =
			value !== undefined
				? String(value)
				: internalRef.current?.value || String(defaultValue || '');
		const charCount = currentValue.length;
		const isNearLimit = maxLength ? charCount / maxLength >= 0.9 : false;

		// Auto-resize handler
		const adjustHeight = useCallback(() => {
			const textarea = internalRef.current;
			if (!textarea || !autoResize) return;

			// Reset height to auto to get the correct scrollHeight
			textarea.style.height = 'auto';

			// Calculate new height, respecting maxHeight
			const newHeight = Math.min(textarea.scrollHeight, maxHeight);
			textarea.style.height = `${newHeight}px`;

			// Set overflow based on whether content exceeds maxHeight
			textarea.style.overflowY =
				textarea.scrollHeight > maxHeight ? 'auto' : 'hidden';
		}, [autoResize, maxHeight]);

		// Handle change with auto-resize
		const handleChange = useCallback(
			(e: React.ChangeEvent<HTMLTextAreaElement>) => {
				onChange?.(e);
				if (autoResize) {
					adjustHeight();
				}
			},
			[onChange, autoResize, adjustHeight]
		);

		// Initial height adjustment
		useEffect(() => {
			if (autoResize) {
				adjustHeight();
			}
		}, [autoResize, adjustHeight, value]);

		// Effective resize - disable manual resize when autoResize is enabled
		const effectiveResize = autoResize ? 'none' : resize;

		const wrapperClasses = [
			'textarea-wrapper',
			`textarea-size-${size}`,
			`textarea-variant-${effectiveVariant}`,
			`textarea-resize-${effectiveResize}`,
			autoResize && 'textarea-auto-resize',
			disabled && 'textarea-disabled',
			className,
		]
			.filter(Boolean)
			.join(' ');

		// Build aria-describedby
		const describedByParts: string[] = [];
		if (ariaDescribedBy) describedByParts.push(ariaDescribedBy);
		if (error) describedByParts.push(errorId);
		if (showCount && maxLength) describedByParts.push(countId);
		const computedAriaDescribedBy =
			describedByParts.length > 0 ? describedByParts.join(' ') : undefined;

		return (
			<div className="textarea-container">
				<div className={wrapperClasses}>
					<textarea
						ref={internalRef}
						id={textareaId}
						className="textarea-field"
						disabled={disabled}
						maxLength={maxLength}
						value={value}
						defaultValue={defaultValue}
						onChange={handleChange}
						aria-invalid={effectiveVariant === 'error' ? true : undefined}
						aria-describedby={computedAriaDescribedBy}
						aria-required={props.required || undefined}
						{...props}
					/>
				</div>
				{showCount && maxLength && (
					<span
						id={countId}
						className={`textarea-char-count ${isNearLimit ? 'textarea-char-count-warning' : ''}`}
					>
						{charCount}/{maxLength}
					</span>
				)}
				{error && (
					<span id={errorId} className="textarea-error-message" role="alert">
						{error}
					</span>
				)}
			</div>
		);
	}
);

Textarea.displayName = 'Textarea';
