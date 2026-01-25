import { Link } from 'react-router-dom';
import { Icon, type IconName } from '@/components/ui/Icon';
import { Tooltip } from '@/components/ui/Tooltip';
import { getInitiativeBadgeTitle } from '@/stores';
import type { Task, TaskState, Plan, PhaseStatus, TokenUsage } from '@/lib/types';
import { CATEGORY_CONFIG, PRIORITY_CONFIG } from '@/lib/types';
import './TimelineTab.css';

interface TimelineTabProps {
	task: Task;
	taskState: TaskState | null;
	plan: Plan | null;
}

export function TimelineTab({ task, taskState, plan }: TimelineTabProps) {
	if (!plan) {
		return (
			<div className="timeline-tab">
				<div className="timeline-empty">
					<Icon name="clock" size={32} />
					<h3>No Plan Available</h3>
					<p>This task doesn't have a plan yet. Run the task to generate phases.</p>
				</div>
			</div>
		);
	}

	return (
		<div className="timeline-tab">
			<div className="timeline-layout">
				{/* Timeline */}
				<div className="timeline-section">
					<h3 className="section-title">
						<Icon name="clock" size={16} />
						Phase Timeline
					</h3>
					<div className="timeline">
						{plan.phases.map((phase, index) => {
							const phaseState = taskState?.phases[phase.name];
							const status = phaseState?.status ?? phase.status;
							const isCurrent = taskState?.current_phase === phase.name;

							return (
								<TimelinePhase
									key={phase.id}
									name={phase.name}
									status={status as PhaseStatus}
									isCurrent={isCurrent}
									startedAt={phaseState?.started_at}
									completedAt={phaseState?.completed_at}
									iterations={phaseState?.iterations ?? phase.iterations}
									commitSha={phaseState?.commit_sha ?? phase.commit_sha}
									error={phaseState?.error ?? phase.error}
									isLast={index === plan.phases.length - 1}
									position={index + 1}
									totalPhases={plan.phases.length}
								/>
							);
						})}
					</div>
				</div>

				{/* Token Usage Stats */}
				{taskState?.tokens && (
					<div className="stats-section">
						<h3 className="section-title">
							<Icon name="dollar" size={16} />
							Token Usage
						</h3>
						<TokenStats tokens={taskState.tokens} />

						{/* Per-phase breakdown */}
						<h4 className="subsection-title">Per Phase</h4>
						<div className="phase-tokens">
							{plan.phases.map((phase) => {
								const phaseState = taskState.phases[phase.name];
								if (!phaseState?.tokens?.total_tokens) return null;

								return (
									<div key={phase.id} className="phase-token-row">
										<span className="phase-name">{phase.name}</span>
										<span className="phase-total">
											{formatNumber(phaseState.tokens.total_tokens)}
										</span>
									</div>
								);
							})}
						</div>
					</div>
				)}

				{/* Task Info */}
				<div className="info-section">
					<h3 className="section-title">
						<Icon name="info" size={16} />
						Task Info
					</h3>
					<TaskInfoList task={task} taskState={taskState} />
				</div>
			</div>
		</div>
	);
}

// Timeline Phase Component
interface TimelinePhaseProps {
	name: string;
	status: PhaseStatus;
	isCurrent: boolean;
	startedAt?: string;
	completedAt?: string;
	iterations: number;
	commitSha?: string;
	error?: string;
	isLast: boolean;
	position: number;
	totalPhases: number;
}

function TimelinePhase({
	name,
	status,
	isCurrent,
	startedAt,
	completedAt,
	iterations,
	commitSha,
	error,
	isLast,
	position,
	totalPhases,
}: TimelinePhaseProps) {
	const getStatusIcon = (): IconName => {
		switch (status) {
			case 'completed':
				return 'check-circle';
			case 'running':
				return 'play-circle';
			case 'failed':
				return 'x-circle';
			case 'skipped':
				return 'circle';
			default:
				return 'circle';
		}
	};

	const getStatusLabel = (): string => {
		switch (status) {
			case 'completed':
				return 'Completed';
			case 'running':
				return 'Running';
			case 'failed':
				return 'Failed';
			case 'skipped':
				return 'Skipped';
			default:
				return 'Pending';
		}
	};

	const getStatusClass = () => {
		if (isCurrent && status === 'running') return 'running';
		return status;
	};

	return (
		<div className={`timeline-phase ${getStatusClass()} ${isCurrent ? 'current' : ''}`}>
			<div className="phase-marker">
				<Icon name={getStatusIcon()} size={20} />
				{!isLast && <div className="phase-connector" />}
			</div>
			<div className="phase-content">
				<div className="phase-header">
					<span className="phase-position">{position} of {totalPhases}</span>
					<span className="phase-name">{name}</span>
					<span className={`phase-status-label ${status}`}>{getStatusLabel()}</span>
					{iterations > 1 && (
						<span className="phase-iterations">{iterations} iterations</span>
					)}
				</div>
				{(startedAt || completedAt) && (
					<div className="phase-time">
						{startedAt && <span>Started: {formatTime(startedAt)}</span>}
						{completedAt && <span>Completed: {formatTime(completedAt)}</span>}
					</div>
				)}
				{commitSha && (
					<div className="phase-commit">
						<Icon name="branch" size={12} />
						<code>{commitSha.slice(0, 7)}</code>
					</div>
				)}
				{error && (
					<div className="phase-error">
						<Icon name="alert-circle" size={12} />
						<span>{error}</span>
					</div>
				)}
			</div>
		</div>
	);
}

