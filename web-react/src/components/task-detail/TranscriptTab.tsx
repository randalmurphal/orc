import { useState, useEffect, useRef, useMemo, useCallback } from 'react';
import type { TranscriptFile } from '@/lib/types';
import { getTranscripts } from '@/lib/api';
import { Icon } from '@/components/ui/Icon';
import { toast } from '@/stores/uiStore';
import './TranscriptTab.css';

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

interface TranscriptTabProps {
	taskId: string;
	streamingContent?: string;
	autoScroll?: boolean;
}

const PAGE_SIZE = 10;

const sectionStyles: Record<string, { icon: string; colorVar: string; bgVar: string }> = {
	prompt: {
		icon: '\u25B6', // ▶
		colorVar: 'var(--accent-primary)',
		bgVar: 'var(--accent-subtle)',
	},
	'retry-context': {
		icon: '\u21BB', // ↻
		colorVar: 'var(--status-warning)',
		bgVar: 'var(--status-warning-bg)',
	},
	response: {
		icon: '\u25C0', // ◀
		colorVar: 'var(--status-success)',
		bgVar: 'var(--status-success-bg)',
	},
};

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
	sections.forEach((s) => (s.content = s.content.trim()));

	return { phase, iteration, sections, metadata };
}

function formatTime(timestamp: string): string {
	const date = new Date(timestamp);
	return date.toLocaleString([], {
		month: 'short',
		day: 'numeric',
		hour: '2-digit',
		minute: '2-digit',
	});
}

