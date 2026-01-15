<script lang="ts">
	import type { ReviewComment, CommentSeverity } from '$lib/types';
	import Icon from '$lib/components/ui/Icon.svelte';

	interface Props {
		comments: ReviewComment[];
		onCommentClick?: (comment: ReviewComment) => void;
	}

	let { comments, onCommentClick }: Props = $props();

	type ReviewStatus = 'pass' | 'needs_input' | 'fail';

	const openComments = $derived(comments.filter((c) => c.status === 'open'));
	const resolvedComments = $derived(comments.filter((c) => c.status !== 'open'));

	const countBySeverity = $derived.by(() => {
		const counts: Record<CommentSeverity, number> = {
			suggestion: 0,
			issue: 0,
			blocker: 0
		};
		for (const c of openComments) {
			counts[c.severity]++;
		}
		return counts;
	});

	const reviewStatus = $derived.by((): ReviewStatus => {
		if (countBySeverity.blocker > 0) return 'fail';
		if (countBySeverity.issue > 0) return 'needs_input';
		if (countBySeverity.suggestion > 0) return 'needs_input';
		return 'pass';
	});

	const statusConfig: Record<ReviewStatus, { color: string; bg: string; icon: string; label: string }> = {
		pass: {
			color: 'var(--status-success)',
			bg: 'var(--status-success-bg)',
			icon: 'check',
			label: 'All Clear'
		},
		needs_input: {
			color: 'var(--status-warning)',
			bg: 'var(--status-warning-bg)',
			icon: 'warning',
			label: 'Needs Attention'
		},
		fail: {
			color: 'var(--status-danger)',
			bg: 'var(--status-danger-bg)',
			icon: 'blocked',
			label: 'Blockers Found'
		}
	};

	const status = $derived(statusConfig[reviewStatus]);

	// Group open comments by file for the list
	const commentsByFile = $derived.by(() => {
		const byFile: Map<string, ReviewComment[]> = new Map();
		for (const c of openComments) {
			const key = c.file_path ?? '(General)';
			const existing = byFile.get(key) ?? [];
			existing.push(c);
			byFile.set(key, existing);
		}
		return byFile;
	});

	function handleCommentClick(comment: ReviewComment) {
		onCommentClick?.(comment);
	}

	function handleKeyDown(event: KeyboardEvent, comment: ReviewComment) {
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			handleCommentClick(comment);
		}
	}
</script>

<div class="review-summary">
	<div class="status-header" style:background={status.bg}>
		<div class="status-icon" style:color={status.color}>
			<Icon name={status.icon} size={20} />
		</div>
		<div class="status-info">
			<span class="status-label" style:color={status.color}>{status.label}</span>
			<span class="status-count">
				{openComments.length} open, {resolvedComments.length} resolved
			</span>
		</div>
	</div>

	<div class="severity-breakdown">
		{#if countBySeverity.blocker > 0}
			<div class="severity-item blocker">
				<span class="severity-count">{countBySeverity.blocker}</span>
				<span class="severity-label">Blocker{countBySeverity.blocker !== 1 ? 's' : ''}</span>
			</div>
		{/if}
		{#if countBySeverity.issue > 0}
			<div class="severity-item issue">
				<span class="severity-count">{countBySeverity.issue}</span>
				<span class="severity-label">Issue{countBySeverity.issue !== 1 ? 's' : ''}</span>
			</div>
		{/if}
		{#if countBySeverity.suggestion > 0}
			<div class="severity-item suggestion">
				<span class="severity-count">{countBySeverity.suggestion}</span>
				<span class="severity-label">Suggestion{countBySeverity.suggestion !== 1 ? 's' : ''}</span>
			</div>
		{/if}
	</div>

	{#if openComments.length > 0}
		<div class="issues-list">
			<h4>Open Issues</h4>
			{#each [...commentsByFile] as [file, fileComments]}
				<div class="file-group">
					<div class="file-header">
						<Icon name="file" size={12} />
						<span class="file-name">{file}</span>
					</div>
					<ul class="comment-list">
						{#each fileComments as comment}
							<li>
								<button
									class="comment-link"
									class:blocker={comment.severity === 'blocker'}
									class:issue={comment.severity === 'issue'}
									class:suggestion={comment.severity === 'suggestion'}
									onclick={() => handleCommentClick(comment)}
									onkeydown={(e) => handleKeyDown(e, comment)}
								>
									<span class="severity-indicator"></span>
									{#if comment.line_number}
										<span class="line-ref">L{comment.line_number}</span>
									{/if}
									<span class="comment-preview">{comment.content}</span>
								</button>
							</li>
						{/each}
					</ul>
				</div>
			{/each}
		</div>
	{/if}
</div>

<style>
	.review-summary {
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-lg);
		overflow: hidden;
	}

	.status-header {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-4);
	}

	.status-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 40px;
		height: 40px;
		background: var(--bg-secondary);
		border-radius: var(--radius-md);
	}

	.status-info {
		display: flex;
		flex-direction: column;
		gap: var(--space-0-5);
	}

	.status-label {
		font-size: var(--text-base);
		font-weight: var(--font-semibold);
	}

	.status-count {
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.severity-breakdown {
		display: flex;
		gap: var(--space-4);
		padding: var(--space-3) var(--space-4);
		border-top: 1px solid var(--border-subtle);
		border-bottom: 1px solid var(--border-subtle);
	}

	.severity-item {
		display: flex;
		align-items: center;
		gap: var(--space-1-5);
	}

	.severity-count {
		font-size: var(--text-lg);
		font-weight: var(--font-bold);
		font-family: var(--font-mono);
	}

	.severity-label {
		font-size: var(--text-xs);
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
	}

	.severity-item.blocker .severity-count {
		color: var(--status-danger);
	}

	.severity-item.issue .severity-count {
		color: var(--status-warning);
	}

	.severity-item.suggestion .severity-count {
		color: var(--status-info);
	}

	.issues-list {
		padding: var(--space-4);
	}

	.issues-list h4 {
		font-size: var(--text-xs);
		font-weight: var(--font-semibold);
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		margin-bottom: var(--space-3);
	}

	.file-group {
		margin-bottom: var(--space-3);
	}

	.file-group:last-child {
		margin-bottom: 0;
	}

	.file-header {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		font-size: var(--text-xs);
		color: var(--text-muted);
		margin-bottom: var(--space-1-5);
	}

	.file-name {
		font-family: var(--font-mono);
	}

	.comment-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
	}

	.comment-link {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		width: 100%;
		padding: var(--space-1-5) var(--space-2);
		background: var(--bg-tertiary);
		border: 1px solid transparent;
		border-radius: var(--radius-sm);
		text-align: left;
		cursor: pointer;
		transition:
			background var(--duration-fast) var(--ease-out),
			border-color var(--duration-fast) var(--ease-out);
	}

	.comment-link:hover {
		background: var(--bg-surface);
		border-color: var(--border-default);
	}

	.comment-link:focus-visible {
		outline: none;
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 3px var(--accent-glow);
	}

	.severity-indicator {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		flex-shrink: 0;
	}

	.comment-link.blocker .severity-indicator {
		background: var(--status-danger);
	}

	.comment-link.issue .severity-indicator {
		background: var(--status-warning);
	}

	.comment-link.suggestion .severity-indicator {
		background: var(--status-info);
	}

	.line-ref {
		font-size: var(--text-xs);
		font-family: var(--font-mono);
		color: var(--text-muted);
		flex-shrink: 0;
	}

	.comment-preview {
		font-size: var(--text-sm);
		color: var(--text-primary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		flex: 1;
		min-width: 0;
	}
</style>
