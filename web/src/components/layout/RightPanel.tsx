import {
	useState,
	useCallback,
	useMemo,
	useRef,
	useEffect,
	createContext,
	useContext,
	type ReactNode,
	type TouchEvent as ReactTouchEvent,
} from 'react';
import { Icon } from '@/components/ui';
import type { IconName } from '@/components/ui/Icon';
import './RightPanel.css';

// =============================================================================
// CONTEXT
// =============================================================================

interface RightPanelContextValue {
	isOpen: boolean;
}

const RightPanelContext = createContext<RightPanelContextValue | null>(null);

// =============================================================================
// TYPES
// =============================================================================

interface RightPanelProps {
	/** Whether the panel is open */
	isOpen: boolean;
	/** Callback when the panel should close */
	onClose: () => void;
	/** Panel content */
	children: ReactNode;
	/** Optional class name */
	className?: string;
}

interface SectionProps {
	/** Section content */
	children: ReactNode;
	/** Whether the section starts collapsed */
	defaultCollapsed?: boolean;
	/** Unique ID for preserving state */
	id?: string;
	/** Optional class name */
	className?: string;
}

interface HeaderProps {
	/** Section title text */
	title: string;
	/** Icon name */
	icon?: IconName;
	/** Icon color variant */
	iconColor?: 'purple' | 'orange' | 'amber' | 'green' | 'blue' | 'cyan' | 'red';
	/** Count to show in badge */
	count?: number;
	/** Badge color variant */
	badgeColor?: 'purple' | 'orange' | 'amber' | 'green' | 'blue' | 'cyan' | 'red';
	/** Whether section is collapsed (passed from Section) */
	collapsed?: boolean;
	/** Toggle callback (passed from Section) */
	onToggle?: () => void;
	/** Optional class name */
	className?: string;
}

interface BodyProps {
	/** Body content */
	children: ReactNode;
	/** Optional class name */
	className?: string;
}

// =============================================================================
// SECTION CONTEXT
// =============================================================================

interface SectionContextValue {
	collapsed: boolean;
	toggle: () => void;
}

const SectionContext = createContext<SectionContextValue | null>(null);

function useSectionContext() {
	return useContext(SectionContext);
}

// =============================================================================
// STORAGE
// =============================================================================

const STORAGE_KEY = 'orc-right-panel-sections';

function loadCollapsedSections(): Set<string> {
	if (typeof window === 'undefined') return new Set();
	try {
		const stored = localStorage.getItem(STORAGE_KEY);
		return stored ? new Set(JSON.parse(stored)) : new Set();
	} catch {
		return new Set();
	}
}

function saveCollapsedSections(sections: Set<string>) {
	if (typeof window === 'undefined') return;
	try {
		localStorage.setItem(STORAGE_KEY, JSON.stringify([...sections]));
	} catch {
		// Ignore localStorage errors
	}
}

// =============================================================================
// MAIN COMPONENT
// =============================================================================

const SWIPE_THRESHOLD = 50;

/**
 * Collapsible right panel for showing contextual information.
 *
 * Features:
 * - 300px fixed width with slide in/out animation
 * - Scrollable content area with custom scrollbar
 * - Touch gesture support for swipe-to-close on mobile
 * - Content not rendered when closed (performance)
 * - Scroll position preserved when closing/opening
 *
 * Usage:
 * ```tsx
 * <RightPanel isOpen={isOpen} onClose={() => setIsOpen(false)}>
 *   <RightPanel.Section>
 *     <RightPanel.Header title="Blocked" icon="alert-circle" count={2} />
 *     <RightPanel.Body>
 *       {blockedContent}
 *     </RightPanel.Body>
 *   </RightPanel.Section>
 * </RightPanel>
 * ```
 */
