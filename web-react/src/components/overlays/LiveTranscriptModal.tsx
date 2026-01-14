/**
 * LiveTranscriptModal - Real-time task transcript viewer
 *
 * Shows Claude's output as it generates via WebSocket streaming.
 * Displays connection status, token counts, and transcript history.
 *
 * WebSocket events handled:
 * - transcript: chunk (streaming) and response (complete)
 * - state: task state updates
 * - tokens: usage tracking (incremental)
 * - phase/complete: triggers reload
 */

import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { createPortal } from 'react-dom';
import type { Task, TaskState, ConnectionStatus, WSEventType, WSEvent } from '@/lib/types';
import { getTaskState } from '@/lib/api';
import { useWebSocket } from '@/hooks/useWebSocket';
import { TranscriptTab } from '@/components/task-detail/TranscriptTab';
import { Icon } from '@/components/ui/Icon';
import './LiveTranscriptModal.css';

interface LiveTranscriptModalProps {
	open: boolean;
	task: Task;
	onClose: () => void;
}

const STATUS_CONFIGS: Record<string, { label: string; color: string; bg: string }> = {
	running: { label: 'Running', color: 'var(--status-success)', bg: 'var(--status-success-bg)' },
	paused: { label: 'Paused', color: 'var(--status-warning)', bg: 'var(--status-warning-bg)' },
	blocked: { label: 'Blocked', color: 'var(--status-danger)', bg: 'var(--status-danger-bg)' },
	finalizing: { label: 'Finalizing', color: 'var(--status-info)', bg: 'var(--status-info-bg)' },
	completed: { label: 'Completed', color: 'var(--accent-primary)', bg: 'var(--accent-subtle)' },
	finished: { label: 'Finished', color: 'var(--status-success)', bg: 'var(--status-success-bg)' },
	failed: { label: 'Failed', color: 'var(--status-danger)', bg: 'var(--status-danger-bg)' },
};

