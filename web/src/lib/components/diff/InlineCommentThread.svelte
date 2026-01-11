<script lang="ts">
	import type { ReviewComment, CreateCommentRequest, CommentSeverity } from '$lib/types';
	import Icon from '$lib/components/ui/Icon.svelte';
	import { formatRelativeTime } from '$lib/utils/format';

	interface Props {
		comments: ReviewComment[];
		filePath: string;
		lineNumber: number;
		isActive?: boolean;
		onAddComment: (comment: CreateCommentRequest) => Promise<void>;
		onResolve: (id: string) => void;
		onWontFix: (id: string) => void;
		onDelete: (id: string) => void;
		onClose?: () => void;
	}

	let { comments, filePath, lineNumber, isActive = false, onAddComment, onResolve, onWontFix, onDelete, onClose }: Props = $props();

	let showForm = $state(false);

	// Auto-open form when component becomes active (user clicked line to add comment)
	$effect(() => {
		if (isActive && comments.length === 0) {
			showForm = true;
		}
	});
	let isSubmitting = $state(false);
	let content = $state('');
	let severity = $state<CommentSeverity>('issue');

	const openComments = $derived(comments.filter(c => c.status === 'open'));
	const resolvedComments = $derived(comments.filter(c => c.status !== 'open'));

	const severityConfig: Record<CommentSeverity, { color: string; bg: string; label: string }> = {
		suggestion: { color: 'var(--status-info)', bg: 'var(--status-info-bg)', label: 'Suggestion' },
		issue: { color: 'var(--status-warning)', bg: 'var(--status-warning-bg)', label: 'Issue' },
		blocker: { color: 'var(--status-danger)', bg: 'var(--status-danger-bg)', label: 'Blocker' }
	};

	async function handleSubmit() {
		if (!content.trim() || isSubmitting) return;

		isSubmitting = true;
		try {
			await onAddComment({
				file_path: filePath,
				line_number: lineNumber,
				content: content.trim(),
				severity
			});
			content = '';
			severity = 'issue';
			showForm = false;
			onClose?.();
		} finally {
			isSubmitting = false;
		}
	}

	function handleKeyDown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			handleCancel();
		} else if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
			handleSubmit();
		}
	}

	function handleCancel() {
		showForm = false;
		content = '';
		severity = 'issue';
		onClose?.();
	}

	export function openCommentForm() {
		showForm = true;
	}
</script>

