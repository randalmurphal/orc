<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { getDashboardStats, type DashboardStats } from '$lib/api';
	import { getWebSocket, type ConnectionStatus } from '$lib/websocket';
	import { tasks as tasksStore, tasksLoading } from '$lib/stores/tasks';
	import type { Task } from '$lib/types';
	import {
		DashboardStats as StatsSection,
		DashboardQuickActions,
		DashboardActiveTasks,
		DashboardRecentActivity,
		DashboardSummary
	} from './dashboard';

	let stats = $state<DashboardStats | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let wsStatus = $state<ConnectionStatus>('disconnected');
	let wsStatusCleanup: (() => void) | null = null;

	// Derive active and recent tasks from global store
	let activeTasks = $derived(
		$tasksStore
			.filter((t) => ['running', 'blocked', 'paused'].includes(t.status))
			.slice(0, 5)
	);

	let recentTasks = $derived(
		$tasksStore
			.filter((t) => ['completed', 'failed'].includes(t.status))
			.sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())
			.slice(0, 5)
	);

	// Combined loading state
	let isLoading = $derived(loading || $tasksLoading);

	onMount(() => {
		loadDashboardStats();

		// Subscribe to WebSocket status (events handled by layout)
		const ws = getWebSocket();
		wsStatusCleanup = ws.onStatusChange((status) => {
			wsStatus = status;
			if (status === 'connected') {
				// Refresh stats on reconnect
				loadDashboardStats();
			}
		});
		// Task list is kept up-to-date via file watcher WebSocket events
	});

	onDestroy(() => {
		if (wsStatusCleanup) {
			wsStatusCleanup();
		}
	});

	async function loadDashboardStats() {
		try {
			stats = await getDashboardStats();
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
	{#if isLoading && !stats}
		<div class="loading">
			<div class="spinner"></div>
			<span>Loading dashboard...</span>
		</div>
	{:else if error}
		<div class="error">
			<p>{error}</p>
			<button onclick={loadDashboardStats}>Retry</button>
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
