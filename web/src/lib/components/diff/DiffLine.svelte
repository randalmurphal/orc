<script lang="ts">
	import type { Line } from '$lib/types';

	interface Props {
		line: Line;
		mode: 'unified' | 'split-old' | 'split-new';
		syntax: string;
	}

	let { line, mode, syntax: _syntax }: Props = $props();

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
</script>

<div class="diff-line" style:background={bgColor}>
	<span class="line-num" style:background={lineNumBg}>
		{lineNum ?? ''}
	</span>
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
		width: 48px;
		padding: 0 var(--space-2);
		text-align: right;
		color: var(--text-muted);
		user-select: none;
		flex-shrink: 0;
		font-size: var(--text-2xs);
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
