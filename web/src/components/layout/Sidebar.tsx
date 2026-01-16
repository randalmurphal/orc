import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { NavLink, useLocation } from 'react-router-dom';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui';
import {
	useSidebarExpanded,
	useMobileMenuOpen,
	useUIStore,
	useInitiatives,
	useCurrentInitiativeId,
	useInitiativeStore,
	useTaskStore,
} from '@/stores';
import { formatShortcut } from '@/lib/platform';
import type { IconName } from '@/components/ui/Icon';
import './Sidebar.css';

// Navigation structure types
interface NavItem {
	label: string;
	href: string;
	icon: IconName;
}

interface NavGroup {
	label: string;
	icon: IconName;
	items: NavItem[];
	basePath: string;
}

// Navigation structure - matches Svelte exactly
const workItems: NavItem[] = [
	{ label: 'Dashboard', href: '/dashboard', icon: 'dashboard' },
	{ label: 'Tasks', href: '/', icon: 'tasks' },
	{ label: 'Board', href: '/board', icon: 'board' },
	{ label: 'Branches', href: '/branches', icon: 'git-branch' },
	{ label: 'Automation', href: '/automation', icon: 'zap' },
];

const environmentOverview: NavItem = {
	label: 'Overview',
	href: '/environment',
	icon: 'layers',
};

const envGroups: NavGroup[] = [
	{
		label: 'Claude Code',
		icon: 'terminal',
		basePath: '/environment/claude',
		items: [
			{ label: 'Skills', href: '/environment/claude/skills', icon: 'skills' },
			{ label: 'Hooks', href: '/environment/claude/hooks', icon: 'hooks' },
			{ label: 'Agents', href: '/environment/claude/agents', icon: 'agents' },
			{ label: 'Tools', href: '/environment/claude/tools', icon: 'tools' },
			{ label: 'MCP Servers', href: '/environment/claude/mcp', icon: 'mcp' },
		],
	},
	{
		label: 'Orchestrator',
		icon: 'layers',
		basePath: '/environment/orchestrator',
		items: [
			{ label: 'Prompts', href: '/environment/orchestrator/prompts', icon: 'prompts' },
			{ label: 'Scripts', href: '/environment/orchestrator/scripts', icon: 'scripts' },
			{ label: 'Automation', href: '/environment/orchestrator/automation', icon: 'config' },
			{ label: 'Export', href: '/environment/orchestrator/export', icon: 'export' },
			{ label: 'Knowledge', href: '/environment/knowledge', icon: 'database' },
		],
	},
];

const docsItem: NavItem = {
	label: 'Documentation',
	href: '/environment/docs',
	icon: 'file-text',
};

const preferencesItem: NavItem = {
	label: 'Preferences',
	href: '/preferences',
	icon: 'user',
};

// Storage keys
const STORAGE_KEY_SECTIONS = 'orc-sidebar-sections';
const STORAGE_KEY_GROUPS = 'orc-sidebar-groups';

// localStorage helpers for section/group expansion state
function loadExpandedState(): { sections: Set<string>; groups: Set<string> } {
	const defaultSections = new Set(['initiatives']);
	if (typeof window === 'undefined') return { sections: defaultSections, groups: new Set() };
	try {
		const sectionsJson = localStorage.getItem(STORAGE_KEY_SECTIONS);
		const groupsJson = localStorage.getItem(STORAGE_KEY_GROUPS);
		return {
			sections: sectionsJson ? new Set(JSON.parse(sectionsJson)) : defaultSections,
			groups: groupsJson ? new Set(JSON.parse(groupsJson)) : new Set(),
		};
	} catch {
		return { sections: defaultSections, groups: new Set() };
	}
}

function saveExpandedState(sections: Set<string>, groups: Set<string>) {
	if (typeof window === 'undefined') return;
	try {
		localStorage.setItem(STORAGE_KEY_SECTIONS, JSON.stringify([...sections]));
		localStorage.setItem(STORAGE_KEY_GROUPS, JSON.stringify([...groups]));
	} catch {
		// Ignore localStorage errors
	}
}

interface SidebarProps {
	onNewInitiative?: () => void;
}

