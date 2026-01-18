/**
 * Tooltip component - CSS-only implementation for hover hints.
 *
 * A lightweight tooltip component that uses pure CSS for show/hide behavior,
 * providing excellent performance for common hover hint use cases.
 * Supports keyboard focus, configurable positions, and customizable delay.
 *
 * For complex tooltips with rich content or controlled state, use the
 * Radix-based Tooltip in components/ui/Tooltip.tsx instead.
 */

import {
	forwardRef,
	useId,
	type HTMLAttributes,
	type ReactNode,
	type CSSProperties,
} from 'react';
import './Tooltip.css';

export type TooltipPosition = 'top' | 'bottom' | 'left' | 'right';

export interface TooltipProps extends HTMLAttributes<HTMLDivElement> {
	/** The tooltip text content */
	content: string;
	/** The trigger element */
	children: ReactNode;
	/** Position of the tooltip relative to the trigger (default: top) */
	position?: TooltipPosition;
	/** Delay in milliseconds before showing the tooltip (default: 300) */
	delay?: number;
	/** Additional class name for the wrapper */
	className?: string;
	/** Disable the tooltip */
	disabled?: boolean;
}

/**
 * CSS-only Tooltip component for hover hints.
 *
 * @example
 * // Basic usage
 * <Tooltip content="Board">
 *   <NavItem icon={LayoutDashboard} />
 * </Tooltip>
 *
 * @example
 * // With custom position
 * <Tooltip content="Edit" position="right">
 *   <IconButton icon={Edit} />
 * </Tooltip>
 *
 * @example
 * // With custom delay
 * <Tooltip content="Save changes" delay={500}>
 *   <Button>Save</Button>
 * </Tooltip>
 */
export const Tooltip = forwardRef<HTMLDivElement, TooltipProps>(
	(
		{
			content,
			children,
			position = 'top',
			delay = 300,
			className = '',
			disabled = false,
			style,
			...props
		},
		ref
	) => {
		const tooltipId = useId();

		// Return just children if disabled or no content
		if (disabled || !content) {
			return <>{children}</>;
		}

		const wrapperClasses = ['tooltip-wrapper', className].filter(Boolean).join(' ');

		const cssVars: CSSProperties = {
			...style,
			'--tooltip-delay': `${delay}ms`,
		} as CSSProperties;

		return (
			<div
				ref={ref}
				className={wrapperClasses}
				style={cssVars}
				{...props}
			>
				{children}
				<span
					id={tooltipId}
					className="tooltip-content"
					data-position={position}
					role="tooltip"
					aria-hidden="true"
				>
					{content}
				</span>
			</div>
		);
	}
);

Tooltip.displayName = 'Tooltip';
