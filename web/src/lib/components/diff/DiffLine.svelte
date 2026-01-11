<script lang="ts">
	import type { Line } from '$lib/types';

	interface Props {
		line: Line;
		mode: 'unified' | 'split-old' | 'split-new';
		filePath: string;
		commentCount?: number;
		onLineClick?: (lineNumber: number, filePath: string) => void;
	}

	let { line, mode, filePath, commentCount = 0, onLineClick }: Props = $props();

	const bgColor = $derived.by(() => {
		if (line.type === 'addition') return 'var(--diff-add-bg)';
		if (line.type === 'deletion') return 'var(--diff-del-bg)';
		return 'transparent';
	});

	const lineNumBg = $derived.by(() => {
		if (line.type === 'addition') return 'var(--diff-add-gutter)';
		if (line.type === 'deletion') return 'var(--diff-del-gutter)';
		return 'var(--bg-tertiary)';
	});

	const lineNum = $derived.by(() => {
		if (mode === 'split-old') return line.old_line;
		if (mode === 'split-new') return line.new_line;
		return line.type === 'deletion' ? line.old_line : line.new_line;
	});

	const prefix = $derived.by(() => {
		if (line.type === 'addition') return '+';
		if (line.type === 'deletion') return '-';
		return ' ';
	});

	const prefixColor = $derived.by(() => {
		if (line.type === 'addition') return 'var(--status-success)';
		if (line.type === 'deletion') return 'var(--status-danger)';
		return 'var(--text-muted)';
	});

	const ariaLabel = $derived.by(() => {
		if (!lineNum) return '';
		if (commentCount > 0) {
			return `Line ${lineNum}, ${commentCount} comment${commentCount > 1 ? 's' : ''}. Click to view or add.`;
		}
		if (onLineClick) {
			return `Line ${lineNum}. Click to add comment.`;
		}
		return `Line ${lineNum}`;
	});
</script>

<div class="diff-line" style:background={bgColor}>
	<button
		type="button"
		class="line-num"
		class:clickable={onLineClick && lineNum}
		class:has-comments={commentCount > 0}
		style:background={lineNumBg}
		onclick={() => lineNum && onLineClick?.(lineNum, filePath)}
		disabled={!onLineClick || !lineNum}
		aria-label={ariaLabel}
	>
		{#if commentCount > 0}
			<span class="comment-badge">{commentCount}</span>
		{:else if onLineClick && lineNum}
			<span class="add-icon">+</span>
		{/if}
		<span class="line-number">{lineNum ?? ''}</span>
	</button>
	{#if mode === 'unified'}
		<span class="line-prefix" style:color={prefixColor}>
			{prefix}
		</span>
	{/if}
	<span class="line-content">{line.content}</span>
</div>

<style>
	.diff-line {
		display: flex;
		height: 22px;
		align-items: center;
		min-width: max-content;
	}

	.line-num {
		position: relative;
		width: 48px;
		padding: 0 var(--space-2);
		text-align: right;
		color: var(--text-muted);
		user-select: none;
		flex-shrink: 0;
		font-size: var(--text-2xs);
		font-family: var(--font-mono);
		border: none;
		cursor: default;
		display: flex;
		align-items: center;
		justify-content: flex-end;
		gap: var(--space-1);
	}

	.line-number {
		min-width: 24px;
		text-align: right;
	}

	.add-icon {
		display: none;
		width: 14px;
		height: 14px;
		border-radius: var(--radius-sm);
		background: var(--accent-primary);
		color: var(--text-inverse);
		font-size: 10px;
		font-weight: var(--font-bold);
		line-height: 14px;
		text-align: center;
	}

	.comment-badge {
		display: flex;
		align-items: center;
		justify-content: center;
		min-width: 14px;
		height: 14px;
		padding: 0 var(--space-0-5);
		border-radius: var(--radius-sm);
		background: var(--status-warning);
		color: white;
		font-size: 9px;
		font-weight: var(--font-bold);
	}

	.line-num.clickable {
		cursor: pointer;
	}

	.line-num.clickable:hover {
		color: var(--accent-primary);
		background: var(--bg-surface) !important;
	}

	.line-num.clickable:hover .add-icon,
	.line-num.clickable:focus .add-icon {
		display: block;
	}

	.line-num.clickable:focus {
		outline: 2px solid var(--accent-primary);
		outline-offset: -2px;
	}

	.line-num.has-comments {
		cursor: pointer;
	}

	.line-num.has-comments:hover {
		background: var(--bg-surface) !important;
	}

	.line-num:disabled {
		cursor: default;
	}

	.line-prefix {
		width: 18px;
		text-align: center;
		flex-shrink: 0;
		font-weight: var(--font-medium);
	}

	.line-content {
		flex: 1;
		padding-right: var(--space-4);
		white-space: pre;
		overflow-x: auto;
	}

	/* Diff-specific color tokens */
	:global(:root) {
		--diff-add-bg: rgba(16, 185, 129, 0.12);
		--diff-add-gutter: rgba(16, 185, 129, 0.2);
		--diff-del-bg: rgba(239, 68, 68, 0.12);
		--diff-del-gutter: rgba(239, 68, 68, 0.2);
	}
</style>
