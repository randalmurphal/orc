<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { getDashboardStats, listTasks, type DashboardStats } from '$lib/api';
	import { getWebSocket, type WSEvent, type ConnectionStatus } from '$lib/websocket';
	import { toast } from '$lib/stores/toast';
	import type { Task } from '$lib/types';
	import {
		DashboardStats as StatsSection,
		DashboardQuickActions,
		DashboardActiveTasks,
		DashboardRecentActivity,
		DashboardSummary
	} from './dashboard';

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

	function navigateToFiltered(status: string) {
		goto(`/?status=${status}`);
	}

	function handleNewTask() {
		window.dispatchEvent(new CustomEvent('orc:new-task'));
	}

	function handleViewTasks() {
		goto('/');
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
		<StatsSection {stats} {wsStatus} onFilterClick={navigateToFiltered} />
		<DashboardQuickActions onNewTask={handleNewTask} onViewTasks={handleViewTasks} />
		<DashboardActiveTasks tasks={activeTasks} />
		<DashboardRecentActivity tasks={recentTasks} />
		<DashboardSummary {stats} />
	{/if}
</div>

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
</style>