export function TranscriptTab({
	taskId,
	streamingContent = '',
	autoScroll = true,
}: TranscriptTabProps) {
	const containerRef = useRef<HTMLDivElement>(null);
	const [files, setFiles] = useState<TranscriptFile[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [isAutoScrollEnabled, setIsAutoScrollEnabled] = useState(autoScroll);
	const [currentPage, setCurrentPage] = useState(1);
	const [expandedFiles, setExpandedFiles] = useState<Set<string>>(new Set());

	// Load transcripts
	useEffect(() => {
		async function loadTranscripts() {
			setLoading(true);
			setError(null);
			try {
				const data = await getTranscripts(taskId);
				setFiles(data);
			} catch (e) {
				setError(e instanceof Error ? e.message : 'Failed to load transcripts');
			} finally {
				setLoading(false);
			}
		}
		loadTranscripts();
	}, [taskId]);

	// Pagination
	const totalPages = Math.ceil(files.length / PAGE_SIZE);
	const paginatedFiles = useMemo(
		() => files.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE),
		[files, currentPage]
	);

	// Auto-expand all files on initial load
	useEffect(() => {
		if (files.length > 0 && expandedFiles.size === 0) {
			setExpandedFiles(new Set(files.map((f) => f.filename)));
		}
	}, [files, expandedFiles.size]);

	// Auto-scroll when new content added
	useEffect(() => {
		if (isAutoScrollEnabled && containerRef.current) {
			containerRef.current.scrollTop = containerRef.current.scrollHeight;
		}
	}, [files.length, streamingContent.length, isAutoScrollEnabled]);

	const toggleFile = useCallback((filename: string) => {
		setExpandedFiles((prev) => {
			const next = new Set(prev);
			if (next.has(filename)) {
				next.delete(filename);
			} else {
				next.add(filename);
			}
			return next;
		});
	}, []);

	const expandAll = useCallback(() => {
		setExpandedFiles(new Set(files.map((f) => f.filename)));
	}, [files]);

	const collapseAll = useCallback(() => {
		setExpandedFiles(new Set());
	}, []);

	const toggleAutoScroll = useCallback(() => {
		setIsAutoScrollEnabled((prev) => !prev);
	}, []);

	// Export transcript to markdown
	const exportToMarkdown = useCallback(() => {
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

		toast.success('Transcript exported');
	}, [files, taskId]);

	// Copy transcript to clipboard
	const copyToClipboard = useCallback(async () => {
		if (files.length === 0) return;

		let content = '';
		for (const file of files) {
			content += file.content + '\n\n---\n\n';
		}

		try {
			await navigator.clipboard.writeText(content);
			toast.success('Transcript copied to clipboard');
		} catch (e) {
			console.error('Failed to copy to clipboard:', e);
			toast.error('Failed to copy to clipboard');
		}
	}, [files]);

	const isEmpty = files.length === 0 && !streamingContent;

	// Loading state
	if (loading) {
		return (
			<div className="transcript-container">
				<div className="transcript-header">
					<h2>Transcript</h2>
				</div>
				<div className="transcript-content">
					<div className="empty-state">
						<div className="loading-spinner" />
						<p className="empty-title">Loading transcripts...</p>
					</div>
				</div>
			</div>
		);
	}

	// Error state
	if (error) {
		return (
			<div className="transcript-container">
				<div className="transcript-header">
					<h2>Transcript</h2>
				</div>
				<div className="transcript-content">
					<div className="empty-state">
						<Icon name="alert-circle" size={32} />
						<p className="empty-title">Failed to load transcripts</p>
						<p className="empty-hint">{error}</p>
					</div>
				</div>
			</div>
		);
	}

	return (
		<div className="transcript-container">
			{/* Header */}
			<div className="transcript-header">
				<h2>Transcript</h2>
				<div className="header-actions">
					{files.length > 1 && (
						<>
							<button className="header-btn" onClick={expandAll} title="Expand all">
								Expand All
							</button>
							<button className="header-btn" onClick={collapseAll} title="Collapse all">
								Collapse All
							</button>
						</>
					)}
					<button
						className="header-btn"
						onClick={copyToClipboard}
						title="Copy transcript to clipboard"
						disabled={files.length === 0}
					>
						<Icon name="clipboard" size={14} />
						Copy
					</button>
					<button
						className="header-btn"
						onClick={exportToMarkdown}
						title="Export transcript as markdown"
						disabled={files.length === 0}
					>
						<Icon name="download" size={14} />
						Export
					</button>
					<button
						className={`header-btn ${isAutoScrollEnabled ? 'active' : ''}`}
						onClick={toggleAutoScroll}
						title={isAutoScrollEnabled ? 'Disable auto-scroll' : 'Enable auto-scroll'}
					>
						<Icon name="chevrons-down" size={14} />
						Auto-scroll
					</button>
				</div>
			</div>

			{/* Content */}
			<div className="transcript-content" ref={containerRef}>
				{isEmpty ? (
					<div className="empty-state">
						<div className="empty-icon">
							<Icon name="terminal" size={32} />
						</div>
						<p className="empty-title">No transcript yet</p>
						<p className="empty-hint">Run the task to see live output</p>
					</div>
				) : (
					<div className="transcript-files">
						{paginatedFiles.map((file) => {
							const parsed = parseTranscript(file.content);
							const isExpanded = expandedFiles.has(file.filename);
							const cacheTotal =
								(parsed.metadata.cacheCreationTokens || 0) +
								(parsed.metadata.cacheReadTokens || 0);

							return (
								<div
									key={file.filename}
									className={`transcript-file ${isExpanded ? 'expanded' : ''}`}
								>
									{/* File Header */}
									<button className="file-header" onClick={() => toggleFile(file.filename)}>
										<div className="file-info">
											<span className={`chevron ${isExpanded ? 'rotated' : ''}`}>
												<Icon name="chevron-right" size={16} />
											</span>
											<span className="phase-badge">{parsed.phase}</span>
											<span className="iteration">Iteration {parsed.iteration}</span>
											{parsed.metadata.complete && (
												<span className="status-badge complete">
													<Icon name="check" size={10} /> Complete
												</span>
											)}
											{parsed.metadata.blocked && (
												<span className="status-badge blocked">
													<Icon name="alert-triangle" size={10} /> Blocked
												</span>
											)}
										</div>
										<div className="file-meta">
											{(parsed.metadata.inputTokens || parsed.metadata.outputTokens) && (
												<span
													className="tokens"
													title={
														cacheTotal > 0
															? `Cache creation: ${(parsed.metadata.cacheCreationTokens || 0).toLocaleString()}\nCache read: ${(parsed.metadata.cacheReadTokens || 0).toLocaleString()}`
															: undefined
													}
												>
													{parsed.metadata.inputTokens?.toLocaleString() ?? 0} in /{' '}
													{parsed.metadata.outputTokens?.toLocaleString() ?? 0} out
													{cacheTotal > 0 && ` (${cacheTotal.toLocaleString()} cached)`}
												</span>
											)}
											<span className="file-time">{formatTime(file.created_at)}</span>
										</div>
									</button>

									{/* File Content */}
									{isExpanded && (
										<div className="file-content">
											{parsed.sections.map((section, idx) => {
												const style =
													sectionStyles[section.type] || sectionStyles.response;
												return (
													<div
														key={idx}
														className="section"
														style={
															{
																'--section-color': style.colorVar,
																'--section-bg': style.bgVar,
															} as React.CSSProperties
														}
													>
														<div className="section-header">
															<span className="section-icon">{style.icon}</span>
															<span className="section-title">
																{section.title.toUpperCase()}
															</span>
														</div>
														<div className="section-content">
															<pre>{section.content}</pre>
														</div>
													</div>
												);
											})}
										</div>
									)}
								</div>
							);
						})}

						{/* Pagination */}
						{totalPages > 1 && (
							<div className="pagination">
								<button
									className="page-btn"
									disabled={currentPage === 1}
									onClick={() => setCurrentPage(1)}
								>
									First
								</button>
								<button
									className="page-btn"
									disabled={currentPage === 1}
									onClick={() => setCurrentPage((p) => p - 1)}
								>
									Prev
								</button>
								<span className="page-info">
									Page {currentPage} of {totalPages} ({files.length} files)
								</span>
								<button
									className="page-btn"
									disabled={currentPage === totalPages}
									onClick={() => setCurrentPage((p) => p + 1)}
								>
									Next
								</button>
								<button
									className="page-btn"
									disabled={currentPage === totalPages}
									onClick={() => setCurrentPage(totalPages)}
								>
									Last
								</button>
							</div>
						)}
					</div>
				)}

				{/* Live streaming content */}
				{streamingContent && (
					<div className="streaming-entry">
						<div className="streaming-header">
							<span className="streaming-icon">●</span>
							<span className="streaming-label">STREAMING</span>
							<span className="streaming-time">Live</span>
						</div>
						<div className="streaming-content">
							<pre>{streamingContent}</pre>
						</div>
					</div>
				)}
			</div>
		</div>
	);
}
