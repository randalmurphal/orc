import { NavLink, Outlet } from 'react-router-dom';
import './EnvironmentLayout.css';

/**
 * Environment section layout with sub-navigation.
 *
 * Now nested under /settings/environment/* with sub-routes:
 * - /settings/environment/hooks
 * - /settings/environment/skills
 * - /settings/environment/tools
 * - /settings/environment/config
 */
export function EnvironmentLayout() {
	return (
		<div className="environment-layout">
			<nav className="environment-nav">
				<NavLink
					to="hooks"
					className={({ isActive }) => `env-nav-link ${isActive ? 'active' : ''}`}
				>
					Hooks
				</NavLink>
				<NavLink
					to="skills"
					className={({ isActive }) => `env-nav-link ${isActive ? 'active' : ''}`}
				>
					Skills
				</NavLink>
				<NavLink
					to="tools"
					className={({ isActive }) => `env-nav-link ${isActive ? 'active' : ''}`}
				>
					Tools
				</NavLink>
				<NavLink
					to="config"
					className={({ isActive }) => `env-nav-link ${isActive ? 'active' : ''}`}
				>
					Config
				</NavLink>
			</nav>
			<div className="environment-content">
				<Outlet />
			</div>
		</div>
	);
}
