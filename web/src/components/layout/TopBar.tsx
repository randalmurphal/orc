/**
 * TopBar component - 48px fixed header with project selector, search, session metrics, and actions.
 *
 * Features:
 * - Project dropdown button (static - dropdown menu is future task)
 * - Search box with Cmd+K shortcut (search functionality is future task)
 * - Session metrics: duration, tokens, cost with colored icon badges
 * - Pause/Resume button that integrates with sessionStore
 * - New Task button
 * - Responsive: hides session stats at 768px, expandable search at 480px
 */

import { useCallback, useContext, useEffect, useRef, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { Button } from '@/components/ui/Button';
import { Icon, Tooltip } from '@/components/ui';
import {
	useFormattedDuration,
	useFormattedCost,
	useFormattedTokens,
	useIsPaused,
	useSessionStore,
	useCurrentProject,
	useCurrentProjectId,
} from '@/stores';
import { AppShellContext } from './AppShellContext';
import './TopBar.css';

interface TopBarProps {
	projectName?: string;
	onProjectChange?: () => void;
	onNewTask?: () => void;
	onSearch?: (query: string) => void;
	className?: string;
}

interface SessionStatProps {
	icon: 'clock' | 'zap' | 'dollar';
	label: string;
	value: string;
	colorClass: 'purple' | 'amber' | 'green';
}

function SessionStat({ icon, label, value, colorClass }: SessionStatProps) {
	return (
		<div className="session-stat">
			<div className={`session-stat-icon ${colorClass}`}>
				<Icon name={icon} size={10} />
			</div>
			<span className="label">{label}</span>
			<span className="value">{value}</span>
		</div>
	);
}

export function TopBar({
	projectName: projectNameProp,
	onProjectChange,
	onNewTask,
	className = '',
}: TopBarProps) {
	const currentProject = useCurrentProject();
	const projectId = useCurrentProjectId();
	const duration = useFormattedDuration();
	const formattedTokens = useFormattedTokens();
	const formattedCost = useFormattedCost();
	const isPaused = useIsPaused();
	const pauseAll = useSessionStore((s) => s.pauseAll);
	const resumeAll = useSessionStore((s) => s.resumeAll);
	const location = useLocation();

	// Get right panel state from AppShell context (null if not in AppShellProvider)
	const appShell = useContext(AppShellContext);

	// Only show panel toggle for routes that have panel content
	// Currently only /board has panel content (BoardCommandPanel)
	const isBoard = location.pathname === '/board';
	const showPanelToggle = appShell && isBoard;

	const searchInputRef = useRef<HTMLInputElement>(null);
	const [searchExpanded, setSearchExpanded] = useState(false);

	const projectName = projectNameProp ?? currentProject?.name ?? 'Select project';

	const handlePauseResume = async () => {
		if (isPaused) {
			await resumeAll(projectId ?? undefined);
		} else {
			await pauseAll(projectId ?? undefined);
		}
	};

	// Focus search on Cmd+K / Ctrl+K
	// No dependency on searchExpanded — Escape always calls setSearchExpanded(false) (idempotent)
	const handleKeyDown = useCallback((e: KeyboardEvent) => {
		if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
			e.preventDefault();
			setSearchExpanded(true);
			searchInputRef.current?.focus();
		}
		// Close expanded search on Escape (idempotent — safe to call when already collapsed)
		if (e.key === 'Escape') {
			setSearchExpanded(false);
			searchInputRef.current?.blur();
		}
	}, []);

	useEffect(() => {
		document.addEventListener('keydown', handleKeyDown);
		return () => document.removeEventListener('keydown', handleKeyDown);
	}, [handleKeyDown]);

	const handleSearchToggle = () => {
		setSearchExpanded((prev) => !prev);
		if (!searchExpanded) {
			// Focus after state update
			setTimeout(() => searchInputRef.current?.focus(), 0);
		}
	};

	const classes = ['top-bar', className].filter(Boolean).join(' ');
	const searchClasses = ['search-box', searchExpanded && 'search-expanded'].filter(Boolean).join(' ');

	return (
		<header className={classes} role="banner">
			<div className="top-bar-left">
				<button
					className="project-selector"
					onClick={onProjectChange}
					aria-haspopup="listbox"
				>
					<Icon name="folder" size={14} />
					<span className="project-name">{projectName}</span>
					<Icon name="chevron-down" size={12} />
				</button>

				{/* Mobile search toggle button (visible <480px) */}
				<button
					className="search-toggle"
					onClick={handleSearchToggle}
					aria-label="Toggle search"
					aria-expanded={searchExpanded}
				>
					<Icon name="search" size={14} />
				</button>

				<div className={searchClasses}>
					<Icon name="search" size={14} />
					<input
						ref={searchInputRef}
						type="text"
						placeholder="Search tasks..."
						aria-label="Search tasks"
					/>
					<span className="search-kbd-hint">
						<kbd>⌘</kbd>
						<kbd>K</kbd>
					</span>
				</div>
			</div>

			<div className="top-bar-center">
				<div className="session-info">
					<SessionStat
						icon="clock"
						label="Session"
						value={duration}
						colorClass="purple"
					/>
					<div className="session-divider" />
					<SessionStat
						icon="zap"
						label="Tokens"
						value={formattedTokens}
						colorClass="amber"
					/>
					<div className="session-divider" />
					<SessionStat
						icon="dollar"
						label="Cost"
						value={formattedCost}
						colorClass="green"
					/>
				</div>
			</div>

			<div className="top-bar-right">
				<Button variant="ghost" size="sm" onClick={handlePauseResume}>
					{isPaused ? 'Resume' : 'Pause'}
				</Button>
				{onNewTask && (
					<Button
						variant="primary"
						size="sm"
						leftIcon={<Icon name="plus" size={14} />}
						onClick={onNewTask}
					>
						New Task
					</Button>
				)}
				{showPanelToggle && appShell && (
					<Tooltip content={<>Toggle panel <kbd>Shift+Alt+R</kbd></>}>
						<Button
							ref={appShell.panelToggleRef}
							variant="ghost"
							size="sm"
							iconOnly
							onClick={appShell.toggleRightPanel}
							aria-label="Toggle right panel"
							aria-expanded={appShell.isRightPanelOpen}
						>
							<Icon name="panel-right" size={16} />
						</Button>
					</Tooltip>
				)}
			</div>
		</header>
	);
}
