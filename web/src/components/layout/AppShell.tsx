/**
 * AppShell component - Main application shell with CSS Grid layout.
 *
 * Provides the overall application structure:
 * - ProjectSidebar in left column
 * - TopBar (48px) in top row
 * - Main content area (scrollable) in center
 * - ContextPanel (resizable) in right column
 * - TerminalDrawer at bottom
 *
 * Grid Layout:
 * ```
 * +---------+---------------------------+--------------+
 * | Project |         TopBar            | ContextPanel |
 * | Sidebar +---------------------------+              |
 * |         |      Main Content         |              |
 * |         |        (scroll)           |              |
 * |         +---------------------------+              |
 * |         |    Terminal Drawer        |              |
 * +---------+---------------------------+--------------+
 * ```
 */

import { type ReactNode, useState, useCallback, useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import { ProjectSidebar } from './ProjectSidebar';
import { TopBar } from './TopBar';
import { ContextPanel, type ContextPanelMode } from './ContextPanel';
import { TerminalDrawer } from './TerminalDrawer';
import { UrlParamSync } from './UrlParamSync';
import { AppShellProvider, useAppShell } from './AppShellContext';
import { BoardCommandPanel } from '@/components/board';
import { useThreadStore } from '@/stores/threadStore';
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
}

// =============================================================================
// INNER COMPONENT (uses context and manages state)
// =============================================================================

function AppShellInner({
	children,
	className = '',
	onNewTask,
	onProjectChange,
}: AppShellProps) {
	const location = useLocation();
	const { isRightPanelOpen } = useAppShell();
	const [contextPanelMode, setContextPanelMode] = useState<ContextPanelMode | undefined>(undefined);
	const selectedThreadId = useThreadStore((state) => state.selectedThreadId);
	const isBoardRoute = location.pathname === '/board';

	// When a thread is selected, switch to discussion mode
	useEffect(() => {
		if (selectedThreadId) {
			setContextPanelMode('discussion');
		}
	}, [selectedThreadId]);

	const handleModeChange = useCallback((mode: ContextPanelMode) => {
		setContextPanelMode(mode);
	}, []);

	const shellClasses = [
		'app-shell',
		isBoardRoute && 'app-shell--board-route',
		className,
	].filter(Boolean).join(' ');

	return (
		<div className={shellClasses}>
			{/* URL parameter sync */}
			<UrlParamSync />

			{/* Skip Link for accessibility */}
			<a href="#main-content" className="app-shell__skip-link">
				Skip to main content
			</a>

			{/* ProjectSidebar (left column) */}
			<div className="app-shell__sidebar">
				<ProjectSidebar onProjectChange={onProjectChange} />
			</div>

			{/* TopBar (48px header) */}
			<div className="app-shell__topbar">
				<TopBar onNewTask={onNewTask} />
			</div>

			{/* Main Content Area (scrollable) */}
			<main
				id="main-content"
				className="app-shell__main"
				role="main"
			>
				{children}
			</main>

			{/* TerminalDrawer (bottom) */}
			<div className="app-shell__terminal-drawer">
				<TerminalDrawer />
			</div>

			{/* ContextPanel (right column) */}
			<div className="app-shell__context-panel">
				{isBoardRoute ? (
					isRightPanelOpen ? <BoardCommandPanel /> : null
				) : (
					<ContextPanel
						mode={contextPanelMode}
						onModeChange={handleModeChange}
						threadId={selectedThreadId ?? undefined}
					/>
				)}
			</div>
		</div>
	);
}

// =============================================================================
// MAIN COMPONENT (provides context)
// =============================================================================

export function AppShell(props: AppShellProps) {
	return (
		<AppShellProvider>
			<AppShellInner {...props} />
		</AppShellProvider>
	);
}
