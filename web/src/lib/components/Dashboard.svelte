<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { getDashboardStats, listTasks, type DashboardStats } from '$lib/api';
	import { getWebSocket, type WSEvent, type ConnectionStatus } from '$lib/websocket';
	import { toast } from '$lib/stores/toast';
	import type { Task } from '$lib/types';
	import TaskCard from './TaskCard.svelte';

	let stats = $state<DashboardStats | null>(null);
	let activeTasks = $state<Task[]>([]);
	let recentTasks = $state<Task[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let wsStatus = $state<ConnectionStatus>('disconnected');
	let refreshInterval: ReturnType<typeof setInterval>;
	let wsCleanup: (() => void) | null = null;

	onMount(() => {
		loadDashboard();
		setupWebSocket();
		// Refresh every 30 seconds (WebSocket handles real-time updates)
		refreshInterval = setInterval(loadDashboard, 30000);
	});

	onDestroy(() => {
		if (refreshInterval) {
			clearInterval(refreshInterval);
		}
		if (wsCleanup) {
			wsCleanup();
		}
	});

	function setupWebSocket() {
		const ws = getWebSocket();

		// Listen for all events (dashboard doesn't subscribe to specific task)
		const unsubEvent = ws.on('all', handleWSEvent);
		const unsubStatus = ws.onStatusChange((status) => {
			wsStatus = status;
			if (status === 'connected') {
				// Refresh data on reconnect
				loadDashboard();
			}
		});

		// Connect without subscribing to specific task
		ws.connect();

		wsCleanup = () => {
			unsubEvent();
			unsubStatus();
		};
	}

	function handleWSEvent(event: WSEvent | { type: 'error'; error: string }) {
		if (event.type === 'error') {
			return;
		}

		const wsEvent = event as WSEvent;

		// Handle state changes for dashboard updates
		if (wsEvent.event === 'state') {
			const data = wsEvent.data as { status?: string; phase?: string };
			const taskId = wsEvent.task_id;

			// Show toast for important state changes
			if (data.status === 'completed') {
				toast.success(`Task ${taskId} completed`, { title: 'Task Complete' });
				loadDashboard();
			} else if (data.status === 'failed') {
				toast.error(`Task ${taskId} failed`, { title: 'Task Failed' });
				loadDashboard();
			} else if (data.status === 'blocked') {
				toast.warning(`Task ${taskId} is blocked`, { title: 'Task Blocked' });
				loadDashboard();
			} else if (data.status === 'running') {
				// Just refresh without toast for running
				loadDashboard();
			}
		}

		// Handle phase changes
		if (wsEvent.event === 'phase') {
			const data = wsEvent.data as { phase?: string; status?: string };
			if (data.status === 'completed') {
				toast.info(`Phase ${data.phase} completed`, { duration: 3000 });
			}
		}
	}

	async function loadDashboard() {
		try {
			const [statsData, tasksData] = await Promise.all([
				getDashboardStats(),
				listTasks() as Promise<Task[]>
			]);

			stats = statsData;

			// Active tasks: running, blocked, paused
			activeTasks = tasksData
				.filter((t) => ['running', 'blocked', 'paused'].includes(t.status))
				.slice(0, 5);

			// Recent tasks: completed or failed, sorted by updated_at
			recentTasks = tasksData
				.filter((t) => ['completed', 'failed'].includes(t.status))
				.sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())
				.slice(0, 5);

			loading = false;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load dashboard';
			loading = false;
		}
	}

	function formatTokens(tokens: number): string {
		if (tokens >= 1_000_000) {
			return `${(tokens / 1_000_000).toFixed(1)}M`;
		}
		if (tokens >= 1_000) {
			return `${(tokens / 1_000).toFixed(1)}K`;
		}
		return String(tokens);
	}

	function formatCost(cost: number): string {
		return `$${cost.toFixed(2)}`;
	}

	function navigateToFiltered(status: string) {
		goto(`/?status=${status}`);
	}

	function handleNewTask() {
		window.dispatchEvent(new CustomEvent('orc:new-task'));
	}
</script>

