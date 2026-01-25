/**
 * EmptyState component - displays placeholder content when no data is available.
 * Used for empty lists, search results, and initial states.
 */

import { forwardRef, type HTMLAttributes, type ReactNode } from 'react';
import { Icon, type IconName } from './Icon';
import './EmptyState.css';

export interface EmptyStateProps extends HTMLAttributes<HTMLDivElement> {
	/** Icon to display */
	icon?: IconName;
	/** Main title */
	title: string;
	/** Optional description text */
	description?: string;
	/** Optional action button or other content */
	action?: ReactNode;
}

export const EmptyState = forwardRef<HTMLDivElement, EmptyStateProps>(
	({ icon, title, description, action, className = '', ...props }, ref) => {
		const classes = ['empty-state', className].filter(Boolean).join(' ');

		return (
			<div ref={ref} className={classes} {...props}>
				{icon && (
					<div className="empty-state__icon">
						<Icon name={icon} size={32} />
					</div>
				)}
				<h3 className="empty-state__title">{title}</h3>
				{description && <p className="empty-state__description">{description}</p>}
				{action && <div className="empty-state__action">{action}</div>}
			</div>
		);
	}
);

EmptyState.displayName = 'EmptyState';
