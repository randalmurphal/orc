<script lang="ts">
	import type { TranscriptLine } from '$lib/types';

	interface Props {
		lines: TranscriptLine[];
		autoScroll?: boolean;
		taskId?: string;
	}

	let { lines, autoScroll = true, taskId = 'task' }: Props = $props();

	let containerRef: HTMLDivElement;
	let isAutoScrollEnabled = $state(true);

	// Sync with prop on initial render and prop changes
	$effect(() => {
		isAutoScrollEnabled = autoScroll;
	});

	// Auto-scroll to bottom when new lines added
	$effect(() => {
		if (isAutoScrollEnabled && containerRef && lines.length > 0) {
			containerRef.scrollTop = containerRef.scrollHeight;
		}
	});

	const typeConfig: Record<string, { icon: string; color: string; bg: string; label: string }> = {
		prompt: {
			icon: '\u25B6', // ▶
			color: 'var(--accent-primary)',
			bg: 'var(--accent-subtle)',
			label: 'PROMPT'
		},
		response: {
			icon: '\u25C0', // ◀
			color: 'var(--status-success)',
			bg: 'var(--status-success-bg)',
			label: 'RESPONSE'
		},
		tool: {
			icon: '\u26A1', // ⚡
			color: 'var(--status-warning)',
			bg: 'var(--status-warning-bg)',
			label: 'TOOL'
		},
		error: {
			icon: '\u2717', // ✗
			color: 'var(--status-danger)',
			bg: 'var(--status-danger-bg)',
			label: 'ERROR'
		}
	};

	function formatTime(timestamp: string): string {
		const date = new Date(timestamp);
		return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
	}

	function toggleAutoScroll() {
		isAutoScrollEnabled = !isAutoScrollEnabled;
	}

	// Truncate long content with expand option
	const MAX_PREVIEW_LENGTH = 500;

	function shouldTruncate(content: string): boolean {
		return content.length > MAX_PREVIEW_LENGTH;
	}

	let expandedLines = $state<Set<number>>(new Set());

	function toggleExpand(index: number) {
		if (expandedLines.has(index)) {
			expandedLines.delete(index);
		} else {
			expandedLines.add(index);
		}
		expandedLines = new Set(expandedLines);
	}

	// Export transcript to markdown
	function exportToMarkdown() {
		if (lines.length === 0) return;

		const timestamp = new Date().toISOString().slice(0, 16).replace('T', '_').replace(':', '-');
		const filename = `${taskId}-transcript-${timestamp}.md`;

		let content = `# Transcript: ${taskId}\n\n`;
		content += `Generated: ${new Date().toLocaleString()}\n\n`;
		content += `---\n\n`;

		for (const line of lines) {
			const config = typeConfig[line.type] || typeConfig.response;
			const time = formatTime(line.timestamp);
			content += `## ${config.label} (${time})\n\n`;
			content += `\`\`\`\n${line.content}\n\`\`\`\n\n`;
		}

		const blob = new Blob([content], { type: 'text/markdown' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = filename;
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
		URL.revokeObjectURL(url);
	}

	// Copy transcript to clipboard
	async function copyToClipboard() {
		if (lines.length === 0) return;

		let content = '';
		for (const line of lines) {
			const config = typeConfig[line.type] || typeConfig.response;
			const time = formatTime(line.timestamp);
			content += `[${config.label} ${time}]\n${line.content}\n\n`;
		}

		try {
			await navigator.clipboard.writeText(content);
			// Could add toast notification here
		} catch (e) {
			console.error('Failed to copy to clipboard:', e);
		}
	}
</script>

<div class="transcript-container">
	<!-- Header -->
	<div class="transcript-header">
		<h2>Transcript</h2>
		<div class="header-actions">
			<button
				class="header-btn"
				onclick={copyToClipboard}
				title="Copy transcript to clipboard"
				disabled={lines.length === 0}
			>
				<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
					<path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
				</svg>
				Copy
			</button>
			<button
				class="header-btn"
				onclick={exportToMarkdown}
				title="Export transcript as markdown"
				disabled={lines.length === 0}
			>
				<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
					<polyline points="7 10 12 15 17 10" />
					<line x1="12" y1="15" x2="12" y2="3" />
				</svg>
				Export
			</button>
			<button
				class="header-btn"
				class:active={isAutoScrollEnabled}
				onclick={toggleAutoScroll}
				title={isAutoScrollEnabled ? 'Disable auto-scroll' : 'Enable auto-scroll'}
			>
				<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<polyline points="17 13 12 18 7 13" />
					<polyline points="17 6 12 11 7 6" />
				</svg>
				Auto-scroll
			</button>
		</div>
	</div>

	<!-- Transcript Lines -->
	<div class="transcript-content" bind:this={containerRef}>
		{#if lines.length === 0}
			<div class="empty-state">
				<div class="empty-icon">
					<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
						<polyline points="4 17 10 11 4 5" />
						<line x1="12" y1="19" x2="20" y2="19" />
					</svg>
				</div>
				<p class="empty-title">No transcript yet</p>
				<p class="empty-hint">Run the task to see live output</p>
			</div>
		{:else}
			<div class="lines stagger-children">
				{#each lines as line, i (i)}
					{@const config = typeConfig[line.type] || typeConfig.response}
					{@const isTruncated = shouldTruncate(line.content)}
					{@const isExpanded = expandedLines.has(i)}

					<div
						class="entry"
						style:--entry-color={config.color}
						style:--entry-bg={config.bg}
					>
						<!-- Entry Header -->
						<div class="entry-header">
							<div class="entry-type">
								<span class="entry-icon">{config.icon}</span>
								<span class="entry-label">{config.label}</span>
							</div>
							<span class="entry-time">{formatTime(line.timestamp)}</span>
						</div>

						<!-- Entry Content -->
						<div class="entry-content">
							<pre class="content-text">{isExpanded || !isTruncated
								? line.content
								: line.content.slice(0, MAX_PREVIEW_LENGTH) + '...'}</pre>

							{#if isTruncated}
								<button class="expand-btn" onclick={() => toggleExpand(i)}>
									{isExpanded ? 'Show less' : 'Show more'}
								</button>
							{/if}
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>
</div>

<style>
	.transcript-container {
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		display: flex;
		flex-direction: column;
		max-height: 600px;
	}

	/* Header */
	.transcript-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--space-4) var(--space-5);
		border-bottom: 1px solid var(--border-subtle);
		flex-shrink: 0;
	}

	.transcript-header h2 {
		font-size: var(--text-xs);
		font-weight: var(--font-semibold);
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		margin: 0;
	}

	.header-actions {
		display: flex;
		gap: var(--space-2);
	}

	.header-btn {
		display: flex;
		align-items: center;
		gap: var(--space-1-5);
		padding: var(--space-1) var(--space-2);
		font-size: var(--text-xs);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-muted);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.header-btn:hover:not(:disabled) {
		background: var(--bg-surface);
		color: var(--text-secondary);
	}

	.header-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.header-btn.active {
		background: var(--accent-subtle);
		border-color: var(--accent-primary);
		color: var(--accent-primary);
	}

	/* Content Area */
	.transcript-content {
		flex: 1;
		overflow-y: auto;
		padding: var(--space-3);
	}

	/* Empty State */
	.empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: var(--space-12) var(--space-6);
		text-align: center;
	}

	.empty-icon {
		color: var(--text-muted);
		margin-bottom: var(--space-4);
		opacity: 0.5;
	}

	.empty-title {
		font-size: var(--text-base);
		font-weight: var(--font-medium);
		color: var(--text-secondary);
		margin-bottom: var(--space-1);
	}

	.empty-hint {
		font-size: var(--text-sm);
		color: var(--text-muted);
	}

	/* Lines */
	.lines {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
	}

	/* Entry */
	.entry {
		border-left: 3px solid var(--entry-color);
		background: var(--entry-bg);
		border-radius: 0 var(--radius-md) var(--radius-md) 0;
		overflow: hidden;
	}

	.entry-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--space-2) var(--space-3);
		background: rgba(0, 0, 0, 0.1);
	}

	.entry-type {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.entry-icon {
		font-size: var(--text-sm);
		color: var(--entry-color);
	}

	.entry-label {
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		color: var(--entry-color);
	}

	.entry-time {
		font-size: var(--text-2xs);
		color: var(--text-muted);
		font-family: var(--font-mono);
	}

	.entry-content {
		padding: var(--space-3);
	}

	.content-text {
		font-family: var(--font-mono);
		font-size: var(--text-sm);
		line-height: var(--leading-relaxed);
		white-space: pre-wrap;
		word-break: break-word;
		color: var(--text-primary);
		background: transparent;
		border: none;
		padding: 0;
		margin: 0;
	}

	.expand-btn {
		display: inline-flex;
		margin-top: var(--space-2);
		padding: var(--space-1) var(--space-2);
		font-size: var(--text-xs);
		background: transparent;
		border: 1px solid var(--border-default);
		border-radius: var(--radius-sm);
		color: var(--text-secondary);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.expand-btn:hover {
		background: var(--bg-tertiary);
		color: var(--text-primary);
	}
</style>
