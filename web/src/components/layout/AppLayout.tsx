import { useState, useCallback } from 'react';
import { Outlet } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { Header } from './Header';
import { UrlParamSync } from './UrlParamSync';
import { useGlobalShortcuts } from '@/hooks';
import { KeyboardShortcutsHelp, ProjectSwitcher } from '@/components/overlays';
import { useUIStore } from '@/stores';
import './AppLayout.css';

/**
 * Main application layout with sidebar, header, and content area.
 *
 * Structure:
 * - AppLayout (root container)
 *   - Sidebar (left navigation, fixed position)
 *   - app-main (main content wrapper with left margin)
 *     - Header (top bar, sticky)
 *     - main/app-content (page content via Outlet)
 *
 * Also handles:
 * - Global keyboard shortcuts
 * - Modals: keyboard shortcuts help, project switcher, new task
 */
export function AppLayout() {
	// Modal states
	const [showShortcutsHelp, setShowShortcutsHelp] = useState(false);
	const [showProjectSwitcher, setShowProjectSwitcher] = useState(false);
	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	const [_showNewTaskModal, setShowNewTaskModal] = useState(false);
	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	const [_showCommandPalette, setShowCommandPalette] = useState(false);

	// Sidebar state for layout margin
	const sidebarExpanded = useUIStore((state) => state.sidebarExpanded);

	// Close all modals
	const closeModals = useCallback(() => {
		setShowShortcutsHelp(false);
		setShowProjectSwitcher(false);
		setShowNewTaskModal(false);
		setShowCommandPalette(false);
	}, []);

	// Wire up global shortcuts
	// Note: toggleSidebar is handled internally by the hook
	useGlobalShortcuts({
		onHelp: () => setShowShortcutsHelp(true),
		onEscape: closeModals,
		onCommandPalette: () => setShowCommandPalette(true),
		onNewTask: () => setShowNewTaskModal(true),
		onProjectSwitcher: () => setShowProjectSwitcher(true),
	});

	return (
		<div className={`app-layout ${sidebarExpanded ? 'sidebar-expanded' : 'sidebar-collapsed'}`}>
			{/* Sync URL params with stores */}
			<UrlParamSync />

			<Sidebar />
			<div className="app-main">
				<Header
					onProjectClick={() => setShowProjectSwitcher(true)}
					onNewTask={() => setShowNewTaskModal(true)}
					onCommandPalette={() => setShowCommandPalette(true)}
				/>
				<main className="app-content">
					<Outlet />
				</main>
			</div>

			{/* Modals */}
			<KeyboardShortcutsHelp
				open={showShortcutsHelp}
				onClose={() => setShowShortcutsHelp(false)}
			/>
			<ProjectSwitcher
				open={showProjectSwitcher}
				onClose={() => setShowProjectSwitcher(false)}
			/>
			{/* TODO: NewTaskModal will be implemented in a future task */}
			{/* TODO: CommandPalette will be implemented in a future task */}
		</div>
	);
}
