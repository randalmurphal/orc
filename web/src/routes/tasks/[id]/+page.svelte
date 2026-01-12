<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import {
		getTask,
		getTaskState,
		getTaskPlan,
		getTranscripts,
		runTask,
		pauseTask,
		resumeTask,
		deleteTask,
		getProjectTask,
		getProjectTaskState,
		getProjectTaskPlan,
		runProjectTask,
		pauseProjectTask,
		resumeProjectTask,
		deleteProjectTask,
		getProjectTranscripts,
		getDiffStats,
		getReviewStats,
		type DiffStatsResponse,
		type ReviewStatsResponse
	} from '$lib/api';
	import {
		subscribeToTaskWS,
		type ConnectionStatus,
		type WSEventType,
		getWebSocket
	} from '$lib/websocket';
	import type { Task, TaskState, Plan, TranscriptFile } from '$lib/types';
	import TaskHeader from '$lib/components/task/TaskHeader.svelte';
	import TabNav, { type TabId } from '$lib/components/task/TabNav.svelte';
	import PRActions from '$lib/components/task/PRActions.svelte';
	import Timeline from '$lib/components/Timeline.svelte';
	import Transcript from '$lib/components/Transcript.svelte';
	import DiffViewer from '$lib/components/diff/DiffViewer.svelte';
	import { currentProjectId } from '$lib/stores/project';

	let task = $state<Task | null>(null);
	let taskState = $state<TaskState | null>(null);
	let plan = $state<Plan | null>(null);
	let transcriptFiles = $state<TranscriptFile[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let connectionStatus = $state<ConnectionStatus>('disconnected');
	let unsubscribe: (() => void) | null = null;

	// Tab state
	let activeTab = $state<TabId>('timeline');
	let diffStats = $state<DiffStatsResponse | null>(null);
	let reviewStats = $state<ReviewStatsResponse | null>(null);

	const taskId = $derived($page.params.id ?? '');
	// Subscribe to currentProjectId reactively - use $effect to track changes
	let projectId = $state<string | null>(null);
	$effect(() => {
		// Subscribe to store and update when it changes
		const unsubscribeProject = currentProjectId.subscribe((value) => {
			projectId = value;
		});
		return unsubscribeProject;
	});

	// Read initial tab from URL query param
	$effect(() => {
		const urlTab = $page.url.searchParams.get('tab');
		// Support old 'review' tab URL by redirecting to 'changes'
		if (urlTab === 'review') {
			activeTab = 'changes' as TabId;
		} else if (urlTab && ['timeline', 'changes', 'transcript'].includes(urlTab)) {
			activeTab = urlTab as TabId;
		}
	});

	// Update URL when tab changes
	function handleTabChange(tab: TabId) {
		activeTab = tab;
		const url = new URL($page.url);
		url.searchParams.set('tab', tab);
		goto(url.toString(), { replaceState: true, noScroll: true });
	}

	// Tab configuration with badges
	// Combined diff stats and review stats into single badge for Changes tab
	const changesBadge = $derived.by(() => {
		const parts: string[] = [];
		if (diffStats) {
			parts.push(`+${diffStats.additions} -${diffStats.deletions}`);
		}
		if (reviewStats?.open_comments) {
			parts.push(`${reviewStats.open_comments} comment${reviewStats.open_comments > 1 ? 's' : ''}`);
		}
		return parts.length > 0 ? parts.join(' · ') : null;
	});

	const tabs = $derived([
		{
			id: 'timeline' as TabId,
			label: 'Timeline',
			badge: null
		},
		{
			id: 'changes' as TabId,
			label: 'Changes',
			badge: changesBadge,
			badgeType: (reviewStats?.blockers ?? 0) > 0 ? ('danger' as const) : ('default' as const)
		},
		{
			id: 'transcript' as TabId,
			label: 'Transcript',
			badge: null
		}
	]);

	onMount(async () => {
		if (!taskId) return;
		await loadTaskData();
		await loadBadgeStats();
		setupStreaming();
	});

	onDestroy(() => {
		if (unsubscribe) unsubscribe();
	});

	async function loadTaskData() {
		loading = true;
		error = null;

		try {
			let t: Task;
			let s: TaskState | null;
			let p: Plan | null;
			let files: TranscriptFile[];

			// Try project-scoped endpoints first if projectId is set
			if (projectId) {
				try {
					[t, s, p, files] = await Promise.all([
						getProjectTask(projectId, taskId),
						getProjectTaskState(projectId, taskId).catch(() => null),
						getProjectTaskPlan(projectId, taskId).catch(() => null),
						getProjectTranscripts(projectId, taskId).catch(() => [])
					]);
				} catch (projectError) {
					// Fall back to CWD-based endpoints if project-scoped fails
					console.warn('Project-scoped load failed, falling back to CWD-based endpoints');
					[t, s, p, files] = await Promise.all([
						getTask(taskId),
						getTaskState(taskId).catch(() => null),
						getTaskPlan(taskId).catch(() => null),
						getTranscripts(taskId).catch(() => [])
					]);
				}
			} else {
				[t, s, p, files] = await Promise.all([
					getTask(taskId),
					getTaskState(taskId).catch(() => null),
					getTaskPlan(taskId).catch(() => null),
					getTranscripts(taskId).catch(() => [])
				]);
			}

			task = t;
			taskState = s;
			plan = p;
			transcriptFiles = files;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load task';
		} finally {
			loading = false;
		}
	}

	async function loadBadgeStats() {
		// Load stats for tab badges in parallel
		const [ds, rs] = await Promise.all([
			getDiffStats(taskId).catch(() => null),
			getReviewStats(taskId).catch(() => null)
		]);
		diffStats = ds;
		reviewStats = rs;
	}

	// Track streaming response content for live updates
	let streamingContent = $state('');
	let streamingPhase = $state('');
	let streamingIteration = $state(0);

	function setupStreaming() {
		unsubscribe = subscribeToTaskWS(
			taskId,
			(eventType: WSEventType, data: unknown) => {
				if (eventType === 'state') {
					taskState = data as TaskState;
				} else if (eventType === 'transcript') {
					const transcriptData = data as { type: string; content: string; phase: string; iteration: number };

					// Handle streaming chunks - accumulate them
					if (transcriptData.type === 'chunk') {
						// If this is a new phase/iteration, reset streaming content
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
						// Full response received - reload to get persisted transcript file
						streamingContent = '';
						loadTaskData();
					}
				} else if (eventType === 'tokens') {
					// Update token display in real-time
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
				} else if (eventType === 'phase') {
					loadTaskData();
					loadBadgeStats();
				} else if (eventType === 'complete') {
					loadTaskData();
					loadBadgeStats();
				} else if (eventType === 'error') {
					const errorData = data as { message: string };
					error = errorData.message;
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
		try {
			projectId ? await runProjectTask(projectId, taskId) : await runTask(taskId);
			await loadTaskData();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to run task';
		}
	}

	async function handlePause() {
		try {
			projectId ? await pauseProjectTask(projectId, taskId) : await pauseTask(taskId);
			await loadTaskData();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to pause task';
		}
	}

	async function handleResume() {
		try {
			projectId ? await resumeProjectTask(projectId, taskId) : await resumeTask(taskId);
			await loadTaskData();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to resume task';
		}
	}

	async function handleDelete() {
		if (!task || !confirm(`Delete task ${task.id}?`)) return;

		try {
			projectId ? await deleteProjectTask(projectId, taskId) : await deleteTask(taskId);
			goto('/');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete task';
		}
	}

	function handleRetry() {
		// Switch to changes tab so user can see diff and add review comments
		handleTabChange('changes');
	}
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
						<path d="M16.72 11.06A10.94 10.94 0 0 1 19 12.55" />
						<path d="M5 12.55a10.94 10.94 0 0 1 5.17-2.39" />
						<path d="M10.71 5.05A16 16 0 0 1 22.58 9" />
						<path d="M1.42 9a15.91 15.91 0 0 1 4.7-2.88" />
						<path d="M8.53 16.11a6 6 0 0 1 6.95 0" />
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
		<TaskHeader
			{task}
			{taskState}
			{plan}
			onRun={handleRun}
			onPause={handlePause}
			onResume={handleResume}
			onCancel={handleCancel}
			onDelete={handleDelete}
			onRetry={handleRetry}
		/>

		<!-- PR Actions (for completed tasks) -->
		<div class="pr-section">
			<PRActions taskId={task.id} taskBranch={task.branch} taskStatus={task.status} />
		</div>

		<!-- Tab Navigation -->
		<div class="tab-section">
			<TabNav {tabs} {activeTab} onTabChange={handleTabChange} />
		</div>

		<!-- Tab Content -->
		<div class="tab-content" role="tabpanel" aria-labelledby={`tab-${activeTab}`}>
			{#if activeTab === 'timeline'}
				{#if plan}
					<Timeline phases={plan.phases} currentPhase={task.current_phase} state={taskState} />
				{:else}
					<div class="empty-tab-state">
						<p>No execution plan available</p>
					</div>
				{/if}

				<!-- Stats Grid -->
				<div class="stats-grid">
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
								{#if taskState.tokens.cache_read_input_tokens}
									<div class="token-divider"></div>
									<div class="token-stat cached">
										<span class="token-value"
											>{(taskState.tokens.cache_read_input_tokens || 0).toLocaleString()}</span
										>
										<span class="token-label">Cached</span>
									</div>
								{/if}
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
									<path
										d="M21.5 2v6h-6M2.5 22v-6h6M2 11.5a10 10 0 0 1 18.8-4.3M22 12.5a10 10 0 0 1-18.8 4.3"
									/>
								</svg>
								<span>Retries</span>
							</div>
							<div class="stat-value">{taskState.retries}</div>
						</div>
					{/if}
				</div>
			{:else if activeTab === 'changes'}
				<div class="diff-container">
					<DiffViewer {taskId} />
				</div>
			{:else if activeTab === 'transcript'}
				<Transcript files={transcriptFiles} taskId={task.id} {streamingContent} />
			{/if}
		</div>
	</div>
{/if}

<style>
	.task-detail {
		max-width: 1200px;
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	/* PR Section */
	.pr-section {
		min-height: 40px;
	}

	.pr-section:empty {
		display: none;
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
		margin-bottom: var(--space-4);
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

	/* Tab Section */
	.tab-section {
		margin-bottom: var(--space-2);
	}

	/* Tab Content */
	.tab-content {
		min-height: 400px;
	}

	.empty-tab-state {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: var(--space-12);
		color: var(--text-muted);
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
	}

	/* Diff Container */
	.diff-container {
		min-height: 500px;
		max-height: calc(100vh - 300px);
	}

	/* Stats Grid */
	.stats-grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
		gap: var(--space-4);
		margin-top: var(--space-6);
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

	.token-stat.cached .token-value {
		color: var(--status-success);
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
</style>
