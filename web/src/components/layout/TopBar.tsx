/**
 * TopBar component - 48px fixed header with project selector, search, session metrics, and actions.
 *
 * Features:
 * - Project dropdown button (static - dropdown menu is future task)
 * - Search box (static - search functionality is future task)
 * - Session metrics: duration, tokens, cost with colored icon badges
 * - Pause/Resume button that integrates with sessionStore
 * - New Task button
 */

import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui';
import { useSessionStore, useCurrentProject } from '@/stores';
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
	const {
		duration,
		formattedTokens,
		formattedCost,
		isPaused,
		pauseAll,
		resumeAll,
	} = useSessionStore();

	const projectName = projectNameProp ?? currentProject?.name ?? 'Select project';

	const handlePauseResume = async () => {
		if (isPaused) {
			await resumeAll();
		} else {
			await pauseAll();
		}
	};

	const classes = ['top-bar', className].filter(Boolean).join(' ');

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

				<div className="search-box">
					<Icon name="search" size={14} />
					<input
						type="text"
						placeholder="Search tasks..."
						aria-label="Search tasks"
					/>
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
			</div>
		</header>
	);
}
