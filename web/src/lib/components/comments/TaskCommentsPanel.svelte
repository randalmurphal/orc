<script lang="ts">
	import type { TaskComment, CreateTaskCommentRequest, TaskCommentAuthorType } from '$lib/types';
	import { getTaskComments, createTaskComment, deleteTaskComment, updateTaskComment } from '$lib/api';
	import TaskCommentThread from './TaskCommentThread.svelte';
	import TaskCommentForm from './TaskCommentForm.svelte';
	import Icon from '$lib/components/ui/Icon.svelte';

	interface Props {
		taskId: string;
		phases?: string[];
	}

	let { taskId, phases = [] }: Props = $props();

	let comments = $state<TaskComment[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let showForm = $state(false);
	let isSubmitting = $state(false);
	let filterAuthorType = $state<TaskCommentAuthorType | ''>('');
	let editingCommentId = $state<string | null>(null);

	// Filtered comments
	const filteredComments = $derived.by(() => {
		if (!filterAuthorType) return comments;
		return comments.filter(c => c.author_type === filterAuthorType);
	});

	// Comments by author type
	const humanComments = $derived(comments.filter(c => c.author_type === 'human'));
	const agentComments = $derived(comments.filter(c => c.author_type === 'agent'));
	const systemComments = $derived(comments.filter(c => c.author_type === 'system'));

	// Load comments on mount and when taskId changes
	$effect(() => {
		if (taskId) {
			loadComments();
		}
	});

	async function loadComments() {
		loading = true;
		error = null;
		try {
			comments = await getTaskComments(taskId);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load comments';
		} finally {
			loading = false;
		}
	}

	async function handleSubmit(comment: CreateTaskCommentRequest) {
		isSubmitting = true;
		try {
			if (editingCommentId) {
				// Update existing comment
				await updateTaskComment(taskId, editingCommentId, {
					content: comment.content,
					phase: comment.phase
				});
				editingCommentId = null;
			} else {
				// Create new comment
				await createTaskComment(taskId, comment);
			}
			showForm = false;
			await loadComments();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save comment';
		} finally {
			isSubmitting = false;
		}
	}

	function handleCancel() {
		showForm = false;
		editingCommentId = null;
	}

	function handleEdit(commentId: string) {
		editingCommentId = commentId;
		showForm = true;
	}

	async function handleDelete(commentId: string) {
		if (!confirm('Delete this comment?')) return;

		try {
			await deleteTaskComment(taskId, commentId);
			await loadComments();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete comment';
		}
	}

	function handleAddComment() {
		editingCommentId = null;
		showForm = true;
	}

	// Get editing comment for form
	const editingComment = $derived(
		editingCommentId ? comments.find(c => c.id === editingCommentId) : null
	);
</script>

<div class="comments-panel">
	<div class="panel-header">
		<div class="header-left">
			<h3>
				<Icon name="message-square" size={16} />
				Comments
				{#if comments.length > 0}
					<span class="comment-count">{comments.length}</span>
				{/if}
			</h3>
		</div>
		<div class="header-right">
			{#if !showForm}
				<button class="add-btn" onclick={handleAddComment}>
					<Icon name="plus" size={14} />
					Add Comment
				</button>
			{/if}
		</div>
	</div>

	{#if error}
		<div class="error-message">
			<Icon name="alert-circle" size={14} />
			{error}
			<button onclick={loadComments}>Retry</button>
		</div>
	{/if}

	{#if showForm}
		<TaskCommentForm
			{phases}
			onSubmit={handleSubmit}
			onCancel={handleCancel}
			isLoading={isSubmitting}
			editMode={!!editingCommentId}
			initialContent={editingComment?.content ?? ''}
			initialPhase={editingComment?.phase}
		/>
	{/if}

	{#if comments.length > 0 && !showForm}
		<div class="filter-bar">
			<span class="filter-label">Filter:</span>
			<button
				class="filter-btn"
				class:active={filterAuthorType === ''}
				onclick={() => filterAuthorType = ''}
			>
				All ({comments.length})
			</button>
			{#if humanComments.length > 0}
				<button
					class="filter-btn human"
					class:active={filterAuthorType === 'human'}
					onclick={() => filterAuthorType = 'human'}
				>
					Human ({humanComments.length})
				</button>
			{/if}
			{#if agentComments.length > 0}
				<button
					class="filter-btn agent"
					class:active={filterAuthorType === 'agent'}
					onclick={() => filterAuthorType = 'agent'}
				>
					Agent ({agentComments.length})
				</button>
			{/if}
			{#if systemComments.length > 0}
				<button
					class="filter-btn system"
					class:active={filterAuthorType === 'system'}
					onclick={() => filterAuthorType = 'system'}
				>
					System ({systemComments.length})
				</button>
			{/if}
		</div>
	{/if}

	{#if loading}
		<div class="loading-state">
			<div class="spinner"></div>
			<span>Loading comments...</span>
		</div>
	{:else if comments.length === 0 && !showForm}
		<div class="empty-state">
			<Icon name="message-square" size={32} />
			<p>No comments yet</p>
			<span>Add comments to track feedback, notes, and context.</span>
		</div>
	{:else if !showForm}
		<div class="comments-list">
			{#each filteredComments as comment (comment.id)}
				<TaskCommentThread
					{comment}
					onEdit={handleEdit}
					onDelete={handleDelete}
				/>
			{/each}
		</div>
	{/if}
</div>

<style>
	.comments-panel {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
		background: var(--bg-primary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-lg);
		padding: var(--space-4);
	}

	.panel-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.header-left h3 {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		font-size: var(--text-sm);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		margin: 0;
	}

	.comment-count {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 20px;
		height: 20px;
		padding: 0 var(--space-1-5);
		background: var(--accent-glow);
		color: var(--accent-primary);
		font-size: var(--text-xs);
		font-weight: var(--font-semibold);
		border-radius: var(--radius-full);
	}

	.add-btn {
		display: inline-flex;
		align-items: center;
		gap: var(--space-1);
		padding: var(--space-1-5) var(--space-3);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		background: var(--accent-primary);
		border: none;
		border-radius: var(--radius-md);
		color: white;
		cursor: pointer;
		transition: background var(--duration-fast) var(--ease-out);
	}

	.add-btn:hover {
		background: var(--accent-hover);
	}

	.error-message {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-3);
		background: var(--status-danger-bg);
		border: 1px solid var(--status-danger);
		border-radius: var(--radius-md);
		color: var(--status-danger);
		font-size: var(--text-sm);
	}

	.error-message button {
		margin-left: auto;
		padding: var(--space-1) var(--space-2);
		background: transparent;
		border: 1px solid var(--status-danger);
		border-radius: var(--radius-sm);
		color: var(--status-danger);
		font-size: var(--text-xs);
		cursor: pointer;
	}

	.filter-bar {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		flex-wrap: wrap;
	}

	.filter-label {
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
	}

	.filter-btn {
		padding: var(--space-1) var(--space-2);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-secondary);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.filter-btn:hover {
		border-color: var(--border-strong);
		color: var(--text-primary);
	}

	.filter-btn.active {
		background: var(--accent-glow);
		border-color: var(--accent-primary);
		color: var(--accent-primary);
	}

	.filter-btn.human.active {
		background: var(--status-info-bg);
		border-color: var(--status-info);
		color: var(--status-info);
	}

	.filter-btn.agent.active {
		background: var(--accent-glow);
		border-color: var(--accent-primary);
		color: var(--accent-primary);
	}

	.filter-btn.system.active {
		background: var(--bg-tertiary);
		border-color: var(--text-muted);
		color: var(--text-secondary);
	}

	.loading-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-8);
		color: var(--text-muted);
	}

	.spinner {
		width: 24px;
		height: 24px;
		border: 2px solid var(--border-default);
		border-top-color: var(--accent-primary);
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}

	.empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-8);
		color: var(--text-muted);
		text-align: center;
	}

	.empty-state p {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-secondary);
		margin: 0;
	}

	.empty-state span {
		font-size: var(--text-xs);
	}

	.comments-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
	}
</style>
