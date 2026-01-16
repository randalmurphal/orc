/**
 * Textarea component - reusable multi-line text input with variants, sizes, and proper accessibility.
 * Supports error states, resize control, and all standard HTML textarea attributes.
 */

import { forwardRef, useId } from 'react';
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
			'aria-describedby': ariaDescribedBy,
			...props
		},
		ref
	) => {
		const generatedId = useId();
		const errorId = `${id || generatedId}-error`;

		// Determine effective variant - error prop takes precedence
		const effectiveVariant = error ? 'error' : variant;

		const wrapperClasses = [
			'textarea-wrapper',
			`textarea-size-${size}`,
			`textarea-variant-${effectiveVariant}`,
			`textarea-resize-${resize}`,
			disabled && 'textarea-disabled',
			className,
		]
			.filter(Boolean)
			.join(' ');

		// Build aria-describedby
		const computedAriaDescribedBy = error
			? ariaDescribedBy
				? `${ariaDescribedBy} ${errorId}`
				: errorId
			: ariaDescribedBy;

		return (
			<div className="textarea-container">
				<div className={wrapperClasses}>
					<textarea
						ref={ref}
						id={id}
						className="textarea-field"
						disabled={disabled}
						aria-invalid={effectiveVariant === 'error' ? true : undefined}
						aria-describedby={computedAriaDescribedBy || undefined}
						aria-required={props.required || undefined}
						{...props}
					/>
				</div>
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
