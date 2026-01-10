<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { get } from 'svelte/store';
	import {
		getProjectTask,
		getProjectTaskState,
		getProjectTaskPlan,
		runProjectTask,
		pauseProjectTask,
		resumeProjectTask,
		deleteProjectTask,
		getProjectTranscripts
	} from '$lib/api';
	import {
		subscribeToTaskWS,
		type ConnectionStatus,
		type WSEventType,
		getWebSocket
	} from '$lib/websocket';
	import type { Task, TaskState, Plan, TranscriptLine } from '$lib/types';
	import Timeline from '$lib/components/Timeline.svelte';
	import Transcript from '$lib/components/Transcript.svelte';
	import StatusIndicator from '$lib/components/ui/StatusIndicator.svelte';
	import { currentProjectId } from '$lib/stores/project';

	let task = $state<Task | null>(null);
	let taskState = $state<TaskState | null>(null);
	let plan = $state<Plan | null>(null);
	let transcript = $state<TranscriptLine[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let connectionStatus = $state<ConnectionStatus>('disconnected');
	let unsubscribe: (() => void) | null = null;

	const taskId = $derived($page.params.id ?? '');
	const projectId = $derived(get(currentProjectId));

	onMount(async () => {
		if (!taskId) return;
		if (!projectId) {
			error = 'No project selected. Please select a project first.';
			loading = false;
			return;
		}
		await loadTaskData();
		setupStreaming();
	});

	onDestroy(() => {
		if (unsubscribe) unsubscribe();
	});

	async function loadTaskData() {
		loading = true;
		error = null;
		const pid = get(currentProjectId);
		if (!pid) {
			error = 'No project selected. Please select a project first.';
			loading = false;
			return;
		}
		try {
			const [t, s, p, transcriptFiles] = await Promise.all([
				getProjectTask(pid, taskId),
				getProjectTaskState(pid, taskId).catch(() => null),
				getProjectTaskPlan(pid, taskId).catch(() => null),
				getProjectTranscripts(pid, taskId).catch(() => [])
			]);
			task = t;
			taskState = s;
			plan = p;

			// Parse transcript files into TranscriptLine format
			transcript = parseTranscriptFiles(transcriptFiles);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load task';
		} finally {
			loading = false;
		}
	}

	function parseTranscriptFiles(
		files: { filename: string; content: string; created_at: string }[]
	): TranscriptLine[] {
		const lines: TranscriptLine[] = [];

		for (const file of files) {
			// Parse markdown format: # phase - Iteration N, ## Prompt, ## Response
			const parts = file.content.split(/^## /m);

			for (const part of parts) {
				if (!part.trim()) continue;

				if (part.startsWith('Prompt\n')) {
					lines.push({
						type: 'prompt',
						content: part.replace('Prompt\n', '').split('\n## ')[0].trim(),
						timestamp: file.created_at
					});
				} else if (part.startsWith('Response\n')) {
					const responseContent = part.replace('Response\n', '').split('\n---')[0].trim();
					lines.push({
						type: 'response',
						content: responseContent,
						timestamp: file.created_at
					});
				}
			}
		}

		return lines;
	}

	function setupStreaming() {
		unsubscribe = subscribeToTaskWS(
			taskId,
			(eventType: WSEventType, data: unknown) => {
				if (eventType === 'state') {
					taskState = data as TaskState;
				} else if (eventType === 'transcript') {
					const transcriptData = data as TranscriptLine;
					transcript = [...transcript, transcriptData];
				} else if (eventType === 'phase') {
					// Reload task data on phase change
					loadTaskData();
				} else if (eventType === 'complete') {
					// Reload task data on completion
					loadTaskData();
				}
			},
			(status: ConnectionStatus) => {
				connectionStatus = status;
			}
		);
	}

	function handleCancel() {
		const ws = getWebSocket();
		ws.cancel(taskId);
	}

	async function handleRun() {
		const pid = get(currentProjectId);
		if (!pid) {
			error = 'No project selected';
			return;
		}
		try {
			await runProjectTask(pid, taskId);
			await loadTaskData();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to run task';
		}
	}

	async function handlePause() {
		const pid = get(currentProjectId);
		if (!pid) {
			error = 'No project selected';
			return;
		}
		try {
			await pauseProjectTask(pid, taskId);
			await loadTaskData();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to pause task';
		}
	}

	async function handleResume() {
		const pid = get(currentProjectId);
		if (!pid) {
			error = 'No project selected';
			return;
		}
		try {
			await resumeProjectTask(pid, taskId);
			await loadTaskData();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to resume task';
		}
	}

	async function handleDelete() {
		if (!task || !confirm(`Delete task ${task.id}?`)) return;

		const pid = get(currentProjectId);
		if (!pid) {
			error = 'No project selected';
			return;
		}
		try {
			await deleteProjectTask(pid, taskId);
			goto('/');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete task';
		}
	}

	const weightConfig: Record<string, { color: string; bg: string }> = {
		trivial: { color: 'var(--weight-trivial)', bg: 'rgba(107, 114, 128, 0.15)' },
		small: { color: 'var(--weight-small)', bg: 'var(--status-success-bg)' },
		medium: { color: 'var(--weight-medium)', bg: 'var(--status-info-bg)' },
		large: { color: 'var(--weight-large)', bg: 'var(--status-warning-bg)' },
		greenfield: { color: 'var(--weight-greenfield)', bg: 'var(--accent-subtle)' }
	};

	const weight = $derived(weightConfig[task?.weight || 'small'] || weightConfig.small);
</script>

<svelte:head>
	<title>{task?.title || 'Task'} - orc</title>
</svelte:head>

{#if loading}
	<div class="loading-state">
		<div class="spinner"></div>
		<span>Loading task...</span>
	</div>
{:else if error}
	<div class="error-state">
		<div class="error-icon">!</div>
		<p>{error}</p>
		<button onclick={loadTaskData}>Retry</button>
	</div>
{:else if task}
	<!-- Connection Status Banner -->
	{#if connectionStatus !== 'connected' && task.status === 'running'}
		<div
			class="connection-banner"
			class:reconnecting={connectionStatus === 'reconnecting'}
			class:connecting={connectionStatus === 'connecting'}
		>
			<div class="connection-content">
				{#if connectionStatus === 'connecting'}
					<div class="connection-spinner"></div>
					<span>Connecting to live updates...</span>
				{:else if connectionStatus === 'reconnecting'}
					<div class="connection-spinner"></div>
					<span>Reconnecting...</span>
				{:else}
					<svg
						xmlns="http://www.w3.org/2000/svg"
						width="16"
						height="16"
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						stroke-linejoin="round"
					>
						<line x1="1" y1="1" x2="23" y2="23" />
						<path
							d="M16.72 11.06A10.94 10.94 0 0 1 19 12.55"
						/>
						<path d="M5 12.55a10.94 10.94 0 0 1 5.17-2.39" />
						<path d="M10.71 5.05A16 16 0 0 1 22.58 9" />
						<path d="M1.42 9a15.91 15.91 0 0 1 4.7-2.88" />
						<path
							d="M8.53 16.11a6 6 0 0 1 6.95 0"
						/>
						<line x1="12" y1="20" x2="12.01" y2="20" />
					</svg>
					<span>Disconnected from live updates</span>
					<button onclick={setupStreaming}>Reconnect</button>
				{/if}
			</div>
		</div>
	{/if}

	<div class="task-detail">
		<!-- Task Header -->
		<header class="task-header">
			<div class="task-info">
				<div class="task-meta">
					<span class="task-id">{task.id}</span>
					<span
						class="weight-badge"
						style:color={weight.color}
						style:background={weight.bg}
					>
						{task.weight}
					</span>
					<div class="status-badge">
						<StatusIndicator status={task.status} size="sm" showLabel />
					</div>
				</div>
				<h1 class="task-title">{task.title}</h1>
				{#if task.description}
					<p class="task-description">{task.description}</p>
				{/if}
			</div>

			<div class="task-actions">
				{#if task.status === 'running'}
					<button onclick={handlePause} title="Pause task">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="16"
							height="16"
							viewBox="0 0 24 24"
							fill="currentColor"
							stroke="none"
						>
							<rect x="6" y="4" width="4" height="16" rx="1" />
							<rect x="14" y="4" width="4" height="16" rx="1" />
						</svg>
						Pause
					</button>
					<button class="danger" onclick={handleCancel} title="Cancel task">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="16"
							height="16"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
							stroke-linecap="round"
							stroke-linejoin="round"
						>
							<rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
						</svg>
						Cancel
					</button>
				{:else if task.status === 'paused'}
					<button class="primary" onclick={handleResume} title="Resume task">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="16"
							height="16"
							viewBox="0 0 24 24"
							fill="currentColor"
							stroke="none"
						>
							<polygon points="5 3 19 12 5 21 5 3" />
						</svg>
						Resume
					</button>
				{:else if ['created', 'planned'].includes(task.status)}
					<button class="primary" onclick={handleRun} title="Run task">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="16"
							height="16"
							viewBox="0 0 24 24"
							fill="currentColor"
							stroke="none"
						>
							<polygon points="5 3 19 12 5 21 5 3" />
						</svg>
						Run Task
					</button>
				{/if}
				{#if task.status !== 'running'}
					<button class="icon-btn delete-btn" onclick={handleDelete} title="Delete task">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="16"
							height="16"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
							stroke-linecap="round"
							stroke-linejoin="round"
						>
							<polyline points="3 6 5 6 21 6"></polyline>
							<path
								d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"
							></path>
						</svg>
					</button>
				{/if}
			</div>
		</header>

		<!-- Timeline Section -->
		{#if plan}
			<section class="section">
				<Timeline phases={plan.phases} currentPhase={task.current_phase} state={taskState} />
			</section>
		{/if}

		<!-- Stats Grid -->
		<div class="stats-grid">
			<!-- Token Usage -->
			{#if taskState?.tokens}
				<div class="stat-card">
					<div class="stat-header">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="16"
							height="16"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
							stroke-linecap="round"
							stroke-linejoin="round"
						>
							<circle cx="12" cy="12" r="10" />
							<line x1="12" y1="8" x2="12" y2="16" />
							<line x1="8" y1="12" x2="16" y2="12" />
						</svg>
						<span>Token Usage</span>
					</div>
					<div class="token-stats">
						<div class="token-stat">
							<span class="token-value"
								>{(taskState.tokens.input_tokens || 0).toLocaleString()}</span
							>
							<span class="token-label">Input</span>
						</div>
						<div class="token-divider"></div>
						<div class="token-stat">
							<span class="token-value"
								>{(taskState.tokens.output_tokens || 0).toLocaleString()}</span
							>
							<span class="token-label">Output</span>
						</div>
						<div class="token-divider"></div>
						<div class="token-stat total">
							<span class="token-value"
								>{(taskState.tokens.total_tokens || 0).toLocaleString()}</span
							>
							<span class="token-label">Total</span>
						</div>
					</div>
				</div>
			{/if}

			<!-- Iterations -->
			{#if taskState?.phases}
				{@const totalIterations = Object.values(taskState.phases).reduce(
					(sum, p) => sum + (p.iterations || 0),
					0
				)}
				<div class="stat-card">
					<div class="stat-header">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="16"
							height="16"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
							stroke-linecap="round"
							stroke-linejoin="round"
						>
							<polyline points="23 4 23 10 17 10" />
							<polyline points="1 20 1 14 7 14" />
							<path
								d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"
							/>
						</svg>
						<span>Iterations</span>
					</div>
					<div class="stat-value">{totalIterations}</div>
				</div>
			{/if}

			<!-- Retries -->
			{#if taskState?.retries}
				<div class="stat-card">
					<div class="stat-header">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							width="16"
							height="16"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
							stroke-linecap="round"
							stroke-linejoin="round"
						>
							<path d="M21.5 2v6h-6M2.5 22v-6h6M2 11.5a10 10 0 0 1 18.8-4.3M22 12.5a10 10 0 0 1-18.8 4.3" />
						</svg>
						<span>Retries</span>
					</div>
					<div class="stat-value">{taskState.retries}</div>
				</div>
			{/if}
		</div>

		<!-- Transcript Section -->
		<section class="section transcript-section">
			<Transcript lines={transcript} />
		</section>
	</div>
{/if}

<style>
	.task-detail {
		max-width: 1000px;
	}

	/* Loading / Error States */
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
		width: 32px;
		height: 32px;
		border: 3px solid var(--border-default);
		border-top-color: var(--accent-primary);
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
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

	/* Connection Banner */
	.connection-banner {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: var(--space-2-5) var(--space-4);
		margin-bottom: var(--space-5);
		background: var(--status-danger-bg);
		border: 1px solid var(--status-danger);
		border-radius: var(--radius-lg);
		color: var(--status-danger);
	}

	.connection-banner.reconnecting,
	.connection-banner.connecting {
		background: var(--status-warning-bg);
		border-color: var(--status-warning);
		color: var(--status-warning);
	}

	.connection-content {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		font-size: var(--text-sm);
	}

	.connection-spinner {
		width: 14px;
		height: 14px;
		border: 2px solid currentColor;
		border-top-color: transparent;
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	.connection-banner button {
		padding: var(--space-1) var(--space-3);
		font-size: var(--text-xs);
		background: rgba(0, 0, 0, 0.1);
		border: 1px solid currentColor;
		border-radius: var(--radius-md);
		color: inherit;
		cursor: pointer;
	}

	.connection-banner button:hover {
		background: rgba(0, 0, 0, 0.2);
	}

	/* Task Header */
	.task-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: var(--space-6);
		margin-bottom: var(--space-6);
		padding-bottom: var(--space-6);
		border-bottom: 1px solid var(--border-subtle);
	}

	.task-info {
		flex: 1;
		min-width: 0;
	}

	.task-meta {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		margin-bottom: var(--space-3);
	}

	.task-id {
		font-family: var(--font-mono);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-muted);
		letter-spacing: var(--tracking-wide);
	}

	.weight-badge {
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		padding: var(--space-0-5) var(--space-2);
		border-radius: var(--radius-sm);
	}

	.status-badge {
		margin-left: var(--space-2);
	}

	.task-title {
		font-size: var(--text-2xl);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		margin: 0 0 var(--space-2) 0;
		letter-spacing: normal;
		text-transform: none;
	}

	.task-description {
		font-size: var(--text-base);
		color: var(--text-secondary);
		line-height: var(--leading-relaxed);
		margin: 0;
	}

	/* Task Actions */
	.task-actions {
		display: flex;
		gap: var(--space-2);
		flex-shrink: 0;
	}

	.task-actions button {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.delete-btn {
		background: transparent;
		border: 1px solid var(--border-default);
		color: var(--text-muted);
	}

	.delete-btn:hover {
		background: var(--status-danger-bg);
		border-color: var(--status-danger);
		color: var(--status-danger);
	}

	/* Sections */
	.section {
		margin-bottom: var(--space-6);
	}

	/* Stats Grid */
	.stats-grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
		gap: var(--space-4);
		margin-bottom: var(--space-6);
	}

	.stat-card {
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		padding: var(--space-4);
	}

	.stat-header {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		font-size: var(--text-xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		color: var(--text-muted);
		margin-bottom: var(--space-3);
	}

	.stat-value {
		font-family: var(--font-mono);
		font-size: var(--text-3xl);
		font-weight: var(--font-bold);
		color: var(--text-primary);
	}

	/* Token Stats */
	.token-stats {
		display: flex;
		align-items: center;
		gap: var(--space-4);
	}

	.token-stat {
		display: flex;
		flex-direction: column;
	}

	.token-stat.total .token-value {
		color: var(--accent-primary);
	}

	.token-value {
		font-family: var(--font-mono);
		font-size: var(--text-lg);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
	}

	.token-label {
		font-size: var(--text-2xs);
		font-weight: var(--font-medium);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		color: var(--text-muted);
	}

	.token-divider {
		width: 1px;
		height: 32px;
		background: var(--border-subtle);
	}

	/* Transcript Section */
	.transcript-section {
		min-height: 400px;
	}
</style>