export function LiveTranscriptModal({ open, task, onClose }: LiveTranscriptModalProps) {
	const navigate = useNavigate();
	const ws = useWebSocket();

	// Data state
	const [taskState, setTaskState] = useState<TaskState | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('disconnected');

	// Streaming state
	const [streamingContent, setStreamingContent] = useState('');
	const [streamingPhase, setStreamingPhase] = useState('');
	const [streamingIteration, setStreamingIteration] = useState(0);

	// Load data
	const loadData = useCallback(async () => {
		setLoading(true);
		setError(null);

		try {
			const state = await getTaskState(task.id).catch(() => null);
			setTaskState(state);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load transcript');
		} finally {
			setLoading(false);
		}
	}, [task.id]);

	// Setup WebSocket
	useEffect(() => {
		if (!open) return;

		loadData();

		// Listen to transcript events
		const unsubscribeEvent = ws.on('all', (event) => {
			if (!('event' in event)) return;
			const wsEvent = event as WSEvent;

			// Only handle events for our task
			if (wsEvent.task_id !== task.id) return;

			const eventType = wsEvent.event as WSEventType;
			const data = wsEvent.data;

			if (eventType === 'state') {
				setTaskState(data as TaskState);
			} else if (eventType === 'transcript') {
				const transcriptData = data as {
					type: string;
					content: string;
					phase: string;
					iteration: number;
				};

				// Handle streaming chunks
				if (transcriptData.type === 'chunk') {
					if (
						transcriptData.phase !== streamingPhase ||
						transcriptData.iteration !== streamingIteration
					) {
						setStreamingPhase(transcriptData.phase);
						setStreamingIteration(transcriptData.iteration);
						setStreamingContent('');
					}
					setStreamingContent((prev) => prev + transcriptData.content);
				} else if (transcriptData.type === 'response') {
					// Full response received - reload transcript files
					setStreamingContent('');
					loadData();
				}
			} else if (eventType === 'tokens') {
				const tokenData = data as {
					input_tokens: number;
					output_tokens: number;
					cache_read_input_tokens?: number;
					total_tokens: number;
				};
				setTaskState((prev) => {
					if (!prev) return prev;
					return {
						...prev,
						tokens: {
							input_tokens: (prev.tokens?.input_tokens || 0) + tokenData.input_tokens,
							output_tokens: (prev.tokens?.output_tokens || 0) + tokenData.output_tokens,
							cache_read_input_tokens:
								(prev.tokens?.cache_read_input_tokens || 0) +
								(tokenData.cache_read_input_tokens || 0),
							total_tokens: (prev.tokens?.total_tokens || 0) + tokenData.total_tokens,
						},
					};
				});
			} else if (eventType === 'complete' || eventType === 'phase') {
				loadData();
			}
		});

		// Track connection status from ws hook
		setConnectionStatus(ws.status);

		return () => {
			unsubscribeEvent();
			setStreamingContent('');
			setStreamingPhase('');
			setStreamingIteration(0);
		};
	}, [open, task.id, loadData, streamingPhase, streamingIteration, ws]);

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

	const handleBackdropClick = (e: React.MouseEvent) => {
		if (e.target === e.currentTarget) {
			onClose();
		}
	};

	const openFullView = () => {
		navigate(`/tasks/${task.id}?tab=transcript`);
		onClose();
	};

	// Get status info
	const statusInfo = STATUS_CONFIGS[task.status] || {
		label: task.status,
		color: 'var(--text-muted)',
		bg: 'var(--bg-tertiary)',
	};

	// Format tokens
	const tokenDisplay = taskState?.tokens
		? {
				input: (taskState.tokens.input_tokens || 0).toLocaleString(),
				output: (taskState.tokens.output_tokens || 0).toLocaleString(),
				cached: taskState.tokens.cache_read_input_tokens
					? taskState.tokens.cache_read_input_tokens.toLocaleString()
					: null,
				total: (taskState.tokens.total_tokens || 0).toLocaleString(),
		  }
		: null;

	if (!open) return null;

	const content = (
		<div
			className="live-transcript-backdrop"
			role="dialog"
			aria-modal="true"
			aria-labelledby="transcript-modal-title"
			onClick={handleBackdropClick}
		>
			<div className="live-transcript-modal">
				{/* Header */}
				<div className="modal-header">
					<div className="header-left">
						<span className="task-id">{task.id}</span>
						<span
							className="status-badge"
							style={{ color: statusInfo.color, background: statusInfo.bg }}
						>
							{task.status === 'running' && <span className="status-dot" />}
							{statusInfo.label}
						</span>
						{task.current_phase && <span className="phase-badge">{task.current_phase}</span>}
					</div>
					<div className="header-right">
						{connectionStatus === 'connected' ? (
							<span className="connection-status connected" title="Live updates active">
								<span className="live-dot" />
								Live
							</span>
						) : connectionStatus === 'connecting' || connectionStatus === 'reconnecting' ? (
							<span className="connection-status connecting">
								<span className="spinner" />
								Connecting...
							</span>
						) : (
							<span className="connection-status disconnected">Disconnected</span>
						)}
						<button
							className="header-btn"
							onClick={openFullView}
							title="Open full view"
						>
							<Icon name="export" size={14} />
						</button>
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
					<h2 id="transcript-modal-title" className="modal-title">
						{task.title}
					</h2>
					{tokenDisplay && (
						<div className="token-summary">
							<span className="token-item">
								<span className="token-value">{tokenDisplay.input}</span>
								<span className="token-label">in</span>
							</span>
							<span className="token-divider">/</span>
							<span className="token-item">
								<span className="token-value">{tokenDisplay.output}</span>
								<span className="token-label">out</span>
							</span>
							{tokenDisplay.cached && (
								<>
									<span className="token-divider">/</span>
									<span className="token-item cached">
										<span className="token-value">{tokenDisplay.cached}</span>
										<span className="token-label">cached</span>
									</span>
								</>
							)}
						</div>
					)}
				</div>

				{/* Body */}
				<div className="modal-body">
					{loading ? (
						<div className="loading-state">
							<div className="spinner" />
							<span>Loading transcript...</span>
						</div>
					) : error ? (
						<div className="error-state">
							<div className="error-icon">!</div>
							<p>{error}</p>
							<button onClick={loadData}>Retry</button>
						</div>
					) : (
						<TranscriptTab
							taskId={task.id}
							streamingContent={streamingContent}
							autoScroll={true}
						/>
					)}
				</div>
			</div>
		</div>
	);

	return createPortal(content, document.body);
}
