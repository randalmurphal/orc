/**
 * Breadcrumbs component - navigation breadcrumb trail based on current route.
 * Only renders for environment and preferences routes.
 */

import { useMemo } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { Icon } from './Icon';
import './Breadcrumbs.css';

interface BreadcrumbItem {
	label: string;
	href?: string;
}

// Route config maps paths to labels
const routeLabels: Record<string, string> = {
	'': 'Home',
	dashboard: 'Dashboard',
	board: 'Board',
	environment: 'Environment',
	claude: 'Claude Code',
	orchestrator: 'Orchestrator',
	skills: 'Skills',
	hooks: 'Hooks',
	agents: 'Agents',
	tools: 'Tools',
	mcp: 'MCP Servers',
	prompts: 'Prompts',
	scripts: 'Scripts',
	automation: 'Automation',
	export: 'Export',
	docs: 'Documentation',
	preferences: 'Preferences',
	knowledge: 'Knowledge Queue',
	settings: 'Settings',
	config: 'Config',
	claudemd: 'CLAUDE.md',
};

// Category segments that don't have their own page - link to parent instead
const categorySegments = new Set(['claude', 'orchestrator']);

export function Breadcrumbs() {
	const location = useLocation();

	const items = useMemo(() => {
		const pathname = location.pathname;
		const segments = pathname.split('/').filter(Boolean);

		// Only show breadcrumbs for environment and preferences pages
		if (segments[0] !== 'environment' && segments[0] !== 'preferences') {
			return [];
		}

		const crumbs: BreadcrumbItem[] = [];
		let currentPath = '';

		for (let i = 0; i < segments.length; i++) {
			const segment = segments[i];
			currentPath += '/' + segment;

			const label = routeLabels[segment] || segment;
			const isLast = i === segments.length - 1;

			// For category segments (claude, orchestrator), link to /environment instead
			let href: string | undefined;
			if (isLast) {
				href = undefined;
			} else if (categorySegments.has(segment)) {
				href = '/environment';
			} else {
				href = currentPath;
			}

			crumbs.push({
				label,
				href,
			});
		}

		return crumbs;
	}, [location.pathname]);

	if (items.length === 0) {
		return null;
	}

	return (
		<nav className="breadcrumbs" aria-label="Breadcrumb">
			<ol>
				{items.map((item, i) => (
					<li key={i}>
						{item.href ? (
							<Link to={item.href}>{item.label}</Link>
						) : (
							<span className="current">{item.label}</span>
						)}
						{i < items.length - 1 && <Icon name="chevron-right" size={14} />}
					</li>
				))}
			</ol>
		</nav>
	);
}