{#if comments.length > 0 || showForm}
	<div class="inline-thread">
		<!-- Open comments -->
		{#each openComments as comment (comment.id)}
			<div class="inline-comment">
				<div class="comment-header">
					<span
						class="severity-badge"
						style:background={severityConfig[comment.severity].bg}
						style:color={severityConfig[comment.severity].color}
					>
						{severityConfig[comment.severity].label}
					</span>
					<span class="timestamp">{formatRelativeTime(comment.created_at)}</span>
				</div>
				<div class="comment-content">{comment.content}</div>
				<div class="comment-actions">
					<button class="action-btn resolve" onclick={() => onResolve(comment.id)}>
						<Icon name="check" size={12} />
						Resolve
					</button>
					<button class="action-btn wont-fix" onclick={() => onWontFix(comment.id)}>
						Won't Fix
					</button>
					<button class="action-btn delete" onclick={() => onDelete(comment.id)}>
						<Icon name="trash" size={12} />
					</button>
				</div>
			</div>
		{/each}

		<!-- Resolved comments (collapsed) -->
		{#if resolvedComments.length > 0}
			<div class="resolved-count">
				{resolvedComments.length} resolved comment{resolvedComments.length > 1 ? 's' : ''}
			</div>
		{/if}

		<!-- Add comment form -->
		{#if showForm}
			<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
			<form class="inline-form" onsubmit={(e) => { e.preventDefault(); handleSubmit(); }} onkeydown={handleKeyDown}>
				<div class="severity-pills">
					{#each (['suggestion', 'issue', 'blocker'] as const) as sev}
						<button
							type="button"
							class="severity-pill"
							class:selected={severity === sev}
							style:--pill-color={severityConfig[sev].color}
							style:--pill-bg={severityConfig[sev].bg}
							onclick={() => severity = sev}
						>
							{severityConfig[sev].label}
						</button>
					{/each}
				</div>
				<textarea
					bind:value={content}
					placeholder="Add a comment..."
					rows="2"
					disabled={isSubmitting}
					aria-label="Review comment"
				></textarea>
				<div class="form-actions">
					<button type="button" class="cancel-btn" onclick={handleCancel}>
						Cancel
					</button>
					<button type="submit" class="submit-btn" disabled={!content.trim() || isSubmitting}>
						{isSubmitting ? 'Adding...' : 'Add'}
					</button>
				</div>
			</form>
		{/if}
	</div>
{/if}

<style>
	.inline-thread {
		margin-left: 48px;
		padding: var(--space-2) var(--space-3);
		background: var(--bg-secondary);
		border-left: 3px solid var(--accent-primary);
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.inline-comment {
		background: var(--bg-tertiary);
		border-radius: var(--radius-md);
		padding: var(--space-2);
		display: flex;
		flex-direction: column;
		gap: var(--space-1-5);
	}

	.comment-header {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.severity-badge {
		padding: var(--space-0-5) var(--space-1-5);
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		border-radius: var(--radius-sm);
	}

	.timestamp {
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.comment-content {
		font-size: var(--text-sm);
		color: var(--text-primary);
		line-height: var(--leading-relaxed);
		white-space: pre-wrap;
	}

	.comment-actions {
		display: flex;
		gap: var(--space-2);
		padding-top: var(--space-1);
	}

	.action-btn {
		display: inline-flex;
		align-items: center;
		gap: var(--space-1);
		padding: var(--space-0-5) var(--space-1-5);
		font-size: var(--text-2xs);
		font-weight: var(--font-medium);
		background: transparent;
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-sm);
		color: var(--text-muted);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.action-btn:hover {
		background: var(--bg-surface);
		border-color: var(--border-default);
		color: var(--text-secondary);
	}

	.action-btn.resolve:hover {
		color: var(--status-success);
		border-color: var(--status-success);
	}

	.action-btn.delete:hover {
		color: var(--status-danger);
		border-color: var(--status-danger);
	}

	.action-btn.delete {
		margin-left: auto;
		padding: var(--space-0-5);
	}

	.resolved-count {
		font-size: var(--text-xs);
		color: var(--text-muted);
		font-style: italic;
	}

	.inline-form {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.severity-pills {
		display: flex;
		gap: var(--space-1);
	}

	.severity-pill {
		padding: var(--space-0-5) var(--space-2);
		font-size: var(--text-2xs);
		font-weight: var(--font-medium);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-full);
		color: var(--text-muted);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.severity-pill:hover {
		border-color: var(--pill-color);
		color: var(--pill-color);
	}

	.severity-pill.selected {
		background: var(--pill-bg);
		border-color: var(--pill-color);
		color: var(--pill-color);
	}

	.inline-form textarea {
		width: 100%;
		padding: var(--space-2);
		font-size: var(--text-sm);
		font-family: var(--font-body);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-primary);
		resize: vertical;
		min-height: 60px;
	}

	.inline-form textarea:focus {
		outline: none;
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 2px var(--accent-glow);
	}

	.inline-form textarea::placeholder {
		color: var(--text-muted);
	}

	.form-actions {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-2);
	}

	.cancel-btn {
		padding: var(--space-1) var(--space-2);
		font-size: var(--text-xs);
		background: transparent;
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-secondary);
		cursor: pointer;
	}

	.cancel-btn:hover {
		background: var(--bg-tertiary);
	}

	.submit-btn {
		padding: var(--space-1) var(--space-3);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		background: var(--accent-primary);
		border: none;
		border-radius: var(--radius-md);
		color: var(--text-inverse);
		cursor: pointer;
		transition: background var(--duration-fast) var(--ease-out);
	}

	.submit-btn:hover:not(:disabled) {
		background: var(--accent-primary-hover);
	}

	.submit-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
</style>
