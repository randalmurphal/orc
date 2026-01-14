/**
 * FinalizeModal - Real-time finalize progress viewer
 *
 * Shows finalize operation progress via WebSocket streaming.
 * Displays step progression, progress bar, and result information.
 *
 * WebSocket events handled:
 * - finalize: progress updates with step/percent/result/error
 */

import { useState, useEffect, useCallback } from 'react';
import { createPortal } from 'react-dom';
import type { Task, ConnectionStatus, WSEvent } from '@/lib/types';
import {
	triggerFinalize,
	getFinalizeStatus,
	type FinalizeState,
	type FinalizeResult,
} from '@/lib/api';
import { useWebSocket } from '@/hooks/useWebSocket';
import { Icon } from '@/components/ui/Icon';
import './FinalizeModal.css';

interface FinalizeModalProps {
	open: boolean;
	task: Task;
	onClose: () => void;
}

const STATUS_CONFIGS: Record<string, { label: string; color: string; bg: string }> = {
	not_started: { label: 'Not Started', color: 'var(--text-muted)', bg: 'var(--bg-tertiary)' },
	pending: { label: 'Pending', color: 'var(--status-warning)', bg: 'var(--status-warning-bg)' },
	running: { label: 'Running', color: 'var(--status-info)', bg: 'var(--status-info-bg)' },
	completed: { label: 'Completed', color: 'var(--status-success)', bg: 'var(--status-success-bg)' },
	failed: { label: 'Failed', color: 'var(--status-danger)', bg: 'var(--status-danger-bg)' },
};

