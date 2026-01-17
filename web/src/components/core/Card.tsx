/**
 * Card component - base container used throughout the UI.
 * Provides consistent styling for cards with hover and active states.
 */

import {
	forwardRef,
	type HTMLAttributes,
	type KeyboardEvent,
	type MouseEvent,
	type ReactNode,
} from 'react';
import './Card.css';

export type CardPadding = 'sm' | 'md' | 'lg';

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
	/** Card contents */
	children: ReactNode;
	/** Enable hover effects (border lightens, optional lift) */
	hoverable?: boolean;
	/** Show active/selected state with primary tint */
	active?: boolean;
	/** Padding size */
	padding?: CardPadding;
	/** Additional CSS classes */
	className?: string;
	/** Click handler - makes card interactive */
	onClick?: (event: MouseEvent<HTMLDivElement>) => void;
}

/**
 * Card component with configurable padding, hover effects, and active state.
 *
 * @example
 * // Basic card
 * <Card>Content here</Card>
 *
 * @example
 * // Hoverable card with medium padding
 * <Card hoverable padding="md">Hover me</Card>
 *
 * @example
 * // Active/selected card
 * <Card active>Selected item</Card>
 *
 * @example
 * // Clickable card
 * <Card hoverable onClick={handleClick}>Click me</Card>
 */
export const Card = forwardRef<HTMLDivElement, CardProps>(
	(
		{
			children,
			hoverable = false,
			active = false,
			padding = 'md',
			className = '',
			onClick,
			onKeyDown,
			...props
		},
		ref
	) => {
		const isInteractive = Boolean(onClick);

		const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
			// Call any existing onKeyDown handler first
			onKeyDown?.(event);

			// Handle Enter/Space for interactive cards
			if (isInteractive && (event.key === 'Enter' || event.key === ' ')) {
				event.preventDefault();
				onClick?.(event as unknown as MouseEvent<HTMLDivElement>);
			}
		};

		const classes = [
			'card',
			`card-padding-${padding}`,
			hoverable && 'card-hoverable',
			active && 'card-active',
			isInteractive && 'card-interactive',
			className,
		]
			.filter(Boolean)
			.join(' ');

		return (
			<div
				ref={ref}
				className={classes}
				onClick={onClick}
				onKeyDown={handleKeyDown}
				role={isInteractive ? 'button' : undefined}
				tabIndex={isInteractive ? 0 : undefined}
				{...props}
			>
				{children}
			</div>
		);
	}
);

Card.displayName = 'Card';
