/**
 * Skeleton component - displays placeholder loading states.
 * Used for content loading states with shimmer animation.
 */

import { forwardRef, type HTMLAttributes } from 'react';
import './Skeleton.css';

export interface SkeletonProps extends HTMLAttributes<HTMLDivElement> {
	/** Width of the skeleton. Use string for percentage (e.g., '100%') or number for pixels */
	width?: string | number;
	/** Height of the skeleton. Use string for percentage (e.g., '100%') or number for pixels */
	height?: string | number;
	/** Visual variant */
	variant?: 'text' | 'circular' | 'rectangular';
}

export const Skeleton = forwardRef<HTMLDivElement, SkeletonProps>(
	({ width, height, variant = 'rectangular', className = '', style, ...props }, ref) => {
		const classes = ['skeleton', `skeleton--${variant}`, className].filter(Boolean).join(' ');

		const computedStyle = {
			...style,
			width: typeof width === 'number' ? `${width}px` : width,
			height: typeof height === 'number' ? `${height}px` : height,
		};

		return <div ref={ref} className={classes} style={computedStyle} {...props} />;
	}
);

Skeleton.displayName = 'Skeleton';
