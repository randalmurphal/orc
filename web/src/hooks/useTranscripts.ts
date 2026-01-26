/**
 * useTranscripts - Hook for transcript loading with phase filtering
 *
 * Uses Connect RPC TranscriptService to load transcripts.
 * Provides phase filtering and live streaming integration.
 */

import { useState, useEffect, useCallback, useRef } from 'react';
import { create } from '@bufbuild/protobuf';
import { transcriptClient } from '@/lib/client';
import {
	type TranscriptFile,
	type Transcript as ProtoTranscript,
	type TranscriptEntry,
	ListTranscriptsRequestSchema,
	GetTranscriptRequestSchema,
} from '@/gen/orc/v1/transcript_pb';
import { timestampToISO } from '@/lib/time';
import type { TranscriptLine } from '@/hooks/useEvents';

/**
 * Flattened transcript entry for UI consumption.
 * Matches the shape expected by existing components.
 */
export interface FlatTranscriptEntry {
	id: number;
	task_id: string;
	phase: string;
	iteration: number;
	session_id: string;
	type: string;
	content: string;
	model?: string;
	input_tokens: number;
	output_tokens: number;
	timestamp: string;
}

/** Phase summary with counts */
export interface PhaseSummary {
	phase: string;
	transcript_count: number;
}

export interface UseTranscriptsOptions {
	/** Task ID to load transcripts for */
	taskId: string;
	/** Initial phase filter */
	initialPhase?: string;
	/** Page size for loading (default: 50) - kept for API compatibility */
	pageSize?: number;
	/** Enable auto-scroll when new content arrives */
	autoScroll?: boolean;
}

export interface UseTranscriptsResult {
	/** Loaded transcripts (flattened entries) */
	transcripts: FlatTranscriptEntry[];
	/** Phase summary with counts */
	phases: PhaseSummary[];
	/** Pagination info (simplified for new model) */
	pagination: { has_more: boolean; next_cursor: number | null; prev_cursor: number | null; total_count: number } | null;
	/** Current phase filter */
	currentPhase: string | null;
	/** Whether currently loading */
	loading: boolean;
	/** Whether loading more (for scroll compat) */
	loadingMore: boolean;
	/** Error message if any */
	error: string | null;
	/** Whether auto-scroll is enabled */
	isAutoScrollEnabled: boolean;
	/** Load more transcripts (no-op in new model, kept for compat) */
	loadMore: () => Promise<void>;
	/** Load previous transcripts (no-op in new model, kept for compat) */
	loadPrevious: () => Promise<void>;
	/** Change phase filter */
	setPhase: (phase: string | null) => void;
	/** Toggle auto-scroll */
	toggleAutoScroll: () => void;
	/** Refresh transcripts */
	refresh: () => Promise<void>;
	/** Whether there are more transcripts to load */
	hasMore: boolean;
	/** Whether there are previous transcripts to load */
	hasPrevious: boolean;
	/** Merge streaming line into transcripts (for live updates) */
	appendStreamingLine: (line: TranscriptLine) => void;
	/** Clear streaming lines */
	clearStreamingLines: () => void;
	/** Streaming lines not yet in DB */
	streamingLines: TranscriptLine[];
}

/**
 * Convert proto TranscriptEntry to flattened format
 */
function flattenEntry(
	entry: TranscriptEntry,
	transcript: ProtoTranscript,
	index: number
): FlatTranscriptEntry {
	return {
		// Generate unique ID from phase/iteration/index
		id: hashCode(`${transcript.phase}-${transcript.iteration}-${index}`),
		task_id: transcript.taskId,
		phase: transcript.phase,
		iteration: transcript.iteration,
		session_id: transcript.sessionId ?? '',
		type: entry.type,
		content: entry.content,
		model: transcript.model,
		input_tokens: entry.tokens?.inputTokens ?? 0,
		output_tokens: entry.tokens?.outputTokens ?? 0,
		timestamp: timestampToISO(entry.timestamp),
	};
}

/**
 * Simple hash function for generating stable IDs
 */
function hashCode(str: string): number {
	let hash = 0;
	for (let i = 0; i < str.length; i++) {
		const char = str.charCodeAt(i);
		hash = ((hash << 5) - hash) + char;
		hash = hash & hash; // Convert to 32bit integer
	}
	return Math.abs(hash);
}

/**
 * Derive phase summary from transcript files
 */
function derivePhaseSummary(files: TranscriptFile[]): PhaseSummary[] {
	const phaseMap = new Map<string, number>();
	for (const file of files) {
		const count = phaseMap.get(file.phase) ?? 0;
		phaseMap.set(file.phase, count + 1);
	}
	return Array.from(phaseMap.entries()).map(([phase, transcript_count]) => ({
		phase,
		transcript_count,
	}));
}

