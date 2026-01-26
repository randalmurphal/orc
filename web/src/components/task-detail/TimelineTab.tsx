import { Link } from 'react-router-dom';
import type { Timestamp } from '@bufbuild/protobuf/wkt';
import { Icon, type IconName } from '@/components/ui/Icon';
import { Tooltip } from '@/components/ui/Tooltip';
import { getInitiativeBadgeTitle } from '@/stores';
import type { Task, TaskPlan, ExecutionState } from '@/gen/orc/v1/task_pb';
import { PhaseStatus, TaskCategory, TaskPriority, TaskWeight, TaskStatus, TaskQueue } from '@/gen/orc/v1/task_pb';
import type { TokenUsage } from '@/gen/orc/v1/common_pb';
import { timestampToDate } from '@/lib/time';
import './TimelineTab.css';

// Config for category display with proto enum keys
const CATEGORY_CONFIG: Record<TaskCategory, { label: string; color: string; icon: IconName }> = {
	[TaskCategory.FEATURE]: { label: 'Feature', color: 'var(--status-success)', icon: 'sparkles' },
	[TaskCategory.BUG]: { label: 'Bug', color: 'var(--status-error)', icon: 'bug' },
	[TaskCategory.REFACTOR]: { label: 'Refactor', color: 'var(--status-info)', icon: 'recycle' },
	[TaskCategory.CHORE]: { label: 'Chore', color: 'var(--text-muted)', icon: 'tools' },
	[TaskCategory.DOCS]: { label: 'Docs', color: 'var(--status-warning)', icon: 'file-text' },
	[TaskCategory.TEST]: { label: 'Test', color: 'var(--cyan)', icon: 'beaker' },
	[TaskCategory.UNSPECIFIED]: { label: '', color: '', icon: 'sparkles' },
};

// Config for priority display with proto enum keys
const PRIORITY_CONFIG: Record<TaskPriority, { label: string; color: string }> = {
	[TaskPriority.CRITICAL]: { label: 'Critical', color: 'var(--status-error)' },
	[TaskPriority.HIGH]: { label: 'High', color: 'var(--status-warning)' },
	[TaskPriority.NORMAL]: { label: 'Normal', color: 'var(--text-muted)' },
	[TaskPriority.LOW]: { label: 'Low', color: 'var(--text-muted)' },
	[TaskPriority.UNSPECIFIED]: { label: 'Normal', color: 'var(--text-muted)' },
};

// Weight labels for display
const WEIGHT_LABELS: Record<TaskWeight, string> = {
	[TaskWeight.TRIVIAL]: 'trivial',
	[TaskWeight.SMALL]: 'small',
	[TaskWeight.MEDIUM]: 'medium',
	[TaskWeight.LARGE]: 'large',
	[TaskWeight.UNSPECIFIED]: '',
};

interface TimelineTabProps {
	task: Task;
	taskState: ExecutionState | null;
	plan: TaskPlan | null;
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
							const isCurrent = task.currentPhase === phase.name;

							return (
								<TimelinePhase
									key={phase.id}
									name={phase.name}
									status={status}
									isCurrent={isCurrent}
									startedAt={phaseState?.startedAt}
									completedAt={phaseState?.completedAt}
									iterations={phaseState?.iterations ?? phase.iterations}
									commitSha={phaseState?.commitSha ?? phase.commitSha}
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
								if (!phaseState?.tokens?.totalTokens) return null;

								return (
									<div key={phase.id} className="phase-token-row">
										<span className="phase-name">{phase.name}</span>
										<span className="phase-total">
											{formatNumber(phaseState.tokens.totalTokens)}
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
	startedAt?: Timestamp;
	completedAt?: Timestamp;
	iterations: number;
	commitSha?: string;
	error?: string;
	isLast: boolean;
	position: number;
	totalPhases: number;
}

// Status class mapping for CSS
// Phase status is completion-only: PENDING, COMPLETED, SKIPPED
// Running/failed state is derived from task status + current phase
const STATUS_CLASS_MAP: Record<PhaseStatus, string> = {
	[PhaseStatus.COMPLETED]: 'completed',
	[PhaseStatus.SKIPPED]: 'skipped',
	[PhaseStatus.PENDING]: 'pending',
	[PhaseStatus.UNSPECIFIED]: 'pending',
};

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
		// Phase status is completion-only. Use isCurrent prop for running state.
		if (isCurrent && status === PhaseStatus.PENDING) {
			return 'play-circle'; // Currently running
		}
		switch (status) {
			case PhaseStatus.COMPLETED:
				return 'check-circle';
			case PhaseStatus.SKIPPED:
				return 'circle';
			default:
				return 'circle';
		}
	};

