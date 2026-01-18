/**
 * Button component - primary, secondary, ghost, danger, success, and icon variants.
 * Provides consistent button styling with loading and disabled states.
 * Based on example_ui/board.html (.btn, .btn-primary, .btn-ghost, .btn-icon)
 */

import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import './Button.css';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'success' | 'icon';
export type ButtonSize = 'sm' | 'md' | 'lg';

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
	/** Visual variant of the button */
	variant?: ButtonVariant;
	/** Button size */
	size?: ButtonSize;
	/** Lucide icon to display before children (or centered for icon variant) */
	icon?: LucideIcon;
	/** Whether the button is in a loading state */
	loading?: boolean;
	/** Whether the button is active (for toggle buttons) */
	active?: boolean;
	/** Make button full width */
	fullWidth?: boolean;
	/** Button content (not used for icon variant) */
	children?: ReactNode;
}

const iconSizeMap: Record<ButtonSize, number> = {
	sm: 12,
	md: 14,
	lg: 16,
};

/**
 * Button component with multiple variants and states.
 *
 * @example
 * // Primary button with icon
 * <Button variant="primary" icon={Plus}>New Task</Button>
 *
 * @example
 * // Ghost button
 * <Button variant="ghost" icon={Pause}>Pause</Button>
 *
 * @example
 * // Icon-only button (requires aria-label)
 * <Button variant="icon" icon={Settings} aria-label="Settings" />
 *
 * @example
 * // Loading button
 * <Button variant="primary" loading>Saving...</Button>
 *
 * @example
 * // Disabled button
 * <Button variant="primary" disabled>Submit</Button>
 */
export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
	(
		{
			variant = 'primary',
			size = 'md',
			icon: Icon,
			loading = false,
			active = false,
			fullWidth = false,
			disabled,
			className = '',
			children,
			type = 'button',
			'aria-label': ariaLabel,
			...props
		},
		ref
	) => {
		const isIconOnly = variant === 'icon';
		const isDisabled = disabled || loading;
		const iconSize = iconSizeMap[size];

		const classes = [
			'btn',
			`btn-${variant}`,
			size !== 'md' && `btn-${size}`,
			loading && 'btn-loading',
			active && 'btn-active',
			fullWidth && 'btn-full',
			isDisabled && 'btn-disabled',
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
				aria-label={ariaLabel}
				aria-busy={loading}
				aria-disabled={isDisabled}
				{...props}
			>
				{loading && <span className="btn-spinner" aria-hidden="true" />}
				<span className="btn-content">
					{Icon && (
						<Icon
							width={iconSize}
							height={iconSize}
							aria-hidden="true"
						/>
					)}
					{!isIconOnly && children}
				</span>
			</button>
		);
	}
);

Button.displayName = 'Button';
