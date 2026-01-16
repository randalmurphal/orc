/**
 * Tooltip component - Radix Tooltip primitive with consistent styling.
 *
 * Provides accessible hover tooltips with keyboard support, configurable
 * placement, and delay. Use instead of native `title` attribute for
 * consistent styling and behavior.
 *
 * Features:
 * - Accessible (ARIA tooltip role, keyboard focusable)
 * - Configurable delay and placement
 * - Arrow indicator
 * - Respects reduced motion preference
 * - Portals to document.body to avoid z-index issues
 */

import { type ReactNode } from 'react';
import * as TooltipPrimitive from '@radix-ui/react-tooltip';
import './Tooltip.css';

export type TooltipSide = 'top' | 'right' | 'bottom' | 'left';
export type TooltipAlign = 'start' | 'center' | 'end';

export interface TooltipProps {
	/** The trigger element that shows the tooltip on hover/focus */
	children: ReactNode;
	/** The tooltip content */
	content: ReactNode;
	/** Side of the trigger to show tooltip (default: top) */
	side?: TooltipSide;
	/** Alignment along the side (default: center) */
	align?: TooltipAlign;
	/** Offset from the trigger in pixels (default: 6) */
	sideOffset?: number;
	/** Delay before showing in ms (uses provider default if not specified) */
	delayDuration?: number;
	/** Whether to disable the tooltip (default: false) */
	disabled?: boolean;
	/** Whether to show the arrow indicator (default: true) */
	showArrow?: boolean;
	/** Open state for controlled usage */
	open?: boolean;
	/** Callback for controlled open state changes */
	onOpenChange?: (open: boolean) => void;
	/** Additional class name for the content */
	className?: string;
}

/**
 * Tooltip component with consistent styling.
 *
 * @example
 * // Basic usage
 * <Tooltip content="Click to save">
 *   <Button>Save</Button>
 * </Tooltip>
 *
 * @example
 * // Custom placement
 * <Tooltip content="Edit task" side="right" align="start">
 *   <IconButton />
 * </Tooltip>
 *
 * @example
 * // Rich content
 * <Tooltip content={<>Press <kbd>⌘K</kbd> to open</>}>
 *   <Button>Commands</Button>
 * </Tooltip>
 *
 * @example
 * // Disabled tooltip
 * <Tooltip content="This won't show" disabled>
 *   <Button>No tooltip</Button>
 * </Tooltip>
 */
export function Tooltip({
	children,
	content,
	side = 'top',
	align = 'center',
	sideOffset = 6,
	delayDuration,
	disabled = false,
	showArrow = true,
	open,
	onOpenChange,
	className = '',
}: TooltipProps) {
	// Don't render tooltip if disabled or no content
	if (disabled || !content) {
		return <>{children}</>;
	}

	const contentClasses = ['tooltip-content', className].filter(Boolean).join(' ');

	return (
		<TooltipPrimitive.Root
			delayDuration={delayDuration}
			open={open}
			onOpenChange={onOpenChange}
		>
			<TooltipPrimitive.Trigger asChild>{children}</TooltipPrimitive.Trigger>
			<TooltipPrimitive.Portal>
				<TooltipPrimitive.Content
					className={contentClasses}
					side={side}
					align={align}
					sideOffset={sideOffset}
				>
					{content}
					{showArrow && <TooltipPrimitive.Arrow className="tooltip-arrow" />}
				</TooltipPrimitive.Content>
			</TooltipPrimitive.Portal>
		</TooltipPrimitive.Root>
	);
}

/**
 * TooltipProvider wraps the application to enable tooltips.
 * Place this at the root of your app or around a section using tooltips.
 *
 * @example
 * <TooltipProvider delayDuration={200}>
 *   <App />
 * </TooltipProvider>
 */
export interface TooltipProviderProps {
	children: ReactNode;
	/** Default delay duration for all tooltips (default: 300ms) */
	delayDuration?: number;
	/** Time to wait before hiding when moving to another tooltip (default: 300ms) */
	skipDelayDuration?: number;
	/** Prevent tooltips from remaining open when hovering content */
	disableHoverableContent?: boolean;
}

export function TooltipProvider({
	children,
	delayDuration = 300,
	skipDelayDuration = 300,
	disableHoverableContent = false,
}: TooltipProviderProps) {
	return (
		<TooltipPrimitive.Provider
			delayDuration={delayDuration}
			skipDelayDuration={skipDelayDuration}
			disableHoverableContent={disableHoverableContent}
		>
			{children}
		</TooltipPrimitive.Provider>
	);
}
