/**
 * LiveOutputPanel - Real-time transcript display for Task Detail page
 *
 * Displays streaming transcript content with:
 * - Real-time updates via WebSocket events
 * - Different styling for prompt/response/tool/error messages
 * - Auto-scroll behavior during streaming
 * - Virtual scrolling for large transcripts
 * - Loading and error states
 */

import { useState, useEffect, useRef, useCallback } from 'react';
import type { ReactNode } from 'react';
import { useTaskSubscription, type TranscriptLine } from '@/hooks/useEvents';
import { useCurrentProjectId } from '@/stores';
import './LiveOutputPanel.css';

export interface LiveOutputPanelProps {
	/** Task ID to display live output for */
	taskId: string;
	/** Whether the task is actively streaming */
	isStreaming?: boolean;
	/** Whether to auto-scroll to new content (default: true when streaming) */
	autoScroll?: boolean;
	/** Loading state for initial connection */
	loading?: boolean;
	/** Error message if connection failed */
	error?: string;
	/** Retry callback for connection errors */
	onRetry?: () => void;
	/** Compact mode for smaller screens */
	compact?: boolean;
}

interface TranscriptMessageProps {
	line: TranscriptLine;
	compact?: boolean;
}

function TranscriptMessage({ line, compact }: TranscriptMessageProps) {
	const formatTimestamp = (timestamp: string) => {
		try {
			const date = new Date(timestamp);
			return date.toLocaleTimeString(undefined, {
				hour: '2-digit',
				minute: '2-digit',
				second: compact ? undefined : '2-digit'
			});
		} catch {
			return timestamp;
		}
	};

	return (
		<div
			className={`transcript-message transcript-message--${line.type}`}
			data-message-type={line.type}
		>
			<div className="transcript-message-header">
				<div className="transcript-message-meta">
					{line.phase && (
						<span className="transcript-phase">{line.phase}</span>
					)}
					<span className="transcript-timestamp">
						{formatTimestamp(line.timestamp)}
					</span>
				</div>
				{line.tokens && (
					<div className="transcript-tokens">
						<span className="token-count">
							{line.tokens.input} input tokens, {line.tokens.output} output tokens
						</span>
					</div>
				)}
			</div>
			<div className="transcript-message-content">
				<pre>{line.content}</pre>
			</div>
		</div>
	);
}

function VirtualizedTranscript({
	transcript,
	compact
}: {
	transcript: TranscriptLine[],
	compact?: boolean
}) {
	// Simple virtualization for large transcripts
	// Only render visible items in the viewport
	const [visibleRange, setVisibleRange] = useState({ start: 0, end: Math.min(50, transcript.length) });
	const containerRef = useRef<HTMLDivElement>(null);

	const updateVisibleRange = useCallback(() => {
		const container = containerRef.current;
		if (!container) return;

		const itemHeight = 100; // Approximate height per message
		const containerHeight = container.clientHeight;
		const scrollTop = container.scrollTop;

		const start = Math.max(0, Math.floor(scrollTop / itemHeight) - 5);
		const end = Math.min(transcript.length, start + Math.ceil(containerHeight / itemHeight) + 10);

		setVisibleRange({ start, end });
	}, [transcript.length]);

	useEffect(() => {
		const container = containerRef.current;
		if (!container) return;

		container.addEventListener('scroll', updateVisibleRange);
		return () => container.removeEventListener('scroll', updateVisibleRange);
	}, [updateVisibleRange]);

	const visibleItems = transcript.slice(visibleRange.start, visibleRange.end);

	return (
		<div
			ref={containerRef}
			className="transcript-virtual-container"
			data-testid="transcript-virtual-list"
			data-virtual="true"
		>
			{/* Spacer for items before visible range */}
			<div style={{ height: visibleRange.start * 100 }} />

			{visibleItems.map((line, index) => (
				<TranscriptMessage
					key={visibleRange.start + index}
					line={line}
					compact={compact}
				/>
			))}

			{/* Spacer for items after visible range */}
			<div style={{ height: (transcript.length - visibleRange.end) * 100 }} />
		</div>
	);
}