	const getStatusLabel = (): string => {
		// Phase status is completion-only. Use isCurrent prop for running state.
		if (isCurrent && status === PhaseStatus.PENDING) {
			return 'Running';
		}
		switch (status) {
			case PhaseStatus.COMPLETED:
				return 'Completed';
			case PhaseStatus.SKIPPED:
				return 'Skipped';
			default:
				return 'Pending';
		}
	};

	const getStatusClass = () => {
		// If this is the current phase and it's not completed, it's running
		if (isCurrent && status === PhaseStatus.PENDING) return 'running';
		return STATUS_CLASS_MAP[status] ?? 'pending';
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
						{startedAt && <span>Started: {formatTime(timestampToDate(startedAt))}</span>}
						{completedAt && <span>Completed: {formatTime(timestampToDate(completedAt))}</span>}
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
		tokens.cacheReadInputTokens && tokens.inputTokens
			? Math.round((tokens.cacheReadInputTokens / (tokens.inputTokens + tokens.cacheReadInputTokens)) * 100)
			: 0;

	return (
		<div className="token-stats">
			<div className="stat-card">
				<span className="stat-label">Total Tokens</span>
				<span className="stat-value">{formatNumber(tokens.totalTokens)}</span>
			</div>
			<div className="stat-card">
				<span className="stat-label">Input</span>
				<span className="stat-value">{formatNumber(tokens.inputTokens)}</span>
			</div>
			<div className="stat-card">
				<span className="stat-label">Output</span>
				<span className="stat-value">{formatNumber(tokens.outputTokens)}</span>
			</div>
			{tokens.cacheReadInputTokens !== undefined && tokens.cacheReadInputTokens > 0 && (
				<>
					<div className="stat-card">
						<span className="stat-label">Cache Read</span>
						<span className="stat-value">{formatNumber(tokens.cacheReadInputTokens)}</span>
					</div>
					<div className="stat-card highlight">
						<span className="stat-label">Cache Rate</span>
						<span className="stat-value">{cacheRate}%</span>
					</div>
				</>
			)}
			{tokens.cacheCreationInputTokens !== undefined && tokens.cacheCreationInputTokens > 0 && (
				<div className="stat-card">
					<span className="stat-label">Cache Created</span>
					<span className="stat-value">{formatNumber(tokens.cacheCreationInputTokens)}</span>
				</div>
			)}
		</div>
	);
}

// Task Info List Component
interface TaskInfoListProps {
	task: Task;
	taskState: ExecutionState | null;
}

// Status labels mapping
const STATUS_LABELS: Record<TaskStatus, string> = {
	[TaskStatus.CREATED]: 'created',
	[TaskStatus.CLASSIFYING]: 'classifying',
	[TaskStatus.PLANNED]: 'planned',
	[TaskStatus.RUNNING]: 'running',
	[TaskStatus.PAUSED]: 'paused',
	[TaskStatus.BLOCKED]: 'blocked',
	[TaskStatus.FINALIZING]: 'finalizing',
	[TaskStatus.COMPLETED]: 'completed',
	[TaskStatus.FAILED]: 'failed',
	[TaskStatus.RESOLVED]: 'resolved',
	[TaskStatus.UNSPECIFIED]: '',
};

// Queue labels mapping
const QUEUE_LABELS: Record<TaskQueue, string> = {
	[TaskQueue.ACTIVE]: 'active',
	[TaskQueue.BACKLOG]: 'backlog',
	[TaskQueue.UNSPECIFIED]: 'active',
};

// Priority keys for CSS class names
const PRIORITY_KEYS: Record<TaskPriority, string> = {
	[TaskPriority.CRITICAL]: 'critical',
	[TaskPriority.HIGH]: 'high',
	[TaskPriority.NORMAL]: 'normal',
	[TaskPriority.LOW]: 'low',
	[TaskPriority.UNSPECIFIED]: 'normal',
};

function getPriorityKey(priority: TaskPriority): string {
	return PRIORITY_KEYS[priority];
}

function TaskInfoList({ task, taskState }: TaskInfoListProps) {
	const categoryConfig = task.category !== TaskCategory.UNSPECIFIED
		? CATEGORY_CONFIG[task.category]
		: null;
	const priority = task.priority || TaskPriority.NORMAL;
	const priorityConfig = PRIORITY_CONFIG[priority];
	const initiativeBadge = task.initiativeId ? getInitiativeBadgeTitle(task.initiativeId) : null;
	const startedDate = timestampToDate(task.startedAt);
	const completedDate = timestampToDate(task.completedAt);
	const duration = calculateDuration(startedDate, completedDate);

	const statusLabel = STATUS_LABELS[task.status];
	const queueLabel = QUEUE_LABELS[task.queue || TaskQueue.ACTIVE];
	const weightLabel = WEIGHT_LABELS[task.weight];
	const priorityKey = getPriorityKey(priority);

	return (
		<dl className="info-list">
			{/* Status & Classification */}
			<div className="info-item">
				<dt>Status</dt>
				<dd className={`status-${statusLabel}`}>{statusLabel}</dd>
			</div>
			<div className="info-item">
				<dt>Weight</dt>
				<dd className="weight-value">{weightLabel}</dd>
			</div>
			<div className="info-item">
				<dt>Queue</dt>
				<dd className={`queue-${queueLabel}`}>{queueLabel}</dd>
			</div>
			<div className="info-item">
				<dt>Priority</dt>
				<dd>
					<span
						className={`info-priority priority-${priorityKey}`}
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
						<Link to={`/initiatives/${task.initiativeId}`} className="info-initiative-link">
							<Icon name="layers" size={12} />
							<Tooltip content={initiativeBadge.full}>
								<span>{initiativeBadge.display}</span>
							</Tooltip>
						</Link>
					</dd>
				</div>
			)}

