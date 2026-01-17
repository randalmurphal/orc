/**
 * Button component matching the mockup design from example_ui/board.html.
 * Provides primary, ghost, and icon variants with loading and disabled states.
 */

import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react';
import './Button.css';

export type ButtonVariant = 'primary' | 'ghost' | 'icon';
export type ButtonSize = 'sm' | 'md' | 'lg';

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
	/** Button visual variant */
	variant?: ButtonVariant;
	/** Button size */
	size?: ButtonSize;
	/** Icon to display before text */
	icon?: ReactNode;
	/** Show loading spinner and disable interactions */
	loading?: boolean;
	/** Disable the button */
	disabled?: boolean;
	/** Children content */
	children?: ReactNode;
}

/**
 * Button component with primary, ghost, and icon variants.
 *
 * @example
 * // Primary button with icon
 * <Button variant="primary" icon={<Icon name="plus" />}>New Task</Button>
 *
 * @example
 * // Ghost button
 * <Button variant="ghost" icon={<Icon name="pause" />}>Pause</Button>
 *
 * @example
 * // Icon-only button (must have aria-label)
 * <Button variant="icon" aria-label="Toggle panel">
 *   <Icon name="panel" />
 * </Button>
 *
 * @example
 * // Loading state
 * <Button variant="primary" loading>Saving...</Button>
 */
export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
	(
		{
			variant = 'ghost',
			size = 'md',
			icon,
			loading = false,
			disabled,
			className = '',
			children,
			type = 'button',
			...props
		},
		ref
	) => {
		const isDisabled = disabled || loading;
		const isIconOnly = variant === 'icon';

		const classes = [
			'btn',
			`btn--${variant}`,
			`btn--${size}`,
			loading && 'btn--loading',
			className,
		]
			.filter(Boolean)
			.join(' ');

		return (
			<button
				ref={ref}
				type={type}
				className={classes}
				disabled={isDisabled}
				aria-disabled={isDisabled || undefined}
				aria-busy={loading || undefined}
				{...props}
			>
				{loading && (
					<span className="btn__spinner" aria-hidden="true">
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
				{!loading && icon && !isIconOnly && (
					<span className="btn__icon">{icon}</span>
				)}
				{isIconOnly ? (
					!loading && children
				) : (
					<span className="btn__content">{children}</span>
				)}
			</button>
		);
	}
);

Button.displayName = 'Button';
