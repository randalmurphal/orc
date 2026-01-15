<script lang="ts">
	import type { TaskComment, TaskCommentAuthorType } from '$lib/types';
	import Icon from '$lib/components/ui/Icon.svelte';
	import { formatRelativeTime } from '$lib/utils/format';

	interface Props {
		comment: TaskComment;
		onEdit?: (id: string) => void;
		onDelete?: (id: string) => void;
	}

	let { comment, onEdit, onDelete }: Props = $props();

	const authorTypeConfig: Record<TaskCommentAuthorType, { color: string; bg: string; label: string; icon: string }> = {
		human: {
			color: 'var(--status-info)',
			bg: 'var(--status-info-bg)',
			label: 'Human',
			icon: 'user'
		},
		agent: {
			color: 'var(--accent-primary)',
			bg: 'var(--accent-glow)',
			label: 'Agent',
			icon: 'cpu'
		},
		system: {
			color: 'var(--text-muted)',
			bg: 'var(--bg-tertiary)',
			label: 'System',
			icon: 'settings'
		}
	};

	const authorType = $derived(authorTypeConfig[comment.author_type]);

	function handleEdit() {
		onEdit?.(comment.id);
	}

	function handleDelete() {
		onDelete?.(comment.id);
	}

	function handleKeyDown(event: KeyboardEvent, action: () => void) {
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			action();
		}
	}
</script>

<div class="comment-thread">
	<div class="comment-header">
		<div class="author-badge" style:background={authorType.bg} style:color={authorType.color}>
			<Icon name={authorType.icon} size={12} />
			<span>{comment.author || authorType.label}</span>
		</div>
		{#if comment.phase}
			<div class="phase-badge">
				<Icon name="layers" size={12} />
				<span>{comment.phase}</span>
			</div>
		{/if}
		<span class="timestamp">{formatRelativeTime(comment.created_at)}</span>
	</div>

	<div class="comment-content">
		{comment.content}
	</div>

	{#if comment.updated_at !== comment.created_at}
		<div class="edited-info">
			<span>edited {formatRelativeTime(comment.updated_at)}</span>
		</div>
	{/if}

	{#if onEdit || onDelete}
		<div class="comment-actions">
			{#if onEdit}
				<button
					class="action-btn edit"
					onclick={handleEdit}
					onkeydown={(e) => handleKeyDown(e, handleEdit)}
					title="Edit comment"
				>
					<Icon name="edit" size={14} />
				</button>
			{/if}
			{#if onDelete}
				<button
					class="action-btn delete"
					onclick={handleDelete}
					onkeydown={(e) => handleKeyDown(e, handleDelete)}
					title="Delete comment"
				>
					<Icon name="trash" size={14} />
				</button>
			{/if}
		</div>
	{/if}
</div>

<style>
	.comment-thread {
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-lg);
		padding: var(--space-3);
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
		transition: border-color var(--duration-fast) var(--ease-out);
	}

	.comment-thread:hover {
		border-color: var(--border-strong);
	}

	.comment-header {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		flex-wrap: wrap;
	}

	.author-badge {
		display: inline-flex;
		align-items: center;
		gap: var(--space-1);
		padding: var(--space-0-5) var(--space-2);
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		border-radius: var(--radius-sm);
	}

	.phase-badge {
		display: inline-flex;
		align-items: center;
		gap: var(--space-1);
		padding: var(--space-0-5) var(--space-2);
		font-size: var(--text-2xs);
		font-weight: var(--font-medium);
		color: var(--text-secondary);
		background: var(--bg-tertiary);
		border-radius: var(--radius-sm);
	}

	.timestamp {
		margin-left: auto;
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.comment-content {
		font-size: var(--text-sm);
		color: var(--text-primary);
		line-height: var(--leading-relaxed);
		white-space: pre-wrap;
		word-break: break-word;
	}

	.edited-info {
		font-size: var(--text-xs);
		color: var(--text-muted);
		font-style: italic;
	}

	.comment-actions {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding-top: var(--space-2);
		border-top: 1px solid var(--border-subtle);
		margin-top: var(--space-1);
		justify-content: flex-end;
	}

	.action-btn {
		display: inline-flex;
		align-items: center;
		gap: var(--space-1);
		padding: var(--space-1);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		border-radius: var(--radius-sm);
		background: transparent;
		border: 1px solid var(--border-default);
		color: var(--text-secondary);
		cursor: pointer;
		transition:
			background var(--duration-fast) var(--ease-out),
			color var(--duration-fast) var(--ease-out),
			border-color var(--duration-fast) var(--ease-out);
	}

	.action-btn:hover {
		background: var(--bg-tertiary);
		border-color: var(--border-strong);
		color: var(--text-primary);
	}

	.action-btn:focus-visible {
		outline: none;
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 3px var(--accent-glow);
	}

	.action-btn.edit:hover {
		color: var(--accent-primary);
		border-color: var(--accent-primary);
	}

	.action-btn.delete:hover {
		color: var(--status-danger);
		border-color: var(--status-danger);
	}
</style>
