<script lang="ts">
	import type { TranscriptLine } from '$lib/types';

	interface Props {
		lines: TranscriptLine[];
	}

	let { lines }: Props = $props();

	const typeColors: Record<string, string> = {
		prompt: 'var(--accent-primary)',
		response: 'var(--accent-success)',
		tool: 'var(--accent-warning)',
		error: 'var(--accent-danger)'
	};

	function formatTime(timestamp: string): string {
		const date = new Date(timestamp);
		return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
	}
</script>

<div class="transcript">
	{#if lines.length === 0}
		<div class="empty">
			<p>No transcript yet</p>
			<p class="hint">Run the task to see live output</p>
		</div>
	{:else}
		<div class="lines">
			{#each lines as line, i (i)}
				<div class="line" style="border-left-color: {typeColors[line.type]}">
					<div class="line-header">
						<span class="line-type">{line.type}</span>
						<span class="line-time">{formatTime(line.timestamp)}</span>
					</div>
					<pre class="line-content">{line.content}</pre>
				</div>
			{/each}
		</div>
	{/if}
</div>

<style>
	.transcript {
		background: var(--bg-secondary);
		border: 1px solid var(--border-color);
		border-radius: 8px;
		max-height: 500px;
		overflow-y: auto;
	}

	.empty {
		padding: 3rem;
		text-align: center;
		color: var(--text-secondary);
	}

	.empty .hint {
		font-size: 0.875rem;
		color: var(--text-muted);
		margin-top: 0.5rem;
	}

	.lines {
		padding: 0.5rem;
	}

	.line {
		border-left: 3px solid;
		padding: 0.5rem 0.75rem;
		margin-bottom: 0.5rem;
		background: var(--bg-tertiary);
		border-radius: 0 6px 6px 0;
	}

	.line:last-child {
		margin-bottom: 0;
	}

	.line-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 0.25rem;
	}

	.line-type {
		font-size: 0.625rem;
		font-weight: 600;
		text-transform: uppercase;
		color: var(--text-secondary);
	}

	.line-time {
		font-size: 0.625rem;
		color: var(--text-muted);
		font-family: var(--font-mono);
	}

	.line-content {
		font-size: 0.8125rem;
		line-height: 1.5;
		white-space: pre-wrap;
		word-break: break-word;
		background: transparent;
		border: none;
		padding: 0;
		margin: 0;
	}
</style>
