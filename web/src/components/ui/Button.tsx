/**
 * Button primitive component - unified button styling for all variants.
 * Provides primary, secondary, danger, ghost, and success variants with
 * small, medium, and large sizes. Supports loading state, icons, and
 * icon-only mode.
 */

import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react';
import './Button.css';

export type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'ghost' | 'success';
export type ButtonSize = 'sm' | 'md' | 'lg';

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
	variant?: ButtonVariant;
	size?: ButtonSize;
	loading?: boolean;
	leftIcon?: ReactNode;
	rightIcon?: ReactNode;
	iconOnly?: boolean;
}

/**
 * Button component with multiple variants, sizes, and states.
 *
 * @example
 * // Primary button
 * <Button variant="primary">Submit</Button>
 *
 * @example
 * // Button with loading state
 * <Button loading>Saving...</Button>
 *
 * @example
 * // Icon-only button
 * <Button iconOnly aria-label="Add item">
 *   <Icon name="plus" />
 * </Button>
 */
export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
	(
		{
			variant = 'secondary',
			size = 'md',
			loading = false,
			leftIcon,
			rightIcon,
			iconOnly = false,
			disabled,
			className = '',
			children,
			...props
		},
		ref
	) => {
		const isDisabled = disabled || loading;

		const classes = [
			'btn',
			`btn-${variant}`,
			`btn-${size}`,
			iconOnly && 'btn-icon-only',
			loading && 'btn-loading',
			className,
		]
			.filter(Boolean)
			.join(' ');

		return (
			<button
				ref={ref}
				className={classes}
				disabled={isDisabled}
				aria-disabled={isDisabled || undefined}
				aria-busy={loading || undefined}
				{...props}
			>
				{loading && (
					<span className="btn-spinner" aria-hidden="true">
						<svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
							<circle
								cx="12"
								cy="12"
								r="10"
								stroke="currentColor"
								strokeWidth="3"
								strokeLinecap="round"
								strokeDasharray="31.4 31.4"
							/>
						</svg>
					</span>
				)}
				{!loading && leftIcon && <span className="btn-icon btn-icon-left">{leftIcon}</span>}
				{!iconOnly && <span className="btn-content">{children}</span>}
				{iconOnly && !loading && children}
				{!loading && rightIcon && <span className="btn-icon btn-icon-right">{rightIcon}</span>}
			</button>
		);
	}
);

Button.displayName = 'Button';
