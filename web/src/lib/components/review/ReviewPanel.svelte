<script lang="ts">
	import { onMount } from 'svelte';

	interface ReviewComment {
		id: string;
		file_path?: string;
		line_number?: number;
		content: string;
		severity: 'suggestion' | 'issue' | 'blocker';
		status: 'open' | 'resolved' | 'wont_fix';
		created_at: string;
		resolved_at?: string;
	}

	interface ReviewStats {
		open_comments: number;
		resolved_comments: number;
		total_comments: number;
		blockers: number;
		issues: number;
		suggestions: number;
	}

	interface Props {
		taskId: string;
	}

	let { taskId }: Props = $props();

	let comments = $state<ReviewComment[]>([]);
	let stats = $state<ReviewStats | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let showResolved = $state(false);

	onMount(async () => {
		await loadReviewData();
	});

	async function loadReviewData() {
		loading = true;
		error = null;
		try {
			const res = await fetch(`/api/tasks/${taskId}/review/comments`);
			if (res.ok) {
				comments = await res.json();
				calculateStats();
			} else if (res.status === 404) {
				// No comments yet, that's fine
				comments = [];
				calculateStats();
			} else {
				throw new Error('Failed to load review comments');
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Unknown error';
		} finally {
			loading = false;
		}
	}

	function calculateStats() {
		const openComments = comments.filter((c) => c.status === 'open');
		const resolvedComments = comments.filter(
			(c) => c.status === 'resolved' || c.status === 'wont_fix'
		);

		stats = {
			open_comments: openComments.length,
			resolved_comments: resolvedComments.length,
			total_comments: comments.length,
			blockers: comments.filter((c) => c.severity === 'blocker' && c.status === 'open').length,
			issues: comments.filter((c) => c.severity === 'issue' && c.status === 'open').length,
			suggestions: comments.filter((c) => c.severity === 'suggestion' && c.status === 'open').length
		};
	}

	const filteredComments = $derived(
		showResolved ? comments : comments.filter((c) => c.status === 'open')
	);

	const severityConfig = {
		blocker: {
			color: 'var(--status-danger)',
			bg: 'var(--status-danger-bg)',
			icon: '!',
			label: 'Blocker'
		},
		issue: {
			color: 'var(--status-warning)',
			bg: 'var(--status-warning-bg)',
			icon: '?',
			label: 'Issue'
		},
		suggestion: {
			color: 'var(--status-info)',
			bg: 'var(--status-info-bg)',
			icon: 'i',
			label: 'Suggestion'
		}
	};
</script>

<div class="review-panel">
	<div class="review-header">
		<h2>Review Comments</h2>
		{#if stats && stats.total_comments > 0}
			<div class="review-toggle">
				<label class="toggle-label">
					<input type="checkbox" bind:checked={showResolved} />
					<span>Show resolved ({stats.resolved_comments})</span>
				</label>
			</div>
		{/if}
	</div>

	{#if stats && stats.total_comments > 0}
		<div class="review-summary">
			{#if stats.blockers > 0}
				<div class="summary-item blocker">
					<span class="summary-count">{stats.blockers}</span>
					<span class="summary-label">Blockers</span>
				</div>
			{/if}
			{#if stats.issues > 0}
				<div class="summary-item issue">
					<span class="summary-count">{stats.issues}</span>
					<span class="summary-label">Issues</span>
				</div>
			{/if}
			{#if stats.suggestions > 0}
				<div class="summary-item suggestion">
					<span class="summary-count">{stats.suggestions}</span>
					<span class="summary-label">Suggestions</span>
				</div>
			{/if}
		</div>
	{/if}

	<div class="review-content">
		{#if loading}
			<div class="loading-state">
				<div class="loading-spinner"></div>
				<span>Loading review comments...</span>
			</div>
		{:else if error}
			<div class="error-state">
				<span class="error-icon">!</span>
				<span>{error}</span>
			</div>
		{:else if filteredComments.length === 0}
			<div class="empty-state">
				<div class="empty-icon">
					<svg
						xmlns="http://www.w3.org/2000/svg"
						width="32"
						height="32"
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						stroke-width="1.5"
						stroke-linecap="round"
						stroke-linejoin="round"
					>
						<path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" />
					</svg>
				</div>
				<p class="empty-title">No review comments</p>
				<p class="empty-hint">
					{#if stats?.resolved_comments}
						All comments have been resolved
					{:else}
						Review comments will appear here when added
					{/if}
				</p>
			</div>
		{:else}
			<div class="comments-list">
				{#each filteredComments as comment (comment.id)}
					{@const config = severityConfig[comment.severity]}
					<div
						class="comment-item"
						class:resolved={comment.status !== 'open'}
						style:--severity-color={config.color}
						style:--severity-bg={config.bg}
					>
						<div class="comment-header">
							<div class="comment-severity">
								<span class="severity-icon">{config.icon}</span>
								<span class="severity-label">{config.label}</span>
							</div>
							{#if comment.file_path}
								<span class="comment-location">
									{comment.file_path}
									{#if comment.line_number}
										<span class="line-number">:{comment.line_number}</span>
									{/if}
								</span>
							{/if}
							{#if comment.status !== 'open'}
								<span class="resolved-badge">
									{comment.status === 'resolved' ? 'Resolved' : "Won't fix"}
								</span>
							{/if}
						</div>
						<div class="comment-content">
							{comment.content}
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>

	{#if stats && stats.open_comments > 0}
		<div class="review-actions">
			<button class="primary" disabled>
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="16"
					height="16"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
				>
					<line x1="22" y1="2" x2="11" y2="13" />
					<polygon points="22 2 15 22 11 13 2 9 22 2" />
				</svg>
				Send to Agent
			</button>
			<span class="action-hint">Retry with all open comments as context</span>
		</div>
	{/if}
</div>

<style>
	.review-panel {
		display: flex;
		flex-direction: column;
		height: 100%;
		background: var(--bg-primary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		overflow: hidden;
	}

	.review-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--space-4) var(--space-5);
		border-bottom: 1px solid var(--border-subtle);
		background: var(--bg-secondary);
		flex-shrink: 0;
	}

	.review-header h2 {
		font-size: var(--text-xs);
		font-weight: var(--font-semibold);
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		margin: 0;
	}

	.review-toggle {
		display: flex;
		align-items: center;
	}

	.toggle-label {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		font-size: var(--text-xs);
		color: var(--text-secondary);
		cursor: pointer;
	}

	.toggle-label input {
		width: 14px;
		height: 14px;
		cursor: pointer;
	}

	.review-summary {
		display: flex;
		gap: var(--space-4);
		padding: var(--space-3) var(--space-5);
		border-bottom: 1px solid var(--border-subtle);
		background: var(--bg-secondary);
		flex-shrink: 0;
	}

	.summary-item {
		display: flex;
		align-items: center;
		gap: var(--space-1-5);
	}

	.summary-count {
		font-family: var(--font-mono);
		font-size: var(--text-lg);
		font-weight: var(--font-bold);
	}

	.summary-label {
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.summary-item.blocker .summary-count {
		color: var(--status-danger);
	}

	.summary-item.issue .summary-count {
		color: var(--status-warning);
	}

	.summary-item.suggestion .summary-count {
		color: var(--status-info);
	}

	.review-content {
		flex: 1;
		overflow-y: auto;
		padding: var(--space-3);
	}

	.comments-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
	}

	.comment-item {
		border-left: 3px solid var(--severity-color);
		background: var(--severity-bg);
		border-radius: 0 var(--radius-md) var(--radius-md) 0;
		overflow: hidden;
	}

	.comment-item.resolved {
		opacity: 0.6;
	}

	.comment-header {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-2) var(--space-3);
		background: rgba(0, 0, 0, 0.1);
		flex-wrap: wrap;
	}

	.comment-severity {
		display: flex;
		align-items: center;
		gap: var(--space-1-5);
	}

	.severity-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 18px;
		height: 18px;
		border-radius: var(--radius-full);
		background: var(--severity-color);
		color: white;
		font-size: var(--text-2xs);
		font-weight: var(--font-bold);
	}

	.severity-label {
		font-size: var(--text-xs);
		font-weight: var(--font-semibold);
		color: var(--severity-color);
		text-transform: uppercase;
	}

	.comment-location {
		font-family: var(--font-mono);
		font-size: var(--text-xs);
		color: var(--text-secondary);
	}

	.line-number {
		color: var(--text-muted);
	}

	.resolved-badge {
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		padding: var(--space-0-5) var(--space-2);
		background: var(--bg-tertiary);
		border-radius: var(--radius-sm);
		color: var(--text-muted);
	}

	.comment-content {
		padding: var(--space-3);
		font-size: var(--text-sm);
		line-height: var(--leading-relaxed);
		color: var(--text-primary);
	}

	.review-actions {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-4) var(--space-5);
		border-top: 1px solid var(--border-subtle);
		background: var(--bg-secondary);
		flex-shrink: 0;
	}

	.action-hint {
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	/* Loading / Error / Empty States */
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
		text-align: center;
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
		color: var(--text-muted);
		opacity: 0.5;
	}

	.empty-title {
		font-size: var(--text-base);
		font-weight: var(--font-medium);
		color: var(--text-secondary);
		margin: 0;
	}

	.empty-hint {
		font-size: var(--text-sm);
		color: var(--text-muted);
		margin: 0;
	}
</style>