export function LiveOutputPanel({
	taskId,
	isStreaming = false,
	autoScroll = true,
	loading = false,
	error,
	onRetry,
	compact = false,
}: LiveOutputPanelProps): ReactNode {
	const projectId = useCurrentProjectId();
	const { transcript } = useTaskSubscription(taskId);
	const [userScrolledUp, setUserScrolledUp] = useState(false);
	const scrollContainerRef = useRef<HTMLDivElement>(null);
	const lastMessageRef = useRef<HTMLDivElement>(null);

	// Auto-scroll to bottom when new messages arrive (if enabled and user hasn't manually scrolled up)
	useEffect(() => {
		if (!isStreaming || !autoScroll || userScrolledUp) return;

		// Scroll to the last message
		if (lastMessageRef.current) {
			lastMessageRef.current.scrollIntoView({
				behavior: 'smooth',
				block: 'end'
			});
		}
	}, [transcript.length, isStreaming, autoScroll, userScrolledUp]);

	// Handle manual scroll to detect if user scrolled up
	const handleScroll = useCallback(() => {
		const container = scrollContainerRef.current;
		if (!container) return;

		const { scrollTop, scrollHeight, clientHeight } = container;
		const isAtBottom = scrollTop + clientHeight >= scrollHeight - 50; // 50px threshold

		setUserScrolledUp(!isAtBottom);

		// Update auto-scroll attribute for tests
		container.setAttribute('data-auto-scroll', isAtBottom.toString());
	}, []);

	// Reset user scroll state when switching tasks
	useEffect(() => {
		setUserScrolledUp(false);
	}, [taskId]);

	// Error state
	if (error) {
		return (
			<div
				className={`live-output-panel ${compact ? 'live-output-panel--compact' : ''}`}
				data-testid="live-output-panel"
				style={{ height: '100%', width: '100%' }}
			>
				<div className="live-output-error">
					<div className="error-icon">⚠️</div>
					<p className="error-message">{error}</p>
					{onRetry && (
						<button
							className="retry-button"
							onClick={onRetry}
							aria-label="Retry connection"
						>
							Retry Connection
						</button>
					)}
				</div>
			</div>
		);
	}

	// Loading state
	if (loading) {
		return (
			<div
				className={`live-output-panel ${compact ? 'live-output-panel--compact' : ''}`}
				data-testid="live-output-panel"
				style={{ height: '100%', width: '100%' }}
			>
				<div className="live-output-loading">
					<div className="loading-spinner" aria-label="Loading indicator" />
					<p>Connecting to live output...</p>
				</div>
			</div>
		);
	}

	// Empty state
	if (transcript.length === 0) {
		return (
			<div
				className={`live-output-panel ${compact ? 'live-output-panel--compact' : ''}`}
				data-testid="live-output-panel"
				style={{ height: '100%', width: '100%' }}
			>
				<div className="live-output-empty">
					<div className="empty-icon">📄</div>
					<p className="empty-title">No output yet</p>
					<p className="empty-subtitle">Waiting for task to start generating output...</p>
				</div>
			</div>
		);
	}

	// Determine if we should use virtual scrolling for large transcripts
	const useVirtualScrolling = transcript.length > 100;

	return (
		<div
			className={`live-output-panel ${compact ? 'live-output-panel--compact' : ''}`}
			data-testid="live-output-panel"
			style={{ height: '100%', width: '100%' }}
			aria-label={`Live output for ${taskId}`}
		>
			{/* Header with streaming indicator */}
			<div className="live-output-header">
				<div className="output-status">
					{isStreaming && (
						<>
							<div className="streaming-indicator" aria-label="Streaming indicator">
								<div className="streaming-pulse" />
								<span>Streaming</span>
							</div>
						</>
					)}
				</div>
			</div>

			{/* Transcript content */}
			<div
				ref={scrollContainerRef}
				className="live-output-content"
				role="log"
				tabIndex={0}
				onScroll={handleScroll}
				data-auto-scroll={!userScrolledUp}
			>
				{useVirtualScrolling ? (
					<VirtualizedTranscript transcript={transcript} compact={compact} />
				) : (
					<>
						{transcript.map((line, index) => (
							<TranscriptMessage
								key={index}
								line={line}
								compact={compact}
							/>
						))}
						{/* Invisible element to scroll to */}
						<div ref={lastMessageRef} />
					</>
				)}
			</div>

			{/* Streaming indicator at bottom when active */}
			{isStreaming && transcript.length > 0 && (
				<div className="live-output-footer">
					<div className="streaming-status">
						<div className="streaming-dot" />
						<span>Live streaming...</span>
					</div>
				</div>
			)}
		</div>
	);
}