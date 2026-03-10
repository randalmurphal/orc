/**
 * TopBar component - 48px fixed header with navigation tabs, search, session metrics, and actions.
 *
 * Features:
 * - Navigation tabs: Home, Board, Knowledge, Workflows, Settings
 * - Search box with Cmd+K shortcut (search functionality is future task)
 * - Session metrics: duration, tokens, cost with colored icon badges
 * - Pause/Resume button that integrates with sessionStore
 * - New Task button
 * - Responsive: hides session stats at 768px, expandable search at 480px
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import { NavLink } from 'react-router-dom';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui';
import {
	useFormattedDuration,
	useFormattedCost,
	useFormattedTokens,
	useIsPaused,
	useSessionStore,
	useCurrentProjectId,
	useRunningTasks,
	formatDuration,
} from '@/stores';
import { formatCost, formatNumber } from '@/lib/format';
import './TopBar.css';

interface TopBarProps {
	onNewTask?: () => void;
	className?: string;
}

interface SessionStatProps {
	icon: 'clock' | 'zap' | 'dollar';
	label: string;
	value: string;
	colorClass: 'purple' | 'amber' | 'green';
}

const NAV_TABS = [
	{ label: 'Home', path: '/', end: true },
	{ label: 'Board', path: '/board', end: false },
	{ label: 'Inbox', path: '/recommendations', end: false },
	{ label: 'Knowledge', path: '/knowledge', end: false },
	{ label: 'Workflows', path: '/workflows', end: false },
	{ label: 'Settings', path: '/settings', end: false },
] as const;

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
	onNewTask,
	className = '',
}: TopBarProps) {
	const projectId = useCurrentProjectId();
	const duration = useFormattedDuration();
	const formattedTokens = useFormattedTokens();
	const formattedCost = useFormattedCost();
	const isPaused = useIsPaused();
	const runningTasks = useRunningTasks();
	const pauseAll = useSessionStore((s) => s.pauseAll);
	const resumeAll = useSessionStore((s) => s.resumeAll);

	const searchInputRef = useRef<HTMLInputElement>(null);
	const [searchExpanded, setSearchExpanded] = useState(false);

	const handlePauseResume = async () => {
		if (isPaused) {
			await resumeAll(projectId ?? undefined);
		} else {
			await pauseAll(projectId ?? undefined);
		}
	};

	// Focus search on Cmd+K / Ctrl+K
	const handleKeyDown = useCallback((e: KeyboardEvent) => {
		if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
			e.preventDefault();
			setSearchExpanded(true);
			searchInputRef.current?.focus();
		}
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
			setTimeout(() => searchInputRef.current?.focus(), 0);
		}
	};

	const classes = ['top-bar', className].filter(Boolean).join(' ');
	const searchClasses = ['search-box', searchExpanded && 'search-expanded'].filter(Boolean).join(' ');
	const fallbackStartTime = runningTasks.reduce<Date | null>((earliest, task) => {
		if (!task.startedAt?.seconds) {
			return earliest;
		}
		const startedAt = new Date(Number(task.startedAt.seconds) * 1000);
		if (!earliest || startedAt < earliest) {
			return startedAt;
		}
		return earliest;
	}, null);
	const fallbackDuration = fallbackStartTime ? formatDuration(fallbackStartTime) : duration;
	const fallbackTokensValue = runningTasks.reduce(
		(total, task) => total + (task.execution?.tokens?.totalTokens ?? 0),
		0
	);
	const fallbackCostValue = runningTasks.reduce(
		(total, task) => total + (task.execution?.cost?.totalCostUsd ?? 0),
		0
	);
	const displayDuration =
		duration === '0m' && runningTasks.length > 0 && fallbackStartTime ? fallbackDuration : duration;
	const displayTokens =
		formattedTokens === '0' && fallbackTokensValue > 0
			? formatNumber(fallbackTokensValue)
			: formattedTokens;
	const displayCost =
		formattedCost === '$0.00' && fallbackCostValue > 0
			? formatCost(fallbackCostValue)
			: formattedCost;

	return (
		<header className={classes} role="banner">
			<nav className="top-bar-nav">
				{NAV_TABS.map((tab) => (
					<NavLink
						key={tab.path}
						to={tab.path}
						end={tab.end}
						className={({ isActive }) =>
							`top-bar-nav-tab ${isActive ? 'active' : ''}`
						}
					>
						{tab.label}
					</NavLink>
				))}
			</nav>

			<div className="top-bar-center">
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

				<div className="session-info">
					<SessionStat
						icon="clock"
						label="Session"
						value={displayDuration}
						colorClass="purple"
					/>
					<div className="session-divider" />
					<SessionStat
						icon="zap"
						label="Tokens"
						value={displayTokens}
						colorClass="amber"
					/>
					<div className="session-divider" />
					<SessionStat
						icon="dollar"
						label="Cost"
						value={displayCost}
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
			</div>
		</header>
	);
}