export function FinalizeModal({ open, task, onClose }: FinalizeModalProps) {
	const ws = useWebSocket();
	const [finalizeState, setFinalizeState] = useState<FinalizeState | null>(null);
	const [loading, setLoading] = useState(false);
	const [triggering, setTriggering] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('disconnected');

	// Load status
	const loadStatus = useCallback(async () => {
		setLoading(true);
		setError(null);

		try {
			const state = await getFinalizeStatus(task.id);
			setFinalizeState(state);
		} catch {
			// Not started is fine
			setFinalizeState({
				task_id: task.id,
				status: 'not_started',
			});
		} finally {
			setLoading(false);
		}
	}, [task.id]);

	// Setup WebSocket
	useEffect(() => {
		if (!open) return;

		loadStatus();

		// Listen to finalize events
		const unsubscribeEvent = ws.on('all', (event) => {
			if (!('event' in event)) return;
			const wsEvent = event as WSEvent;

			// Only handle events for our task
			if (wsEvent.task_id !== task.id) return;

			if (wsEvent.event === 'finalize') {
				const data = wsEvent.data as Record<string, unknown>;
				setFinalizeState({
					task_id: data.task_id as string,
					status: data.status as FinalizeState['status'],
					step: data.step as string | undefined,
					progress: data.progress as string | undefined,
					step_percent: data.step_percent as number | undefined,
					updated_at: data.updated_at as string | undefined,
					result: data.result as FinalizeResult | undefined,
					error: data.error as string | undefined,
				});
			}
		});

		// Track connection status from ws hook
		setConnectionStatus(ws.status);

		return () => {
			unsubscribeEvent();
		};
	}, [open, task.id, loadStatus, ws]);

	// Handle keyboard
	useEffect(() => {
		if (!open) return;

		const handleKeyDown = (e: KeyboardEvent) => {
			if (e.key === 'Escape') {
				onClose();
			}
		};

		window.addEventListener('keydown', handleKeyDown);
		document.body.style.overflow = 'hidden';

		return () => {
			window.removeEventListener('keydown', handleKeyDown);
			document.body.style.overflow = '';
		};
	}, [open, onClose]);

	// Trigger finalize
	const handleTriggerFinalize = async () => {
		setTriggering(true);
		setError(null);

		try {
			await triggerFinalize(task.id);
			// Status will update via WebSocket
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to trigger finalize');
		} finally {
			setTriggering(false);
		}
	};

	const handleBackdropClick = (e: React.MouseEvent) => {
		if (e.target === e.currentTarget) {
			onClose();
		}
	};

	// Derived states
	const statusInfo = finalizeState
		? STATUS_CONFIGS[finalizeState.status] || STATUS_CONFIGS.not_started
		: STATUS_CONFIGS.not_started;

	const isRunning = finalizeState?.status === 'running' || finalizeState?.status === 'pending';
	const isCompleted = finalizeState?.status === 'completed';
	const isFailed = finalizeState?.status === 'failed';
	const canTrigger =
		!finalizeState || finalizeState.status === 'not_started' || finalizeState.status === 'failed';

	if (!open) return null;

	const content = (
		<div
			className="finalize-modal-backdrop"
			role="dialog"
			aria-modal="true"
			aria-labelledby="finalize-modal-title"
			onClick={handleBackdropClick}
		>
			<div className="finalize-modal">
				{/* Header */}
				<div className="modal-header">
					<div className="header-left">
						<span className="task-id">{task.id}</span>
						<span
							className="status-badge"
							style={{ color: statusInfo.color, background: statusInfo.bg }}
						>
							{isRunning && <span className="status-dot" />}
							{statusInfo.label}
						</span>
					</div>
					<div className="header-right">
						{connectionStatus === 'connected' ? (
							<span className="connection-status connected" title="Live updates active">
								<span className="live-dot" />
								Live
							</span>
						) : connectionStatus === 'connecting' || connectionStatus === 'reconnecting' ? (
							<span className="connection-status connecting">
								<span className="spinner small" />
								Connecting...
							</span>
						) : (
							<span className="connection-status disconnected">Disconnected</span>
						)}
						<button
							className="header-btn close-btn"
							onClick={onClose}
							title="Close (Esc)"
						>
							<Icon name="close" size={18} />
						</button>
					</div>
				</div>

				{/* Title bar */}
				<div className="title-bar">
					<h2 id="finalize-modal-title" className="modal-title">
						Finalize Task
					</h2>
				</div>

				{/* Body */}
				<div className="modal-body">
					{loading ? (
						<div className="loading-state">
							<div className="spinner" />
							<span>Loading status...</span>
						</div>
					) : error ? (
						<div className="error-state">
							<div className="error-icon">!</div>
							<p>{error}</p>
							<button onClick={loadStatus}>Retry</button>
						</div>
					) : (
						<div className="finalize-content">
							{/* Progress section */}
							{(isRunning || isCompleted) && (
								<div className="progress-section">
									<div className="step-info">
										<span className="step-label">
											{finalizeState?.step || 'Processing'}
										</span>
										{finalizeState?.step_percent !== undefined && (
											<span className="step-percent">
												{finalizeState.step_percent}%
											</span>
										)}
									</div>
									{finalizeState?.progress && (
										<p className="progress-detail">{finalizeState.progress}</p>
									)}
									<div className="progress-bar">
										<div
											className="progress-fill"
											style={{ width: `${finalizeState?.step_percent || 0}%` }}
										/>
									</div>
								</div>
							)}

							{/* Result section */}
							{isCompleted && finalizeState?.result && (
								<div className="result-section success">
									<div className="result-header">
										<Icon name="check" size={20} />
										<span>Finalize Completed</span>
									</div>
									<div className="result-details">
										{finalizeState.result.commit_sha && (
											<div className="detail-row">
												<span className="detail-label">Commit</span>
												<span className="detail-value mono">
													{finalizeState.result.commit_sha.slice(0, 7)}
												</span>
											</div>
										)}
										<div className="detail-row">
											<span className="detail-label">Target Branch</span>
											<span className="detail-value">
												{finalizeState.result.target_branch}
											</span>
										</div>
										{finalizeState.result.files_changed > 0 && (
											<div className="detail-row">
												<span className="detail-label">Files Changed</span>
												<span className="detail-value">
													{finalizeState.result.files_changed}
												</span>
											</div>
										)}
										{finalizeState.result.conflicts_resolved > 0 && (
											<div className="detail-row">
												<span className="detail-label">Conflicts Resolved</span>
												<span className="detail-value">
													{finalizeState.result.conflicts_resolved}
												</span>
											</div>
										)}
										<div className="detail-row">
											<span className="detail-label">Tests</span>
											<span
												className={`detail-value ${finalizeState.result.tests_passed ? 'success' : 'danger'}`}
											>
												{finalizeState.result.tests_passed ? 'Passed' : 'Failed'}
											</span>
										</div>
										<div className="detail-row">
											<span className="detail-label">Risk Level</span>
											<span
												className={`detail-value risk-level ${finalizeState.result.risk_level}`}
											>
												{finalizeState.result.risk_level}
											</span>
										</div>
									</div>
								</div>
							)}

							{/* Failed section */}
							{isFailed && (
								<div className="result-section failed">
									<div className="result-header">
										<Icon name="close" size={20} />
										<span>Finalize Failed</span>
									</div>
									{finalizeState?.error && (
										<p className="error-message">{finalizeState.error}</p>
									)}
								</div>
							)}

							{/* Not started section */}
							{canTrigger && !isFailed && (
								<div className="not-started-section">
									<p className="info-text">
										The finalize phase will sync your branch with the target, resolve any
										conflicts, run tests, and prepare the code for merge.
									</p>
								</div>
							)}
						</div>
					)}
				</div>

				{/* Footer */}
				<div className="modal-footer">
					<button className="btn-secondary" onClick={onClose}>
						Close
					</button>
					{canTrigger && (
						<button
							className="btn-primary"
							onClick={handleTriggerFinalize}
							disabled={triggering}
						>
							{triggering ? (
								<>
									<span className="spinner small" />
									Starting...
								</>
							) : (
								<>
									<Icon name="git-branch" size={14} />
									{isFailed ? 'Retry Finalize' : 'Start Finalize'}
								</>
							)}
						</button>
					)}
				</div>
			</div>
		</div>
	);

	return createPortal(content, document.body);
}
