<!--
  LiveTranscriptModal - Real-time task transcript viewer

  Shows Claude's output as it generates via WebSocket streaming.
  Displays connection status, token counts, and transcript history.

  WebSocket events handled:
  - transcript: chunk (streaming) and response (complete)
  - state: task state updates
  - tokens: usage tracking (incremental)
  - phase/complete: triggers reload
-->
<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import type { Task, TranscriptFile, TaskState } from '$lib/types';
	import { getTranscripts, getProjectTranscripts, getTaskState, getProjectTaskState } from '$lib/api';
	import {
		getWebSocket,
		type WSEventType,
		type ConnectionStatus
	} from '$lib/websocket';
	import Transcript from '$lib/components/Transcript.svelte';
	import { currentProjectId } from '$lib/stores/project';

	interface Props {
		open: boolean;
		task: Task;
		onClose: () => void;
	}

	let { open, task, onClose }: Props = $props();

	let transcriptFiles = $state<TranscriptFile[]>([]);
	let taskState = $state<TaskState | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let connectionStatus = $state<ConnectionStatus>('disconnected');

	// Streaming content
	let streamingContent = $state('');
	let streamingPhase = $state('');
	let streamingIteration = $state(0);

	// Project ID subscription
	let projectId = $state<string | null>(null);
	let unsubscribeProject: (() => void) | null = null;

	// WebSocket event listener cleanup
	let unsubscribeEvent: (() => void) | null = null;
	let unsubscribeStatus: (() => void) | null = null;

	$effect(() => {
		// Subscribe to project store
		unsubscribeProject = currentProjectId.subscribe((value) => {
			projectId = value;
		});
		return () => unsubscribeProject?.();
	});

	onDestroy(() => {
		cleanup();
	});

	// React to open state changes - handles both initial mount and subsequent toggles
	$effect(() => {
		if (open && task) {
			loadData();
			setupWebSocket();
		} else {
			cleanup();
		}
	});

	function cleanup() {
		unsubscribeEvent?.();
		unsubscribeStatus?.();
		unsubscribeEvent = null;
		unsubscribeStatus = null;
		streamingContent = '';
		streamingPhase = '';
		streamingIteration = 0;
	}

	async function loadData() {
		loading = true;
		error = null;

		try {
			let files: TranscriptFile[];
			let state: TaskState | null;

			if (projectId) {
				try {
					[files, state] = await Promise.all([
						getProjectTranscripts(projectId, task.id).catch(() => []),
						getProjectTaskState(projectId, task.id).catch(() => null)
					]);
				} catch {
					// Fallback to CWD-based endpoints
					[files, state] = await Promise.all([
						getTranscripts(task.id).catch(() => []),
						getTaskState(task.id).catch(() => null)
					]);
				}
			} else {
				[files, state] = await Promise.all([
					getTranscripts(task.id).catch(() => []),
					getTaskState(task.id).catch(() => null)
				]);
			}

			transcriptFiles = files;
			taskState = state;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load transcript';
		} finally {
			loading = false;
		}
	}

	function setupWebSocket() {
		const ws = getWebSocket();

		// Listen to transcript events
		unsubscribeEvent = ws.on('all', (event) => {
			if (!('event' in event)) return;

			// Only handle events for our task
			if (event.task_id !== task.id) return;

			const eventType = event.event as WSEventType;
			const data = event.data;

			if (eventType === 'state') {
				taskState = data as TaskState;
			} else if (eventType === 'transcript') {
				const transcriptData = data as { type: string; content: string; phase: string; iteration: number };

				// Handle streaming chunks
				if (transcriptData.type === 'chunk') {
					if (
						transcriptData.phase !== streamingPhase ||
						transcriptData.iteration !== streamingIteration
					) {
						streamingPhase = transcriptData.phase;
						streamingIteration = transcriptData.iteration;
						streamingContent = '';
					}
					streamingContent += transcriptData.content;
				} else if (transcriptData.type === 'response') {
					// Full response received - reload transcript files
					streamingContent = '';
					loadData();
				}
			} else if (eventType === 'tokens') {
				const tokenData = data as { input_tokens: number; output_tokens: number; cache_read_input_tokens?: number; total_tokens: number };
				if (taskState) {
					taskState = {
						...taskState,
						tokens: {
							input_tokens: (taskState.tokens?.input_tokens || 0) + tokenData.input_tokens,
							output_tokens: (taskState.tokens?.output_tokens || 0) + tokenData.output_tokens,
							cache_read_input_tokens: (taskState.tokens?.cache_read_input_tokens || 0) + (tokenData.cache_read_input_tokens || 0),
							total_tokens: (taskState.tokens?.total_tokens || 0) + tokenData.total_tokens
						}
					};
				}
			} else if (eventType === 'complete' || eventType === 'phase') {
				loadData();
			}
		});

		// Listen to connection status
		unsubscribeStatus = ws.onStatusChange((status) => {
			connectionStatus = status;
		});
	}

	function handleBackdropClick(e: MouseEvent) {
		if (e.target === e.currentTarget) {
			onClose();
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			onClose();
		}
	}

	function openFullView() {
		goto(`/tasks/${task.id}?tab=transcript`);
		onClose();
	}

	// Derived status badge
	const statusInfo = $derived.by(() => {
		const status = task.status;
		const configs: Record<string, { label: string; color: string; bg: string }> = {
			running: { label: 'Running', color: 'var(--status-success)', bg: 'var(--status-success-bg)' },
			paused: { label: 'Paused', color: 'var(--status-warning)', bg: 'var(--status-warning-bg)' },
			blocked: { label: 'Blocked', color: 'var(--status-danger)', bg: 'var(--status-danger-bg)' },
			completed: { label: 'Completed', color: 'var(--accent-primary)', bg: 'var(--accent-subtle)' },
			failed: { label: 'Failed', color: 'var(--status-danger)', bg: 'var(--status-danger-bg)' }
		};
		return configs[status] || { label: status, color: 'var(--text-muted)', bg: 'var(--bg-tertiary)' };
	});

	// Token display
	const tokenDisplay = $derived.by(() => {
		if (!taskState?.tokens) return null;
		const { input_tokens, output_tokens, cache_read_input_tokens, total_tokens } = taskState.tokens;
		return {
			input: (input_tokens || 0).toLocaleString(),
			output: (output_tokens || 0).toLocaleString(),
			cached: cache_read_input_tokens ? cache_read_input_tokens.toLocaleString() : null,
			total: (total_tokens || 0).toLocaleString()
		};
	});
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="modal-backdrop"
		role="dialog"
		aria-modal="true"
		aria-labelledby="transcript-modal-title"
		tabindex="-1"
		onclick={handleBackdropClick}
		onkeydown={handleKeydown}
	>
		<div class="modal-content">
			<!-- Header -->
			<div class="modal-header">
				<div class="header-left">
					<span class="task-id">{task.id}</span>
					<span class="status-badge" style:color={statusInfo.color} style:background={statusInfo.bg}>
						{#if task.status === 'running'}
							<span class="status-dot"></span>
						{/if}
						{statusInfo.label}
					</span>
					{#if task.current_phase}
						<span class="phase-badge">{task.current_phase}</span>
					{/if}
				</div>
				<div class="header-right">
					{#if connectionStatus === 'connected'}
						<span class="connection-status connected" title="Live updates active">
							<span class="live-dot"></span>
							Live
						</span>
					{:else if connectionStatus === 'connecting' || connectionStatus === 'reconnecting'}
						<span class="connection-status connecting">
							<span class="spinner"></span>
							Connecting...
						</span>
					{:else}
						<span class="connection-status disconnected">
							Disconnected
						</span>
					{/if}
					<button class="header-btn" onclick={openFullView} title="Open full view">
						<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
							<path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
							<polyline points="15 3 21 3 21 9" />
							<line x1="10" y1="14" x2="21" y2="3" />
						</svg>
					</button>
					<button class="header-btn close-btn" onclick={onClose} title="Close (Esc)">
						<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
							<line x1="18" y1="6" x2="6" y2="18" />
							<line x1="6" y1="6" x2="18" y2="18" />
						</svg>
					</button>
				</div>
			</div>

			<!-- Title bar -->
			<div class="title-bar">
				<h2 id="transcript-modal-title" class="modal-title">{task.title}</h2>
				{#if tokenDisplay}
					<div class="token-summary">
						<span class="token-item">
							<span class="token-value">{tokenDisplay.input}</span>
							<span class="token-label">in</span>
						</span>
						<span class="token-divider">/</span>
						<span class="token-item">
							<span class="token-value">{tokenDisplay.output}</span>
							<span class="token-label">out</span>
						</span>
						{#if tokenDisplay.cached}
							<span class="token-divider">/</span>
							<span class="token-item cached">
								<span class="token-value">{tokenDisplay.cached}</span>
								<span class="token-label">cached</span>
							</span>
						{/if}
					</div>
				{/if}
			</div>

			<!-- Body -->
			<div class="modal-body">
				{#if loading}
					<div class="loading-state">
						<div class="spinner"></div>
						<span>Loading transcript...</span>
					</div>
				{:else if error}
					<div class="error-state">
						<div class="error-icon">!</div>
						<p>{error}</p>
						<button onclick={loadData}>Retry</button>
					</div>
				{:else}
					<Transcript
						files={transcriptFiles}
						taskId={task.id}
						{streamingContent}
						autoScroll={true}
					/>
				{/if}
			</div>
		</div>
	</div>
{/if}

<style>
	.modal-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.7);
		backdrop-filter: blur(4px);
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding: var(--space-8) var(--space-4);
		z-index: 1000;
		animation: fade-in var(--duration-normal) var(--ease-out);
	}

	@keyframes fade-in {
		from { opacity: 0; }
		to { opacity: 1; }
	}

	.modal-content {
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-xl);
		box-shadow: var(--shadow-2xl);
		width: 100%;
		max-width: 1000px;
		max-height: calc(100vh - var(--space-16));
		display: flex;
		flex-direction: column;
		animation: modal-slide-in var(--duration-normal) var(--ease-out);
	}

	@keyframes modal-slide-in {
		from {
			opacity: 0;
			transform: translateY(-20px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	/* Header */
	.modal-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--space-3) var(--space-4);
		border-bottom: 1px solid var(--border-subtle);
		flex-shrink: 0;
	}

	.header-left {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.header-right {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.task-id {
		font-family: var(--font-mono);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-muted);
	}

	.status-badge {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		padding: var(--space-0-5) var(--space-2);
		font-size: var(--text-xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		border-radius: var(--radius-sm);
	}

	.status-dot {
		width: 6px;
		height: 6px;
		background: currentColor;
		border-radius: 50%;
		animation: pulse 1.5s ease-in-out infinite;
	}

	@keyframes pulse {
		0%, 100% { opacity: 1; }
		50% { opacity: 0.4; }
	}

	.phase-badge {
		padding: var(--space-0-5) var(--space-2);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		text-transform: capitalize;
		background: var(--accent-subtle);
		color: var(--accent-primary);
		border-radius: var(--radius-sm);
	}

	.connection-status {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		font-size: var(--text-xs);
		padding: var(--space-1) var(--space-2);
		border-radius: var(--radius-sm);
	}

	.connection-status.connected {
		background: var(--status-success-bg);
		color: var(--status-success);
	}

	.connection-status.connecting {
		background: var(--status-warning-bg);
		color: var(--status-warning);
	}

	.connection-status.disconnected {
		background: var(--bg-tertiary);
		color: var(--text-muted);
	}

	.live-dot {
		width: 6px;
		height: 6px;
		background: currentColor;
		border-radius: 50%;
		animation: pulse 1.5s ease-in-out infinite;
	}

	.header-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		background: transparent;
		border: none;
		border-radius: var(--radius-md);
		color: var(--text-muted);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.header-btn:hover {
		background: var(--bg-tertiary);
		color: var(--text-primary);
	}

	.header-btn:focus-visible {
		outline: none;
		box-shadow: 0 0 0 2px var(--accent-glow);
	}

	/* Title bar */
	.title-bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--space-2) var(--space-4);
		border-bottom: 1px solid var(--border-subtle);
		flex-shrink: 0;
	}

	.modal-title {
		font-size: var(--text-base);
		font-weight: var(--font-medium);
		color: var(--text-primary);
		margin: 0;
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.token-summary {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		font-family: var(--font-mono);
		font-size: var(--text-xs);
		color: var(--text-muted);
		flex-shrink: 0;
	}

	.token-item {
		display: flex;
		align-items: baseline;
		gap: var(--space-0-5);
	}

	.token-item.cached .token-value {
		color: var(--status-success);
	}

	.token-value {
		font-weight: var(--font-medium);
		color: var(--text-secondary);
	}

	.token-label {
		font-size: var(--text-2xs);
		color: var(--text-muted);
	}

	.token-divider {
		color: var(--border-default);
	}

	/* Body */
	.modal-body {
		flex: 1;
		overflow: hidden;
		display: flex;
		flex-direction: column;
	}

	/* Override Transcript container height */
	.modal-body :global(.transcript-container) {
		max-height: none;
		height: 100%;
		border: none;
		border-radius: 0;
	}

	/* Loading/Error States */
	.loading-state,
	.error-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: var(--space-4);
		padding: var(--space-16);
		text-align: center;
	}

	.spinner {
		width: 24px;
		height: 24px;
		border: 3px solid var(--border-default);
		border-top-color: var(--accent-primary);
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}

	.loading-state span {
		font-size: var(--text-sm);
		color: var(--text-muted);
	}

	.error-icon {
		width: 48px;
		height: 48px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--status-danger-bg);
		border-radius: 50%;
		font-size: var(--text-xl);
		font-weight: var(--font-bold);
		color: var(--status-danger);
	}

	.error-state p {
		font-size: var(--text-sm);
		color: var(--text-secondary);
	}

	.error-state button {
		padding: var(--space-2) var(--space-4);
		background: var(--accent-primary);
		color: white;
		border: none;
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		cursor: pointer;
		transition: background var(--duration-fast) var(--ease-out);
	}

	.error-state button:hover {
		background: var(--accent-primary-hover);
	}
</style>
