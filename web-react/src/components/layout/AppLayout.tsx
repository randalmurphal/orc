import { useState, useCallback } from 'react';
import { Outlet } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { Header } from './Header';
import { UrlParamSync } from './UrlParamSync';
import { useGlobalShortcuts } from '@/hooks';
import { KeyboardShortcutsHelp } from '@/components/overlays';
import './AppLayout.css';

/**
 * Main application layout with sidebar, header, and content area.
 *
 * Structure:
 * - AppLayout (root container)
 *   - Sidebar (left navigation)
 *   - app-main (main content wrapper)
 *     - Header (top bar)
 *     - main/app-content (page content via Outlet)
 *
 * Also handles:
 * - Global keyboard shortcuts
 * - Keyboard shortcuts help modal
 */
export function AppLayout() {
	// Modal states
	const [showShortcutsHelp, setShowShortcutsHelp] = useState(false);

	// Close any open modal
	const closeModals = useCallback(() => {
		setShowShortcutsHelp(false);
	}, []);

	// Wire up global shortcuts
	useGlobalShortcuts({
		onHelp: () => setShowShortcutsHelp(true),
		onEscape: closeModals,
		// TODO: Add these when components are implemented
		// onCommandPalette: () => setShowCommandPalette(true),
		// onNewTask: () => setShowNewTaskModal(true),
		// onProjectSwitcher: () => setShowProjectSwitcher(true),
		// onSearch: () => searchInputRef.current?.focus(),
	});

	return (
		<div className="app-layout">
			{/* Sync URL params with stores */}
			<UrlParamSync />

			<Sidebar />
			<div className="app-main">
				<Header />
				<main className="app-content">
					<Outlet />
				</main>
			</div>

			{/* Modals */}
			<KeyboardShortcutsHelp open={showShortcutsHelp} onClose={() => setShowShortcutsHelp(false)} />
		</div>
	);
}
