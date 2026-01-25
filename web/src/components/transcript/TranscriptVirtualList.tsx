/**
 * TranscriptVirtualList - Virtual scrolling list for large transcripts.
 *
 * Uses windowing technique to only render visible items, enabling smooth
 * scrolling for transcripts with thousands of entries.
 */

import { useState, useCallback, useRef, useEffect, type ReactNode } from 'react';
import { TranscriptSection } from './TranscriptSection';
import { formatNumber } from '@/lib/format';
import type { SectionData } from './TranscriptViewer';
import './TranscriptVirtualList.css';

export interface TranscriptVirtualListProps {
	/** Sections to render */
	sections: SectionData[];
	/** Current search query for highlighting */
	searchQuery?: string;
	/** ID of the currently highlighted item */
	highlightedId?: number;
	/** Height of each item in pixels (estimate for dynamic sizing) */
	estimatedItemHeight?: number;
	/** Overscan count (number of items to render outside visible area) */
	overscan?: number;
}

interface VirtualItem {
	index: number;
	section: SectionData;
	offset: number;
	height: number;
}

const DEFAULT_ITEM_HEIGHT = 60; // Collapsed section height
const EXPANDED_ITEM_HEIGHT = 200; // Estimate for expanded sections
const DEFAULT_OVERSCAN = 5;

export function TranscriptVirtualList({
	sections,
	searchQuery = '',
	highlightedId,
	estimatedItemHeight = DEFAULT_ITEM_HEIGHT,
	overscan = DEFAULT_OVERSCAN,
}: TranscriptVirtualListProps) {
	const containerRef = useRef<HTMLDivElement>(null);
	const [scrollTop, setScrollTop] = useState(0);
	const [containerHeight, setContainerHeight] = useState(0);
	const [expandedItems, setExpandedItems] = useState<Set<number>>(new Set());

	// Track measured heights for accurate positioning
	const measuredHeightsRef = useRef<Map<number, number>>(new Map());

	// Update container height on resize
	useEffect(() => {
		const container = containerRef.current;
		if (!container) return;

		const resizeObserver = new ResizeObserver((entries) => {
			for (const entry of entries) {
				setContainerHeight(entry.contentRect.height);
			}
		});

		resizeObserver.observe(container);
		setContainerHeight(container.clientHeight);

		return () => resizeObserver.disconnect();
	}, []);

	// Handle scroll events
	const handleScroll = useCallback(() => {
		const container = containerRef.current;
		if (container) {
			setScrollTop(container.scrollTop);
		}
	}, []);

	// Calculate item heights (use measured or estimate)
	const getItemHeight = useCallback(
		(index: number, section: SectionData): number => {
			const measured = measuredHeightsRef.current.get(index);
			if (measured) return measured;

			// Estimate based on expanded state
			if (expandedItems.has(section.id)) {
				// Estimate based on content length
				const contentLength = section.content?.length || 0;
				const childCount = section.children?.length || 0;
				const baseHeight = EXPANDED_ITEM_HEIGHT;
				const contentHeight = Math.min(contentLength / 50, 400); // Rough estimate
				const childHeight = childCount * estimatedItemHeight;
				return baseHeight + contentHeight + childHeight;
			}

			return estimatedItemHeight;
		},
		[expandedItems, estimatedItemHeight]
	);

	// Calculate total height and visible range
	const calculateLayout = useCallback(() => {
		let totalHeight = 0;
		const items: VirtualItem[] = [];

		for (let i = 0; i < sections.length; i++) {
			const section = sections[i];
			const height = getItemHeight(i, section);
			items.push({
				index: i,
				section,
				offset: totalHeight,
				height,
			});
			totalHeight += height;
		}

		// Find visible range
		const startOffset = scrollTop - overscan * estimatedItemHeight;
		const endOffset = scrollTop + containerHeight + overscan * estimatedItemHeight;

		const visibleItems = items.filter(
			(item) => item.offset + item.height >= startOffset && item.offset <= endOffset
		);

		return { totalHeight, visibleItems };
	}, [sections, scrollTop, containerHeight, overscan, estimatedItemHeight, getItemHeight]);

	const { totalHeight, visibleItems } = calculateLayout();

	// Toggle expanded state
	const handleToggleExpand = useCallback((id: number, expanded: boolean) => {
		setExpandedItems((prev) => {
			const next = new Set(prev);
			if (expanded) {
				next.add(id);
			} else {
				next.delete(id);
			}
			return next;
		});
	}, []);

	// Measure item after render
	const measureItem = useCallback((index: number, element: HTMLElement | null) => {
		if (element) {
			const height = element.getBoundingClientRect().height;
			measuredHeightsRef.current.set(index, height);
		}
	}, []);

	// Render section content with search highlighting
	const renderContent = (section: SectionData): ReactNode => {
		if (!section.content) return null;

		const content = highlightSearchTerms(section.content, searchQuery);

		return (
			<pre
				className={section.id === highlightedId ? 'highlighted' : ''}
				id={`transcript-${section.id}`}
			>
				{content}
			</pre>
		);
	};

	return (
		<div
			ref={containerRef}
			className="transcript-virtual-list"
			onScroll={handleScroll}
		>
			<div className="virtual-list-content" style={{ height: totalHeight }}>
				{visibleItems.map((item) => (
					<div
						key={item.section.id}
						className="virtual-list-item"
						style={{
							position: 'absolute',
							top: item.offset,
							left: 0,
							right: 0,
						}}
						ref={(el) => measureItem(item.index, el)}
					>
						<TranscriptSection
							type={item.section.type}
							title={item.section.title}
							subtitle={item.section.subtitle}
							timestamp={formatTimestamp(item.section.timestamp)}
							badge={
								item.section.tokens
									? `${formatNumber(item.section.tokens)} tokens`
									: undefined
							}
							defaultExpanded={false}
							expanded={expandedItems.has(item.section.id)}
							onExpandedChange={(expanded) =>
								handleToggleExpand(item.section.id, expanded)
							}
							testId={`transcript-${item.section.id}`}
						>
							{item.section.children && item.section.children.length > 0 ? (
								item.section.children.map((child) => (
									<TranscriptSection
										key={child.id}
										type={child.type}
										title={child.title}
										subtitle={child.subtitle}
										timestamp={formatTimestamp(child.timestamp)}
										badge={
											child.tokens
												? `${formatNumber(child.tokens)} tokens`
												: undefined
										}
										depth={1}
										testId={`transcript-${child.id}`}
									>
										{renderContent(child)}
									</TranscriptSection>
								))
							) : (
								renderContent(item.section)
							)}
						</TranscriptSection>
					</div>
				))}
			</div>
		</div>
	);
}

// Utility functions
function formatTimestamp(timestamp: string): string {
	try {
		const date = new Date(timestamp);
		return date.toLocaleTimeString(undefined, {
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit',
		});
	} catch {
		return timestamp;
	}
}


function highlightSearchTerms(text: string, query: string): ReactNode {
	if (!query) return text;

	const escapedQuery = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
	const parts = text.split(new RegExp(`(${escapedQuery})`, 'gi'));

	return parts.map((part, i) =>
		part.toLowerCase() === query.toLowerCase() ? (
			<mark key={i} className="search-highlight">
				{part}
			</mark>
		) : (
			part
		)
	);
}
