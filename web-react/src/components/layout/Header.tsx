import { useLocation } from 'react-router-dom';
import { useCurrentProject, useConnectionStatus } from '@/hooks';
import './Header.css';

/**
 * Application header with page title and connection status.
 */
export function Header() {
	const location = useLocation();
	const project = useCurrentProject();
	const wsStatus = useConnectionStatus();

	// Derive page title from route
	const pageTitle = getPageTitle(location.pathname);

	return (
		<header className="app-header">
			<div className="header-left">
				<h1 className="header-title">{pageTitle}</h1>
				{project && (
					<span className="header-project">{project.name}</span>
				)}
			</div>

			<div className="header-right">
				<div className={`connection-indicator ${wsStatus}`} title={`WebSocket: ${wsStatus}`}>
					<span className="connection-dot" />
					<span className="connection-label">{wsStatus}</span>
				</div>
			</div>
		</header>
	);
}

function getPageTitle(pathname: string): string {
	if (pathname === '/') return 'Tasks';
	if (pathname === '/board') return 'Board';
	if (pathname === '/dashboard') return 'Dashboard';
	if (pathname.startsWith('/tasks/')) return 'Task Detail';
	if (pathname.startsWith('/initiatives/')) return 'Initiative';
	if (pathname === '/preferences') return 'Preferences';
	if (pathname.startsWith('/environment')) return 'Environment';
	return 'Orc';
}
