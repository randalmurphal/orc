import { Outlet } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { Header } from './Header';
import { UrlParamSync } from './UrlParamSync';
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
 */
export function AppLayout() {
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
		</div>
	);
}
