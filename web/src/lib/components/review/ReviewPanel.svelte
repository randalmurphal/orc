<script lang="ts">
	import type { ReviewComment, CreateCommentRequest, CommentSeverity } from '$lib/types';
	import {
		getReviewComments,
		createReviewComment,
		updateReviewComment,
		deleteReviewComment,
		triggerReviewRetry
	} from '$lib/api';
	import { toast } from '$lib/stores/toast.svelte';
	import Icon from '$lib/components/ui/Icon.svelte';
	import CommentThread from './CommentThread.svelte';
	import CommentForm from './CommentForm.svelte';
	import ReviewSummary from './ReviewSummary.svelte';

	interface Props {
		taskId: string;
	}

	let { taskId }: Props = $props();

	type FilterValue = 'all' | CommentSeverity;

	let comments = $state<ReviewComment[]>([]);
	let isLoading = $state(true);
	let isSubmitting = $state(false);
	let isRetrying = $state(false);
	let showForm = $state(false);
	let filter = $state<FilterValue>('all');
	let selectedCommentId = $state<string | null>(null);

	const openComments = $derived(comments.filter((c) => c.status === 'open'));
	const resolvedComments = $derived(comments.filter((c) => c.status !== 'open'));

	const filteredComments = $derived.by(() => {
		if (filter === 'all') return openComments;
		return openComments.filter((c) => c.severity === filter);
	});

	// Group comments by file
	const commentsByFile = $derived.by(() => {
		const byFile: Map<string, ReviewComment[]> = new Map();
		const general: ReviewComment[] = [];

		for (const c of filteredComments) {
			if (c.file_path) {
				const existing = byFile.get(c.file_path) ?? [];
				existing.push(c);
				byFile.set(c.file_path, existing);
			} else {
				general.push(c);
			}
		}

		return { byFile, general };
	});

	const hasBlockers = $derived(openComments.some((c) => c.severity === 'blocker'));
	const canRetry = $derived(openComments.length > 0 && !isRetrying);

	async function loadComments() {
		isLoading = true;
		try {
			comments = await getReviewComments(taskId);
		} catch (error) {
			toast.error('Failed to load comments');
			console.error('Failed to load comments:', error);
		} finally {
			isLoading = false;
		}
	}

	async function handleCreateComment(data: CreateCommentRequest) {
		isSubmitting = true;
		try {
			const newComment = await createReviewComment(taskId, data);
			comments = [...comments, newComment];
			showForm = false;
			toast.success('Comment added');
		} catch (error) {
			toast.error('Failed to add comment');
			console.error('Failed to create comment:', error);
		} finally {
			isSubmitting = false;
		}
	}

	async function handleResolve(id: string) {
		try {
			const updated = await updateReviewComment(taskId, id, { status: 'resolved' });
			comments = comments.map((c) => (c.id === id ? updated : c));
			toast.success('Comment resolved');
		} catch (error) {
			toast.error('Failed to resolve comment');
			console.error('Failed to resolve comment:', error);
		}
	}

	async function handleWontFix(id: string) {
		try {
			const updated = await updateReviewComment(taskId, id, { status: 'wont_fix' });
			comments = comments.map((c) => (c.id === id ? updated : c));
			toast.info("Marked as won't fix");
		} catch (error) {
			toast.error('Failed to update comment');
			console.error('Failed to update comment:', error);
		}
	}

	async function handleDelete(id: string) {
		try {
			await deleteReviewComment(taskId, id);
			comments = comments.filter((c) => c.id !== id);
			toast.success('Comment deleted');
		} catch (error) {
			toast.error('Failed to delete comment');
			console.error('Failed to delete comment:', error);
		}
	}

	async function handleRetry() {
		if (!canRetry) return;

		isRetrying = true;
		try {
			await triggerReviewRetry(taskId);
			toast.success('Retry started with review comments');
		} catch (error) {
			toast.error('Failed to start retry');
			console.error('Failed to trigger retry:', error);
		} finally {
			isRetrying = false;
		}
	}

	function handleCommentClick(comment: ReviewComment) {
		selectedCommentId = comment.id;
	}

	function handleKeyDown(event: KeyboardEvent) {
		if (event.target instanceof HTMLInputElement || event.target instanceof HTMLTextAreaElement) {
			return;
		}

		switch (event.key) {
			case 'j':
				event.preventDefault();
				if (filteredComments.length > 0) {
					const currentIdx = selectedCommentId
						? filteredComments.findIndex((c) => c.id === selectedCommentId)
						: -1;
					const nextIdx = Math.min(currentIdx + 1, filteredComments.length - 1);
					selectedCommentId = filteredComments[nextIdx].id;
				}
				break;
			case 'k':
				event.preventDefault();
				if (filteredComments.length > 0) {
					const currentIdx = selectedCommentId
						? filteredComments.findIndex((c) => c.id === selectedCommentId)
						: filteredComments.length;
					const prevIdx = Math.max(currentIdx - 1, 0);
					selectedCommentId = filteredComments[prevIdx].id;
				}
				break;
			case 'n':
				event.preventDefault();
				showForm = true;
				break;
			case 'Escape':
				if (showForm) {
					event.preventDefault();
					showForm = false;
				}
				break;
		}
	}

	// Load comments on mount and when taskId changes
	$effect(() => {
		// Explicitly track taskId as dependency
		const currentTaskId = taskId;
		if (currentTaskId) {
			loadComments();
		}
	});
