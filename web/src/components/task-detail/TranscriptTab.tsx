import { useState, useEffect, useRef, useMemo, useCallback } from 'react';
import type { Transcript } from '@/lib/api';
import type { TranscriptLine } from '@/hooks/useWebSocket';
import { getTranscripts } from '@/lib/api';
import { Icon } from '@/components/ui/Icon';
import { toast } from '@/stores/uiStore';
import './TranscriptTab.css';

// Content block types from Claude's JSONL format
interface TextBlock {
	type: 'text';
	text: string;
}

interface ToolUseBlock {
	type: 'tool_use';
	id: string;
	name: string;
	input: Record<string, unknown>;
}

interface ToolResultBlock {
	type: 'tool_result';
	tool_use_id: string;
	content: string | Array<{ type: string; text?: string }>;
}

type ContentBlock = TextBlock | ToolUseBlock | ToolResultBlock | { type: string };

interface TranscriptTabProps {
	taskId: string;
	/** Streaming transcript lines from WebSocket */
	streamingLines?: TranscriptLine[];
	autoScroll?: boolean;
}

const PAGE_SIZE = 50; // Messages per page

// Group transcripts by phase
function groupByPhase(transcripts: Transcript[]): Map<string, Transcript[]> {
	const groups = new Map<string, Transcript[]>();
	for (const t of transcripts) {
		const existing = groups.get(t.phase) || [];
		existing.push(t);
		groups.set(t.phase, existing);
	}
	return groups;
}

// Parse content blocks from JSON string
function parseContent(content: string): ContentBlock[] {
	try {
		const parsed = JSON.parse(content);
		return Array.isArray(parsed) ? parsed : [{ type: 'text', text: String(content) }];
	} catch {
		// If not JSON, treat as plain text
		return [{ type: 'text', text: content }];
	}
}

// Extract text from content blocks
function extractText(blocks: ContentBlock[]): string {
	return blocks
		.filter((b): b is TextBlock => b.type === 'text')
		.map((b) => b.text)
		.join('\n');
}

