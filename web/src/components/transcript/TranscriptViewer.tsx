/**
 * TranscriptViewer - Main transcript display with virtual scroll and live streaming
 *
 * Features:
 * - Virtual scrolling for large transcripts (15-30MB)
 * - Cursor-based pagination with infinite scroll
 * - Live streaming via WebSocket with auto-scroll
 * - Collapsible sections by phase/iteration
 * - Search within transcript
 * - Syntax highlighting for code blocks
 */

import { useState, useCallback, useRef, useEffect, type ReactNode } from 'react';
import type { TranscriptLine } from '@/hooks/useEvents';
import { useTranscripts, type FlatTranscriptEntry } from '@/hooks/useTranscripts';
import { formatNumber } from '@/lib/format';
import { TranscriptNav, type TranscriptNavPhase } from './TranscriptNav';
import { TranscriptSection, type TranscriptSectionType } from './TranscriptSection';
import { TranscriptVirtualList } from './TranscriptVirtualList';
import { TranscriptSearch } from './TranscriptSearch';
import { Icon } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';
import './TranscriptViewer.css';

export interface TranscriptViewerProps {
	/** Task ID to display transcripts for */
	taskId: string;
	/** Whether the task is currently running (enables streaming) */
	isRunning?: boolean;
	/** Height of the viewer (default: '600px') */
	height?: string | number;
	/** Whether to show the navigation sidebar (default: true) */
	showNav?: boolean;
	/** Whether to show the search bar (default: true) */
	showSearch?: boolean;
	/** Initial phase filter */
	initialPhase?: string;
}

/** Section data structure for hierarchical display */
export interface SectionData {
	type: TranscriptSectionType;
	title: string;
	subtitle?: string;
	content?: string;
	id: number;
	timestamp: string;
	children?: SectionData[];
	tokens?: number;
}