</script>

<svelte:window onkeydown={handleKeyDown} />

<div class="review-panel">
	<div class="panel-header">
		<div class="header-title">
			<Icon name="file" size={18} />
			<h2>Review</h2>
			{#if openComments.length > 0}
				<span class="comment-count">{openComments.length}</span>
			{/if}
		</div>
		<div class="header-actions">
			<button class="icon-btn sm" onclick={() => (showForm = !showForm)} title="Add comment (n)">
				<Icon name="plus" size={16} />
			</button>
		</div>
	</div>

	{#if isLoading}
		<div class="loading-state">
			<div class="skeleton skeleton-text"></div>
			<div class="skeleton skeleton-text"></div>
			<div class="skeleton skeleton-text" style="width: 60%;"></div>
		</div>
	{:else}
		<ReviewSummary comments={comments} onCommentClick={handleCommentClick} />

		{#if showForm}
			<div class="form-container">
				<CommentForm
					onSubmit={handleCreateComment}
					onCancel={() => (showForm = false)}
					isLoading={isSubmitting}
				/>
			</div>
		{/if}

		<div class="filter-bar">
			<div class="filter-tabs">
				<button
					class="filter-tab"
					class:active={filter === 'all'}
					onclick={() => (filter = 'all')}
				>
					All
					<span class="filter-count">{openComments.length}</span>
				</button>
				<button
					class="filter-tab blocker"
					class:active={filter === 'blocker'}
					onclick={() => (filter = 'blocker')}
				>
					Blockers
					<span class="filter-count">{openComments.filter((c) => c.severity === 'blocker').length}</span>
				</button>
				<button
					class="filter-tab issue"
					class:active={filter === 'issue'}
					onclick={() => (filter = 'issue')}
				>
					Issues
					<span class="filter-count">{openComments.filter((c) => c.severity === 'issue').length}</span>
				</button>
				<button
					class="filter-tab suggestion"
					class:active={filter === 'suggestion'}
					onclick={() => (filter = 'suggestion')}
				>
					Suggestions
					<span class="filter-count">{openComments.filter((c) => c.severity === 'suggestion').length}</span>
				</button>
			</div>
		</div>

		<div class="comments-container">
			{#if filteredComments.length === 0}
				<div class="empty-state">
					{#if filter === 'all'}
						<Icon name="check" size={32} />
						<p>No open comments</p>
						<button class="ghost" onclick={() => (showForm = true)}>Add a comment</button>
					{:else}
						<p>No {filter}s found</p>
					{/if}
				</div>
			{:else}
				{#if commentsByFile.general.length > 0}
					<div class="file-section">
						<div class="file-header">
							<Icon name="file" size={14} />
							<span>General</span>
						</div>
						<div class="comments-list">
							{#each commentsByFile.general as comment}
								<div class="comment-wrapper" class:selected={selectedCommentId === comment.id}>
									<CommentThread
										{comment}
										onResolve={handleResolve}
										onWontFix={handleWontFix}
										onDelete={handleDelete}
									/>
								</div>
							{/each}
						</div>
					</div>
				{/if}

				{#each [...commentsByFile.byFile] as [filePath, fileComments]}
					<div class="file-section">
						<div class="file-header">
							<Icon name="file" size={14} />
							<span class="file-path">{filePath}</span>
						</div>
						<div class="comments-list">
							{#each fileComments as comment}
								<div class="comment-wrapper" class:selected={selectedCommentId === comment.id}>
									<CommentThread
										{comment}
										onResolve={handleResolve}
										onWontFix={handleWontFix}
										onDelete={handleDelete}
									/>
								</div>
							{/each}
						</div>
					</div>
				{/each}
			{/if}

			{#if resolvedComments.length > 0}
				<details class="resolved-section">
					<summary>
						<Icon name="check" size={14} />
						<span>Resolved ({resolvedComments.length})</span>
					</summary>
					<div class="comments-list resolved">
						{#each resolvedComments as comment}
							<CommentThread {comment} onDelete={handleDelete} />
						{/each}
					</div>
				</details>
			{/if}
		</div>

		{#if openComments.length > 0}
			<div class="panel-footer">
				<button
					class="primary retry-btn"
					class:has-blockers={hasBlockers}
					onclick={handleRetry}
					disabled={!canRetry}
				>
					{#if isRetrying}
						<span class="spinner"></span>
						Sending to Agent...
					{:else}
						<Icon name="play" size={14} />
						Send {openComments.length} Comment{openComments.length !== 1 ? 's' : ''} to Agent
					{/if}
				</button>
				{#if hasBlockers}
					<p class="blocker-warning">
						<Icon name="warning" size={12} />
						Contains blockers that must be addressed
					</p>
				{/if}
			</div>
		{/if}
	{/if}
</div>

<style>
	.review-panel {
		display: flex;
		flex-direction: column;
		height: 100%;
		background: var(--bg-primary);
	}

	.panel-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--space-3) var(--space-4);
		border-bottom: 1px solid var(--border-subtle);
	}

	.header-title {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.header-title h2 {
		font-size: var(--text-sm);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		text-transform: none;
		letter-spacing: normal;
	}

	.comment-count {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 20px;
		height: 20px;
		padding: 0 var(--space-1-5);
		background: var(--accent-subtle);
		color: var(--accent-primary);
		font-size: var(--text-xs);
		font-weight: var(--font-semibold);
		border-radius: var(--radius-full);
	}

	.header-actions {
		display: flex;
		gap: var(--space-1);
	}

	.loading-state {
		padding: var(--space-4);
	}

	.form-container {
		padding: var(--space-4);
		border-bottom: 1px solid var(--border-subtle);
	}

	.filter-bar {
		padding: var(--space-2) var(--space-4);
		border-bottom: 1px solid var(--border-subtle);
		overflow-x: auto;
	}

	.filter-tabs {
		display: flex;
		gap: var(--space-1);
	}

	.filter-tab {
		display: inline-flex;
		align-items: center;
		gap: var(--space-1);
		padding: var(--space-1) var(--space-2);
		background: transparent;
		border: 1px solid transparent;
		border-radius: var(--radius-sm);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		color: var(--text-muted);
		cursor: pointer;
		white-space: nowrap;
		transition:
			background var(--duration-fast) var(--ease-out),
			color var(--duration-fast) var(--ease-out),
			border-color var(--duration-fast) var(--ease-out);
	}

	.filter-tab:hover {
		background: var(--bg-tertiary);
		color: var(--text-secondary);
	}

	.filter-tab.active {
		background: var(--bg-tertiary);
		border-color: var(--border-default);
		color: var(--text-primary);
	}

	.filter-tab.blocker.active {
		border-color: var(--status-danger);
		color: var(--status-danger);
	}

	.filter-tab.issue.active {
		border-color: var(--status-warning);
		color: var(--status-warning);
	}

	.filter-tab.suggestion.active {
		border-color: var(--status-info);
		color: var(--status-info);
	}

	.filter-count {
		font-size: var(--text-2xs);
		opacity: 0.7;
	}

	.comments-container {
		flex: 1;
		overflow-y: auto;
		padding: var(--space-4);
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	.empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: var(--space-8);
		text-align: center;
		color: var(--text-muted);
		gap: var(--space-2);
	}

	.file-section {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.file-header {
		display: flex;
		align-items: center;
		gap: var(--space-1-5);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
	}

	.file-path {
		font-family: var(--font-mono);
		text-transform: none;
		letter-spacing: normal;
		color: var(--accent-primary);
	}

	.comments-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.comment-wrapper {
		border-radius: var(--radius-lg);
		transition: box-shadow var(--duration-fast) var(--ease-out);
	}

	.comment-wrapper.selected {
		box-shadow: 0 0 0 2px var(--accent-primary);
	}

	.resolved-section {
		border-top: 1px solid var(--border-subtle);
		padding-top: var(--space-3);
	}

	.resolved-section summary {
		display: flex;
		align-items: center;
		gap: var(--space-1-5);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		color: var(--text-muted);
		cursor: pointer;
		padding: var(--space-1) 0;
	}

	.resolved-section summary:hover {
		color: var(--text-secondary);
	}

	.resolved-section[open] summary {
		margin-bottom: var(--space-2);
	}

	.comments-list.resolved {
		opacity: 0.7;
	}

	.panel-footer {
		padding: var(--space-4);
		border-top: 1px solid var(--border-subtle);
		background: var(--bg-secondary);
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.retry-btn {
		width: 100%;
		justify-content: center;
	}

	.retry-btn.has-blockers {
		background: var(--status-danger);
		border-color: var(--status-danger);
	}

	.retry-btn.has-blockers:hover {
		background: #dc2626;
		border-color: #dc2626;
	}

	.blocker-warning {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-1);
		font-size: var(--text-xs);
		color: var(--status-danger);
	}

	.spinner {
		width: 14px;
		height: 14px;
		border: 2px solid transparent;
		border-top-color: currentColor;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
