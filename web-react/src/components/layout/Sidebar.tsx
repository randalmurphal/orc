import { NavLink } from 'react-router-dom';
import { useSidebarExpanded, useUIStore } from '@/stores';
import './Sidebar.css';

/**
 * Navigation sidebar with collapsible state.
 * Links:
 * - Tasks (/)
 * - Board (/board)
 * - Dashboard (/dashboard)
 * - Preferences (/preferences)
 * - Environment (/environment)
 */
export function Sidebar() {
	const expanded = useSidebarExpanded();
	const toggleSidebar = useUIStore((state) => state.toggleSidebar);

	return (
		<aside className={`sidebar ${expanded ? 'expanded' : 'collapsed'}`}>
			<div className="sidebar-header">
				<span className="sidebar-logo">
					{expanded ? 'Orc' : 'O'}
				</span>
				<button
					className="sidebar-toggle"
					onClick={toggleSidebar}
					aria-label={expanded ? 'Collapse sidebar' : 'Expand sidebar'}
				>
					{expanded ? '◀' : '▶'}
				</button>
			</div>

			<nav className="sidebar-nav">
				<NavLink
					to="/"
					end
					className={({ isActive }) => `sidebar-link ${isActive ? 'active' : ''}`}
				>
					<span className="sidebar-icon">📋</span>
					{expanded && <span className="sidebar-label">Tasks</span>}
				</NavLink>

				<NavLink
					to="/board"
					className={({ isActive }) => `sidebar-link ${isActive ? 'active' : ''}`}
				>
					<span className="sidebar-icon">📊</span>
					{expanded && <span className="sidebar-label">Board</span>}
				</NavLink>

				<NavLink
					to="/dashboard"
					className={({ isActive }) => `sidebar-link ${isActive ? 'active' : ''}`}
				>
					<span className="sidebar-icon">📈</span>
					{expanded && <span className="sidebar-label">Dashboard</span>}
				</NavLink>

				<div className="sidebar-divider" />

				<NavLink
					to="/preferences"
					className={({ isActive }) => `sidebar-link ${isActive ? 'active' : ''}`}
				>
					<span className="sidebar-icon">⚙️</span>
					{expanded && <span className="sidebar-label">Preferences</span>}
				</NavLink>

				<NavLink
					to="/environment"
					className={({ isActive }) => `sidebar-link ${isActive ? 'active' : ''}`}
				>
					<span className="sidebar-icon">🔧</span>
					{expanded && <span className="sidebar-label">Environment</span>}
				</NavLink>
			</nav>
		</aside>
	);
}
