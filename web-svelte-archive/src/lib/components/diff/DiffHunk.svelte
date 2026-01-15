<script lang="ts">
	import DiffLine from './DiffLine.svelte';
	import VirtualScroller from './VirtualScroller.svelte';
	import InlineCommentThread from './InlineCommentThread.svelte';
	import type { Hunk, Line, ReviewComment, CreateCommentRequest } from '$lib/types';

	interface Props {
		hunk: Hunk;
		viewMode: 'split' | 'unified';
		filePath: string;
		comments?: ReviewComment[];
		activeLineNumber?: number | null;
		onLineClick?: (lineNumber: number, filePath: string) => void;
		onAddComment?: (comment: CreateCommentRequest) => Promise<void>;
		onResolveComment?: (id: string) => void;
		onWontFixComment?: (id: string) => void;
		onDeleteComment?: (id: string) => void;
		onCloseThread?: () => void;
	}

	let {
		hunk,
		viewMode,
		filePath,
		comments = [],
		activeLineNumber = null,
		onLineClick,
		onAddComment,
		onResolveComment,
		onWontFixComment,
		onDeleteComment,
		onCloseThread
	}: Props = $props();

	function getCommentsForLine(lineNumber: number | undefined): ReviewComment[] {
		if (!lineNumber) return [];
		return comments.filter(c => c.line_number === lineNumber);
	}

	function getCommentCount(lineNumber: number | undefined): number {
		if (!lineNumber) return 0;
		return comments.filter(c => c.line_number === lineNumber && c.status === 'open').length;
	}

	function shouldShowThread(lineNumber: number | undefined): boolean {
		if (!lineNumber) return false;
		return getCommentsForLine(lineNumber).length > 0 || activeLineNumber === lineNumber;
	}

	interface LinePair {
		old?: Line;
		new?: Line;
	}

	// For split view, pair up lines
	const pairedLines = $derived.by(() => {
		if (viewMode !== 'split') return [];

		const pairs: LinePair[] = [];
		const deletions: Line[] = [];
		const additions: Line[] = [];

		for (const line of hunk.lines) {
			if (line.type === 'deletion') {
				deletions.push(line);
			} else if (line.type === 'addition') {
				additions.push(line);
			} else {
				// Flush pending changes before context line
				while (deletions.length || additions.length) {
					pairs.push({
						old: deletions.shift(),
						new: additions.shift()
					});
				}
				pairs.push({ old: line, new: line });
			}
		}
		// Flush remaining
		while (deletions.length || additions.length) {
			pairs.push({
				old: deletions.shift(),
				new: additions.shift()
			});
		}

		return pairs;
	});

	// Threshold for virtual scrolling
	const VIRTUAL_THRESHOLD = 100;
</script>

