import { Icon, type IconName } from '@/components/ui/Icon';
import type { Task, TaskState, Plan, PhaseStatus, TokenUsage } from '@/lib/types';
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
					<dl className="info-list">
						<div className="info-item">
							<dt>Weight</dt>
							<dd>{task.weight}</dd>
						</div>
						<div className="info-item">
							<dt>Status</dt>
							<dd className={`status-${task.status}`}>{task.status}</dd>
						</div>
						{taskState?.retries !== undefined && taskState.retries > 0 && (
							<div className="info-item">
								<dt>Retries</dt>
								<dd>{taskState.retries}</dd>
							</div>
						)}
						<div className="info-item">
							<dt>Created</dt>
							<dd>{formatDate(task.created_at)}</dd>
						</div>
						{task.started_at && (
							<div className="info-item">
								<dt>Started</dt>
								<dd>{formatDate(task.started_at)}</dd>
							</div>
						)}
						{task.completed_at && (
							<div className="info-item">
								<dt>Completed</dt>
								<dd>{formatDate(task.completed_at)}</dd>
							</div>
						)}
					</dl>
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
					<span className="phase-name">{name}</span>
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

function formatDate(dateStr: string): string {
	return new Date(dateStr).toLocaleDateString(undefined, {
		year: 'numeric',
		month: 'short',
		day: 'numeric',
	});
}

function formatTime(dateStr: string): string {
	return new Date(dateStr).toLocaleString(undefined, {
		month: 'short',
		day: 'numeric',
		hour: '2-digit',
		minute: '2-digit',
	});
}
