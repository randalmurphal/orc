/**
 * SettingsTabs component - top-level tabbed navigation for Settings page.
 *
 * Features:
 * - Three tabs: General, Agents, Environment
 * - URL-driven tab state (syncs with React Router)
 * - Radix Tabs for accessibility
 *
 * Tab content is rendered via nested routes (Outlet).
 *
 * URL mapping:
 * - /settings/general/* -> General tab (SettingsLayout)
 * - /settings/agents -> Agents tab (AgentsView)
 * - /settings/environment/* -> Environment tab (EnvironmentLayout)
 */

import * as Tabs from '@radix-ui/react-tabs';
import { NavLink, Outlet, useLocation } from 'react-router-dom';
import './SettingsTabs.css';

type TabId = 'general' | 'agents' | 'environment';

interface TabConfig {
	id: TabId;
	label: string;
	path: string;
}

const TABS: TabConfig[] = [
	{ id: 'general', label: 'General', path: '/settings/general' },
	{ id: 'agents', label: 'Agents', path: '/settings/agents' },
	{ id: 'environment', label: 'Environment', path: '/settings/environment' },
];

/**
 * Derive active tab from current URL path.
 * Handles sub-routes (e.g., /settings/general/commands -> 'general')
 */
function getActiveTabFromPath(pathname: string): TabId {
	if (pathname.startsWith('/settings/agents')) {
		return 'agents';
	}
	if (pathname.startsWith('/settings/environment')) {
		return 'environment';
	}
	// Default to general for /settings/general and all other /settings/* paths
	return 'general';
}

export function SettingsTabs() {
	const location = useLocation();
	const activeTab = getActiveTabFromPath(location.pathname);

	return (
		<div className="settings-tabs">
			<Tabs.Root
				value={activeTab}
				className="settings-tabs-root"
			>
				<Tabs.List className="settings-tabs-list" aria-label="Settings sections">
					{TABS.map((tab) => (
						<Tabs.Trigger
							key={tab.id}
							value={tab.id}
							className="settings-tabs-trigger"
							asChild
						>
							<NavLink to={tab.path}>{tab.label}</NavLink>
						</Tabs.Trigger>
					))}
				</Tabs.List>

				{/*
				 * Tab content rendered via React Router's Outlet.
				 * Each tab has a Tabs.Content wrapper for proper ARIA association,
				 * but only the active tab's content is shown.
				 */}
				{TABS.map((tab) => (
					<Tabs.Content
						key={tab.id}
						value={tab.id}
						className="settings-tabs-content"
						forceMount
					>
						{activeTab === tab.id && <Outlet />}
					</Tabs.Content>
				))}
			</Tabs.Root>
		</div>
	);
}
