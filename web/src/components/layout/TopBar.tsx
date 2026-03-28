/**
 * TopBar component - 48px fixed header with navigation tabs, command palette trigger,
 * session metrics, and actions.
 */

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
	{ label: 'Project', path: '/project', end: false },
	{ label: 'Board', path: '/board', end: false },
	{ label: 'Inbox', path: '/recommendations', end: false },
	{ label: 'Knowledge', path: '/knowledge', end: false },
	{ label: 'Workflows', path: '/workflows', end: false },
	{ label: 'Settings', path: '/settings', end: false },
] as const;

const COMMAND_PALETTE_EVENT = 'orc:command-palette';

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

	const handlePauseResume = async () => {
		if (isPaused) {
			await resumeAll(projectId ?? undefined);
		} else {
			await pauseAll(projectId ?? undefined);
		}
	};

	const classes = ['top-bar', className].filter(Boolean).join(' ');
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
				<button
					type="button"
					className="command-palette-trigger"
					onClick={() => window.dispatchEvent(new CustomEvent(COMMAND_PALETTE_EVENT))}
					aria-label="Open command palette"
				>
					<Icon name="search" size={14} />
					<span className="command-palette-trigger-label">Search</span>
					<span className="command-palette-trigger-hint">
						<kbd>⌘</kbd>
						<kbd>K</kbd>
					</span>
				</button>

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
