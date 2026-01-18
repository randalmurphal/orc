/**
 * Input component for text entry.
 * Provides consistent styling matching the design system.
 */

import { forwardRef, type InputHTMLAttributes } from 'react';
import './Input.css';

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
	/** Additional class name */
	className?: string;
}

/**
 * Input component for text entry forms.
 *
 * @example
 * // Basic text input
 * <Input placeholder="Enter name..." />
 *
 * @example
 * // Password input
 * <Input type="password" placeholder="Password" />
 *
 * @example
 * // Disabled input
 * <Input value="Locked" disabled />
 *
 * @example
 * // With form name
 * <Input name="email" type="email" placeholder="Email address" />
 */
export const Input = forwardRef<HTMLInputElement, InputProps>(
	({ className = '', ...props }, ref) => {
		const classes = ['input', className].filter(Boolean).join(' ');

		return <input ref={ref} className={classes} {...props} />;
	}
);

Input.displayName = 'Input';
