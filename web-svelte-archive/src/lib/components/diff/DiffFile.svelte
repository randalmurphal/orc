<script lang="ts">
	import DiffHunk from './DiffHunk.svelte';
	import Icon from '$lib/components/ui/Icon.svelte';
	import type { FileDiff, ReviewComment, CreateCommentRequest } from '$lib/types';

	interface Props {
		file: FileDiff;
		expanded: boolean;
		viewMode: 'split' | 'unified';
		comments?: ReviewComment[];
		activeLineNumber?: number | null;
		onToggle: () => void;
		onLineClick?: (lineNumber: number, filePath: string) => void;
		onAddComment?: (comment: CreateCommentRequest) => Promise<void>;
		onResolveComment?: (id: string) => void;
		onWontFixComment?: (id: string) => void;
		onDeleteComment?: (id: string) => void;
		onCloseThread?: () => void;
	}

	let {
		file,
		expanded,
		viewMode,
		comments = [],
		activeLineNumber = null,
		onToggle,
		onLineClick,
		onAddComment,
		onResolveComment,
		onWontFixComment,
		onDeleteComment,
		onCloseThread
	}: Props = $props();

	const fileComments = $derived(comments.filter(c => c.file_path === file.path));
	const openCommentCount = $derived(fileComments.filter(c => c.status === 'open').length);

	const statusConfig: Record<string, { label: string; color: string }> = {
		added: { label: 'A', color: 'var(--status-success)' },
		deleted: { label: 'D', color: 'var(--status-danger)' },
		modified: { label: 'M', color: 'var(--status-warning)' },
		renamed: { label: 'R', color: 'var(--status-info)' }
	};

	const config = $derived(statusConfig[file.status] || statusConfig.modified);
</script>

<div class="diff-file">
	<button class="file-header" onclick={onToggle} aria-expanded={expanded}>
		<span class="expand-icon" class:expanded>
			<Icon name="chevron-right" size={14} />
		</span>
		<span class="file-status" style:color={config.color}>
			{config.label}
		</span>
		<span class="file-path">
			{file.path}
			{#if file.old_path && file.old_path !== file.path}
				<span class="old-path">(from {file.old_path})</span>
			{/if}
		</span>
		<span class="file-stats">
			{#if openCommentCount > 0}
				<span class="comments-count">{openCommentCount}</span>
			{/if}
			{#if file.additions > 0}
				<span class="additions">+{file.additions}</span>
			{/if}
			{#if file.deletions > 0}
				<span class="deletions">-{file.deletions}</span>
			{/if}
		</span>
	</button>

	{#if expanded}
		<div class="file-content">
			{#if file.binary}
				<div class="binary-notice">Binary file not shown</div>
			{:else if file.loadError}
				<div class="error-hunks">
					<span class="error-icon">!</span>
					<span>{file.loadError}</span>
				</div>
			{:else if file.hunks && file.hunks.length > 0}
				{#each file.hunks as hunk, i (i)}
					<DiffHunk
						{hunk}
						{viewMode}
						filePath={file.path}
						comments={fileComments}
						{activeLineNumber}
						{onLineClick}
						{onAddComment}
						onResolveComment={onResolveComment}
						onWontFixComment={onWontFixComment}
						onDeleteComment={onDeleteComment}
						{onCloseThread}
					/>
				{/each}
			{:else}
				<div class="loading-hunks">
					<div class="loading-spinner"></div>
					<span>Loading...</span>
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.diff-file {
		border-bottom: 1px solid var(--border-subtle);
	}

	.diff-file:last-child {
		border-bottom: none;
	}

	.file-header {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		width: 100%;
		padding: var(--space-2-5) var(--space-4);
		background: var(--bg-secondary);
		border: none;
		cursor: pointer;
		text-align: left;
		font-family: var(--font-mono);
		font-size: var(--text-xs);
		color: var(--text-primary);
		transition: background var(--duration-fast) var(--ease-out);
	}

	.file-header:hover {
		background: var(--bg-tertiary);
	}

	.expand-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		color: var(--text-muted);
		transition: transform var(--duration-fast) var(--ease-out);
		flex-shrink: 0;
	}

	.expand-icon.expanded {
		transform: rotate(90deg);
	}

	.file-status {
		font-weight: var(--font-semibold);
		width: 1rem;
		text-align: center;
		flex-shrink: 0;
	}

	.file-path {
		flex: 1;
		color: var(--accent-primary);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.old-path {
		color: var(--text-muted);
		font-size: var(--text-2xs);
		margin-left: var(--space-1);
	}

	.file-stats {
		display: flex;
		gap: var(--space-2);
		font-size: var(--text-2xs);
		flex-shrink: 0;
	}

	.comments-count {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 16px;
		height: 16px;
		padding: 0 var(--space-1);
		border-radius: var(--radius-full);
		background: var(--status-warning);
		color: white;
		font-size: var(--text-2xs);
		font-weight: var(--font-bold);
	}

	.additions {
		color: var(--status-success);
		font-weight: var(--font-medium);
	}

	.deletions {
		color: var(--status-danger);
		font-weight: var(--font-medium);
	}

	.file-content {
		overflow: auto;
		background: var(--bg-primary);
		/* Height controlled by parent flex layout */
	}

	.binary-notice,
	.loading-hunks,
	.error-hunks {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-2);
		padding: var(--space-6);
		text-align: center;
		color: var(--text-muted);
		font-style: italic;
		font-size: var(--text-sm);
	}

	.error-hunks {
		color: var(--status-danger);
	}

	.error-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 20px;
		height: 20px;
		border-radius: 50%;
		background: var(--status-danger-bg);
		font-weight: var(--font-bold);
		font-style: normal;
		font-size: var(--text-xs);
	}

	.loading-spinner {
		width: 16px;
		height: 16px;
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
</style>