// Token Stats Component
interface TokenStatsProps {
	tokens: TokenUsage;
}

function TokenStats({ tokens }: TokenStatsProps) {
	const cacheRate =
		tokens.cache_read_input_tokens && tokens.input_tokens
			? Math.round((tokens.cache_read_input_tokens / (tokens.input_tokens + tokens.cache_read_input_tokens)) * 100)
			: 0;

	return (
		<div className="token-stats">
			<div className="stat-card">
				<span className="stat-label">Total Tokens</span>
				<span className="stat-value">{formatNumber(tokens.total_tokens)}</span>
			</div>
			<div className="stat-card">
				<span className="stat-label">Input</span>
				<span className="stat-value">{formatNumber(tokens.input_tokens)}</span>
			</div>
			<div className="stat-card">
				<span className="stat-label">Output</span>
				<span className="stat-value">{formatNumber(tokens.output_tokens)}</span>
			</div>
			{tokens.cache_read_input_tokens !== undefined && tokens.cache_read_input_tokens > 0 && (
				<>
					<div className="stat-card">
						<span className="stat-label">Cache Read</span>
						<span className="stat-value">{formatNumber(tokens.cache_read_input_tokens)}</span>
					</div>
					<div className="stat-card highlight">
						<span className="stat-label">Cache Rate</span>
						<span className="stat-value">{cacheRate}%</span>
					</div>
				</>
			)}
			{tokens.cache_creation_input_tokens !== undefined && tokens.cache_creation_input_tokens > 0 && (
				<div className="stat-card">
					<span className="stat-label">Cache Created</span>
					<span className="stat-value">{formatNumber(tokens.cache_creation_input_tokens)}</span>
				</div>
			)}
		</div>
	);
}

// Task Info List Component
interface TaskInfoListProps {
	task: Task;
	taskState: TaskState | null;
}

