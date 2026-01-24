/**
 * useTranscripts - Hook for paginated transcript loading with infinite scroll
 *
 * Provides cursor-based pagination, live streaming integration, and phase filtering.
 */

import { useState, useEffect, useCallback, useRef } from 'react';
import type { Transcript, PhaseSummary, TranscriptPaginationResult } from '@/lib/api';
import { getTranscriptsPaginated } from '@/lib/api';
import type { TranscriptLine } from '@/hooks/useWebSocket';

const DEFAULT_PAGE_SIZE = 50;

export interface UseTranscriptsOptions {
	/** Task ID to load transcripts for */
	taskId: string;
	/** Initial phase filter */
	initialPhase?: string;
	/** Page size for loading (default: 50) */
	pageSize?: number;
	/** Enable auto-scroll when new content arrives */
	autoScroll?: boolean;
}

export interface UseTranscriptsResult {
	/** Loaded transcripts */
	transcripts: Transcript[];
	/** Phase summary with counts */
	phases: PhaseSummary[];
	/** Pagination info */
	pagination: TranscriptPaginationResult | null;
	/** Current phase filter */
	currentPhase: string | null;
	/** Whether currently loading */
	loading: boolean;
	/** Whether loading more (infinite scroll) */
	loadingMore: boolean;
	/** Error message if any */
	error: string | null;
	/** Whether auto-scroll is enabled */
	isAutoScrollEnabled: boolean;
	/** Load more transcripts (for infinite scroll) */
	loadMore: () => Promise<void>;
	/** Load previous transcripts (for reverse scroll) */
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

export function useTranscripts({
	taskId,
	initialPhase,
	pageSize = DEFAULT_PAGE_SIZE,
	autoScroll = true,
}: UseTranscriptsOptions): UseTranscriptsResult {
	const [transcripts, setTranscripts] = useState<Transcript[]>([]);
	const [phases, setPhases] = useState<PhaseSummary[]>([]);
	const [pagination, setPagination] = useState<TranscriptPaginationResult | null>(null);
	const [currentPhase, setCurrentPhase] = useState<string | null>(initialPhase ?? null);
	const [loading, setLoading] = useState(true);
	const [loadingMore, setLoadingMore] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [isAutoScrollEnabled, setIsAutoScrollEnabled] = useState(autoScroll);
	const [streamingLines, setStreamingLines] = useState<TranscriptLine[]>([]);

	// Track the most recent transcript ID to detect new streaming content
	const lastKnownIdRef = useRef<number>(0);
	// Track task ID changes for reset
	const prevTaskIdRef = useRef<string>(taskId);

	// Initial load
	useEffect(() => {
		// Reset if task changes
		if (prevTaskIdRef.current !== taskId) {
			setTranscripts([]);
			setPhases([]);
			setPagination(null);
			setError(null);
			setStreamingLines([]);
			lastKnownIdRef.current = 0;
			prevTaskIdRef.current = taskId;
		}

		async function loadInitial() {
			setLoading(true);
			setError(null);
			try {
				const response = await getTranscriptsPaginated(taskId, {
					limit: pageSize,
					direction: 'asc',
					phase: currentPhase ?? undefined,
				});
				setTranscripts(response.transcripts);
				setPhases(response.phases);
				setPagination(response.pagination);

				// Track last known ID
				if (response.transcripts.length > 0) {
					lastKnownIdRef.current = response.transcripts[response.transcripts.length - 1].id;
				}
			} catch (e) {
				setError(e instanceof Error ? e.message : 'Failed to load transcripts');
			} finally {
				setLoading(false);
			}
		}

		loadInitial();
	}, [taskId, currentPhase, pageSize]);

	// Load more transcripts (forward)
	const loadMore = useCallback(async () => {
		if (!pagination?.next_cursor || loadingMore) return;

		setLoadingMore(true);
		try {
			const response = await getTranscriptsPaginated(taskId, {
				limit: pageSize,
				cursor: pagination.next_cursor,
				direction: 'asc',
				phase: currentPhase ?? undefined,
			});
			setTranscripts((prev) => [...prev, ...response.transcripts]);
			setPagination(response.pagination);

			// Update last known ID
			if (response.transcripts.length > 0) {
				lastKnownIdRef.current = response.transcripts[response.transcripts.length - 1].id;
			}
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load more transcripts');
		} finally {
			setLoadingMore(false);
		}
	}, [taskId, currentPhase, pageSize, pagination, loadingMore]);

	// Load previous transcripts (backward)
	const loadPrevious = useCallback(async () => {
		if (!pagination?.prev_cursor || loadingMore) return;

		setLoadingMore(true);
		try {
			const response = await getTranscriptsPaginated(taskId, {
				limit: pageSize,
				cursor: pagination.prev_cursor,
				direction: 'asc',
				phase: currentPhase ?? undefined,
			});
			setTranscripts((prev) => [...response.transcripts, ...prev]);
			setPagination(response.pagination);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load previous transcripts');
		} finally {
			setLoadingMore(false);
		}
	}, [taskId, currentPhase, pageSize, pagination, loadingMore]);

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
		setLoading(true);
		setError(null);
		setStreamingLines([]);
		try {
			const response = await getTranscriptsPaginated(taskId, {
				limit: pageSize,
				direction: 'asc',
				phase: currentPhase ?? undefined,
			});
			setTranscripts(response.transcripts);
			setPhases(response.phases);
			setPagination(response.pagination);

			if (response.transcripts.length > 0) {
				lastKnownIdRef.current = response.transcripts[response.transcripts.length - 1].id;
			}
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to refresh transcripts');
		} finally {
			setLoading(false);
		}
	}, [taskId, currentPhase, pageSize]);

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
		pagination,
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
		hasMore: pagination?.has_more ?? false,
		hasPrevious: pagination?.prev_cursor != null,
		appendStreamingLine,
		clearStreamingLines,
		streamingLines,
	};
}