// Extract tool calls from content blocks
function extractToolCalls(blocks: ContentBlock[]): ToolUseBlock[] {
	return blocks.filter((b): b is ToolUseBlock => b.type === 'tool_use');
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

function formatTokens(transcript: Transcript): string {
	const parts = [];
	const inputTokens = transcript.input_tokens || 0;
	const outputTokens = transcript.output_tokens || 0;
	const cacheRead = transcript.cache_read_tokens || 0;
	const cacheCreation = transcript.cache_creation_tokens || 0;

	if (inputTokens) parts.push(`${inputTokens.toLocaleString()} in`);
	if (outputTokens) parts.push(`${outputTokens.toLocaleString()} out`);
	const cached = cacheRead + cacheCreation;
	if (cached > 0) parts.push(`(${cached.toLocaleString()} cached)`);
	return parts.join(' / ');
}

// Tool call display component
function ToolCallView({ tool }: { tool: ToolUseBlock }) {
	const [expanded, setExpanded] = useState(false);

	return (
		<div className="tool-call">
			<button className="tool-call-header" onClick={() => setExpanded(!expanded)}>
				<Icon name={expanded ? 'chevron-down' : 'chevron-right'} size={12} />
				<span className="tool-name">{tool.name}</span>
			</button>
			{expanded && (
				<pre className="tool-input">{JSON.stringify(tool.input, null, 2)}</pre>
			)}
		</div>
	);
}

// Single message display
function MessageView({ transcript }: { transcript: Transcript }) {
	const blocks = parseContent(transcript.content);
	const text = extractText(blocks);
	const toolCalls = extractToolCalls(blocks);
	const isAssistant = transcript.type === 'assistant';

	return (
		<div className={`transcript-message ${transcript.type}`}>
			<div className="message-header">
				<span className={`message-type ${transcript.type}`}>
					{transcript.type.toUpperCase()}
				</span>
				{transcript.model && <span className="message-model">{transcript.model}</span>}
				<span className="message-time">{formatTime(transcript.timestamp)}</span>
				{isAssistant && (transcript.input_tokens > 0 || transcript.output_tokens > 0) && (
					<span className="message-tokens">{formatTokens(transcript)}</span>
				)}
			</div>
			<div className="message-content">
				{text && <pre className="message-text">{text}</pre>}
				{toolCalls.length > 0 && (
					<div className="tool-calls">
						<div className="tool-calls-label">Tool Calls ({toolCalls.length})</div>
						{toolCalls.map((tool) => (
							<ToolCallView key={tool.id} tool={tool} />
						))}
					</div>
				)}
			</div>
		</div>
	);
}

// Phase group display
function PhaseGroup({ phase, transcripts }: { phase: string; transcripts: Transcript[] }) {
	const [expanded, setExpanded] = useState(true);

	// Calculate totals for this phase (with defensive null handling)
	const totals = useMemo(() => {
		return transcripts.reduce(
			(acc, t) => ({
				input: acc.input + (t.input_tokens || 0),
				output: acc.output + (t.output_tokens || 0),
				cached: acc.cached + (t.cache_read_tokens || 0) + (t.cache_creation_tokens || 0),
			}),
			{ input: 0, output: 0, cached: 0 }
		);
	}, [transcripts]);

	return (
		<div className={`phase-group ${expanded ? 'expanded' : ''}`}>
			<button className="phase-header" onClick={() => setExpanded(!expanded)}>
				<Icon name={expanded ? 'chevron-down' : 'chevron-right'} size={16} />
				<span className="phase-name">{phase}</span>
				<span className="phase-count">{transcripts.length} messages</span>
				{totals.input > 0 && (
					<span className="phase-tokens">
						{totals.input.toLocaleString()} in / {totals.output.toLocaleString()} out
						{totals.cached > 0 && ` (${totals.cached.toLocaleString()} cached)`}
					</span>
				)}
			</button>
			{expanded && (
				<div className="phase-messages">
					{transcripts.map((t) => (
						<MessageView key={t.message_uuid} transcript={t} />
					))}
				</div>
			)}
		</div>
	);
}

export function TranscriptTab({
	taskId,
	streamingLines = [],
	autoScroll = true,
}: TranscriptTabProps) {
	const containerRef = useRef<HTMLDivElement>(null);
	const [transcripts, setTranscripts] = useState<Transcript[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [isAutoScrollEnabled, setIsAutoScrollEnabled] = useState(autoScroll);
	const [currentPage, setCurrentPage] = useState(1);

	// Load transcripts
	useEffect(() => {
		async function loadTranscripts() {
			setLoading(true);
			setError(null);
			try {
				const data = await getTranscripts(taskId);
				setTranscripts(data);
			} catch (e) {
				setError(e instanceof Error ? e.message : 'Failed to load transcripts');
			} finally {
				setLoading(false);
			}
		}
		loadTranscripts();
	}, [taskId]);

	// Group by phase
	const phaseGroups = useMemo(() => groupByPhase(transcripts), [transcripts]);

	// Pagination
	const totalPages = Math.ceil(transcripts.length / PAGE_SIZE);
	const paginatedTranscripts = useMemo(
		() => transcripts.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE),
		[transcripts, currentPage]
	);
	const paginatedPhases = useMemo(
		() => groupByPhase(paginatedTranscripts),
		[paginatedTranscripts]
	);

	// Auto-scroll when new content added
	useEffect(() => {
		if (isAutoScrollEnabled && containerRef.current) {
			containerRef.current.scrollTop = containerRef.current.scrollHeight;
		}
	}, [transcripts.length, streamingLines.length, isAutoScrollEnabled]);

	const toggleAutoScroll = useCallback(() => {
		setIsAutoScrollEnabled((prev) => !prev);
	}, []);

	// Export transcript to markdown
	const exportToMarkdown = useCallback(() => {
		if (transcripts.length === 0) return;

		const timestamp = new Date().toISOString().slice(0, 16).replace('T', '_').replace(':', '-');
		const filename = `${taskId}-transcript-${timestamp}.md`;

		let content = `# Transcript: ${taskId}\n\n`;
		content += `Generated: ${new Date().toLocaleString()}\n\n`;

		for (const [phase, messages] of phaseGroups) {
			content += `## ${phase}\n\n`;
			for (const t of messages) {
				content += `### ${t.type} (${formatTime(t.timestamp)})\n\n`;
				const blocks = parseContent(t.content);
				const text = extractText(blocks);
				if (text) content += `${text}\n\n`;
				const tools = extractToolCalls(blocks);
				for (const tool of tools) {
					content += `**Tool: ${tool.name}**\n\`\`\`json\n${JSON.stringify(tool.input, null, 2)}\n\`\`\`\n\n`;
				}
			}
			content += `---\n\n`;
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
	}, [transcripts, phaseGroups, taskId]);

	// Copy transcript to clipboard
	const copyToClipboard = useCallback(async () => {
		if (transcripts.length === 0) return;

		let content = '';
		for (const [phase, messages] of phaseGroups) {
			content += `=== ${phase} ===\n\n`;
			for (const t of messages) {
				content += `[${t.type}] ${formatTime(t.timestamp)}\n`;
				const blocks = parseContent(t.content);
				const text = extractText(blocks);
				if (text) content += `${text}\n`;
				content += '\n';
			}
		}

		try {
			await navigator.clipboard.writeText(content);
			toast.success('Transcript copied to clipboard');
		} catch (e) {
			console.error('Failed to copy to clipboard:', e);
			toast.error('Failed to copy to clipboard');
		}
	}, [transcripts, phaseGroups]);

	const hasStreamingContent = streamingLines.length > 0;
	const isEmpty = transcripts.length === 0 && !hasStreamingContent;

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
					<button
						className="header-btn"
						onClick={copyToClipboard}
						title="Copy transcript to clipboard"
						disabled={transcripts.length === 0}
					>
						<Icon name="clipboard" size={14} />
						Copy
					</button>
					<button
						className="header-btn"
						onClick={exportToMarkdown}
						title="Export transcript as markdown"
						disabled={transcripts.length === 0}
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
						<p className="empty-hint">Run the task to see Claude's output</p>
					</div>
				) : (
					<div className="transcript-messages">
						{Array.from(paginatedPhases).map(([phase, messages]) => (
							<PhaseGroup key={phase} phase={phase} transcripts={messages} />
						))}

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
									Page {currentPage} of {totalPages} ({transcripts.length} messages)
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
				{hasStreamingContent && (
					<div className="streaming-entry">
						<div className="streaming-header">
							<span className="streaming-icon">●</span>
							<span className="streaming-label">STREAMING</span>
							<span className="streaming-time">Live</span>
						</div>
						<div className="streaming-content">
							{streamingLines.map((line, idx) => (
								<div
									key={idx}
									className={`streaming-line streaming-line-${line.type}`}
								>
									{line.type !== 'chunk' && (
										<div className="streaming-line-header">
											<span className="streaming-line-type">{line.type.toUpperCase()}</span>
											<span className="streaming-line-phase">{line.phase} #{line.iteration}</span>
										</div>
									)}
									<pre className="streaming-line-content">{line.content}</pre>
								</div>
							))}
						</div>
					</div>
				)}
			</div>
		</div>
	);
}