function TaskInfoList({ task, taskState }: TaskInfoListProps) {
	const categoryConfig = task.category ? CATEGORY_CONFIG[task.category] : null;
	const priority = task.priority || 'normal';
	const priorityConfig = PRIORITY_CONFIG[priority];
	const initiativeBadge = task.initiative_id ? getInitiativeBadgeTitle(task.initiative_id) : null;
	const duration = calculateDuration(task.started_at, task.completed_at);

	return (
		<dl className="info-list">
			{/* Status & Classification */}
			<div className="info-item">
				<dt>Status</dt>
				<dd className={`status-${task.status}`}>{task.status}</dd>
			</div>
			<div className="info-item">
				<dt>Weight</dt>
				<dd className="weight-value">{task.weight}</dd>
			</div>
			<div className="info-item">
				<dt>Queue</dt>
				<dd className={`queue-${task.queue || 'active'}`}>{task.queue || 'active'}</dd>
			</div>
			<div className="info-item">
				<dt>Priority</dt>
				<dd>
					<span
						className={`info-priority priority-${priority}`}
						style={{ '--priority-color': priorityConfig.color } as React.CSSProperties}
					>
						{priorityConfig.label}
					</span>
				</dd>
			</div>
			{categoryConfig && (
				<div className="info-item">
					<dt>Category</dt>
					<dd>
						<span
							className="info-category"
							style={{ '--category-color': categoryConfig.color } as React.CSSProperties}
						>
							<Icon name={categoryConfig.icon} size={12} />
							{categoryConfig.label}
						</span>
					</dd>
				</div>
			)}

			{/* Initiative */}
			{initiativeBadge && (
				<div className="info-item">
					<dt>Initiative</dt>
					<dd>
						<Link to={`/initiatives/${task.initiative_id}`} className="info-initiative-link">
							<Icon name="layers" size={12} />
							<Tooltip content={initiativeBadge.full}>
								<span>{initiativeBadge.display}</span>
							</Tooltip>
						</Link>
					</dd>
				</div>
			)}

			{/* Blocked By */}
			{task.blocked_by && task.blocked_by.length > 0 && (
				<div className="info-item">
					<dt>Blocked By</dt>
					<dd className="info-blocked-by">
						<Icon name="alert-circle" size={12} />
						{task.blocked_by.length} {task.blocked_by.length === 1 ? 'task' : 'tasks'}
					</dd>
				</div>
			)}

			{/* Git Info */}
			{task.branch && (
				<div className="info-item">
					<dt>Branch</dt>
					<dd>
						<code className="info-branch">{task.branch}</code>
					</dd>
				</div>
			)}
			{task.target_branch && (
				<div className="info-item">
					<dt>Target</dt>
					<dd>
						<code className="info-branch">{task.target_branch}</code>
					</dd>
				</div>
			)}

			{/* Execution Info (when running) */}
			{taskState?.current_phase && task.status === 'running' && (
				<div className="info-item">
					<dt>Current Phase</dt>
					<dd className="info-phase">{taskState.current_phase}</dd>
				</div>
			)}
			{taskState?.execution && task.status === 'running' && (
				<div className="info-item">
					<dt>Executor</dt>
					<dd>
						<Tooltip content={`PID ${taskState.execution.pid} on ${taskState.execution.hostname}`}>
							<span className="info-executor">
								<Icon name="cpu" size={12} />
								{taskState.execution.hostname}
							</span>
						</Tooltip>
					</dd>
				</div>
			)}

			{/* Retries */}
			{taskState?.retries !== undefined && taskState.retries > 0 && (
				<div className="info-item">
					<dt>Retries</dt>
					<dd className="info-retries">{taskState.retries}</dd>
				</div>
			)}

			{/* Timestamps */}
			<div className="info-item">
				<dt>Created</dt>
				<dd>{formatDateTime(task.created_at)}</dd>
			</div>
			{task.started_at && (
				<div className="info-item">
					<dt>Started</dt>
					<dd>{formatDateTime(task.started_at)}</dd>
				</div>
			)}
			{task.completed_at && (
				<div className="info-item">
					<dt>Completed</dt>
					<dd>{formatDateTime(task.completed_at)}</dd>
				</div>
			)}
			{duration && (
				<div className="info-item">
					<dt>Duration</dt>
					<dd className="info-duration">{duration}</dd>
				</div>
			)}
			<div className="info-item">
				<dt>Updated</dt>
				<dd className="info-updated">{formatDateTime(task.updated_at)}</dd>
			</div>
		</dl>
	);
}

// Utility functions
function formatNumber(num: number): string {
	if (num >= 1000000) {
		return (num / 1000000).toFixed(1) + 'M';
	}
	if (num >= 1000) {
		return (num / 1000).toFixed(1) + 'K';
	}
	return num.toString();
}

function formatTime(dateStr: string): string {
	return new Date(dateStr).toLocaleString(undefined, {
		month: 'short',
		day: 'numeric',
		hour: '2-digit',
		minute: '2-digit',
	});
}

function formatDateTime(dateStr: string): string {
	if (!dateStr) return '';
	const date = new Date(dateStr);
	if (isNaN(date.getTime())) return '';
	// Check for Go's zero time (year 1 AD) - display "Never" instead of garbage
	// Use getUTCFullYear() for consistent results across timezones.
	if (date.getUTCFullYear() <= 1) return 'Never';
	return date.toLocaleString(undefined, {
		year: 'numeric',
		month: 'short',
		day: 'numeric',
		hour: '2-digit',
		minute: '2-digit',
	});
}

function calculateDuration(startedAt?: string, completedAt?: string): string | null {
	if (!startedAt) return null;

	const start = new Date(startedAt);
	const end = completedAt ? new Date(completedAt) : new Date();

	if (isNaN(start.getTime())) return null;
	if (completedAt && isNaN(end.getTime())) return null;

	const diffMs = end.getTime() - start.getTime();
	if (diffMs < 0) return null;

	const seconds = Math.floor(diffMs / 1000);
	const minutes = Math.floor(seconds / 60);
	const hours = Math.floor(minutes / 60);
	const days = Math.floor(hours / 24);

	if (days > 0) {
		const remainingHours = hours % 24;
		return remainingHours > 0 ? `${days}d ${remainingHours}h` : `${days}d`;
	}
	if (hours > 0) {
		const remainingMinutes = minutes % 60;
		return remainingMinutes > 0 ? `${hours}h ${remainingMinutes}m` : `${hours}h`;
	}
	if (minutes > 0) {
		const remainingSeconds = seconds % 60;
		return remainingSeconds > 0 ? `${minutes}m ${remainingSeconds}s` : `${minutes}m`;
	}
	return `${seconds}s`;
}