/**
 * Navigation sidebar with collapsible state.
 *
 * Sections:
 * - Work: Dashboard, Tasks, Board
 * - Initiatives: Filterable list with progress counts
 * - Environment: Claude Code and Orchestrator sub-groups
 * - Preferences: User settings
 */
export function Sidebar({ onNewInitiative }: SidebarProps) {
	const location = useLocation();
	const expanded = useSidebarExpanded();
	const mobileMenuOpen = useMobileMenuOpen();
	const toggleSidebar = useUIStore((state) => state.toggleSidebar);
	const closeMobileMenu = useUIStore((state) => state.closeMobileMenu);
	const sidebarRef = useRef<HTMLElement>(null);

	// Initiative data
	const initiatives = useInitiatives();
	const currentInitiativeId = useCurrentInitiativeId();
	const selectInitiative = useInitiativeStore((state) => state.selectInitiative);
	const getInitiativeProgress = useInitiativeStore((state) => state.getInitiativeProgress);
	const tasks = useTaskStore((state) => state.tasks);

	// Section/group expansion state
	const [expandedSections, setExpandedSections] = useState<Set<string>>(() => loadExpandedState().sections);
	const [expandedGroups, setExpandedGroups] = useState<Set<string>>(() => loadExpandedState().groups);

	// Save state when it changes
	useEffect(() => {
		saveExpandedState(expandedSections, expandedGroups);
	}, [expandedSections, expandedGroups]);

	// Close mobile menu on route change
	useEffect(() => {
		closeMobileMenu();
	}, [location.pathname, closeMobileMenu]);

	// Close mobile menu on click outside
	useEffect(() => {
		if (!mobileMenuOpen) return;

		const handleClickOutside = (event: MouseEvent | TouchEvent) => {
			if (sidebarRef.current && !sidebarRef.current.contains(event.target as Node)) {
				closeMobileMenu();
			}
		};

		// Handle escape key
		const handleEscape = (event: KeyboardEvent) => {
			if (event.key === 'Escape') {
				closeMobileMenu();
			}
		};

		document.addEventListener('mousedown', handleClickOutside);
		document.addEventListener('touchstart', handleClickOutside);
		document.addEventListener('keydown', handleEscape);

		return () => {
			document.removeEventListener('mousedown', handleClickOutside);
			document.removeEventListener('touchstart', handleClickOutside);
			document.removeEventListener('keydown', handleEscape);
		};
	}, [mobileMenuOpen, closeMobileMenu]);

	// Sort initiatives: active first, then by updated_at
	const sortedInitiatives = useMemo(() => {
		return [...initiatives].sort((a, b) => {
			if (a.status === 'active' && b.status !== 'active') return -1;
			if (b.status === 'active' && a.status !== 'active') return 1;
			return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime();
		});
	}, [initiatives]);

	// Calculate initiative progress
	const initiativeProgress = useMemo(() => {
		return getInitiativeProgress(tasks);
	}, [getInitiativeProgress, tasks]);

	const toggleSection = useCallback((sectionId: string) => {
		setExpandedSections((prev) => {
			const next = new Set(prev);
			if (next.has(sectionId)) {
				next.delete(sectionId);
			} else {
				next.add(sectionId);
			}
			return next;
		});
	}, []);

	const toggleGroup = useCallback((groupLabel: string) => {
		setExpandedGroups((prev) => {
			const next = new Set(prev);
			if (next.has(groupLabel)) {
				next.delete(groupLabel);
			} else {
				next.add(groupLabel);
			}
			return next;
		});
	}, []);

	// Check if route is active
	const isActive = useCallback(
		(href: string): boolean => {
			const pathname = location.pathname;
			if (href === '/') {
				return pathname === '/' || pathname.startsWith('/tasks');
			}
			if (href === '/dashboard') {
				return pathname === '/dashboard';
			}
			if (href === '/environment') {
				return pathname === '/environment';
			}
			return pathname.startsWith(href);
		},
		[location.pathname]
	);

	const isGroupActive = useCallback(
		(basePath: string): boolean => {
			return location.pathname.startsWith(basePath);
		},
		[location.pathname]
	);

	// Handle initiative click
	const handleInitiativeClick = useCallback(
		(id: string | null, e: React.MouseEvent) => {
			e.preventDefault();
			selectInitiative(id);
		},
		[selectInitiative]
	);

	// Get progress for an initiative
	const getProgress = useCallback(
		(id: string) => {
			return initiativeProgress.get(id) || { completed: 0, total: 0 };
		},
		[initiativeProgress]
	);

	return (
		<aside
			ref={sidebarRef}
			className={`sidebar ${expanded ? 'expanded' : ''} ${mobileMenuOpen ? 'mobile-open' : ''}`}
			role="navigation"
			aria-label="Main navigation"
		>
			{/* Logo Section */}
			<div className="logo-section">
				{expanded && (
					<NavLink to="/" className="logo">
						<span className="logo-icon">&gt;_</span>
						<span className="logo-text">ORC</span>
					</NavLink>
				)}
				<Button
					variant="ghost"
					size="sm"
					iconOnly
					className="toggle-btn"
					onClick={toggleSidebar}
					title={expanded ? 'Collapse sidebar' : 'Expand sidebar'}
					aria-label={expanded ? 'Collapse sidebar' : 'Expand sidebar'}
				>
					<Icon name={expanded ? 'panel-left-close' : 'panel-left-open'} size={18} />
				</Button>
			</div>

			{/* Scrollable Navigation */}
			<div className="nav-container">
				{/* Work Section */}
				<nav className="nav-section">
					{expanded && <div className="section-header">Work</div>}
					<ul className="nav-list">
						{workItems.map((item) => (
							<li key={item.href}>
								<NavLink
									to={item.href}
									end={item.href === '/'}
									className={`nav-item ${isActive(item.href) ? 'active' : ''}`}
									title={!expanded ? item.label : undefined}
								>
									<span className="nav-icon">
										<Icon name={item.icon} size={18} />
									</span>
									{expanded && <span className="nav-label">{item.label}</span>}
								</NavLink>
							</li>
						))}
					</ul>
				</nav>

				{/* Initiatives Section */}
				{expanded && (
					<nav className="nav-section initiatives-section">
						<button
							className="section-header clickable"
							onClick={() => toggleSection('initiatives')}
							aria-expanded={expandedSections.has('initiatives')}
						>
							<span>Initiatives</span>
							<Icon
								name={expandedSections.has('initiatives') ? 'chevron-down' : 'chevron-right'}
								size={14}
							/>
						</button>

						{expandedSections.has('initiatives') && (
							<ul className="nav-list initiative-list">
								{/* All Tasks option */}
								<li>
									<a
										href="/"
										className={`nav-item initiative-item ${currentInitiativeId === null ? 'active' : ''}`}
										onClick={(e) => handleInitiativeClick(null, e)}
									>
										<span className={`initiative-indicator ${currentInitiativeId === null ? 'selected' : ''}`}>
											<span className={`indicator-dot ${currentInitiativeId === null ? 'filled' : ''}`} />
										</span>
										<span className="nav-label">All Tasks</span>
									</a>
								</li>

								{/* Initiative list */}
								{sortedInitiatives.map((initiative) => {
									const progress = getProgress(initiative.id);
									return (
										<li key={initiative.id}>
											<a
												href={`/?initiative=${initiative.id}`}
												className={`nav-item initiative-item ${currentInitiativeId === initiative.id ? 'active' : ''}`}
												onClick={(e) => handleInitiativeClick(initiative.id, e)}
												title={initiative.title}
											>
												<span className={`initiative-indicator ${currentInitiativeId === initiative.id ? 'selected' : ''}`}>
													<span className={`indicator-dot ${currentInitiativeId === initiative.id ? 'filled' : ''}`} />
												</span>
												<span className="nav-label initiative-title">{initiative.title}</span>
												{initiative.status !== 'active' ? (
													<span className={`initiative-status-badge status-${initiative.status}`}>
														{initiative.status}
													</span>
												) : progress.total > 0 ? (
													<span className="initiative-progress">
														({progress.completed}/{progress.total})
													</span>
												) : null}
											</a>
										</li>
									);
								})}

								{/* New Initiative button */}
								{onNewInitiative && (
									<li>
										<button
											className="nav-item new-initiative-btn"
											onClick={onNewInitiative}
											title="Create new initiative"
										>
											<span className="nav-icon">
												<Icon name="plus" size={14} />
											</span>
											<span className="nav-label">New Initiative</span>
										</button>
									</li>
								)}
							</ul>
						)}
					</nav>
				)}

				{/* Divider */}
				<div className="nav-divider" />

				{/* Environment Section */}
				<nav className="nav-section environment-section">
					{expanded ? (
						<button
							className="section-header clickable"
							onClick={() => toggleSection('environment')}
							aria-expanded={expandedSections.has('environment')}
						>
							<span>Environment</span>
							<Icon
								name={expandedSections.has('environment') ? 'chevron-down' : 'chevron-right'}
								size={14}
							/>
						</button>
					) : (
						<NavLink
							to="/environment"
							className={`nav-item ${location.pathname.startsWith('/environment') ? 'active' : ''}`}
							title="Environment"
						>
							<span className="nav-icon">
								<Icon name="layers" size={18} />
							</span>
						</NavLink>
					)}

					{expanded && expandedSections.has('environment') && (
						<>
							{/* Overview link */}
							<ul className="nav-list">
								<li>
									<NavLink
										to={environmentOverview.href}
										end
										className={`nav-item sub-item ${isActive(environmentOverview.href) ? 'active' : ''}`}
									>
										<span className="nav-icon">
											<Icon name={environmentOverview.icon} size={16} />
										</span>
										<span className="nav-label">{environmentOverview.label}</span>
									</NavLink>
								</li>
							</ul>

							{/* Groups */}
							{envGroups.map((group) => (
								<div key={group.label} className="nav-group">
									<button
										className={`group-header ${isGroupActive(group.basePath) ? 'active' : ''}`}
										onClick={() => toggleGroup(group.label)}
										aria-expanded={expandedGroups.has(group.label)}
									>
										<span className="group-icon">
											<Icon name={group.icon} size={16} />
										</span>
										<span className="group-label">{group.label}</span>
										<Icon
											name={expandedGroups.has(group.label) ? 'chevron-down' : 'chevron-right'}
											size={12}
										/>
									</button>

									{expandedGroups.has(group.label) && (
										<ul className="nav-list nested">
											{group.items.map((item) => (
												<li key={item.href}>
													<NavLink
														to={item.href}
														className={`nav-item nested-item ${isActive(item.href) ? 'active' : ''}`}
													>
														<span className="nav-icon">
															<Icon name={item.icon} size={14} />
														</span>
														<span className="nav-label">{item.label}</span>
													</NavLink>
												</li>
											))}
										</ul>
									)}
								</div>
							))}

							{/* Documentation link */}
							<ul className="nav-list">
								<li>
									<NavLink
										to={docsItem.href}
										className={`nav-item sub-item ${isActive(docsItem.href) ? 'active' : ''}`}
									>
										<span className="nav-icon">
											<Icon name={docsItem.icon} size={16} />
										</span>
										<span className="nav-label">{docsItem.label}</span>
									</NavLink>
								</li>
							</ul>
						</>
					)}
				</nav>
			</div>

			{/* Bottom Section: Preferences */}
			<div className="bottom-section">
				<div className="nav-divider" />
				<nav className="nav-section">
					<ul className="nav-list">
						<li>
							<NavLink
								to={preferencesItem.href}
								className={`nav-item ${isActive(preferencesItem.href) ? 'active' : ''}`}
								title={!expanded ? preferencesItem.label : undefined}
							>
								<span className="nav-icon">
									<Icon name={preferencesItem.icon} size={18} />
								</span>
								{expanded && <span className="nav-label">{preferencesItem.label}</span>}
							</NavLink>
						</li>
					</ul>
				</nav>
			</div>

			{/* Keyboard hint */}
			{expanded && (
				<div className="keyboard-hint">
					<kbd>{formatShortcut('B')}</kbd> to toggle
				</div>
			)}
		</aside>
	);
}
