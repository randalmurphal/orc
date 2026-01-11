<script lang="ts">
	import { onMount } from 'svelte';
	import DiffFile from './DiffFile.svelte';
	import DiffStats from './DiffStats.svelte';
	import type { DiffResult, FileDiff } from '$lib/types';

	interface Props {
		taskId: string;
	}

	let { taskId }: Props = $props();

	let diff = $state<DiffResult | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let viewMode = $state<'split' | 'unified'>('split');
	let expandedFiles = $state<Set<string>>(new Set());

	onMount(async () => {
		await loadDiff();
	});

	async function loadDiff() {
		loading = true;
		error = null;
		try {
			const res = await fetch(`/api/tasks/${taskId}/diff?files=true`);
			if (!res.ok) throw new Error('Failed to load diff');
			diff = await res.json();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Unknown error';
		} finally {
			loading = false;
		}
	}

	async function loadFileHunks(filePath: string) {
		const res = await fetch(`/api/tasks/${taskId}/diff/file/${encodeURIComponent(filePath)}`);
		if (!res.ok) return;
		const fileDiff = (await res.json()) as FileDiff;

		// Update the file in diff.files with hunks
		if (diff) {
			diff = {
				...diff,
				files: diff.files.map((f) => (f.path === filePath ? { ...f, hunks: fileDiff.hunks } : f))
			};
		}
	}

	function toggleFile(path: string) {
		const file = diff?.files.find((f) => f.path === path);
		if (!file?.hunks?.length) {
			loadFileHunks(path);
		}

		if (expandedFiles.has(path)) {
			expandedFiles.delete(path);
		} else {
			expandedFiles.add(path);
		}
		expandedFiles = new Set(expandedFiles);
	}

	function expandAll() {
		if (!diff) return;
		for (const file of diff.files) {
			if (!file.hunks?.length) {
				loadFileHunks(file.path);
			}
			expandedFiles.add(file.path);
		}
		expandedFiles = new Set(expandedFiles);
	}

	function collapseAll() {
		expandedFiles = new Set();
	}

	const allExpanded = $derived(diff ? expandedFiles.size === diff.files.length : false);
</script>

<div class="diff-viewer">
	<div class="diff-toolbar">
		<div class="toolbar-left">
			<div class="view-toggle">
				<button class:active={viewMode === 'split'} onclick={() => (viewMode = 'split')}>
					Split
				</button>
				<button class:active={viewMode === 'unified'} onclick={() => (viewMode = 'unified')}>
					Unified
				</button>
			</div>

			{#if diff && diff.files.length > 0}
				<button class="expand-btn" onclick={() => (allExpanded ? collapseAll() : expandAll())}>
					{allExpanded ? 'Collapse all' : 'Expand all'}
				</button>
			{/if}
		</div>

		{#if diff}
			<DiffStats stats={diff.stats} />
		{/if}
	</div>

	{#if loading}
		<div class="loading-state">
			<div class="loading-spinner"></div>
			<span>Loading diff...</span>
		</div>
	{:else if error}
		<div class="error-state">
			<span class="error-icon">!</span>
			<span>{error}</span>
		</div>
	{:else if diff && diff.files.length > 0}
		<div class="file-list">
			{#each diff.files as file (file.path)}
				<DiffFile
					{file}
					expanded={expandedFiles.has(file.path)}
					{viewMode}
					onToggle={() => toggleFile(file.path)}
				/>
			{/each}
		</div>
	{:else}
		<div class="empty-state">
			<span class="empty-icon">~</span>
			<span>No changes to display</span>
		</div>
	{/if}
</div>

<style>
	.diff-viewer {
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: hidden;
		background: var(--bg-primary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
	}

	.diff-toolbar {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: var(--space-3) var(--space-4);
		border-bottom: 1px solid var(--border-subtle);
		background: var(--bg-secondary);
		flex-shrink: 0;
	}

	.toolbar-left {
		display: flex;
		align-items: center;
		gap: var(--space-3);
	}

	.view-toggle {
		display: flex;
		gap: 0;
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		overflow: hidden;
	}

	.view-toggle button {
		padding: var(--space-1-5) var(--space-3);
		border: none;
		background: var(--bg-tertiary);
		color: var(--text-secondary);
		cursor: pointer;
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		transition: all var(--duration-fast) var(--ease-out);
	}

	.view-toggle button:first-child {
		border-right: 1px solid var(--border-default);
	}

	.view-toggle button:hover {
		background: var(--bg-surface);
		color: var(--text-primary);
	}

	.view-toggle button.active {
		background: var(--accent-primary);
		color: var(--text-inverse);
	}

	.expand-btn {
		padding: var(--space-1-5) var(--space-3);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-secondary);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.expand-btn:hover {
		background: var(--bg-surface);
		color: var(--text-primary);
		border-color: var(--border-strong);
	}

	.file-list {
		flex: 1;
		overflow-y: auto;
	}

	.loading-state,
	.error-state,
	.empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: var(--space-3);
		padding: var(--space-12);
		color: var(--text-muted);
	}

	.loading-spinner {
		width: 24px;
		height: 24px;
		border: 2px solid var(--border-default);
		border-top-color: var(--accent-primary);
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.error-state {
		color: var(--status-danger);
	}

	.error-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		border-radius: 50%;
		background: var(--status-danger-bg);
		font-weight: var(--font-bold);
	}

	.empty-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		border-radius: 50%;
		background: var(--bg-tertiary);
		font-family: var(--font-mono);
		font-weight: var(--font-bold);
	}
</style>
