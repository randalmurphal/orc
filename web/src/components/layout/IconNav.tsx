/**
 * IconNav component - 56px icon-based navigation sidebar.
 *
 * Compact vertical navigation with icons and small labels.
 * Uses React Router NavLink for active state detection.
 * Matches the design from example_ui/board.html.
 */

import { memo } from 'react';
import { NavLink, useLocation } from 'react-router-dom';
import { Icon, Tooltip } from '@/components/ui';
import type { IconName } from '@/components/ui/Icon';
import './IconNav.css';

/** Props for a single navigation item */
interface NavItemConfig {
	/** Icon name from the Icon component */
	icon: IconName;
	/** Short label displayed below the icon */
	label: string;
	/** Route path */
	path: string;
	/** Full description for tooltip and accessibility */
	description: string;
}

/** Configuration for main navigation items */
const mainNavItems: NavItemConfig[] = [
	{
		icon: 'board',
		label: 'Board',
		path: '/board',
		description: 'Task board view',
	},
	{
		icon: 'layers',
		label: 'Initiatives',
		path: '/initiatives',
		description: 'View and manage initiatives',
	},
	{
		icon: 'activity',
		label: 'Timeline',
		path: '/timeline',
		description: 'Activity timeline',
	},
	{
		icon: 'bar-chart',
		label: 'Stats',
		path: '/stats',
		description: 'Statistics and metrics',
	},
];

/** Configuration for secondary navigation items */
const secondaryNavItems: NavItemConfig[] = [
	{
		icon: 'workflow',
		label: 'Workflows',
		path: '/workflows',
		description: 'Workflow configuration',
	},
	{
		icon: 'robot',
		label: 'Agents',
		path: '/agents',
		description: 'Agent management',
	},
	{
		icon: 'terminal',
		label: 'Environ',
		path: '/environment',
		description: 'Environment: hooks, skills, and configuration',
	},
	{
		icon: 'settings',
		label: 'Settings',
		path: '/settings',
		description: 'Application settings',
	},
];

/** Configuration for bottom navigation items */
const bottomNavItems: NavItemConfig[] = [
	{
		icon: 'help',
		label: 'Help',
		path: '/help',
		description: 'Help and documentation',
	},
];

export interface IconNavProps {
	/** Optional class name for additional styling */
	className?: string;
}

/**
 * Determines if a path is active based on current location.
 * Handles nested routes by checking if pathname starts with the nav item path.
 */
function checkIsActive(path: string, pathname: string): boolean {
	// Exact match for these paths (no nested routes expected)
	if (path === '/board' || path === '/help') {
		return pathname === path || pathname.startsWith(`${path}/`);
	}
	// For settings, initiatives, agents, stats, workflows - match prefix for nested routes
	return pathname.startsWith(path);
}

/**
 * Renders a single navigation item with icon, label, and tooltip.
 * Memoized to prevent unnecessary re-renders when other nav items change state.
 */
const NavItem = memo(function NavItem({ item, isActive }: { item: NavItemConfig; isActive: boolean }) {
	const itemClasses = ['icon-nav__item', isActive ? 'icon-nav__item--active' : '']
		.filter(Boolean)
		.join(' ');

	return (
		<Tooltip content={item.description} side="right" delayDuration={200}>
			<NavLink
				to={item.path}
				className={itemClasses}
				aria-label={item.description}
			>
				<Icon name={item.icon} size={18} className="icon-nav__icon" />
				<span className="icon-nav__label">{item.label}</span>
				{isActive && <span className="icon-nav__active-indicator" />}
			</NavLink>
		</Tooltip>
	);
});

/**
 * IconNav - 56px icon-based navigation sidebar.
 *
 * Structure:
 * - Logo section with gradient "O" mark
 * - Main nav: Board, Initiatives, Stats
 * - Divider
 * - Secondary nav: Agents, Settings
 * - Bottom section: Help
 *
 * @example
 * <IconNav />
 *
 * @example
 * // With custom class
 * <IconNav className="custom-nav" />
 */
export function IconNav({ className = '' }: IconNavProps) {
	const location = useLocation();
	const navClasses = ['icon-nav', className].filter(Boolean).join(' ');

	// Check if a path is currently active
	const isActive = (path: string) => checkIsActive(path, location.pathname);

	return (
		<nav
			className={navClasses}
			role="navigation"
			aria-label="Main navigation"
		>
			{/* Logo Section */}
			<div className="icon-nav__logo">
				<div className="icon-nav__logo-mark">O</div>
			</div>

			{/* Main Navigation Items */}
			<div className="icon-nav__items">
				{mainNavItems.map((item) => (
					<NavItem key={item.path} item={item} isActive={isActive(item.path)} />
				))}

				{/* Divider */}
				<div className="icon-nav__divider" />

				{/* Secondary Navigation Items */}
				{secondaryNavItems.map((item) => (
					<NavItem key={item.path} item={item} isActive={isActive(item.path)} />
				))}
			</div>

			{/* Bottom Navigation Items */}
			<div className="icon-nav__bottom">
				{bottomNavItems.map((item) => (
					<NavItem key={item.path} item={item} isActive={isActive(item.path)} />
				))}
			</div>
		</nav>
	);
}
