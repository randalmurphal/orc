import { useState, useCallback } from 'react';
import type { Transcript } from '@/lib/api';
import type { TranscriptLine } from '@/hooks/useWebSocket';
import { TranscriptViewer } from '@/components/transcript';
import { Icon } from '@/components/ui/Icon';
import { toast } from '@/stores/uiStore';
import { getTranscripts } from '@/lib/api';
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

type ContentBlock = TextBlock | ToolUseBlock | { type: string };

interface TranscriptTabProps {
	taskId: string;
	/** Streaming transcript lines from WebSocket */
	streamingLines?: TranscriptLine[];
	/** Whether the task is currently running */
	isRunning?: boolean;
}

// Parse content blocks from JSON string
function parseContent(content: string): ContentBlock[] {
	try {
		const parsed = JSON.parse(content);
		return Array.isArray(parsed) ? parsed : [{ type: 'text', text: String(content) }];
	} catch {
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

export function TranscriptTab({
	taskId,
	streamingLines = [],
	isRunning = false,
}: TranscriptTabProps) {
	const [transcriptsForExport, setTranscriptsForExport] = useState<Transcript[]>([]);
	const [exportLoading, setExportLoading] = useState(false);

	// Load all transcripts for export (not paginated)
	const loadAllForExport = useCallback(async (): Promise<Transcript[]> => {
		if (transcriptsForExport.length > 0) {
			return transcriptsForExport;
		}
		setExportLoading(true);
		try {
			const data = await getTranscripts(taskId);
			setTranscriptsForExport(data);
			return data;
		} finally {
			setExportLoading(false);
		}
	}, [taskId, transcriptsForExport]);

	// Export transcript to markdown
	const exportToMarkdown = useCallback(async () => {
		const transcripts = await loadAllForExport();
		if (transcripts.length === 0) return;

		const groups = groupByPhase(transcripts);
		const timestamp = new Date().toISOString().slice(0, 16).replace('T', '_').replace(':', '-');
		const filename = `${taskId}-transcript-${timestamp}.md`;

		let content = `# Transcript: ${taskId}\n\n`;
		content += `Generated: ${new Date().toLocaleString()}\n\n`;

		for (const [phase, messages] of groups) {
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
	}, [loadAllForExport, taskId]);

	// Copy transcript to clipboard
	const copyToClipboard = useCallback(async () => {
		const transcripts = await loadAllForExport();
		if (transcripts.length === 0) return;

		const groups = groupByPhase(transcripts);
		let content = '';
		for (const [phase, messages] of groups) {
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
	}, [loadAllForExport]);

	return (
		<div className="transcript-tab-container">
			{/* Export actions header */}
			<div className="transcript-tab-header">
				<h2>Transcript</h2>
				<div className="header-actions">
					<button
						className="header-btn"
						onClick={copyToClipboard}
						title="Copy transcript to clipboard"
						disabled={exportLoading}
					>
						<Icon name="clipboard" size={14} />
						{exportLoading ? 'Loading...' : 'Copy'}
					</button>
					<button
						className="header-btn"
						onClick={exportToMarkdown}
						title="Export transcript as markdown"
						disabled={exportLoading}
					>
						<Icon name="download" size={14} />
						{exportLoading ? 'Loading...' : 'Export'}
					</button>
				</div>
			</div>

			{/* TranscriptViewer with all features */}
			<TranscriptViewer
				taskId={taskId}
				isRunning={isRunning || streamingLines.length > 0}
				height="calc(100% - 60px)"
				showNav={true}
				showSearch={true}
			/>
		</div>
	);
}