<div class="dashboard">
	{#if loading && !stats}
		<div class="loading">
			<div class="spinner"></div>
			<span>Loading dashboard...</span>
		</div>
	{:else if error}
		<div class="error">
			<p>{error}</p>
			<button onclick={loadDashboard}>Retry</button>
		</div>
	{:else if stats}
		<!-- Quick Stats -->
		<section class="stats-section">
			<div class="section-header">
				<h2 class="section-title">Quick Stats</h2>
				<div class="connection-status" class:connected={wsStatus === 'connected'} class:connecting={wsStatus === 'connecting' || wsStatus === 'reconnecting'}>
					<span class="status-dot"></span>
					<span class="status-text">{wsStatus === 'connected' ? 'Live' : wsStatus === 'connecting' ? 'Connecting' : wsStatus === 'reconnecting' ? 'Reconnecting' : 'Offline'}</span>
				</div>
			</div>
			<div class="stats-grid">
				<button class="stat-card running" onclick={() => navigateToFiltered('running')}>
					<div class="stat-icon">
						<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<circle cx="12" cy="12" r="10" />
							<polyline points="12 6 12 12 16 14" />
						</svg>
					</div>
					<div class="stat-content">
						<span class="stat-value">{stats.running}</span>
						<span class="stat-label">Running</span>
					</div>
				</button>

				<button class="stat-card blocked" onclick={() => navigateToFiltered('blocked')}>
					<div class="stat-icon">
						<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<circle cx="12" cy="12" r="10" />
							<line x1="4.93" y1="4.93" x2="19.07" y2="19.07" />
						</svg>
					</div>
					<div class="stat-content">
						<span class="stat-value">{stats.blocked}</span>
						<span class="stat-label">Blocked</span>
					</div>
				</button>

				<button class="stat-card today" onclick={() => navigateToFiltered('all')}>
					<div class="stat-icon">
						<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<rect x="3" y="4" width="18" height="18" rx="2" ry="2" />
							<line x1="16" y1="2" x2="16" y2="6" />
							<line x1="8" y1="2" x2="8" y2="6" />
							<line x1="3" y1="10" x2="21" y2="10" />
						</svg>
					</div>
					<div class="stat-content">
						<span class="stat-value">{stats.today}</span>
						<span class="stat-label">Today</span>
					</div>
				</button>

				<div class="stat-card tokens">
					<div class="stat-icon">
						<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6" />
						</svg>
					</div>
					<div class="stat-content">
						<span class="stat-value">{formatTokens(stats.tokens)}</span>
						<span class="stat-label">Tokens</span>
					</div>
				</div>
			</div>
		</section>

		<!-- Quick Actions -->
		<section class="actions-section">
			<div class="quick-actions">
				<button class="action-btn primary" onclick={handleNewTask}>
					<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
						<line x1="12" y1="5" x2="12" y2="19" />
						<line x1="5" y1="12" x2="19" y2="12" />
					</svg>
					New Task
				</button>
				<button class="action-btn" onclick={() => goto('/')}>
					<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
						<rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
						<line x1="9" y1="9" x2="15" y2="15" />
						<line x1="15" y1="9" x2="9" y2="15" />
					</svg>
					View All Tasks
				</button>
			</div>
		</section>

		<!-- Active Tasks -->
		{#if activeTasks.length > 0}
			<section class="tasks-section">
				<div class="section-header">
					<h2 class="section-title">Active Tasks</h2>
					<span class="section-count">{activeTasks.length}</span>
				</div>
				<div class="task-list">
					{#each activeTasks as task (task.id)}
						<TaskCard {task} compact />
					{/each}
				</div>
			</section>
		{/if}

		<!-- Recent Activity -->
		{#if recentTasks.length > 0}
			<section class="tasks-section">
				<div class="section-header">
					<h2 class="section-title">Recent Activity</h2>
				</div>
				<div class="activity-list">
					{#each recentTasks as task (task.id)}
						<a href="/tasks/{task.id}" class="activity-item">
							<span class="activity-status" class:completed={task.status === 'completed'} class:failed={task.status === 'failed'}>
								{#if task.status === 'completed'}
									<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
										<polyline points="20 6 9 17 4 12" />
									</svg>
								{:else}
									<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
										<line x1="18" y1="6" x2="6" y2="18" />
										<line x1="6" y1="6" x2="18" y2="18" />
									</svg>
								{/if}
							</span>
							<div class="activity-content">
								<span class="activity-id">{task.id}</span>
								<span class="activity-title">{task.title}</span>
							</div>
							<span class="activity-time">{formatRelativeTime(task.updated_at)}</span>
						</a>
					{/each}
				</div>
			</section>
		{/if}

		<!-- Summary Footer -->
		<section class="summary-section">
			<div class="summary-stats">
				<div class="summary-item">
					<span class="summary-label">Total Tasks</span>
					<span class="summary-value">{stats.total}</span>
				</div>
				<div class="summary-item">
					<span class="summary-label">Completed</span>
					<span class="summary-value success">{stats.completed}</span>
				</div>
				<div class="summary-item">
					<span class="summary-label">Failed</span>
					<span class="summary-value danger">{stats.failed}</span>
				</div>
			</div>
		</section>
	{/if}
</div>

<script module lang="ts">
	function formatRelativeTime(dateStr: string): string {
		const date = new Date(dateStr);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffMins = Math.floor(diffMs / 60000);
		const diffHours = Math.floor(diffMs / 3600000);
		const diffDays = Math.floor(diffMs / 86400000);

		if (diffMins < 1) return 'just now';
		if (diffMins < 60) return `${diffMins}m ago`;
		if (diffHours < 24) return `${diffHours}h ago`;
		if (diffDays < 7) return `${diffDays}d ago`;
		return date.toLocaleDateString();
	}
</script>

<style>
	.dashboard {
		max-width: 900px;
		display: flex;
		flex-direction: column;
		gap: var(--space-6);
	}

	.loading,
	.error {
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

	.section-title {
		font-size: var(--text-lg);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		margin: 0;
	}

	.section-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-2);
		margin-bottom: var(--space-4);
	}

	.section-count {
		font-size: var(--text-xs);
		font-family: var(--font-mono);
		padding: var(--space-0-5) var(--space-2);
		background: var(--bg-tertiary);
		border-radius: var(--radius-full);
		color: var(--text-muted);
	}

	/* Connection Status */
	.connection-status {
		display: flex;
		align-items: center;
		gap: var(--space-1-5);
		padding: var(--space-1) var(--space-2);
		background: var(--bg-tertiary);
		border-radius: var(--radius-full);
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.status-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--text-muted);
	}

	.connection-status.connected .status-dot {
		background: var(--status-success);
		box-shadow: 0 0 4px var(--status-success);
	}

	.connection-status.connected .status-text {
		color: var(--status-success);
	}

	.connection-status.connecting .status-dot {
		background: var(--status-warning);
		animation: pulse 1s ease-in-out infinite;
	}

	.connection-status.connecting .status-text {
		color: var(--status-warning);
	}

	@keyframes pulse {
		0%, 100% { opacity: 1; }
		50% { opacity: 0.4; }
	}

	/* Stats Grid */
	.stats-section .section-header {
		margin-bottom: var(--space-4);
	}

	.stats-grid {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: var(--space-4);
	}

	@media (max-width: 768px) {
		.stats-grid {
			grid-template-columns: repeat(2, 1fr);
		}
	}

	.stat-card {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-4);
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-lg);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
		text-align: left;
	}

	.stat-card:hover {
		border-color: var(--accent-primary);
		transform: translateY(-2px);
	}

	.stat-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 40px;
		height: 40px;
		border-radius: var(--radius-md);
		color: var(--text-muted);
	}

	.stat-card.running .stat-icon {
		background: var(--status-info-bg);
		color: var(--status-info);
	}

	.stat-card.blocked .stat-icon {
		background: var(--status-warning-bg);
		color: var(--status-warning);
	}

	.stat-card.today .stat-icon {
		background: rgba(168, 85, 247, 0.1);
		color: rgb(168, 85, 247);
	}

	.stat-card.tokens .stat-icon {
		background: var(--status-success-bg);
		color: var(--status-success);
	}

	.stat-content {
		display: flex;
		flex-direction: column;
	}

	.stat-value {
		font-size: var(--text-xl);
		font-weight: var(--font-bold);
		font-family: var(--font-mono);
		color: var(--text-primary);
	}

	.stat-label {
		font-size: var(--text-sm);
		color: var(--text-muted);
	}

	/* Quick Actions */
	.quick-actions {
		display: flex;
		gap: var(--space-3);
	}

	.action-btn {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2-5) var(--space-4);
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		color: var(--text-secondary);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.action-btn:hover {
		border-color: var(--accent-primary);
		color: var(--text-primary);
	}

	.action-btn.primary {
		background: var(--accent-primary);
		border-color: var(--accent-primary);
		color: var(--text-inverse);
	}

	.action-btn.primary:hover {
		background: var(--accent-secondary);
		border-color: var(--accent-secondary);
	}

	/* Task List */
	.task-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
	}

	/* Activity List */
	.activity-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.activity-item {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-3);
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		text-decoration: none;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.activity-item:hover {
		border-color: var(--accent-primary);
		background: var(--bg-tertiary);
	}

	.activity-status {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		border-radius: var(--radius-full);
	}

	.activity-status.completed {
		background: var(--status-success-bg);
		color: var(--status-success);
	}

	.activity-status.failed {
		background: var(--status-danger-bg);
		color: var(--status-danger);
	}

	.activity-content {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: var(--space-0-5);
		overflow: hidden;
	}

	.activity-id {
		font-size: var(--text-xs);
		font-family: var(--font-mono);
		color: var(--text-muted);
	}

	.activity-title {
		font-size: var(--text-sm);
		color: var(--text-primary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.activity-time {
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	/* Summary Section */
	.summary-section {
		padding-top: var(--space-4);
		border-top: 1px solid var(--border-subtle);
	}

	.summary-stats {
		display: flex;
		gap: var(--space-8);
	}

	.summary-item {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
	}

	.summary-label {
		font-size: var(--text-xs);
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.summary-value {
		font-size: var(--text-lg);
		font-weight: var(--font-semibold);
		font-family: var(--font-mono);
		color: var(--text-primary);
	}

	.summary-value.success {
		color: var(--status-success);
	}

	.summary-value.danger {
		color: var(--status-danger);
	}
</style>