export function useTranscripts({
	taskId,
	initialPhase,
	autoScroll = true,
}: UseTranscriptsOptions): UseTranscriptsResult {
	const [transcripts, setTranscripts] = useState<FlatTranscriptEntry[]>([]);
	const [phases, setPhases] = useState<PhaseSummary[]>([]);
	const [currentPhase, setCurrentPhase] = useState<string | null>(initialPhase ?? null);
	const [loading, setLoading] = useState(true);
	const [loadingMore, setLoadingMore] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [isAutoScrollEnabled, setIsAutoScrollEnabled] = useState(autoScroll);
	const [streamingLines, setStreamingLines] = useState<TranscriptLine[]>([]);

	// Track task ID changes for reset
	const prevTaskIdRef = useRef<string>(taskId);

	// Load transcripts from Connect RPC
	const loadTranscripts = useCallback(async () => {
		setLoading(true);
		setError(null);
		try {
			// List all transcript files for this task
			const listRequest = create(ListTranscriptsRequestSchema, {
				taskId,
				phase: currentPhase ?? undefined,
			});
			const listResponse = await transcriptClient.listTranscripts(listRequest);
			const files = listResponse.transcripts;

			// Derive phase summary
			// For phase summary, we need all files regardless of filter
			const allFilesRequest = create(ListTranscriptsRequestSchema, { taskId });
			const allFilesResponse = await transcriptClient.listTranscripts(allFilesRequest);
			setPhases(derivePhaseSummary(allFilesResponse.transcripts));

			// Fetch each transcript and flatten entries
			const allEntries: FlatTranscriptEntry[] = [];
			for (const file of files) {
				try {
					const getRequest = create(GetTranscriptRequestSchema, {
						taskId,
						phase: file.phase,
						iteration: file.iteration,
					});
					const getResponse = await transcriptClient.getTranscript(getRequest);
					if (getResponse.transcript) {
						const transcript = getResponse.transcript;
						for (let i = 0; i < transcript.entries.length; i++) {
							allEntries.push(flattenEntry(transcript.entries[i], transcript, i));
						}
					}
				} catch (e) {
					// Log but continue loading other transcripts
					console.warn(`Failed to load transcript ${file.phase}/${file.iteration}:`, e);
				}
			}

			// Sort by timestamp
			allEntries.sort((a, b) => a.timestamp.localeCompare(b.timestamp));
			setTranscripts(allEntries);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load transcripts');
		} finally {
			setLoading(false);
		}
	}, [taskId, currentPhase]);

	// Initial load
	useEffect(() => {
		// Reset if task changes
		if (prevTaskIdRef.current !== taskId) {
			setTranscripts([]);
			setPhases([]);
			setError(null);
			setStreamingLines([]);
			prevTaskIdRef.current = taskId;
		}

		loadTranscripts();
	}, [taskId, currentPhase, loadTranscripts]);

	// Load more (no-op in new model - kept for API compatibility)
	const loadMore = useCallback(async () => {
		// New model loads all at once - this is a no-op
		setLoadingMore(false);
	}, []);

	// Load previous (no-op in new model - kept for API compatibility)
	const loadPrevious = useCallback(async () => {
		// New model loads all at once - this is a no-op
		setLoadingMore(false);
	}, []);

	// Change phase filter
	const setPhase = useCallback((phase: string | null) => {
		setCurrentPhase(phase);
	}, []);

	// Toggle auto-scroll
	const toggleAutoScroll = useCallback(() => {
		setIsAutoScrollEnabled((prev) => !prev);
	}, []);

	// Refresh transcripts
	const refresh = useCallback(async () => {
		setStreamingLines([]);
		await loadTranscripts();
	}, [loadTranscripts]);

	// Append streaming line (called from WebSocket handler)
	const appendStreamingLine = useCallback((line: TranscriptLine) => {
		setStreamingLines((prev) => [...prev, line]);
	}, []);

	// Clear streaming lines (e.g., after DB sync)
	const clearStreamingLines = useCallback(() => {
		setStreamingLines([]);
	}, []);

	return {
		transcripts,
		phases,
		pagination: {
			has_more: false,
			next_cursor: null,
			prev_cursor: null,
			total_count: transcripts.length,
		},
		currentPhase,
		loading,
		loadingMore,
		error,
		isAutoScrollEnabled,
		loadMore,
		loadPrevious,
		setPhase,
		toggleAutoScroll,
		refresh,
		hasMore: false,
		hasPrevious: false,
		appendStreamingLine,
		clearStreamingLines,
		streamingLines,
	};
}
