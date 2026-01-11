<script lang="ts">
	import { onMount } from 'svelte';
	import DiffFile from './DiffFile.svelte';
	import DiffStats from './DiffStats.svelte';
	import Icon from '$lib/components/ui/Icon.svelte';
	import type { DiffResult, FileDiff, ReviewComment, CreateCommentRequest } from '$lib/types';
	import { getReviewComments, createReviewComment, updateReviewComment, deleteReviewComment, triggerReviewRetry } from '$lib/api';

	interface Props {
		taskId: string;
	}

	let { taskId }: Props = $props();

	let diff = $state<DiffResult | null>(null);
	let comments = $state<ReviewComment[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let viewMode = $state<'split' | 'unified'>('split');
	let expandedFiles = $state<Set<string>>(new Set());
	let activeLineNumber = $state<number | null>(null);
	let activeFilePath = $state<string | null>(null);
	let sendingToAgent = $state(false);

	// Comment stats
	const openComments = $derived(comments.filter(c => c.status === 'open'));
	const blockerCount = $derived(openComments.filter(c => c.severity === 'blocker').length);
	const issueCount = $derived(openComments.filter(c => c.severity === 'issue').length);
	const suggestionCount = $derived(openComments.filter(c => c.severity === 'suggestion').length);
	const hasBlockers = $derived(blockerCount > 0);

	// General comments (not tied to a specific line)
	const generalComments = $derived(comments.filter(c => !c.file_path && !c.line_number));

	onMount(async () => {
		await Promise.all([loadDiff(), loadComments()]);
	});

	async function loadComments() {
		try {
			comments = await getReviewComments(taskId);
		} catch (e) {
			// Silently fail - comments are optional
			console.error('Failed to load comments:', e);
		}
	}

	async function handleAddComment(comment: CreateCommentRequest): Promise<void> {
		const newComment = await createReviewComment(taskId, comment);
		comments = [...comments, newComment];
		activeLineNumber = null;
		activeFilePath = null;
	}

	async function handleResolveComment(id: string) {
		const updated = await updateReviewComment(taskId, id, { status: 'resolved' });
		comments = comments.map(c => c.id === id ? updated : c);
	}

	async function handleWontFixComment(id: string) {
		const updated = await updateReviewComment(taskId, id, { status: 'wont_fix' });
		comments = comments.map(c => c.id === id ? updated : c);
	}

	async function handleDeleteComment(id: string) {
		await deleteReviewComment(taskId, id);
		comments = comments.filter(c => c.id !== id);
	}

	function handleLineClick(lineNumber: number, filePath: string) {
		if (activeLineNumber === lineNumber && activeFilePath === filePath) {
			// Toggle off
			activeLineNumber = null;
			activeFilePath = null;
		} else {
			activeLineNumber = lineNumber;
			activeFilePath = filePath;
		}
	}

	async function handleSendToAgent() {
		if (openComments.length === 0 || sendingToAgent) return;
		sendingToAgent = true;
		try {
			await triggerReviewRetry(taskId);
		} finally {
			sendingToAgent = false;
		}
	}

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
		try {
			const res = await fetch(`/api/tasks/${taskId}/diff/file/${encodeURIComponent(filePath)}`);
			if (!res.ok) {
				const errorMsg = `Failed to load file diff (${res.status})`;
				if (diff) {
					diff = {
						...diff,
						files: diff.files.map((f) =>
							f.path === filePath ? { ...f, loadError: errorMsg } : f
						)
					};
				}
				return;
			}
			const fileDiff = (await res.json()) as FileDiff;

			// Update the file in diff.files with hunks
			if (diff) {
				diff = {
					...diff,
					files: diff.files.map((f) =>
						f.path === filePath ? { ...f, hunks: fileDiff.hunks, loadError: undefined } : f
					)
				};
			}
		} catch (e) {
			const errorMsg = e instanceof Error ? e.message : 'Unknown error loading file';
			if (diff) {
				diff = {
					...diff,
					files: diff.files.map((f) => (f.path === filePath ? { ...f, loadError: errorMsg } : f))
				};
			}
		}
	}

	function toggleFile(path: string) {
		const file = diff?.files.find((f) => f.path === path);
		if (!file?.hunks?.length && !file?.loadError) {
			loadFileHunks(path);
		}

		if (expandedFiles.has(path)) {
			expandedFiles = new Set([...expandedFiles].filter((p) => p !== path));
		} else {
			expandedFiles = new Set([...expandedFiles, path]);
		}
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
			<div class="view-toggle" role="tablist" aria-label="Diff view mode">
				<button
					role="tab"
					aria-selected={viewMode === 'split'}
					class:active={viewMode === 'split'}
					onclick={() => (viewMode = 'split')}
				>
					Split
				</button>
				<button
					role="tab"
					aria-selected={viewMode === 'unified'}
					class:active={viewMode === 'unified'}
					onclick={() => (viewMode = 'unified')}
				>
					Unified
				</button>
			</div>

			{#if diff && diff.files.length > 0}
				<button class="expand-btn" onclick={() => (allExpanded ? collapseAll() : expandAll())}>
					{allExpanded ? 'Collapse all' : 'Expand all'}
				</button>
			{/if}
		</div>

		<div class="toolbar-right">
			{#if openComments.length > 0}
				<div class="review-summary" class:has-blockers={hasBlockers}>
					{#if blockerCount > 0}
						<span class="count blocker">{blockerCount} blocker{blockerCount > 1 ? 's' : ''}</span>
					{/if}
					{#if issueCount > 0}
						<span class="count issue">{issueCount} issue{issueCount > 1 ? 's' : ''}</span>
					{/if}
					{#if suggestionCount > 0}
						<span class="count suggestion">{suggestionCount} suggestion{suggestionCount > 1 ? 's' : ''}</span>
					{/if}
				</div>

				<button
					class="send-to-agent-btn"
					onclick={handleSendToAgent}
					disabled={sendingToAgent}
				>
					{#if sendingToAgent}
						<span class="spinner"></span>
						Sending...
					{:else}
						<Icon name="play" size={14} />
						Send to Agent
					{/if}
				</button>
			{/if}

			{#if diff}
				<DiffStats stats={diff.stats} />
			{/if}
		</div>
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
					{comments}
					activeLineNumber={activeFilePath === file.path ? activeLineNumber : null}
					onToggle={() => toggleFile(file.path)}
					onLineClick={handleLineClick}
					onAddComment={handleAddComment}
					onResolveComment={handleResolveComment}
					onWontFixComment={handleWontFixComment}
					onDeleteComment={handleDeleteComment}
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
		gap: var(--space-4);
		flex-wrap: wrap;
	}

	.toolbar-left {
		display: flex;
		align-items: center;
		gap: var(--space-3);
	}

	.toolbar-right {
		display: flex;
		align-items: center;
		gap: var(--space-3);
	}

	.review-summary {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-1) var(--space-2);
		background: var(--bg-tertiary);
		border-radius: var(--radius-md);
		font-size: var(--text-xs);
	}

	.review-summary .count {
		font-weight: var(--font-medium);
	}

	.review-summary .count.blocker {
		color: var(--status-danger);
	}

	.review-summary .count.issue {
		color: var(--status-warning);
	}

	.review-summary .count.suggestion {
		color: var(--status-info);
	}

	.send-to-agent-btn {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		padding: var(--space-1-5) var(--space-3);
		background: var(--accent-primary);
		border: none;
		border-radius: var(--radius-md);
		color: var(--text-inverse);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		cursor: pointer;
		transition: background var(--duration-fast) var(--ease-out);
	}

	.send-to-agent-btn:hover:not(:disabled) {
		background: var(--accent-primary-hover);
	}

	.send-to-agent-btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.send-to-agent-btn .spinner {
		width: 12px;
		height: 12px;
		border: 2px solid rgba(255, 255, 255, 0.3);
		border-top-color: white;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
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
