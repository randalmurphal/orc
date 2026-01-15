/**
 * Input component - reusable form input with variants, sizes, and proper accessibility.
 * Supports icons, error states, and all standard HTML input attributes.
 */

import { forwardRef, useId } from 'react';
import './Input.css';

export type InputSize = 'sm' | 'md' | 'lg';
export type InputVariant = 'default' | 'error';

export interface InputProps
	extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'size'> {
	variant?: InputVariant;
	size?: InputSize;
	leftIcon?: React.ReactNode;
	rightIcon?: React.ReactNode;
	error?: string;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
	(
		{
			variant = 'default',
			size = 'md',
			leftIcon,
			rightIcon,
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
			'input-wrapper',
			`input-size-${size}`,
			`input-variant-${effectiveVariant}`,
			disabled && 'input-disabled',
			leftIcon && 'has-left-icon',
			rightIcon && 'has-right-icon',
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
			<div className="input-container">
				<div className={wrapperClasses}>
					{leftIcon && <span className="input-icon input-icon-left">{leftIcon}</span>}
					<input
						ref={ref}
						id={id}
						className="input-field"
						disabled={disabled}
						aria-invalid={effectiveVariant === 'error' ? true : undefined}
						aria-describedby={computedAriaDescribedBy || undefined}
						aria-required={props.required || undefined}
						{...props}
					/>
					{rightIcon && <span className="input-icon input-icon-right">{rightIcon}</span>}
				</div>
				{error && (
					<span id={errorId} className="input-error-message" role="alert">
						{error}
					</span>
				)}
			</div>
		);
	}
);

Input.displayName = 'Input';
