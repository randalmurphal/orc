<!--
  FinalizeModal - Real-time finalize progress viewer

  Shows finalize operation progress via WebSocket streaming.
  Displays step progression, progress bar, and result information.

  WebSocket events handled:
  - finalize: progress updates with step/percent/result/error
-->
<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import type { Task } from '$lib/types';
	import { triggerFinalize, getFinalizeStatus, type FinalizeState, type FinalizeResult } from '$lib/api';
	import { getWebSocket, type ConnectionStatus } from '$lib/websocket';

	interface Props {
		open: boolean;
		task: Task;
		onClose: () => void;
	}

	let { open, task, onClose }: Props = $props();

	let finalizeState = $state<FinalizeState | null>(null);
	let loading = $state(false);
	let triggering = $state(false);
	let error = $state<string | null>(null);
	let connectionStatus = $state<ConnectionStatus>('disconnected');

	// WebSocket event listener cleanup
	let unsubscribeEvent: (() => void) | null = null;
	let unsubscribeStatus: (() => void) | null = null;

	onDestroy(() => {
		cleanup();
	});

	// React to open state changes
	$effect(() => {
		if (open && task) {
			loadStatus();
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
	}

	async function loadStatus() {
		loading = true;
		error = null;

		try {
			finalizeState = await getFinalizeStatus(task.id);
		} catch (e) {
			// Not started is fine
			finalizeState = {
				task_id: task.id,
				status: 'not_started'
			};
		} finally {
			loading = false;
		}
	}

	function setupWebSocket() {
		const ws = getWebSocket();

		// Listen to finalize events
		unsubscribeEvent = ws.on('all', (event) => {
			if (!('event' in event)) return;

			// Only handle events for our task
			if (event.task_id !== task.id) return;

			const eventType = event.event;
			const data = event.data as Record<string, unknown>;

			if (eventType === 'finalize') {
				// Update finalize state from WebSocket
				finalizeState = {
					task_id: data.task_id as string,
					status: data.status as FinalizeState['status'],
					step: data.step as string | undefined,
					progress: data.progress as string | undefined,
					step_percent: data.step_percent as number | undefined,
					updated_at: data.updated_at as string | undefined,
					result: data.result as FinalizeResult | undefined,
					error: data.error as string | undefined
				};
			}
		});

		// Listen to connection status
		unsubscribeStatus = ws.onStatusChange((status) => {
			connectionStatus = status;
		});
	}

	async function handleTriggerFinalize() {
		triggering = true;
		error = null;

		try {
			await triggerFinalize(task.id);
			// Status will update via WebSocket
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to trigger finalize';
		} finally {
			triggering = false;
		}
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

	// Derived status info
	const statusInfo = $derived.by(() => {
		if (!finalizeState) return { label: 'Unknown', color: 'var(--text-muted)', bg: 'var(--bg-tertiary)' };
		const configs: Record<string, { label: string; color: string; bg: string }> = {
			not_started: { label: 'Not Started', color: 'var(--text-muted)', bg: 'var(--bg-tertiary)' },
			pending: { label: 'Pending', color: 'var(--status-warning)', bg: 'var(--status-warning-bg)' },
			running: { label: 'Running', color: 'var(--status-info)', bg: 'var(--status-info-bg)' },
			completed: { label: 'Completed', color: 'var(--status-success)', bg: 'var(--status-success-bg)' },
			failed: { label: 'Failed', color: 'var(--status-danger)', bg: 'var(--status-danger-bg)' }
		};
		return configs[finalizeState.status] || configs.not_started;
	});

	const isRunning = $derived(finalizeState?.status === 'running' || finalizeState?.status === 'pending');
	const isCompleted = $derived(finalizeState?.status === 'completed');
	const isFailed = $derived(finalizeState?.status === 'failed');
	const canTrigger = $derived(!finalizeState || finalizeState.status === 'not_started' || finalizeState.status === 'failed');
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="modal-backdrop"
		role="dialog"
		aria-modal="true"
		aria-labelledby="finalize-modal-title"
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
						{#if isRunning}
							<span class="status-dot"></span>
						{/if}
						{statusInfo.label}
					</span>
				</div>
				<div class="header-right">
					{#if connectionStatus === 'connected'}
						<span class="connection-status connected" title="Live updates active">
							<span class="live-dot"></span>
							Live
						</span>
					{:else if connectionStatus === 'connecting' || connectionStatus === 'reconnecting'}
						<span class="connection-status connecting">
							<span class="spinner small"></span>
							Connecting...
						</span>
					{:else}
						<span class="connection-status disconnected">
							Disconnected
						</span>
					{/if}
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
				<h2 id="finalize-modal-title" class="modal-title">Finalize Task</h2>
			</div>

			<!-- Body -->
			<div class="modal-body">
				{#if loading}
					<div class="loading-state">
						<div class="spinner"></div>
						<span>Loading status...</span>
					</div>
				{:else if error}
					<div class="error-state">
						<div class="error-icon">!</div>
						<p>{error}</p>
						<button onclick={loadStatus}>Retry</button>
					</div>
				{:else}
					<div class="finalize-content">
						<!-- Progress section -->
						{#if isRunning || isCompleted}
							<div class="progress-section">
								<div class="step-info">
									<span class="step-label">{finalizeState?.step || 'Processing'}</span>
									{#if finalizeState?.step_percent !== undefined}
										<span class="step-percent">{finalizeState.step_percent}%</span>
									{/if}
								</div>
								{#if finalizeState?.progress}
									<p class="progress-detail">{finalizeState.progress}</p>
								{/if}
								<div class="progress-bar">
									<div class="progress-fill" style:width="{finalizeState?.step_percent || 0}%"></div>
								</div>
							</div>
						{/if}

						<!-- Result section -->
						{#if isCompleted && finalizeState?.result}
							<div class="result-section success">
								<div class="result-header">
									<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
										<polyline points="20 6 9 17 4 12"/>
									</svg>
									<span>Finalize Completed</span>
								</div>
								<div class="result-details">
									{#if finalizeState.result.commit_sha}
										<div class="detail-row">
											<span class="detail-label">Commit</span>
											<span class="detail-value mono">{finalizeState.result.commit_sha.slice(0, 7)}</span>
										</div>
									{/if}
									<div class="detail-row">
										<span class="detail-label">Target Branch</span>
										<span class="detail-value">{finalizeState.result.target_branch}</span>
									</div>
									{#if finalizeState.result.files_changed > 0}
										<div class="detail-row">
											<span class="detail-label">Files Changed</span>
											<span class="detail-value">{finalizeState.result.files_changed}</span>
										</div>
									{/if}
									{#if finalizeState.result.conflicts_resolved > 0}
										<div class="detail-row">
											<span class="detail-label">Conflicts Resolved</span>
											<span class="detail-value">{finalizeState.result.conflicts_resolved}</span>
										</div>
									{/if}
									<div class="detail-row">
										<span class="detail-label">Tests</span>
										<span class="detail-value" class:success={finalizeState.result.tests_passed} class:danger={!finalizeState.result.tests_passed}>
											{finalizeState.result.tests_passed ? 'Passed' : 'Failed'}
										</span>
									</div>
									<div class="detail-row">
										<span class="detail-label">Risk Level</span>
										<span class="detail-value risk-level" class:low={finalizeState.result.risk_level === 'low'} class:medium={finalizeState.result.risk_level === 'medium'} class:high={finalizeState.result.risk_level === 'high'}>
											{finalizeState.result.risk_level}
										</span>
									</div>
								</div>
							</div>
						{/if}

						<!-- Failed section -->
						{#if isFailed}
							<div class="result-section failed">
								<div class="result-header">
									<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
										<circle cx="12" cy="12" r="10"/>
										<line x1="15" y1="9" x2="9" y2="15"/>
										<line x1="9" y1="9" x2="15" y2="15"/>
									</svg>
									<span>Finalize Failed</span>
								</div>
								{#if finalizeState?.error}
									<p class="error-message">{finalizeState.error}</p>
								{/if}
							</div>
						{/if}

						<!-- Not started section -->
						{#if canTrigger && !isFailed}
							<div class="not-started-section">
								<p class="info-text">
									The finalize phase will sync your branch with the target, resolve any conflicts,
									run tests, and prepare the code for merge.
								</p>
							</div>
						{/if}
					</div>
				{/if}
			</div>

			<!-- Footer -->
			<div class="modal-footer">
				<button class="btn-secondary" onclick={onClose}>Close</button>
				{#if canTrigger}
					<button class="btn-primary" onclick={handleTriggerFinalize} disabled={triggering}>
						{#if triggering}
							<div class="spinner small"></div>
							Starting...
						{:else}
							<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
								<circle cx="18" cy="18" r="3"/>
								<circle cx="6" cy="6" r="3"/>
								<path d="M6 21V9a9 9 0 0 0 9 9"/>
							</svg>
							{isFailed ? 'Retry Finalize' : 'Start Finalize'}
						{/if}
					</button>
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
		max-width: 500px;
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

	/* Title bar */
	.title-bar {
		display: flex;
		align-items: center;
		padding: var(--space-2) var(--space-4);
		border-bottom: 1px solid var(--border-subtle);
		flex-shrink: 0;
	}

	.modal-title {
		font-size: var(--text-base);
		font-weight: var(--font-medium);
		color: var(--text-primary);
		margin: 0;
	}

	/* Body */
	.modal-body {
		flex: 1;
		padding: var(--space-4);
		overflow-y: auto;
	}

	.finalize-content {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	/* Progress section */
	.progress-section {
		background: var(--bg-tertiary);
		border-radius: var(--radius-md);
		padding: var(--space-4);
	}

	.step-info {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: var(--space-2);
	}

	.step-label {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
	}

	.step-percent {
		font-family: var(--font-mono);
		font-size: var(--text-sm);
		color: var(--status-info);
	}

	.progress-detail {
		font-size: var(--text-sm);
		color: var(--text-secondary);
		margin: 0 0 var(--space-3);
	}

	.progress-bar {
		height: 6px;
		background: var(--bg-primary);
		border-radius: var(--radius-full);
		overflow: hidden;
	}

	.progress-fill {
		height: 100%;
		background: var(--status-info);
		border-radius: var(--radius-full);
		transition: width 0.3s ease-out;
	}

	/* Result section */
	.result-section {
		border-radius: var(--radius-md);
		padding: var(--space-4);
	}

	.result-section.success {
		background: var(--status-success-bg);
		border: 1px solid color-mix(in srgb, var(--status-success) 30%, transparent);
	}

	.result-section.failed {
		background: var(--status-danger-bg);
		border: 1px solid color-mix(in srgb, var(--status-danger) 30%, transparent);
	}

	.result-header {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		margin-bottom: var(--space-3);
		font-size: var(--text-sm);
		font-weight: var(--font-semibold);
	}

	.result-section.success .result-header {
		color: var(--status-success);
	}

	.result-section.failed .result-header {
		color: var(--status-danger);
	}

	.result-details {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.detail-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		font-size: var(--text-sm);
	}

	.detail-label {
		color: var(--text-muted);
	}

	.detail-value {
		color: var(--text-primary);
		font-weight: var(--font-medium);
	}

	.detail-value.mono {
		font-family: var(--font-mono);
	}

	.detail-value.success {
		color: var(--status-success);
	}

	.detail-value.danger {
		color: var(--status-danger);
	}

	.risk-level {
		text-transform: capitalize;
	}

	.risk-level.low {
		color: var(--status-success);
	}

	.risk-level.medium {
		color: var(--status-warning);
	}

	.risk-level.high {
		color: var(--status-danger);
	}

	.error-message {
		font-size: var(--text-sm);
		color: var(--status-danger);
		margin: 0;
		padding: var(--space-2);
		background: var(--bg-primary);
		border-radius: var(--radius-sm);
		font-family: var(--font-mono);
	}

	/* Not started section */
	.not-started-section {
		text-align: center;
		padding: var(--space-4);
	}

	.info-text {
		font-size: var(--text-sm);
		color: var(--text-secondary);
		line-height: 1.6;
		margin: 0;
	}

	/* Loading/Error States */
	.loading-state,
	.error-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: var(--space-4);
		padding: var(--space-8);
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

	.spinner.small {
		width: 14px;
		height: 14px;
		border-width: 2px;
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
		margin: 0;
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

	/* Footer */
	.modal-footer {
		display: flex;
		align-items: center;
		justify-content: flex-end;
		gap: var(--space-2);
		padding: var(--space-4);
		border-top: 1px solid var(--border-subtle);
		flex-shrink: 0;
	}

	.btn-secondary {
		padding: var(--space-2) var(--space-4);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-primary);
		cursor: pointer;
		transition: background var(--duration-fast) var(--ease-out);
	}

	.btn-secondary:hover {
		background: var(--bg-secondary);
	}

	.btn-primary {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-4);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		background: var(--status-info);
		border: none;
		border-radius: var(--radius-md);
		color: white;
		cursor: pointer;
		transition: filter var(--duration-fast) var(--ease-out);
	}

	.btn-primary:hover:not(:disabled) {
		filter: brightness(1.1);
	}

	.btn-primary:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}
</style>
