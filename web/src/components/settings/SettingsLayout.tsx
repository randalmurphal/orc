/**
 * SettingsLayout component - main settings page layout with 240px sidebar and content area.
 *
 * Features:
 * - CSS Grid layout: 240px sidebar + 1fr content
 * - Grouped navigation sections: CLAUDE CODE, ORC, ACCOUNT
 * - Badge support for count indicators (fetched from API)
 * - Active nav item highlighting with primary color
 * - Independent scrolling for sidebar and content
 */

import { useState, useEffect } from 'react';
import { NavLink, Outlet } from 'react-router-dom';
import { Icon, type IconName } from '../ui/Icon';
import { configClient } from '@/lib/client';
import './SettingsLayout.css';

interface NavItemProps {
	to: string;
	icon: IconName;
	label: string;
	badge?: number;
}

function NavItem({ to, icon, label, badge }: NavItemProps) {
	return (
		<NavLink
			to={to}
			className={({ isActive }) =>
				`settings-nav-item ${isActive ? 'settings-nav-item--active' : ''}`
			}
		>
			<Icon name={icon} size={16} />
			<span className="settings-nav-item__label">{label}</span>
			{badge !== undefined && badge > 0 && (
				<span className="settings-nav-item__badge">{badge}</span>
			)}
		</NavLink>
	);
}

interface NavGroupProps {
	title: string;
	children: React.ReactNode;
}

function NavGroup({ title, children }: NavGroupProps) {
	return (
		<div className="settings-nav-group">
			<div className="settings-nav-group__title">{title}</div>
			<div className="settings-nav-group__items">{children}</div>
		</div>
	);
}

interface SettingsCounts {
	commandsCount: number;
	mcpServersCount: number;
}

export function SettingsLayout() {
	const [counts, setCounts] = useState<SettingsCounts>({
		commandsCount: 0,
		mcpServersCount: 0,
	});

	useEffect(() => {
		const fetchCounts = async () => {
			try {
				const configStats = await configClient.getConfigStats({});
				setCounts({
					commandsCount: configStats.stats?.slashCommandsCount ?? 0,
					mcpServersCount: configStats.stats?.mcpServersCount ?? 0,
				});
			} catch (err) {
				console.error('Failed to fetch settings counts:', err);
				// Keep counts at 0 on error - badges won't show
			}
		};

		fetchCounts();
	}, []);

	return (
		<div className="settings-layout">
			{/* Sidebar */}
			<aside className="settings-sidebar" role="navigation" aria-label="Settings navigation">
				<div className="settings-sidebar__header">
					<h1 className="settings-sidebar__title">Settings</h1>
					<p className="settings-sidebar__subtitle">Configure ORC and Claude</p>
				</div>

				<nav className="settings-nav">
					<NavGroup title="CLAUDE CODE">
						<NavItem
							to="/settings/commands"
							icon="terminal"
							label="Slash Commands"
							badge={counts.commandsCount}
						/>
						<NavItem to="/settings/claude-md" icon="file-text" label="CLAUDE.md" />
						<NavItem
							to="/settings/mcp"
							icon="mcp"
							label="MCP Servers"
							badge={counts.mcpServersCount}
						/>
						<NavItem to="/settings/permissions" icon="shield" label="Permissions" />
					</NavGroup>

					<NavGroup title="ORC">
						<NavItem to="/settings/projects" icon="folder" label="Projects" />
						<NavItem to="/settings/git" icon="git-branch" label="Git Settings" />
						<NavItem to="/settings/billing" icon="dollar" label="Billing & Usage" />
						<NavItem to="/settings/import-export" icon="export" label="Import / Export" />
						<NavItem to="/settings/constitution" icon="shield" label="Constitution" />
					</NavGroup>

					<NavGroup title="ACCOUNT">
						<NavItem to="/settings/profile" icon="user" label="Profile" />
						<NavItem to="/settings/api-keys" icon="settings" label="API Keys" />
					</NavGroup>
				</nav>
			</aside>

			{/* Content */}
			<div className="settings-content">
				<Outlet />
			</div>
		</div>
	);
}