			{/* Blocked By */}
			{task.blockedBy && task.blockedBy.length > 0 && (
				<div className="info-item">
					<dt>Blocked By</dt>
					<dd className="info-blocked-by">
						<Icon name="alert-circle" size={12} />
						{task.blockedBy.length} {task.blockedBy.length === 1 ? 'task' : 'tasks'}
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
			{task.targetBranch && (
				<div className="info-item">
					<dt>Target</dt>
					<dd>
						<code className="info-branch">{task.targetBranch}</code>
					</dd>
				</div>
			)}

			{/* Execution Info (when running) */}
			{task.currentPhase && task.status === TaskStatus.RUNNING && (
				<div className="info-item">
					<dt>Current Phase</dt>
					<dd className="info-phase">{task.currentPhase}</dd>
				</div>
			)}
			{taskState?.session && task.status === TaskStatus.RUNNING && (
				<div className="info-item">
					<dt>Session</dt>
					<dd>
						<Tooltip content={`Session ${taskState.session.id}`}>
							<span className="info-executor">
								<Icon name="cpu" size={12} />
								{taskState.session.id?.slice(0, 8) ?? 'N/A'}
							</span>
						</Tooltip>
					</dd>
				</div>
			)}

			{/* Retries */}
			{taskState?.retryContext && (
				<div className="info-item">
					<dt>Retry Info</dt>
					<dd className="info-retries">From: {taskState.retryContext.fromPhase}</dd>
				</div>
			)}

			{/* Timestamps */}
			<div className="info-item">
				<dt>Created</dt>
				<dd>{formatDateTime(timestampToDate(task.createdAt))}</dd>
			</div>
			{startedDate && (
				<div className="info-item">
					<dt>Started</dt>
					<dd>{formatDateTime(startedDate)}</dd>
				</div>
			)}
			{completedDate && (
				<div className="info-item">
					<dt>Completed</dt>
					<dd>{formatDateTime(completedDate)}</dd>
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
				<dd className="info-updated">{formatDateTime(timestampToDate(task.updatedAt))}</dd>
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

function formatTime(date: Date | null): string {
	if (!date) return 'N/A';
	return date.toLocaleString(undefined, {
		month: 'short',
		day: 'numeric',
		hour: '2-digit',
		minute: '2-digit',
	});
}

function formatDateTime(date: Date | null): string {
	if (!date) return '';
	return date.toLocaleString(undefined, {
		year: 'numeric',
		month: 'short',
		day: 'numeric',
		hour: '2-digit',
		minute: '2-digit',
	});
}

function calculateDuration(startedAt: Date | null, completedAt: Date | null): string | null {
	if (!startedAt) return null;

	const start = startedAt;
	const end = completedAt ?? new Date();

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
