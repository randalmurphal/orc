<script lang="ts">
	import type { ReviewComment, CommentSeverity, CommentStatus } from '$lib/types';
	import Icon from '$lib/components/ui/Icon.svelte';
	import { formatRelativeTime } from '$lib/utils/format';

	interface Props {
		comment: ReviewComment;
		onResolve?: (id: string) => void;
		onWontFix?: (id: string) => void;
		onDelete?: (id: string) => void;
	}

	let { comment, onResolve, onWontFix, onDelete }: Props = $props();

	const severityConfig: Record<CommentSeverity, { color: string; bg: string; label: string }> = {
		suggestion: {
			color: 'var(--status-info)',
			bg: 'var(--status-info-bg)',
			label: 'Suggestion'
		},
		issue: {
			color: 'var(--status-warning)',
			bg: 'var(--status-warning-bg)',
			label: 'Issue'
		},
		blocker: {
			color: 'var(--status-danger)',
			bg: 'var(--status-danger-bg)',
			label: 'Blocker'
		}
	};

	const statusConfig: Record<CommentStatus, { color: string; label: string }> = {
		open: {
			color: 'var(--text-secondary)',
			label: 'Open'
		},
		resolved: {
			color: 'var(--status-success)',
			label: 'Resolved'
		},
		wont_fix: {
			color: 'var(--text-muted)',
			label: "Won't Fix"
		}
	};

	const severity = $derived(severityConfig[comment.severity]);
	const status = $derived(statusConfig[comment.status]);
	const isOpen = $derived(comment.status === 'open');
	const hasLocation = $derived(comment.file_path !== undefined);

	function handleResolve() {
		onResolve?.(comment.id);
	}

	function handleWontFix() {
		onWontFix?.(comment.id);
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

<div class="comment-thread" class:resolved={!isOpen}>
	<div class="comment-header">
		<div class="severity-badge" style:background={severity.bg} style:color={severity.color}>
			{severity.label}
		</div>
		{#if !isOpen}
			<div class="status-badge" style:color={status.color}>
				{status.label}
			</div>
		{/if}
		<span class="timestamp">{formatRelativeTime(comment.created_at)}</span>
	</div>

	{#if hasLocation}
		<div class="location">
			<Icon name="file" size={14} />
			<span class="file-path">{comment.file_path}</span>
			{#if comment.line_number}
				<span class="line-number">:{comment.line_number}</span>
			{/if}
		</div>
	{/if}

	<div class="comment-content">
		{comment.content}
	</div>

	{#if comment.resolved_at}
		<div class="resolution-info">
			<Icon name="check" size={12} />
			<span>
				{status.label} {formatRelativeTime(comment.resolved_at)}
				{#if comment.resolved_by}
					by {comment.resolved_by}
				{/if}
			</span>
		</div>
	{/if}

	{#if isOpen}
		<div class="comment-actions">
			<button
				class="action-btn resolve"
				onclick={handleResolve}
				onkeydown={(e) => handleKeyDown(e, handleResolve)}
				title="Mark as resolved"
			>
				<Icon name="check" size={14} />
				Resolve
			</button>
			<button
				class="action-btn wont-fix"
				onclick={handleWontFix}
				onkeydown={(e) => handleKeyDown(e, handleWontFix)}
				title="Mark as won't fix"
			>
				<Icon name="close" size={14} />
				Won't Fix
			</button>
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
		transition: opacity var(--duration-normal) var(--ease-out);
	}

	.comment-thread.resolved {
		opacity: 0.7;
	}

	.comment-thread.resolved:hover {
		opacity: 1;
	}

	.comment-header {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		flex-wrap: wrap;
	}

	.severity-badge {
		display: inline-flex;
		align-items: center;
		padding: var(--space-0-5) var(--space-2);
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		border-radius: var(--radius-sm);
	}

	.status-badge {
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
	}

	.timestamp {
		margin-left: auto;
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.location {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		font-size: var(--text-sm);
		color: var(--text-secondary);
		font-family: var(--font-mono);
	}

	.file-path {
		color: var(--accent-primary);
	}

	.line-number {
		color: var(--text-muted);
	}

	.comment-content {
		font-size: var(--text-sm);
		color: var(--text-primary);
		line-height: var(--leading-relaxed);
		white-space: pre-wrap;
		word-break: break-word;
	}

	.resolution-info {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		font-size: var(--text-xs);
		color: var(--status-success);
	}

	.comment-actions {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding-top: var(--space-2);
		border-top: 1px solid var(--border-subtle);
		margin-top: var(--space-1);
	}

	.action-btn {
		display: inline-flex;
		align-items: center;
		gap: var(--space-1);
		padding: var(--space-1) var(--space-2);
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

	.action-btn.resolve:hover {
		color: var(--status-success);
		border-color: var(--status-success);
	}

	.action-btn.wont-fix:hover {
		color: var(--text-muted);
	}

	.action-btn.delete {
		margin-left: auto;
		padding: var(--space-1);
	}

	.action-btn.delete:hover {
		color: var(--status-danger);
		border-color: var(--status-danger);
	}
</style>
