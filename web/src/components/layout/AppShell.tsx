/**
 * AppShell component - Main application shell with CSS Grid layout.
 *
 * Provides the overall application structure:
 * - IconNav (56px) in left column, spanning full height
 * - TopBar (48px) in top row
 * - Main content area (scrollable) in center
 * - RightPanel (300px, collapsible) in right column
 *
 * Grid Layout:
 * ```
 * +-------+---------------------------+------------+
 * | Icon  |         TopBar            | RightPanel |
 * | Nav   +---------------------------+   (opt)    |
 * | (56px)|      Main Content         |  (300px)   |
 * |       |        (scroll)           |            |
 * +-------+---------------------------+------------+
 * ```
 *
 * The right panel renders route-specific content:
 * - /board: BoardCommandPanel (reads from stores directly)
 * - Other routes: defaultPanelContent prop (if provided)
 */

import { type ReactNode, useState, useCallback } from 'react';
import { useLocation } from 'react-router-dom';
import { IconNav } from './IconNav';
import { TopBar } from './TopBar';
import { RightPanel } from './RightPanel';
import { UrlParamSync } from './UrlParamSync';
import { AppShellProvider, useAppShell } from './AppShellContext';
import { BoardCommandPanel } from '@/components/board/BoardCommandPanel';
import './AppShell.css';

// =============================================================================
// TYPES
// =============================================================================

export interface AppShellProps {
	/** Main content to render in the content area */
	children: ReactNode;
	/** Optional class name for the shell container */
	className?: string;
	/** Callback when New Task button is clicked */
	onNewTask?: () => void;
	/** Callback when project selector is clicked */
	onProjectChange?: () => void;
	/** Default right panel content (when no route-specific content applies) */
	defaultPanelContent?: ReactNode;
}

// =============================================================================
// INNER COMPONENT (uses context)
// =============================================================================

interface AppShellInnerProps extends AppShellProps {
	/** Whether mobile nav is open (hamburger menu) */
	mobileNavOpen: boolean;
	/** Toggle mobile nav */
	onToggleMobileNav: () => void;
}

function AppShellInner({
	children,
	className = '',
	onNewTask,
	onProjectChange,
	defaultPanelContent,
	mobileNavOpen,
	onToggleMobileNav,
}: AppShellInnerProps) {
	const {
		isRightPanelOpen,
		toggleRightPanel,
		isMobileNavMode,
	} = useAppShell();

	const location = useLocation();

	const shellClasses = ['app-shell', isRightPanelOpen && 'app-shell--panel-open', mobileNavOpen && 'app-shell--mobile-nav-open', className].filter(Boolean).join(' ');

	// Route-aware panel content: board gets its own panel, others use default
	const isBoard = location.pathname === '/board';
	const panelContent = isBoard ? <BoardCommandPanel /> : defaultPanelContent;

	// Handle closing panel
	const handlePanelClose = useCallback(() => {
		if (isRightPanelOpen) {
			toggleRightPanel();
		}
	}, [isRightPanelOpen, toggleRightPanel]);

	// Handle backdrop click (closes mobile nav or panel)
	const handleBackdropClick = useCallback(() => {
		if (mobileNavOpen) {
			onToggleMobileNav();
		} else if (isRightPanelOpen && isMobileNavMode) {
			toggleRightPanel();
		}
	}, [mobileNavOpen, onToggleMobileNav, isRightPanelOpen, isMobileNavMode, toggleRightPanel]);

	return (
		<div className={shellClasses}>
			{/* URL parameter sync */}
			<UrlParamSync />

			{/* Skip Link for accessibility */}
			<a href="#main-content" className="app-shell__skip-link">
				Skip to main content
			</a>

			{/* IconNav (56px sidebar) */}
			<div className="app-shell__nav">
				<IconNav />
			</div>

			{/* TopBar (48px header) */}
			<div className="app-shell__topbar">
				<TopBar
					onNewTask={onNewTask}
					onProjectChange={onProjectChange}
				/>
			</div>

			{/* Main Content Area (scrollable) */}
			<main
				id="main-content"
				className="app-shell__main"
				role="main"
			>
				{children}
			</main>

			{/* RightPanel (300px, collapsible) */}
			<div className="app-shell__panel">
				<RightPanel
					isOpen={isRightPanelOpen}
					onClose={handlePanelClose}
				>
					{panelContent}
				</RightPanel>
			</div>

			{/* Mobile backdrop */}
			<div
				className="app-shell__backdrop"
				onClick={handleBackdropClick}
				aria-hidden="true"
			/>
		</div>
	);
}

// =============================================================================
// MAIN COMPONENT (provides context)
// =============================================================================

/**
 * AppShell - Main application layout shell.
 *
 * Wraps content in AppShellProvider for state management.
 * Right panel content is determined by route:
 * - /board renders BoardCommandPanel (reads from stores)
 * - Other routes use defaultPanelContent prop
 *
 * @example
 * ```tsx
 * <AppShell
 *   onNewTask={() => setShowNewTaskModal(true)}
 *   onProjectChange={() => setShowProjectSwitcher(true)}
 * >
 *   <Outlet />
 * </AppShell>
 * ```
 */
export function AppShell(props: AppShellProps) {
	// Mobile nav state (not part of context since it's shell-specific)
	const [mobileNavOpen, setMobileNavOpen] = useState(false);

	const toggleMobileNav = useCallback(() => {
		setMobileNavOpen((prev) => !prev);
	}, []);

	return (
		<AppShellProvider>
			<AppShellInner
				{...props}
				mobileNavOpen={mobileNavOpen}
				onToggleMobileNav={toggleMobileNav}
			/>
		</AppShellProvider>
	);
}