export function TranscriptViewer({
	taskId,
	isRunning = false,
	height = '600px',
	showNav = true,
	showSearch = true,
	initialPhase,
}: TranscriptViewerProps) {
	const [searchQuery, setSearchQuery] = useState('');
	const [searchResults, setSearchResults] = useState<number[]>([]);
	const [currentResultIndex, setCurrentResultIndex] = useState(-1);
	const [navCollapsed, setNavCollapsed] = useState(false);
	const scrollContainerRef = useRef<HTMLDivElement>(null);

	// Use paginated transcript hook
	const {
		transcripts,
		phases,
		loading,
		loadingMore,
		error,
		hasMore,
		hasPrevious,
		loadMore,
		loadPrevious,
		setPhase,
		currentPhase,
		isAutoScrollEnabled,
		toggleAutoScroll,
		streamingLines,
		clearStreamingLines,
		refresh,
	} = useTranscripts({
		taskId,
		initialPhase,
		pageSize: 50,
		autoScroll: isRunning,
	});

	// TODO: Transcript streaming will be implemented via Connect RPC event streaming
	// The event handler will dispatch transcript events to this component via stores

	// Refresh to sync streaming lines to DB periodically
	useEffect(() => {
		if (!isRunning || streamingLines.length === 0) return;

		// Refresh every 5 seconds to sync streaming lines
		const interval = setInterval(() => {
			refresh();
			clearStreamingLines();
		}, 5000);

		return () => clearInterval(interval);
	}, [isRunning, streamingLines.length, refresh, clearStreamingLines]);

	// Handle infinite scroll
	const handleScroll = useCallback(() => {
		const container = scrollContainerRef.current;
		if (!container || loadingMore) return;

		const { scrollTop, scrollHeight, clientHeight } = container;

		// Load more when near bottom (within 200px)
		if (scrollTop + clientHeight >= scrollHeight - 200 && hasMore) {
			loadMore();
		}

		// Load previous when near top (within 200px)
		if (scrollTop <= 200 && hasPrevious) {
			loadPrevious();
		}
	}, [loadMore, loadPrevious, hasMore, hasPrevious, loadingMore]);

	// Search functionality
	const handleSearch = useCallback(
		(query: string) => {
			setSearchQuery(query);

			if (!query) {
				setSearchResults([]);
				setCurrentResultIndex(-1);
				return;
			}

			// Find matching transcript IDs
			const results: number[] = [];
			transcripts.forEach((t) => {
				if (t.content.toLowerCase().includes(query.toLowerCase())) {
					results.push(t.id);
				}
			});
			setSearchResults(results);
			setCurrentResultIndex(results.length > 0 ? 0 : -1);
		},
		[transcripts]
	);

	const handleNextResult = useCallback(() => {
		if (searchResults.length === 0) return;
		setCurrentResultIndex((prev) => (prev + 1) % searchResults.length);
	}, [searchResults.length]);

	const handlePrevResult = useCallback(() => {
		if (searchResults.length === 0) return;
		setCurrentResultIndex((prev) => (prev - 1 + searchResults.length) % searchResults.length);
	}, [searchResults.length]);

	// Scroll to search result
	useEffect(() => {
		if (currentResultIndex < 0 || searchResults.length === 0) return;

		const targetId = searchResults[currentResultIndex];
		const element = document.getElementById(`transcript-${targetId}`);
		if (element) {
			element.scrollIntoView({ behavior: 'smooth', block: 'center' });
		}
	}, [currentResultIndex, searchResults]);

	// Navigate to phase/iteration
	const handleNavClick = useCallback(
		(phase: string, _iteration?: number) => {
			setPhase(phase);
		},
		[setPhase]
	);

	// Compute phase stats for nav
	const phaseStats: TranscriptNavPhase[] = phases.map((p) => ({
		phase: p.phase,
		iterations: 0, // TODO: Calculate from transcripts
		transcript_count: p.transcript_count,
		status: 'completed' as const, // TODO: Get actual status from task state
	}));

	// Build section hierarchy for current view
	const buildSections = useCallback(
		(transcriptList: FlatTranscriptEntry[], streaming: TranscriptLine[]): SectionData[] => {
			const sections: SectionData[] = [];
			let currentPhaseSection: SectionData | null = null;

			for (const t of transcriptList) {
				// Check if this is a new phase
				if (!currentPhaseSection || currentPhaseSection.title !== t.phase) {
					currentPhaseSection = {
						type: 'phase',
						title: t.phase,
						id: t.id,
						timestamp: t.timestamp,
						children: [],
						tokens: 0,
					};
					sections.push(currentPhaseSection);
				}

				// Parse content
				let contentText = '';
				try {
					const contentBlocks = JSON.parse(t.content || '[]');
					if (Array.isArray(contentBlocks)) {
						contentText = contentBlocks
							.map((block: { text?: string; type?: string }) => {
								if (typeof block === 'string') return block;
								if (block.text) return block.text;
								if (block.type === 'tool_use') return '[Tool Use]';
								return '';
							})
							.join('\n');
					} else if (typeof contentBlocks === 'string') {
						contentText = contentBlocks;
					}
				} catch {
					contentText = t.content;
				}

				// Determine section type
				let sectionType: TranscriptSectionType = 'response';
				if (t.type === 'user') {
					sectionType = 'prompt';
				} else if (t.type === 'assistant') {
					sectionType = 'response';
				} else if (t.type === 'queue-operation') {
					sectionType = 'system';
				}

				// Add to current phase
				const section: SectionData = {
					type: sectionType,
					title: t.type === 'user' ? 'Prompt' : t.type === 'assistant' ? 'Response' : 'System',
					subtitle: t.model,
					content: contentText,
					id: t.id,
					timestamp: t.timestamp,
					tokens: t.input_tokens + t.output_tokens,
				};

				if (currentPhaseSection.children) {
					currentPhaseSection.children.push(section);
				}
				currentPhaseSection.tokens = (currentPhaseSection.tokens || 0) + (section.tokens || 0);
			}

			// Add streaming lines as temporary sections
			if (streaming.length > 0) {
				const streamingPhase = streaming[0]?.phase || 'streaming';

				// Find or create phase section for streaming content
				let streamingSection = sections.find(s => s.title === streamingPhase);
				if (!streamingSection) {
					streamingSection = {
						type: 'phase',
						title: streamingPhase,
						id: -1,
						timestamp: streaming[0]?.timestamp || new Date().toISOString(),
						children: [],
					};
					sections.push(streamingSection);
				}

				// Add streaming lines as children
				for (let i = 0; i < streaming.length; i++) {
					const line = streaming[i];
					let lineType: TranscriptSectionType = 'response';
					if (line.type === 'prompt') {
						lineType = 'prompt';
					} else if (line.type === 'tool') {
						lineType = 'tool_call';
					} else if (line.type === 'error') {
						lineType = 'error';
					}

					streamingSection.children?.push({
						type: lineType,
						title: line.type,
						content: line.content,
						id: -(i + 1),
						timestamp: line.timestamp,
					});
				}
			}

			return sections;
		},
		[]
	);

	const sections = buildSections(transcripts, streamingLines);

	// Render content for a section
	const renderSectionContent = (section: SectionData, highlightedId?: number): ReactNode => {
		if (section.content) {
			return (
				<pre
					className={section.id === highlightedId ? 'highlighted' : ''}
					id={`transcript-${section.id}`}
				>
					{highlightSearchTerms(section.content, searchQuery)}
				</pre>
			);
		}
		return null;
	};

	// Highlight search terms in text
	const highlightSearchTerms = (text: string, query: string): ReactNode => {
		if (!query) return text;

		const parts = text.split(new RegExp(`(${escapeRegExp(query)})`, 'gi'));
		return parts.map((part, i) =>
			part.toLowerCase() === query.toLowerCase() ? (
				<mark key={i} className="search-highlight">
					{part}
				</mark>
			) : (
				part
			)
		);
	};

	if (loading && transcripts.length === 0) {
		return (
			<div className="transcript-viewer" style={{ height }}>
				<div className="transcript-viewer-loading">
					<div className="loading-spinner" />
					<p>Loading transcripts...</p>
				</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className="transcript-viewer" style={{ height }}>
				<div className="transcript-viewer-error">
					<Icon name="alert-circle" size={24} />
					<p>{error}</p>
					<Button variant="secondary" onClick={refresh}>
						Retry
					</Button>
				</div>
			</div>
		);
	}

	if (transcripts.length === 0 && streamingLines.length === 0) {
		return (
			<div className="transcript-viewer" style={{ height }}>
				<div className="transcript-viewer-empty">
					<Icon name="file-text" size={32} className="empty-icon" />
					<p className="empty-title">No transcripts yet</p>
					<p className="empty-hint">
						{isRunning
							? 'Waiting for output...'
							: 'Run the task to generate transcripts'}
					</p>
				</div>
			</div>
		);
	}

	return (
		<div className="transcript-viewer" style={{ height }}>
			{/* Header */}
			<div className="transcript-viewer-header">
				<div className="header-left">
					{showNav && (
						<Button
							variant="ghost"
							iconOnly
							size="sm"
							className="nav-toggle-btn"
							onClick={() => setNavCollapsed((prev) => !prev)}
							title={navCollapsed ? 'Show navigation' : 'Hide navigation'}
							aria-label={navCollapsed ? 'Show navigation' : 'Hide navigation'}
						>
							<Icon name={navCollapsed ? 'panel-left-open' : 'panel-left-close'} size={16} />
						</Button>
					)}
					<span className="transcript-count">
						{transcripts.length} messages
						{streamingLines.length > 0 && ` + ${streamingLines.length} streaming`}
					</span>
				</div>

				<div className="header-right">
					{showSearch && (
						<TranscriptSearch
							value={searchQuery}
							onChange={handleSearch}
							resultCount={searchResults.length}
							currentIndex={currentResultIndex}
							onNext={handleNextResult}
							onPrev={handlePrevResult}
						/>
					)}

					{isRunning && (
						<Button
							variant={isAutoScrollEnabled ? 'primary' : 'ghost'}
							size="sm"
							className={`auto-scroll-btn ${isAutoScrollEnabled ? 'active' : ''}`}
							onClick={toggleAutoScroll}
							title={isAutoScrollEnabled ? 'Auto-scroll enabled' : 'Auto-scroll disabled'}
							leftIcon={<Icon name="chevrons-down" size={14} />}
						>
							Auto-scroll
						</Button>
					)}

					<Button variant="ghost" iconOnly size="sm" className="refresh-btn" onClick={refresh} title="Refresh" aria-label="Refresh">
						<Icon name="refresh" size={14} />
					</Button>
				</div>
			</div>

			{/* Main content area */}
			<div className="transcript-viewer-body">
				{/* Navigation sidebar */}
				{showNav && !navCollapsed && (
					<div className="transcript-viewer-nav">
						<TranscriptNav
							phases={phaseStats}
							currentPhase={currentPhase ?? undefined}
							onNavigate={handleNavClick}
						/>
					</div>
				)}

				{/* Transcript content */}
				<div
					className="transcript-viewer-content"
					ref={scrollContainerRef}
					onScroll={handleScroll}
				>
					{/* Loading indicator for previous pages */}
					{loadingMore && hasPrevious && (
						<div className="loading-more-indicator top">
							<div className="loading-spinner-small" />
							<span>Loading previous...</span>
						</div>
					)}

					{/* Virtual list or regular sections */}
					{transcripts.length > 100 ? (
						<TranscriptVirtualList
							sections={sections}
							searchQuery={searchQuery}
							highlightedId={
								currentResultIndex >= 0 ? searchResults[currentResultIndex] : undefined
							}
						/>
					) : (
						<div className="transcript-sections">
							{sections.map((section) => (
								<TranscriptSection
									key={section.id}
									type={section.type}
									title={section.title}
									subtitle={section.subtitle}
									timestamp={formatTimestamp(section.timestamp)}
									badge={section.tokens ? `${formatNumber(section.tokens)} tokens` : undefined}
									defaultExpanded={sections.length <= 3}
									testId={`transcript-${section.id}`}
								>
									{section.children && section.children.length > 0 ? (
										section.children.map((child) => (
											<TranscriptSection
												key={child.id}
												type={child.type}
												title={child.title}
												subtitle={child.subtitle}
												timestamp={formatTimestamp(child.timestamp)}
												badge={child.tokens ? `${formatNumber(child.tokens)} tokens` : undefined}
												depth={1}
												testId={`transcript-${child.id}`}
											>
												{renderSectionContent(child, searchResults[currentResultIndex])}
											</TranscriptSection>
										))
									) : (
										renderSectionContent(section, searchResults[currentResultIndex])
									)}
								</TranscriptSection>
							))}
						</div>
					)}

					{/* Loading indicator for next pages */}
					{loadingMore && hasMore && (
						<div className="loading-more-indicator bottom">
							<div className="loading-spinner-small" />
							<span>Loading more...</span>
						</div>
					)}

					{/* Streaming indicator */}
					{isRunning && streamingLines.length > 0 && (
						<div className="streaming-indicator">
							<Icon name="activity" size={14} className="streaming-pulse" />
							<span>Live streaming...</span>
						</div>
					)}
				</div>
			</div>
		</div>
	);
}

// Utility functions
function formatTimestamp(timestamp: string): string {
	try {
		const date = new Date(timestamp);
		return date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' });
	} catch {
		return timestamp;
	}
}


function escapeRegExp(string: string): string {
	return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}
