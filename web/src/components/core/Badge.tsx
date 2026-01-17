/**
 * Badge component - status indicators, count badges, and tool pills.
 * Provides status variant for colored status badges, count variant for
 * numeric indicators, and tool variant for small pill-shaped tool labels.
 */

import { forwardRef, type HTMLAttributes, type ReactNode } from 'react';
import './Badge.css';

export type BadgeVariant = 'status' | 'count' | 'tool';
export type BadgeStatus = 'active' | 'paused' | 'completed' | 'failed' | 'idle';

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
	/** Visual variant of the badge */
	variant?: BadgeVariant;
	/** Status color (only applies when variant is 'status') */
	status?: BadgeStatus;
	/** Badge content */
	children?: ReactNode;
}

/**
 * Badge component for status indicators, counts, and tool labels.
 *
 * @example
 * // Status badge
 * <Badge variant="status" status="active">Active</Badge>
 *
 * @example
 * // Count badge
 * <Badge variant="count">27</Badge>
 *
 * @example
 * // Tool badge
 * <Badge variant="tool">File Read</Badge>
 */
export const Badge = forwardRef<HTMLSpanElement, BadgeProps>(
	({ variant = 'status', status = 'idle', className = '', children, ...props }, ref) => {
		// Render nothing if empty children
		if (children === null || children === undefined || children === '') {
			return null;
		}

		const classes = [
			'badge',
			`badge-${variant}`,
			variant === 'status' && `badge-status-${status}`,
			className,
		]
			.filter(Boolean)
			.join(' ');

		return (
			<span ref={ref} className={classes} {...props}>
				<span className="badge-content">{children}</span>
			</span>
		);
	}
);

Badge.displayName = 'Badge';
