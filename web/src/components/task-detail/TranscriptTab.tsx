import { useState, useCallback } from 'react';
import { create } from '@bufbuild/protobuf';
import { transcriptClient } from '@/lib/client';
import {
	ListTranscriptsRequestSchema,
	GetTranscriptRequestSchema,
	type Transcript as ProtoTranscript,
	type TranscriptEntry,
} from '@/gen/orc/v1/transcript_pb';
import { timestampToISO } from '@/lib/time';
import type { TranscriptLine } from '@/hooks';
import { TranscriptViewer } from '@/components/transcript';
import { Icon } from '@/components/ui/Icon';
import { toast } from '@/stores/uiStore';
import { useCurrentProjectId } from '@/stores';
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

/** Flattened transcript entry for export */
interface FlatExportEntry {
	id: number;
	phase: string;
	type: string;
	content: string;
	timestamp: string;
	model?: string;
}

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
function groupByPhase(transcripts: FlatExportEntry[]): Map<string, FlatExportEntry[]> {
	const groups = new Map<string, FlatExportEntry[]>();
	for (const t of transcripts) {
		const existing = groups.get(t.phase) || [];
		existing.push(t);
		groups.set(t.phase, existing);
	}
	return groups;
}

/** Simple hash function for generating stable IDs */
function hashCode(str: string): number {
	let hash = 0;
	for (let i = 0; i < str.length; i++) {
		const char = str.charCodeAt(i);
		hash = ((hash << 5) - hash) + char;
		hash = hash & hash;
	}
	return Math.abs(hash);
}

/** Flatten a proto transcript entry to export format */
function flattenEntryForExport(
	entry: TranscriptEntry,
	transcript: ProtoTranscript,
	index: number
): FlatExportEntry {
	return {
		id: hashCode(`${transcript.phase}-${transcript.iteration}-${index}`),
		phase: transcript.phase,
		type: entry.type,
		content: entry.content,
		timestamp: timestampToISO(entry.timestamp),
		model: transcript.model,
	};
}

export function TranscriptTab({
	taskId,
	streamingLines = [],
	isRunning = false,
}: TranscriptTabProps) {
	const projectId = useCurrentProjectId();
	const [transcriptsForExport, setTranscriptsForExport] = useState<FlatExportEntry[]>([]);
	const [exportLoading, setExportLoading] = useState(false);

	// Load all transcripts for export using Connect RPC
	const loadAllForExport = useCallback(async (): Promise<FlatExportEntry[]> => {
		if (!projectId) return [];
		if (transcriptsForExport.length > 0) {
			return transcriptsForExport;
		}
		setExportLoading(true);
		try {
			// List all transcript files
			const listRequest = create(ListTranscriptsRequestSchema, { projectId, taskId });
			const listResponse = await transcriptClient.listTranscripts(listRequest);
			const files = listResponse.transcripts;

			// Fetch each transcript and flatten entries
			const allEntries: FlatExportEntry[] = [];
			for (const file of files) {
				try {
					const getRequest = create(GetTranscriptRequestSchema, {
						projectId,
						taskId,
						phase: file.phase,
						iteration: file.iteration,
					});
					const getResponse = await transcriptClient.getTranscript(getRequest);
					if (getResponse.transcript) {
						const transcript = getResponse.transcript;
						for (let i = 0; i < transcript.entries.length; i++) {
							allEntries.push(flattenEntryForExport(transcript.entries[i], transcript, i));
						}
					}
				} catch (e) {
					console.warn(`Failed to load transcript ${file.phase}/${file.iteration}:`, e);
				}
			}

			// Sort by timestamp
			allEntries.sort((a, b) => a.timestamp.localeCompare(b.timestamp));
			setTranscriptsForExport(allEntries);
			return allEntries;
		} finally {
			setExportLoading(false);
		}
	}, [projectId, taskId, transcriptsForExport]);

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
