/**
 * ExecutionHeader - Header bar showing workflow execution status and metrics
 *
 * TASK-639: Live execution visualization on workflow canvas
 *
 * Displays:
 * - Status badge (Running/Completed/Failed/Cancelled) with pulse animation for running
 * - Session metrics (duration, tokens, cost)
 * - Cancel button (for running workflows)
 * - Reconnecting indicator (when WebSocket disconnects)
 */

import { useState } from 'react';
import { RunStatus } from '@/gen/orc/v1/workflow_pb';
import { toast } from '@/stores';
import './ExecutionHeader.css';

export interface ExecutionHeaderProps {
	runStatus: RunStatus;
	duration: string;
	totalTokens: number;
	totalCost: number;
	onCancel: () => Promise<void>;
	isReconnecting?: boolean;
}

/** Format tokens for display (e.g., 45200 → "45.2K", 1500000 → "1.5M") */
function formatTokens(tokens: number): string {
	if (tokens >= 1_000_000) {
		return `${(tokens / 1_000_000).toFixed(1)}M`;
	}
	if (tokens >= 1_000) {
		return `${(tokens / 1_000).toFixed(1)}K`;
	}
	return tokens.toString();
}

/** Format cost for display (e.g., 1.23 → "$1.23") */
function formatCost(cost: number): string {
	return `$${cost.toFixed(2)}`;
}

/** Get status badge label */
function getStatusLabel(status: RunStatus): string {
	switch (status) {
		case RunStatus.RUNNING:
			return 'Running';
		case RunStatus.COMPLETED:
			return 'Completed';
		case RunStatus.FAILED:
			return 'Failed';
		case RunStatus.CANCELLED:
			return 'Cancelled';
		case RunStatus.PENDING:
			return 'Pending';
		default:
			return 'Unknown';
	}
}

/** Get status badge variant class */
function getStatusVariant(status: RunStatus): string {
	switch (status) {
		case RunStatus.RUNNING:
			return 'execution-badge--running';
		case RunStatus.COMPLETED:
			return 'execution-badge--completed';
		case RunStatus.FAILED:
			return 'execution-badge--failed';
		case RunStatus.CANCELLED:
			return 'execution-badge--cancelled';
		default:
			return '';
	}
}

export function ExecutionHeader({
	runStatus,
	duration,
	totalTokens,
	totalCost,
	onCancel,
	isReconnecting = false,
}: ExecutionHeaderProps) {
	const [showConfirm, setShowConfirm] = useState(false);
	const [cancelling, setCancelling] = useState(false);

	const isRunning = runStatus === RunStatus.RUNNING;
	const isPending = runStatus === RunStatus.PENDING;
	const showMetrics = !isPending;

	const handleCancelClick = () => {
		setShowConfirm(true);
	};

	const handleConfirmCancel = async () => {
		setCancelling(true);
		try {
			await onCancel();
			setShowConfirm(false);
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Unknown error';
			toast.error(`Failed to cancel: ${message}`);
		} finally {
			setCancelling(false);
		}
	};

	const handleCancelConfirm = () => {
		setShowConfirm(false);
	};

	const statusLabel = getStatusLabel(runStatus);
	const statusVariant = getStatusVariant(runStatus);
	const badgeClasses = ['execution-badge', statusVariant];
	if (isRunning) {
		badgeClasses.push('execution-badge--pulse');
	}

	return (
		<div className="execution-header">
			{/* Status badge */}
			<span className={badgeClasses.join(' ')}>{statusLabel}</span>

			{/* Reconnecting indicator */}
			{isReconnecting && (
				<span className="execution-reconnecting">Reconnecting...</span>
			)}

			{/* Metrics */}
			{showMetrics && (
				<div className="execution-metrics">
					<span className="execution-metric">
						<span className="execution-metric-icon">⏱</span>
						{duration}
					</span>
					<span className="execution-metric">
						<span className="execution-metric-icon">🔤</span>
						{formatTokens(totalTokens)}
					</span>
					<span className="execution-metric">
						<span className="execution-metric-icon">💵</span>
						{formatCost(totalCost)}
					</span>
				</div>
			)}

			{/* Cancel button */}
			{isRunning && (
				<button
					className="execution-cancel-btn"
					onClick={handleCancelClick}
					disabled={cancelling}
				>
					{cancelling ? 'Cancelling...' : 'Cancel'}
				</button>
			)}

			{/* Confirmation dialog */}
			{showConfirm && (
				<div className="execution-confirm-overlay">
					<div className="execution-confirm-dialog">
						<p>Are you sure you want to cancel this workflow?</p>
						<div className="execution-confirm-actions">
							<button
								className="execution-confirm-btn"
								onClick={handleConfirmCancel}
								disabled={cancelling}
							>
								{cancelling ? 'Cancelling...' : 'Confirm'}
							</button>
							<button
								className="execution-cancel-confirm-btn"
								onClick={handleCancelConfirm}
								disabled={cancelling}
							>
								No, keep running
							</button>
						</div>
					</div>
				</div>
			)}
		</div>
	);
}