<div class="diff-hunk">
	<div class="hunk-header">
		@@ -{hunk.old_start},{hunk.old_lines} +{hunk.new_start},{hunk.new_lines} @@
	</div>

	{#if viewMode === 'unified'}
		<div class="unified-view">
			{#if hunk.lines.length > VIRTUAL_THRESHOLD}
				<VirtualScroller items={hunk.lines} itemHeight={22}>
					{#snippet children({ item })}
						{@const lineNum = item.type === 'deletion' ? item.old_line : item.new_line}
						<DiffLine line={item} mode="unified" {filePath} {onLineClick} commentCount={getCommentCount(lineNum)} />
						{#if shouldShowThread(lineNum) && onAddComment && onResolveComment && onWontFixComment && onDeleteComment}
							<InlineCommentThread
								comments={getCommentsForLine(lineNum)}
								{filePath}
								lineNumber={lineNum ?? 0}
								isActive={activeLineNumber === lineNum}
								onAddComment={onAddComment}
								onResolve={onResolveComment}
								onWontFix={onWontFixComment}
								onDelete={onDeleteComment}
								onClose={onCloseThread}
							/>
						{/if}
					{/snippet}
				</VirtualScroller>
			{:else}
				{#each hunk.lines as line, i (i)}
					{@const lineNum = line.type === 'deletion' ? line.old_line : line.new_line}
					<DiffLine {line} mode="unified" {filePath} {onLineClick} commentCount={getCommentCount(lineNum)} />
					{#if shouldShowThread(lineNum) && onAddComment && onResolveComment && onWontFixComment && onDeleteComment}
						<InlineCommentThread
							comments={getCommentsForLine(lineNum)}
							{filePath}
							lineNumber={lineNum ?? 0}
							isActive={activeLineNumber === lineNum}
							onAddComment={onAddComment}
							onResolve={onResolveComment}
							onWontFix={onWontFixComment}
							onDelete={onDeleteComment}
							onClose={onCloseThread}
						/>
					{/if}
				{/each}
			{/if}
		</div>
	{:else}
		<div class="split-view">
			{#if pairedLines.length > VIRTUAL_THRESHOLD}
				<VirtualScroller items={pairedLines} itemHeight={22}>
					{#snippet children({ item: pair })}
						{@const lineNum = pair.new?.new_line}
						<div class="split-row">
							<div class="split-left">
								{#if pair.old}
									<DiffLine line={pair.old} mode="split-old" {filePath} {onLineClick} />
								{:else}
									<div class="empty-line"></div>
								{/if}
							</div>
							<div class="split-right">
								{#if pair.new}
									<DiffLine line={pair.new} mode="split-new" {filePath} {onLineClick} commentCount={getCommentCount(lineNum)} />
								{:else}
									<div class="empty-line"></div>
								{/if}
							</div>
						</div>
						{#if shouldShowThread(lineNum) && onAddComment && onResolveComment && onWontFixComment && onDeleteComment}
							<InlineCommentThread
								comments={getCommentsForLine(lineNum)}
								{filePath}
								lineNumber={lineNum ?? 0}
								isActive={activeLineNumber === lineNum}
								onAddComment={onAddComment}
								onResolve={onResolveComment}
								onWontFix={onWontFixComment}
								onDelete={onDeleteComment}
								onClose={onCloseThread}
							/>
						{/if}
					{/snippet}
				</VirtualScroller>
			{:else}
				{#each pairedLines as pair, i (i)}
					{@const lineNum = pair.new?.new_line}
					<div class="split-row">
						<div class="split-left">
							{#if pair.old}
								<DiffLine line={pair.old} mode="split-old" {filePath} {onLineClick} />
							{:else}
								<div class="empty-line"></div>
							{/if}
						</div>
						<div class="split-right">
							{#if pair.new}
								<DiffLine line={pair.new} mode="split-new" {filePath} {onLineClick} commentCount={getCommentCount(lineNum)} />
							{:else}
								<div class="empty-line"></div>
							{/if}
						</div>
					</div>
					{#if shouldShowThread(lineNum) && onAddComment && onResolveComment && onWontFixComment && onDeleteComment}
						<InlineCommentThread
							comments={getCommentsForLine(lineNum)}
							{filePath}
							lineNumber={lineNum ?? 0}
							isActive={activeLineNumber === lineNum}
							onAddComment={onAddComment}
							onResolve={onResolveComment}
							onWontFix={onWontFixComment}
							onDelete={onDeleteComment}
							onClose={onCloseThread}
						/>
					{/if}
				{/each}
			{/if}
		</div>
	{/if}
</div>

<style>
	.diff-hunk {
		font-family: var(--font-mono);
		font-size: var(--text-xs);
		line-height: 1.5;
	}

	.hunk-header {
		padding: var(--space-2) var(--space-4);
		background: var(--status-info-bg);
		color: var(--status-info);
		font-size: var(--text-2xs);
		border-top: 1px solid var(--border-subtle);
		border-bottom: 1px solid var(--border-subtle);
	}

	.unified-view {
		display: flex;
		flex-direction: column;
	}

	.split-view {
		display: flex;
		flex-direction: column;
	}

	.split-row {
		display: flex;
	}

	.split-left,
	.split-right {
		flex: 1;
		min-width: 0;
		overflow: hidden;
	}

	.split-left {
		border-right: 1px solid var(--border-subtle);
	}

	.empty-line {
		height: 22px;
		background: var(--bg-tertiary);
	}
</style>