function RightPanelRoot({ isOpen, onClose, children, className = '' }: RightPanelProps) {
	const panelRef = useRef<HTMLElement>(null);
	const scrollRef = useRef<HTMLDivElement>(null);
	const scrollPositionRef = useRef(0);
	const touchStartRef = useRef<{ x: number; y: number } | null>(null);
	const [isAnimating, setIsAnimating] = useState(false);

	// Track animation state
	useEffect(() => {
		if (isOpen) {
			setIsAnimating(true);
			const timer = setTimeout(() => setIsAnimating(false), 200);
			return () => clearTimeout(timer);
		}
	}, [isOpen]);

	// Preserve scroll position
	useEffect(() => {
		if (!isOpen && scrollRef.current) {
			scrollPositionRef.current = scrollRef.current.scrollTop;
		}
	}, [isOpen]);

	useEffect(() => {
		if (isOpen && scrollRef.current) {
			scrollRef.current.scrollTop = scrollPositionRef.current;
		}
	}, [isOpen]);

	// Touch gesture handling for swipe-to-close
	const handleTouchStart = useCallback((e: ReactTouchEvent) => {
		const touch = e.touches[0];
		touchStartRef.current = { x: touch.clientX, y: touch.clientY };
	}, []);

	const handleTouchEnd = useCallback(
		(e: ReactTouchEvent) => {
			if (!touchStartRef.current) return;

			const touch = e.changedTouches[0];
			const deltaX = touch.clientX - touchStartRef.current.x;
			const deltaY = Math.abs(touch.clientY - touchStartRef.current.y);

			// Only close if swiping right and mostly horizontal
			if (deltaX > SWIPE_THRESHOLD && deltaX > deltaY * 2) {
				onClose();
			}

			touchStartRef.current = null;
		},
		[onClose]
	);

	// Don't render content when closed (performance optimization)
	const shouldRenderContent = isOpen || isAnimating;

	const contextValue = useMemo(() => ({ isOpen }), [isOpen]);

	return (
		<RightPanelContext.Provider value={contextValue}>
			<aside
				ref={panelRef}
				className={`right-panel ${isOpen ? 'open' : ''} ${className}`}
				role="complementary"
				aria-label="Context panel"
				aria-hidden={!isOpen}
				onTouchStart={handleTouchStart}
				onTouchEnd={handleTouchEnd}
			>
				{shouldRenderContent && (
					<div ref={scrollRef} className="right-panel-scroll">
						{children}
					</div>
				)}
			</aside>
		</RightPanelContext.Provider>
	);
}

// =============================================================================
// SECTION COMPONENT
// =============================================================================

function Section({ children, defaultCollapsed = false, id, className = '' }: SectionProps) {
	const [collapsedSections, setCollapsedSections] = useState<Set<string>>(loadCollapsedSections);

	// Determine if this section is collapsed
	const sectionId = id || 'default';
	const isCollapsed = id ? collapsedSections.has(sectionId) : defaultCollapsed;
	const [localCollapsed, setLocalCollapsed] = useState(defaultCollapsed);

	// Use persisted state if ID provided, otherwise use local state
	const collapsed = id ? isCollapsed : localCollapsed;

	const toggle = useCallback(() => {
		if (id) {
			setCollapsedSections((prev: Set<string>) => {
				const next = new Set<string>(prev);
				if (next.has(sectionId)) {
					next.delete(sectionId);
				} else {
					next.add(sectionId);
				}
				saveCollapsedSections(next);
				return next;
			});
		} else {
			setLocalCollapsed((prev: boolean) => !prev);
		}
	}, [id, sectionId]);

	const sectionValue = useMemo(() => ({ collapsed, toggle }), [collapsed, toggle]);

	return (
		<SectionContext.Provider value={sectionValue}>
			<div className={`right-panel-section ${collapsed ? 'collapsed' : ''} ${className}`}>
				{children}
			</div>
		</SectionContext.Provider>
	);
}

// =============================================================================
// HEADER COMPONENT
// =============================================================================

function Header({
	title,
	icon,
	iconColor = 'purple',
	count,
	badgeColor,
	collapsed: collapsedProp,
	onToggle: onToggleProp,
	className = '',
}: HeaderProps) {
	const sectionContext = useSectionContext();

	// Use props if provided directly, otherwise use section context
	const collapsed = collapsedProp !== undefined ? collapsedProp : sectionContext?.collapsed ?? false;
	const onToggle = onToggleProp ?? sectionContext?.toggle;

	const effectiveBadgeColor = badgeColor || iconColor;

	return (
		<button
			type="button"
			className={`right-panel-header ${className}`}
			onClick={onToggle}
			aria-expanded={!collapsed}
		>
			<div className="right-panel-title">
				{icon && (
					<span className={`right-panel-title-icon ${iconColor}`}>
						<Icon name={icon} size={10} />
					</span>
				)}
				<span className="right-panel-title-text">{title}</span>
				{count !== undefined && count > 0 && (
					<span className={`right-panel-badge ${effectiveBadgeColor}`}>{count}</span>
				)}
			</div>
			<span className="right-panel-chevron">
				<Icon name="chevron-down" size={14} />
			</span>
		</button>
	);
}

// =============================================================================
// BODY COMPONENT
// =============================================================================

function Body({ children, className = '' }: BodyProps) {
	const sectionContext = useSectionContext();
	const collapsed = sectionContext?.collapsed ?? false;

	if (collapsed) {
		return null;
	}

	return <div className={`right-panel-body ${className}`}>{children}</div>;
}

// =============================================================================
// COMPOUND COMPONENT EXPORT
// =============================================================================

export const RightPanel = Object.assign(RightPanelRoot, {
	Section,
	Header,
	Body,
});

export type { RightPanelProps, SectionProps, HeaderProps, BodyProps };
