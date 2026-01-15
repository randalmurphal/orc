import { NavLink, Outlet } from 'react-router-dom';
import './EnvironmentLayout.css';

/**
 * Environment section layout with sub-navigation.
 *
 * Sub-routes:
 * - /environment/settings
 * - /environment/prompts
 * - /environment/scripts
 * - /environment/hooks
 * - /environment/skills
 * - /environment/mcp
 * - /environment/config
 * - /environment/claudemd
 * - /environment/tools
 * - /environment/agents
 */
export function EnvironmentLayout() {
	return (
		<div className="environment-layout">
			<nav className="environment-nav">
				<NavLink
					to="settings"
					className={({ isActive }) => `env-nav-link ${isActive ? 'active' : ''}`}
				>
					Settings
				</NavLink>
				<NavLink
					to="prompts"
					className={({ isActive }) => `env-nav-link ${isActive ? 'active' : ''}`}
				>
					Prompts
				</NavLink>
				<NavLink
					to="scripts"
					className={({ isActive }) => `env-nav-link ${isActive ? 'active' : ''}`}
				>
					Scripts
				</NavLink>
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
					to="mcp"
					className={({ isActive }) => `env-nav-link ${isActive ? 'active' : ''}`}
				>
					MCP
				</NavLink>
				<NavLink
					to="config"
					className={({ isActive }) => `env-nav-link ${isActive ? 'active' : ''}`}
				>
					Config
				</NavLink>
				<NavLink
					to="claudemd"
					className={({ isActive }) => `env-nav-link ${isActive ? 'active' : ''}`}
				>
					CLAUDE.md
				</NavLink>
				<NavLink
					to="tools"
					className={({ isActive }) => `env-nav-link ${isActive ? 'active' : ''}`}
				>
					Tools
				</NavLink>
				<NavLink
					to="agents"
					className={({ isActive }) => `env-nav-link ${isActive ? 'active' : ''}`}
				>
					Agents
				</NavLink>
			</nav>
			<div className="environment-content">
				<Outlet />
			</div>
		</div>
	);
}
