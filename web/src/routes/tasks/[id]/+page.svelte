<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { getTask, getTaskState, getTaskPlan, runTask, pauseTask, subscribeToTask, getTranscripts } from '$lib/api';
	import type { Task, TaskState, Plan, TranscriptLine } from '$lib/types';
	import Timeline from '$lib/components/Timeline.svelte';
	import Transcript from '$lib/components/Transcript.svelte';

	let task = $state<Task | null>(null);
	let taskState = $state<TaskState | null>(null);
	let plan = $state<Plan | null>(null);
	let transcript = $state<TranscriptLine[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let unsubscribe: (() => void) | null = null;

	const taskId = $derived($page.params.id);

	onMount(async () => {
		await loadTaskData();
		setupStreaming();
	});

	onDestroy(() => {
		if (unsubscribe) unsubscribe();
	});

	async function loadTaskData() {
		loading = true;
		error = null;
		try {
			const [t, s, p, transcriptFiles] = await Promise.all([
				getTask(taskId),
				getTaskState(taskId).catch(() => null),
				getTaskPlan(taskId).catch(() => null),
				getTranscripts(taskId).catch(() => [])
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

	function parseTranscriptFiles(files: { filename: string; content: string; created_at: string }[]): TranscriptLine[] {
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
		unsubscribe = subscribeToTask(taskId, (eventType, data) => {
			if (eventType === 'state') {
				taskState = data as TaskState;
			} else if (eventType === 'transcript') {
				transcript = [...transcript, data as TranscriptLine];
			} else if (eventType === 'error') {
				// Reconnect after delay
				setTimeout(setupStreaming, 3000);
			}
		});
	}

	async function handleRun() {
		try {
			await runTask(taskId);
			await loadTaskData();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to run task';
		}
	}

	async function handlePause() {
		try {
			await pauseTask(taskId);
			await loadTaskData();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to pause task';
		}
	}

	const statusColors: Record<string, string> = {
		created: 'var(--text-secondary)',
		running: 'var(--accent-primary)',
		paused: 'var(--accent-warning)',
		completed: 'var(--accent-success)',
		failed: 'var(--accent-danger)'
	};
</script>

<svelte:head>
	<title>{task?.title || 'Task'} - orc</title>
</svelte:head>

{#if loading}
	<div class="loading">Loading task...</div>
{:else if error}
	<div class="error">
		<p>{error}</p>
		<button onclick={loadTaskData}>Retry</button>
	</div>
{:else if task}
	<div class="task-detail">
		<header class="task-header">
			<div class="task-info">
				<div class="task-meta">
					<span class="task-id">{task.id}</span>
					<span class="task-weight">{task.weight}</span>
					<span class="task-status" style="color: {statusColors[task.status]}">
						{task.status}
					</span>
				</div>
				<h1>{task.title}</h1>
				{#if task.description}
					<p class="task-description">{task.description}</p>
				{/if}
			</div>

			<div class="task-actions">
				{#if task.status === 'running'}
					<button onclick={handlePause}>Pause</button>
				{:else if ['created', 'planned', 'paused'].includes(task.status)}
					<button class="primary" onclick={handleRun}>Run</button>
				{/if}
			</div>
		</header>

		{#if plan}
			<section class="section">
				<h2>Timeline</h2>
				<Timeline phases={plan.phases} currentPhase={task.current_phase} state={taskState} />
			</section>
		{/if}

		{#if taskState?.tokens}
			<section class="section">
				<h2>Token Usage</h2>
				<div class="tokens">
					<div class="token-stat">
						<span class="token-value">{(taskState.tokens.input_tokens || 0).toLocaleString()}</span>
						<span class="token-label">Input</span>
					</div>
					<div class="token-stat">
						<span class="token-value">{(taskState.tokens.output_tokens || 0).toLocaleString()}</span>
						<span class="token-label">Output</span>
					</div>
					<div class="token-stat">
						<span class="token-value">{(taskState.tokens.total_tokens || 0).toLocaleString()}</span>
						<span class="token-label">Total</span>
					</div>
				</div>
			</section>
		{/if}

		<section class="section">
			<h2>Transcript</h2>
			<Transcript lines={transcript} />
		</section>
	</div>
{/if}

<style>
	.task-detail {
		max-width: 900px;
	}

	.task-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 1rem;
		margin-bottom: 2rem;
		padding-bottom: 1.5rem;
		border-bottom: 1px solid var(--border-color);
	}

	.task-info {
		flex: 1;
	}

	.task-meta {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		margin-bottom: 0.5rem;
	}

	.task-id {
		font-family: var(--font-mono);
		font-size: 0.875rem;
		color: var(--text-secondary);
	}

	.task-weight {
		font-size: 0.75rem;
		font-weight: 600;
		text-transform: uppercase;
		padding: 0.125rem 0.5rem;
		border-radius: 4px;
		background: var(--bg-tertiary);
		color: var(--text-secondary);
	}

	.task-status {
		font-size: 0.75rem;
		font-weight: 600;
		text-transform: uppercase;
	}

	h1 {
		font-size: 1.5rem;
		font-weight: 600;
		margin-bottom: 0.5rem;
	}

	.task-description {
		color: var(--text-secondary);
		font-size: 0.9375rem;
	}

	.task-actions {
		display: flex;
		gap: 0.5rem;
	}

	.section {
		margin-bottom: 2rem;
	}

	.section h2 {
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		margin-bottom: 1rem;
	}

	.tokens {
		display: flex;
		gap: 2rem;
	}

	.token-stat {
		display: flex;
		flex-direction: column;
	}

	.token-value {
		font-family: var(--font-mono);
		font-size: 1.5rem;
		font-weight: 600;
	}

	.token-label {
		font-size: 0.75rem;
		color: var(--text-secondary);
		text-transform: uppercase;
	}

	.loading, .error {
		text-align: center;
		padding: 3rem;
		color: var(--text-secondary);
	}

	.error p {
		margin-bottom: 1rem;
	}
</style>
