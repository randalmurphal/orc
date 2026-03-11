/**
 * AttentionDashboard - New board design focused on attention management
 *
 * This component implements the UX Simplification redesign as an attention
 * management dashboard with three main sections:
 * - Running: Active tasks with progress and timing
 * - Needs Attention: Blocked tasks, decisions, gates requiring action
 * - Queue: Ready tasks organized by initiative
 */

import { useEffect, useState, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { cn } from '@/lib/utils';
import './AttentionDashboard.css';
import { useCurrentProjectId } from '@/stores/projectStore';
import { attentionDashboardClient } from '@/lib/client';
import { toast } from '@/stores';
import { onAttentionDashboardSignal } from '@/lib/events/attentionDashboardSignals';
import type {
	GetAttentionDashboardDataResponse,
	RunningTask,
	AttentionItem,
	QueuedTask,
	InitiativeSwimlane,
} from '@/gen/orc/v1/attention_dashboard_pb';
import {
	AttentionAction,
	AttentionItemType,
	PhaseStepStatus,
} from '@/gen/orc/v1/attention_dashboard_pb';
import type { TaskPriority, TaskCategory } from '@/gen/orc/v1/task_pb';
import { TaskPriority as TaskPriorityEnum, TaskCategory as TaskCategoryEnum } from '@/gen/orc/v1/task_pb';

export interface AttentionDashboardProps {
	className?: string;
}

interface CollapsedState {
	[swimlaneId: string]: boolean;
}

interface LoadDashboardOptions {
	background?: boolean;
}

// Use centralized client from @/lib/client

/**
 * AttentionDashboard component - implements attention management dashboard
 */
export function AttentionDashboard({ className }: AttentionDashboardProps) {
	const navigate = useNavigate();
	const projectId = useCurrentProjectId() || '';

	const [dashboardData, setDashboardData] = useState<GetAttentionDashboardDataResponse | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [collapsedState, setCollapsedState] = useState<CollapsedState>({});
	const [expandedRunningTasks, setExpandedRunningTasks] = useState<Set<string>>(new Set());
	const [pendingActions, setPendingActions] = useState<Record<string, AttentionAction>>({});
	const hasLoadedDashboardDataRef = useRef(false);
	const backgroundRefreshInFlightRef = useRef(false);
	const backgroundRefreshQueuedRef = useRef(false);

	// Load dashboard data
	const loadDashboardData = useCallback(async (options: LoadDashboardOptions = {}) => {
		const isBackgroundRefresh = options.background ?? false;
		const shouldShowLoadingState = !isBackgroundRefresh || !hasLoadedDashboardDataRef.current;

		try {
			if (shouldShowLoadingState) {
				setLoading(true);
			}
			setError(null);

			const response = await attentionDashboardClient.getAttentionDashboardData({
				projectId,
			});

			hasLoadedDashboardDataRef.current = true;
			setDashboardData(response);
			return true;
		} catch (err) {
			console.error('Failed to load dashboard data:', err);
			if (!isBackgroundRefresh || !hasLoadedDashboardDataRef.current) {
				setError(err instanceof Error ? err.message : 'Failed to load dashboard data');
			}
			return false;
		} finally {
			if (shouldShowLoadingState) {
				setLoading(false);
			}
		}
	}, [projectId]);

	const requestBackgroundRefresh = useCallback(() => {
		if (backgroundRefreshInFlightRef.current) {
			backgroundRefreshQueuedRef.current = true;
			return;
		}

		backgroundRefreshInFlightRef.current = true;
		void loadDashboardData({ background: true }).finally(() => {
			backgroundRefreshInFlightRef.current = false;
			if (!backgroundRefreshQueuedRef.current) {
				return;
			}

			backgroundRefreshQueuedRef.current = false;
			requestBackgroundRefresh();
		});
	}, [loadDashboardData]);

	// Load data on mount and project change
	useEffect(() => {
		loadDashboardData();
	}, [loadDashboardData]);

	// Keep a slow fallback refresh for missed events or manual backend changes.
	useEffect(() => {
		const interval = setInterval(() => {
			requestBackgroundRefresh();
		}, 30000);
		return () => clearInterval(interval);
	}, [requestBackgroundRefresh]);

	useEffect(() => {
		return onAttentionDashboardSignal((signal) => {
			if (signal.projectId !== projectId) {
				return;
			}
			requestBackgroundRefresh();
		});
	}, [projectId, requestBackgroundRefresh]);

	// Handle task navigation
	const handleTaskClick = useCallback((taskId: string) => {
		navigate(`/tasks/${taskId}`);
	}, [navigate]);

	// Handle running task expansion
	const handleRunningTaskToggle = useCallback((taskId: string) => {
		setExpandedRunningTasks((prev: Set<string>) => {
			const newSet = new Set(prev);
			if (newSet.has(taskId)) {
				newSet.delete(taskId);
			} else {
				newSet.add(taskId);
			}
			return newSet;
		});
	}, []);

	// Handle swimlane collapse
	const handleSwimlaneToggle = useCallback((initiativeId: string) => {
		setCollapsedState((prev: CollapsedState) => ({
			...prev,
			[initiativeId]: !prev[initiativeId]
		}));
	}, []);

	const handleAttentionAction = useCallback(async (
		item: AttentionItem,
		action: AttentionAction,
		decisionOptionId?: string,
	) => {
		setPendingActions((prev) => ({
			...prev,
			[item.id]: action,
		}));

		try {
			const response = await attentionDashboardClient.performAttentionAction({
				projectId,
				attentionItemId: item.id,
				action,
				decisionOptionId: decisionOptionId ?? '',
			});
			if (!response.success) {
				throw new Error(response.errorMessage || 'Attention action failed');
			}

			const refreshed = await loadDashboardData({ background: true });
			if (!refreshed) {
				toast.warning('Action succeeded, but the dashboard did not refresh.');
			}
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Attention action failed';
			toast.error(message);
		} finally {
			setPendingActions((prev) => {
				const next = { ...prev };
				delete next[item.id];
				return next;
			});
		}
	}, [loadDashboardData, projectId]);

	if (loading) {
		return (
			<div className="attention-dashboard-loading">
				<p>Loading attention dashboard...</p>
			</div>
		);
	}

	if (error) {
		return (
			<div className="attention-dashboard-error">
				<p>Error loading dashboard: {error}</p>
				<button onClick={() => void loadDashboardData()}>Retry</button>
			</div>
		);
	}

	if (!dashboardData) {
		return (
			<div className="attention-dashboard-empty">
				<p>No dashboard data available</p>
			</div>
		);
	}

	return (
		<div className={cn('attention-dashboard responsive', className)} style={{ display: 'grid' }}>
			{/* Running Section */}
			<section
				className="running-section"
				role="region"
				aria-labelledby="running-header"
			>
				<h2 id="running-header">Running Tasks</h2>
				{dashboardData.runningSummary && (
					<RunningSection
						summary={dashboardData.runningSummary}
						expandedTasks={expandedRunningTasks}
						onTaskClick={handleTaskClick}
						onTaskToggle={handleRunningTaskToggle}
					/>
				)}
			</section>

			{/* Needs Attention Section */}
			<section
				className="attention-section"
				role="region"
				aria-labelledby="attention-header"
			>
				<h2 id="attention-header">Needs Attention</h2>
				<AttentionSection
					items={dashboardData.attentionItems}
					onTaskClick={handleTaskClick}
					pendingActions={pendingActions}
					onAction={handleAttentionAction}
				/>
			</section>

			{/* Queue Section */}
			<section
				className="queue-section"
				role="region"
				aria-labelledby="queue-header"
			>
				<h2 id="queue-header">Task Queue</h2>
				{dashboardData.queueSummary && (
					<QueueSection
						summary={dashboardData.queueSummary}
						collapsedState={collapsedState}
						onTaskClick={handleTaskClick}
						onSwimlaneToggle={handleSwimlaneToggle}
					/>
				)}
			</section>
		</div>
	);
}

// Helper functions
function formatElapsedTime(elapsedSeconds: number): string {
	const minutes = Math.floor(elapsedSeconds / 60);
	const seconds = elapsedSeconds % 60;
	return `${minutes}:${seconds.toString().padStart(2, '0')}`;
}

function getPriorityClass(priority: TaskPriority): string {
	switch (priority) {
		case TaskPriorityEnum.CRITICAL:
			return 'critical';
		case TaskPriorityEnum.HIGH:
			return 'high';
		case TaskPriorityEnum.NORMAL:
			return 'normal';
		case TaskPriorityEnum.LOW:
			return 'low';
		default:
			return 'normal';
	}
}

function getPriorityLabel(priority: TaskPriority): string {
	switch (priority) {
		case TaskPriorityEnum.CRITICAL:
			return 'Critical';
		case TaskPriorityEnum.HIGH:
			return 'High';
		case TaskPriorityEnum.NORMAL:
			return 'Normal';
		case TaskPriorityEnum.LOW:
			return 'Low';
		default:
			return 'Normal';
	}
}

function getCategoryClass(category: TaskCategory): string {
	switch (category) {
		case TaskCategoryEnum.FEATURE:
			return 'feature';
		case TaskCategoryEnum.BUG:
			return 'bug';
		case TaskCategoryEnum.REFACTOR:
			return 'refactor';
		case TaskCategoryEnum.CHORE:
			return 'chore';
		case TaskCategoryEnum.DOCS:
			return 'docs';
		case TaskCategoryEnum.TEST:
			return 'test';
		default:
			return 'feature';
	}
}

// RunningSection component
interface RunningSectionProps {
	summary: GetAttentionDashboardDataResponse['runningSummary'];
	expandedTasks: Set<string>;
	onTaskClick: (taskId: string) => void;
	onTaskToggle: (taskId: string) => void;
}

function RunningSection({ summary, expandedTasks, onTaskClick, onTaskToggle }: RunningSectionProps) {
	if (!summary || summary.tasks.length === 0) {
		return <div className="empty-section">No running tasks</div>;
	}

	return (
		<div className="running-tasks">
			{summary.tasks.map((task) => (
				<RunningTaskCard
					key={task.id}
					task={task}
					expanded={expandedTasks.has(task.id)}
					onClick={() => onTaskClick(task.id)}
					onToggle={() => onTaskToggle(task.id)}
				/>
			))}
		</div>
	);
}

// RunningTaskCard component
interface RunningTaskCardProps {
	task: RunningTask;
	expanded: boolean;
	onClick: () => void;
	onToggle: () => void;
}

function RunningTaskCard({ task, expanded, onClick, onToggle }: RunningTaskCardProps) {
	// Check if any phase has failed
	const hasFailures = task.phaseProgress?.steps.some(step => step.status === PhaseStepStatus.FAILED) || false;

	return (
		<div
			className={cn('running-card', {
				expanded,
				'has-failures': hasFailures
			})}
			onClick={(e) => {
				e.stopPropagation();
				onToggle();
			}}
		>
			<div className="card-header">
				<div className="task-info">
					<span className="task-id">{task.id}</span>
					<h3 className="task-title">{task.title}</h3>
				</div>
				<div className="timing-info">
					{task.elapsedTimeSeconds && (
						<span className="elapsed-time">{formatElapsedTime(Number(task.elapsedTimeSeconds))}</span>
					)}
				</div>
			</div>

			{task.phaseProgress && (
				<div className="phase-pipeline">
					<PhasePipeline progress={task.phaseProgress} />
				</div>
			)}

			{task.initiativeTitle && (
				<div className="initiative-badge">
					<span>{task.initiativeTitle}</span>
				</div>
			)}

			<div className={cn('running-output', { expanded })}>
				{task.outputLines && task.outputLines.length > 0 ? (
					<pre>{task.outputLines.join('\n')}</pre>
				) : (
					<p>No output available</p>
				)}
			</div>

			<button
				className="view-task-btn"
				onClick={(e) => {
					e.stopPropagation();
					onClick();
				}}
			>
				View Task Details
			</button>
		</div>
	);
}

// PhasePipeline component
interface PhasePipelineProps {
	progress: RunningTask['phaseProgress'];
}

function PhasePipeline({ progress }: PhasePipelineProps) {
	if (!progress) return null;

	return (
		<div className="pipeline">
			{progress.steps.map((step, _index) => (
				<div
					key={step.name}
					className={cn('pipeline-step', {
						active: step.status === PhaseStepStatus.ACTIVE,
						completed: step.status === PhaseStepStatus.COMPLETED,
						failed: step.status === PhaseStepStatus.FAILED,
						pending: step.status === PhaseStepStatus.PENDING,
					})}
				>
					<span className="step-name">
						{step.name.charAt(0).toUpperCase() + step.name.slice(1)}
					</span>
					{step.status === PhaseStepStatus.FAILED && (
						<span className="error-indicator" title="Phase failed">⚠</span>
					)}
				</div>
			))}
		</div>
	);
}

// AttentionSection component
interface AttentionSectionProps {
	items: AttentionItem[];
	onTaskClick: (taskId: string) => void;
	pendingActions: Record<string, AttentionAction>;
	onAction: (item: AttentionItem, action: AttentionAction, decisionOptionId?: string) => void;
}

function AttentionSection({ items, onTaskClick, pendingActions, onAction }: AttentionSectionProps) {
	if (items.length === 0) {
		return <div className="empty-section">No items need attention</div>;
	}

	return (
		<div className="attention-items">
			{items.map((item) => (
				<AttentionItemCard
					key={item.id}
					item={item}
					onTaskClick={onTaskClick}
					pendingAction={pendingActions[item.id]}
					onAction={onAction}
				/>
			))}
		</div>
	);
}

// AttentionItemCard component
interface AttentionItemCardProps {
	item: AttentionItem;
	onTaskClick: (taskId: string) => void;
	pendingAction?: AttentionAction;
	onAction: (item: AttentionItem, action: AttentionAction, decisionOptionId?: string) => void;
}

function AttentionItemCard({ item, onTaskClick, pendingAction, onAction }: AttentionItemCardProps) {
	// Get type-specific styling class
	const getTypeClass = (type: typeof item.type) => {
		switch (type) {
			case AttentionItemType.FAILED_TASK:
				return 'failed-task';
			case AttentionItemType.ERROR_STATE:
				return 'error-state';
			case AttentionItemType.BLOCKED_TASK:
				return 'blocked-task';
			default:
				return 'normal';
		}
	};

	const isActionPending = (action: AttentionAction) => pendingAction === action;

	const renderActionButton = (
		action: AttentionAction,
		label: string,
		className: string,
	) => (
		<button
			className={className}
			disabled={pendingAction !== undefined}
			onClick={() => onAction(item, action)}
		>
			{isActionPending(action) ? 'Working…' : label}
		</button>
	);

	return (
		<div className={cn('attention-item', getPriorityClass(item.priority), getTypeClass(item.type))}>
			<div className="item-header">
				<span className="task-id">{item.taskId}</span>
				<span className={cn('priority-badge', getPriorityClass(item.priority))}>
					{getPriorityLabel(item.priority)}
				</span>
			</div>

			<h3 className="item-title">{item.title}</h3>
			<p className="item-description">{item.description}</p>

			{/* Show error message if available */}
			{item.errorMessage && (
				<div className="error-message">
					<span className="error-icon">⚠</span>
					<span>{item.errorMessage}</span>
				</div>
			)}

			<div className="item-actions">
				{item.availableActions.includes(AttentionAction.VIEW) && (
					<button
						className="action-btn"
						onClick={() => onTaskClick(item.taskId)}
						disabled={pendingAction !== undefined}
					>
						View
					</button>
				)}

				{item.availableActions.includes(AttentionAction.SKIP) && (
					renderActionButton(AttentionAction.SKIP, 'Skip', 'action-btn secondary')
				)}

				{item.availableActions.includes(AttentionAction.FORCE) && (
					renderActionButton(AttentionAction.FORCE, 'Force', 'action-btn secondary')
				)}

				{item.availableActions.includes(AttentionAction.APPROVE) && (
					(item.decisionOptions && item.decisionOptions.length > 0)
						? null
						: renderActionButton(AttentionAction.APPROVE, 'Approve', 'action-btn primary')
				)}

				{item.availableActions.includes(AttentionAction.REJECT) && (
					renderActionButton(AttentionAction.REJECT, 'Reject', 'action-btn secondary')
				)}

				{item.availableActions.includes(AttentionAction.RETRY) && (
					renderActionButton(AttentionAction.RETRY, 'Retry', 'action-btn primary')
				)}

				{item.availableActions.includes(AttentionAction.RESOLVE) && (
					renderActionButton(AttentionAction.RESOLVE, 'Resolve', 'action-btn primary')
				)}
			</div>

			{/* Decision options for pending decisions */}
			{item.decisionOptions && item.decisionOptions.length > 0 && (
				<div className="decision-options">
					<h4>Choose an option:</h4>
					{item.decisionOptions.map((option) => (
						<button
							key={option.id}
							className={cn('decision-option', {
								recommended: option.recommended,
							})}
							onClick={() => void onAction(item, AttentionAction.APPROVE, option.id)}
							disabled={pendingAction !== undefined}
						>
							{option.label}
						</button>
					))}
				</div>
			)}
		</div>
	);
}

// QueueSection component
interface QueueSectionProps {
	summary: GetAttentionDashboardDataResponse['queueSummary'];
	collapsedState: CollapsedState;
	onTaskClick: (taskId: string) => void;
	onSwimlaneToggle: (initiativeId: string) => void;
}

function QueueSection({ summary, collapsedState, onTaskClick, onSwimlaneToggle }: QueueSectionProps) {
	if (!summary) {
		return <div className="empty-section">No queued tasks</div>;
	}

	return (
		<div className="queue-tasks">
			{/* Initiative swimlanes */}
			{summary.swimlanes.map((swimlane) => (
				<InitiativeSwimlaneCard
					key={swimlane.initiativeId}
					swimlane={swimlane}
					collapsed={collapsedState[swimlane.initiativeId] || false}
					onTaskClick={onTaskClick}
					onToggle={() => onSwimlaneToggle(swimlane.initiativeId)}
				/>
			))}

			{/* Unassigned tasks */}
			{summary.unassignedTasks.length > 0 && (
				<div className="unassigned-tasks">
					<h3>Unassigned Tasks</h3>
					<div className="task-list">
						{summary.unassignedTasks.map((task, index) => (
							<QueuedTaskCard
								key={task.id}
								task={task}
								position={index + 1}
								onClick={() => onTaskClick(task.id)}
							/>
						))}
					</div>
				</div>
			)}
		</div>
	);
}

// InitiativeSwimlaneCard component
interface InitiativeSwimlaneCardProps {
	swimlane: InitiativeSwimlane;
	collapsed: boolean;
	onTaskClick: (taskId: string) => void;
	onToggle: () => void;
}

function InitiativeSwimlaneCard({ swimlane, collapsed, onTaskClick, onToggle }: InitiativeSwimlaneCardProps) {
	return (
		<div className={cn('swimlane', { collapsed })}>
			<div
				className="swimlane-header"
				onClick={onToggle}
			>
				<div className="initiative-info">
					<h3>{swimlane.initiativeTitle}</h3>
					<span className="task-count">{swimlane.taskCount} tasks</span>
				</div>
				<div className="swimlane-controls">
					<span className="collapse-indicator">{collapsed ? '▶' : '▼'}</span>
				</div>
			</div>

			{!collapsed && (
				<div className="task-list">
					{swimlane.tasks.map((task) => (
						<QueuedTaskCard
							key={task.id}
							task={task}
							position={task.position}
							onClick={() => onTaskClick(task.id)}
						/>
					))}
				</div>
			)}
		</div>
	);
}

// QueuedTaskCard component
interface QueuedTaskCardProps {
	task: QueuedTask;
	position: number;
	onClick: () => void;
}

function QueuedTaskCard({ task, position, onClick }: QueuedTaskCardProps) {
	return (
		<div
			className={cn(
				'task-card',
				getCategoryClass(task.category),
				getPriorityClass(task.priority),
				{
					'high-priority': task.priority === TaskPriorityEnum.HIGH || task.priority === TaskPriorityEnum.CRITICAL,
				}
			)}
			onClick={onClick}
		>
			<div className="task-position">{position}</div>
			<div className="task-content">
				<div className="task-header">
					<span className="task-id">{task.id}</span>
					{(task.priority === TaskPriorityEnum.HIGH || task.priority === TaskPriorityEnum.CRITICAL) && (
						<span className={cn('priority-badge', getPriorityClass(task.priority))}>
							{getPriorityLabel(task.priority)}
						</span>
					)}
				</div>
				<h4 className="task-title">{task.title}</h4>
			</div>
		</div>
	);
}
