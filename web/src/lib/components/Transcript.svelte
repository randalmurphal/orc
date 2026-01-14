<script lang="ts">
	import type { TranscriptFile } from '$lib/types';

	interface Props {
		files?: TranscriptFile[];
		autoScroll?: boolean;
		taskId?: string;
		streamingContent?: string;
	}

	let { files = [], autoScroll = true, taskId = 'task', streamingContent = '' }: Props = $props();

	let containerRef: HTMLDivElement;
	let isAutoScrollEnabled = $state(true);

	// Pagination
	const PAGE_SIZE = 10;
	let currentPage = $state(1);

	const totalPages = $derived(Math.ceil(files.length / PAGE_SIZE));
	const paginatedFiles = $derived(
		files.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE)
	);

	// Track expanded files - expand all by default
	let expandedFiles = $state<Set<string>>(new Set());

	// Auto-expand all files by default
	$effect(() => {
		if (files.length > 0 && expandedFiles.size === 0) {
			expandedFiles = new Set(files.map(f => f.filename));
		}
	});

	// Sync with prop on initial render
	$effect(() => {
		isAutoScrollEnabled = autoScroll;
	});

	// Auto-scroll when new content added
	$effect(() => {
		const _ = files.length + streamingContent.length;
		if (isAutoScrollEnabled && containerRef) {
			containerRef.scrollTop = containerRef.scrollHeight;
		}
	});

	// Parsed transcript section
	interface ParsedSection {
		type: 'prompt' | 'retry-context' | 'response' | 'metadata';
		title: string;
		content: string;
	}

	interface ParsedTranscript {
		phase: string;
		iteration: number;
		sections: ParsedSection[];
		metadata: {
			inputTokens?: number;
			outputTokens?: number;
			cacheCreationTokens?: number;
			cacheReadTokens?: number;
			complete?: boolean;
			blocked?: boolean;
		};
	}

	function parseTranscript(content: string): ParsedTranscript {
		const lines = content.split('\n');

		// Parse title: "# implement - Iteration 1"
		const titleMatch = lines[0]?.match(/^# (\w+) - Iteration (\d+)/);
		const phase = titleMatch?.[1] || 'unknown';
		const iteration = titleMatch ? parseInt(titleMatch[2], 10) : 1;

		const sections: ParsedSection[] = [];
		let currentSection: ParsedSection | null = null;
		let inMetadata = false;
		const metadata: ParsedTranscript['metadata'] = {};

		for (let i = 1; i < lines.length; i++) {
			const line = lines[i];

			// Check for section headers
			if (line.startsWith('## Prompt')) {
				if (currentSection) sections.push(currentSection);
				currentSection = { type: 'prompt', title: 'Prompt', content: '' };
				continue;
			}
			if (line.startsWith('## Retry Context')) {
				if (currentSection) sections.push(currentSection);
				currentSection = { type: 'retry-context', title: 'Retry Context', content: '' };
				continue;
			}
			if (line.startsWith('## Response')) {
				if (currentSection) sections.push(currentSection);
				currentSection = { type: 'response', title: 'Response', content: '' };
				continue;
			}

			// Check for metadata section (starts with ---)
			if (line === '---' && currentSection?.type === 'response') {
				inMetadata = true;
				if (currentSection) sections.push(currentSection);
				currentSection = null;
				continue;
			}

			// Parse metadata
			if (inMetadata) {
				// Try new format with cache tokens first
				const tokensWithCacheMatch = line.match(
					/^Tokens: (\d+) input, (\d+) output, (\d+) cache_creation, (\d+) cache_read/
				);
				if (tokensWithCacheMatch) {
					metadata.inputTokens = parseInt(tokensWithCacheMatch[1], 10);
					metadata.outputTokens = parseInt(tokensWithCacheMatch[2], 10);
					metadata.cacheCreationTokens = parseInt(tokensWithCacheMatch[3], 10);
					metadata.cacheReadTokens = parseInt(tokensWithCacheMatch[4], 10);
				} else {
					// Fall back to old format without cache tokens
					const tokensMatch = line.match(/^Tokens: (\d+) input, (\d+) output/);
					if (tokensMatch) {
						metadata.inputTokens = parseInt(tokensMatch[1], 10);
						metadata.outputTokens = parseInt(tokensMatch[2], 10);
					}
				}
				if (line.startsWith('Complete:')) {
					metadata.complete = line.includes('true');
				}
				if (line.startsWith('Blocked:')) {
					metadata.blocked = line.includes('true');
				}
				continue;
			}

			// Add content to current section
			if (currentSection) {
				currentSection.content += (currentSection.content ? '\n' : '') + line;
			}
		}

		// Push last section
		if (currentSection) sections.push(currentSection);

		// Trim content
		sections.forEach(s => s.content = s.content.trim());

		return { phase, iteration, sections, metadata };
	}

	function toggleFile(filename: string) {
		if (expandedFiles.has(filename)) {
			expandedFiles.delete(filename);
		} else {
			expandedFiles.add(filename);
		}
		expandedFiles = new Set(expandedFiles);
	}

	function expandAll() {
		expandedFiles = new Set(files.map(f => f.filename));
	}

	function collapseAll() {
		expandedFiles = new Set();
	}

	function formatTime(timestamp: string): string {
		const date = new Date(timestamp);
		return date.toLocaleString([], {
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}

	function toggleAutoScroll() {
		isAutoScrollEnabled = !isAutoScrollEnabled;
	}

	// Export transcript to markdown
	function exportToMarkdown() {
		if (files.length === 0) return;

		const timestamp = new Date().toISOString().slice(0, 16).replace('T', '_').replace(':', '-');
		const filename = `${taskId}-transcript-${timestamp}.md`;

		let content = `# Transcript: ${taskId}\n\n`;
		content += `Generated: ${new Date().toLocaleString()}\n\n`;
		content += `---\n\n`;

		for (const file of files) {
			content += file.content + '\n\n---\n\n';
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
		if (files.length === 0) return;

		let content = '';
		for (const file of files) {
			content += file.content + '\n\n---\n\n';
		}

		try {
			await navigator.clipboard.writeText(content);
		} catch (e) {
			console.error('Failed to copy to clipboard:', e);
		}
	}

	// Section styling config
	const sectionStyles: Record<string, { icon: string; color: string; bg: string }> = {
		prompt: {
			icon: '▶',
			color: 'var(--accent-primary)',
			bg: 'var(--accent-subtle)'
		},
		'retry-context': {
			icon: '↻',
			color: 'var(--status-warning)',
			bg: 'var(--status-warning-bg)'
		},
		response: {
			icon: '◀',
			color: 'var(--status-success)',
			bg: 'var(--status-success-bg)'
		}
	};
</script>

<div class="transcript-container">
	<!-- Header -->
	<div class="transcript-header">
		<h2>Transcript</h2>
		<div class="header-actions">
			{#if files.length > 1}
				<button class="header-btn" onclick={expandAll} title="Expand all">
					Expand All
				</button>
				<button class="header-btn" onclick={collapseAll} title="Collapse all">
					Collapse All
				</button>
			{/if}
			<button
				class="header-btn"
				onclick={copyToClipboard}
				title="Copy transcript to clipboard"
				disabled={files.length === 0}
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
				disabled={files.length === 0}
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

	<!-- Transcript Content -->
	<div class="transcript-content" bind:this={containerRef}>
		{#if files.length === 0 && !streamingContent}
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
			<div class="transcript-files">
				{#each paginatedFiles as file (file.filename)}
					{@const parsed = parseTranscript(file.content)}
					{@const isExpanded = expandedFiles.has(file.filename)}

					<div class="transcript-file" class:expanded={isExpanded}>
						<!-- File Header -->
						<button class="file-header" onclick={() => toggleFile(file.filename)}>
							<div class="file-info">
								<span class="chevron" class:rotated={isExpanded}>
									<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
										<polyline points="9 18 15 12 9 6" />
									</svg>
								</span>
								<span class="phase-badge">{parsed.phase}</span>
								<span class="iteration">Iteration {parsed.iteration}</span>
								{#if parsed.metadata.complete}
									<span class="status-badge complete">✓ Complete</span>
								{:else if parsed.metadata.blocked}
									<span class="status-badge blocked">⚠ Blocked</span>
								{/if}
							</div>
							<div class="file-meta">
								{#if parsed.metadata.inputTokens || parsed.metadata.outputTokens}
									{@const cacheTotal =
										(parsed.metadata.cacheCreationTokens || 0) +
										(parsed.metadata.cacheReadTokens || 0)}
									<span
										class="tokens"
										title={cacheTotal > 0
											? `Cache creation: ${(parsed.metadata.cacheCreationTokens || 0).toLocaleString()}\nCache read: ${(parsed.metadata.cacheReadTokens || 0).toLocaleString()}`
											: ''}
									>
										{parsed.metadata.inputTokens?.toLocaleString() ?? 0} in / {parsed.metadata.outputTokens?.toLocaleString() ??
											0} out{cacheTotal > 0 ? ` (${cacheTotal.toLocaleString()} cached)` : ''}
									</span>
								{/if}
								<span class="file-time">{formatTime(file.created_at)}</span>
							</div>
						</button>

						<!-- File Content -->
						{#if isExpanded}
							<div class="file-content">
								{#each parsed.sections as section}
									{@const style = sectionStyles[section.type] || sectionStyles.response}
									<div
										class="section"
										style:--section-color={style.color}
										style:--section-bg={style.bg}
									>
										<div class="section-header">
											<span class="section-icon">{style.icon}</span>
											<span class="section-title">{section.title.toUpperCase()}</span>
										</div>
										<div class="section-content">
											<pre>{section.content}</pre>
										</div>
									</div>
								{/each}
							</div>
						{/if}
					</div>
				{/each}

				<!-- Pagination -->
				{#if totalPages > 1}
					<div class="pagination">
						<button
							class="page-btn"
							disabled={currentPage === 1}
							onclick={() => currentPage = 1}
						>
							First
						</button>
						<button
							class="page-btn"
							disabled={currentPage === 1}
							onclick={() => currentPage--}
						>
							Prev
						</button>
						<span class="page-info">
							Page {currentPage} of {totalPages} ({files.length} files)
						</span>
						<button
							class="page-btn"
							disabled={currentPage === totalPages}
							onclick={() => currentPage++}
						>
							Next
						</button>
						<button
							class="page-btn"
							disabled={currentPage === totalPages}
							onclick={() => currentPage = totalPages}
						>
							Last
						</button>
					</div>
				{/if}
			</div>
		{/if}

		<!-- Live streaming content -->
		{#if streamingContent}
			<div class="streaming-entry">
				<div class="streaming-header">
					<span class="streaming-icon">●</span>
					<span class="streaming-label">STREAMING</span>
					<span class="streaming-time">Live</span>
				</div>
				<div class="streaming-content">
					<pre>{streamingContent}</pre>
				</div>
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
		max-height: 800px;
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

	/* Content */
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

	/* Transcript Files */
	.transcript-files {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	.transcript-file {
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		background: var(--bg-primary);
		overflow: hidden;
	}

	.transcript-file.expanded {
		border-color: var(--border-default);
	}

	.file-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		width: 100%;
		padding: var(--space-3) var(--space-4);
		background: var(--bg-tertiary);
		border: none;
		cursor: pointer;
		text-align: left;
		transition: background var(--duration-fast) var(--ease-out);
	}

	.file-header:hover {
		background: var(--bg-surface);
	}

	.file-info {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.file-meta {
		display: flex;
		align-items: center;
		gap: var(--space-3);
	}

	.chevron {
		display: flex;
		color: var(--text-muted);
		transition: transform var(--duration-fast) var(--ease-out);
	}

	.chevron.rotated {
		transform: rotate(90deg);
	}

	.phase-badge {
		padding: var(--space-1) var(--space-2);
		font-size: var(--text-xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		background: var(--accent-subtle);
		color: var(--accent-primary);
		border-radius: var(--radius-sm);
	}

	.iteration {
		font-size: var(--text-sm);
		color: var(--text-secondary);
	}

	.status-badge {
		padding: var(--space-0-5) var(--space-2);
		font-size: var(--text-2xs);
		font-weight: var(--font-medium);
		border-radius: var(--radius-sm);
	}

	.status-badge.complete {
		background: var(--status-success-bg);
		color: var(--status-success);
	}

	.status-badge.blocked {
		background: var(--status-warning-bg);
		color: var(--status-warning);
	}

	.tokens {
		font-size: var(--text-xs);
		color: var(--text-muted);
		font-family: var(--font-mono);
	}

	.file-time {
		font-size: var(--text-xs);
		color: var(--text-muted);
		font-family: var(--font-mono);
	}

	.file-content {
		padding: var(--space-4);
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
		border-top: 1px solid var(--border-subtle);
	}

	/* Sections */
	.section {
		border-left: 3px solid var(--section-color);
		background: var(--section-bg);
		border-radius: 0 var(--radius-md) var(--radius-md) 0;
		overflow: hidden;
	}

	.section-header {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		background: rgba(0, 0, 0, 0.1);
	}

	.section-icon {
		font-size: var(--text-sm);
		color: var(--section-color);
	}

	.section-title {
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		color: var(--section-color);
	}

	.section-content {
		padding: var(--space-3);
		max-height: 500px;
		overflow-y: auto;
	}

	.section-content pre {
		font-family: var(--font-mono);
		font-size: var(--text-sm);
		line-height: var(--leading-relaxed);
		white-space: pre-wrap;
		word-break: break-word;
		color: var(--text-primary);
		margin: 0;
	}

	/* Pagination */
	.pagination {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-2);
		padding: var(--space-4);
		margin-top: var(--space-2);
	}

	.page-btn {
		padding: var(--space-1-5) var(--space-3);
		font-size: var(--text-sm);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-secondary);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.page-btn:hover:not(:disabled) {
		background: var(--bg-surface);
		color: var(--text-primary);
	}

	.page-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.page-info {
		font-size: var(--text-sm);
		color: var(--text-muted);
		padding: 0 var(--space-4);
	}

	/* Streaming */
	.streaming-entry {
		margin-top: var(--space-4);
		border: 1px solid var(--status-success);
		border-radius: var(--radius-md);
		background: var(--status-success-bg);
		overflow: hidden;
	}

	.streaming-header {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		background: rgba(0, 0, 0, 0.1);
	}

	.streaming-icon {
		color: var(--status-success);
		animation: pulse 1.5s ease-in-out infinite;
	}

	.streaming-label {
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		color: var(--status-success);
	}

	.streaming-time {
		font-size: var(--text-2xs);
		color: var(--text-muted);
		margin-left: auto;
	}

	.streaming-content {
		padding: var(--space-3);
	}

	.streaming-content pre {
		font-family: var(--font-mono);
		font-size: var(--text-sm);
		line-height: var(--leading-relaxed);
		white-space: pre-wrap;
		word-break: break-word;
		color: var(--text-primary);
		margin: 0;
	}

	@keyframes pulse {
		0%, 100% { opacity: 1; }
		50% { opacity: 0.4; }
	}
</style>
