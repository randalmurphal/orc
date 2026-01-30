/**
 * InitiativeCard component - displays individual initiative information in a card layout.
 * Shows initiative metadata (icon, title, description, status), progress tracking,
 * and metrics (time remaining, cost, tokens).
 */

import { forwardRef, useCallback, type HTMLAttributes, type KeyboardEvent } from 'react';
import { Tooltip } from '@/components/ui/Tooltip';
import { formatNumber, formatCost } from '@/lib/format';
import { type Initiative, InitiativeStatus } from '@/gen/orc/v1/initiative_pb';
import { extractEmoji, getStatusColor, getIconColor, isPaused, getStatusLabel } from './initiative-utils';
import './InitiativeCard.css';

// =============================================================================
// Types
// =============================================================================

export interface InitiativeCardProps extends HTMLAttributes<HTMLDivElement> {
	initiative: Initiative;
	/** Number of completed tasks (parent computes from tasks) */
	completedTasks?: number;
	/** Total number of tasks (parent computes from tasks) */
	totalTasks?: number;
	/** Estimated time remaining (e.g., "8h remaining", "15m remaining") */
	estimatedTimeRemaining?: string;
	/** Cost spent in dollars */
	costSpent?: number;
	/** Tokens used */
	tokensUsed?: number;
	/** Click handler for card navigation */
	onClick?: () => void;
	className?: string;
}

// =============================================================================
// Icons
// =============================================================================

function ClockIcon() {
	return (
		<svg
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			aria-hidden="true"
		>
			<circle cx="12" cy="12" r="10" />
			<polyline points="12 6 12 12 16 14" />
		</svg>
	);
}

function DollarIcon() {
	return (
		<svg
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			aria-hidden="true"
		>
			<path d="M12 2v20M17 5H9.5a3.5 3.5 0 000 7h5a3.5 3.5 0 010 7H6" />
		</svg>
	);
}

function LightningIcon() {
	return (
		<svg
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			aria-hidden="true"
		>
			<path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z" />
		</svg>
	);
}

// =============================================================================
// StatusBadge Component
// =============================================================================

interface StatusBadgeProps {
	status: InitiativeStatus;
}

function StatusBadge({ status }: StatusBadgeProps) {
	const colorClass = `initiative-card-status-${getStatusColor(status)}`;
	const label = getStatusLabel(status);

	return (
		<span
			className={`initiative-card-status ${colorClass}`}
			role="status"
			aria-label={`Status: ${label}`}
		>
			{label}
		</span>
	);
}

// =============================================================================
// InitiativeCard Component
// =============================================================================

/**
 * InitiativeCard component displaying an initiative with progress and metrics.
 *
 * @example
 * // Basic usage
 * <InitiativeCard
 *   initiative={initiative}
 *   completedTasks={15}
 *   totalTasks={20}
 *   onClick={() => navigate(`/initiatives/${initiative.id}`)}
 * />
 *
 * @example
 * // With all metrics
 * <InitiativeCard
 *   initiative={initiative}
 *   completedTasks={15}
 *   totalTasks={20}
 *   estimatedTimeRemaining="Est. 2h remaining"
 *   costSpent={18.45}
 *   tokensUsed={542000}
 *   onClick={handleClick}
 * />
 */
export const InitiativeCard = forwardRef<HTMLDivElement, InitiativeCardProps>(
	(
		{
			initiative,
			completedTasks = 0,
			totalTasks = 0,
			estimatedTimeRemaining,
			costSpent,
			tokensUsed,
			onClick,
			className = '',
			...props
		},
		ref
	) => {
		const paused = isPaused(initiative.status);
		const statusColor = getStatusColor(initiative.status);
		const iconColor = getIconColor(initiative.status);
		const emoji = extractEmoji(initiative.title) || extractEmoji(initiative.vision);

		// Calculate progress percentage
		const progressPercent = totalTasks > 0 ? (completedTasks / totalTasks) * 100 : 0;

		// Build class names
		const classes = [
			'initiative-card',
			paused ? 'initiative-card-paused' : '',
			onClick ? 'initiative-card-clickable' : '',
			className,
		]
			.filter(Boolean)
			.join(' ');

		// Handle keyboard interaction
		const handleKeyDown = useCallback(
			(event: KeyboardEvent<HTMLDivElement>) => {
				if (onClick && (event.key === 'Enter' || event.key === ' ')) {
					event.preventDefault();
					onClick();
				}
			},
			[onClick]
		);

		// Determine if we have any meta items to display
		const hasMetaItems = estimatedTimeRemaining || costSpent !== undefined || tokensUsed !== undefined;

		return (
			<article
				ref={ref}
				className={classes}
				onClick={onClick}
				onKeyDown={handleKeyDown}
				tabIndex={onClick ? 0 : undefined}
				role={onClick ? 'button' : undefined}
				aria-label={`Initiative: ${initiative.title}. Status: ${initiative.status}. Progress: ${completedTasks} of ${totalTasks} tasks complete.`}
				{...props}
			>
				{/* Header */}
				<div className="initiative-card-header">
					<div className={`initiative-card-icon initiative-card-icon-${iconColor}`}>
						{emoji}
					</div>
					<div className="initiative-card-info">
						<Tooltip content={initiative.title} side="top">
							<h3 className="initiative-card-name">{initiative.title}</h3>
						</Tooltip>
						{initiative.vision && (
							<Tooltip content={initiative.vision} side="top">
								<p className="initiative-card-desc">{initiative.vision}</p>
							</Tooltip>
						)}
					</div>
					<StatusBadge status={initiative.status} />
				</div>

				{/* Progress Section */}
				<div className="initiative-card-progress">
					<div className="initiative-card-progress-header">
						<span className="initiative-card-progress-label">Progress</span>
						<span className="initiative-card-progress-value">
							{completedTasks} / {totalTasks} tasks
						</span>
					</div>
					<div
						className="initiative-card-progress-bar"
						role="progressbar"
						aria-valuenow={progressPercent}
						aria-valuemin={0}
						aria-valuemax={100}
						aria-label={`Progress: ${Math.round(progressPercent)}%`}
					>
						<div
							className={`initiative-card-progress-fill initiative-card-progress-fill-${statusColor}`}
							style={{ width: `${progressPercent}%` }}
						/>
					</div>
				</div>

				{/* Meta Row */}
				{hasMetaItems && (
					<div className="initiative-card-meta">
						{estimatedTimeRemaining && (
							<div className="initiative-card-meta-item">
								<ClockIcon />
								<span>{estimatedTimeRemaining}</span>
							</div>
						)}
						{costSpent !== undefined && (
							<div className="initiative-card-meta-item">
								<DollarIcon />
								<span>{formatCost(costSpent)} spent</span>
							</div>
						)}
						{tokensUsed !== undefined && (
							<div className="initiative-card-meta-item">
								<LightningIcon />
								<span>{formatNumber(tokensUsed)} tokens</span>
							</div>
						)}
					</div>
				)}
			</article>
		);
	}
);

InitiativeCard.displayName = 'InitiativeCard';
