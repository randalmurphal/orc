/**
 * AttentionDashboard - New board design focused on attention management
 *
 * This component implements the UX Simplification redesign as an attention
 * management dashboard with three main sections:
 * - Running: Active tasks with progress and timing
 * - Needs Attention: Blocked tasks, decisions, gates requiring action
 * - Queue: Ready tasks organized by initiative
 */

import { useEffect, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { cn } from '@/lib/utils';
import './AttentionDashboard.css';
import { useCurrentProjectId } from '@/stores/projectStore';
import { createConnectTransport } from '@connectrpc/connect-web';
import { createClient } from '@connectrpc/connect';
import { AttentionDashboardService } from '@/gen/orc/v1/attention_dashboard_connect';
import type {
	GetAttentionDashboardDataResponse,
	RunningTask,
	AttentionItem,
	QueuedTask,
	InitiativeSwimlane,
} from '@/gen/orc/v1/attention_dashboard_pb';
import {
	AttentionAction,
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

// Create Connect client
const transport = createConnectTransport({
	baseUrl: '/api',
});
const client = createClient(AttentionDashboardService, transport);

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

	// Load dashboard data
	const loadDashboardData = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);

			const response = await client.getAttentionDashboardData({
				projectId,
			});

			setDashboardData(response);
		} catch (err) {
			console.error('Failed to load dashboard data:', err);
			setError(err instanceof Error ? err.message : 'Failed to load dashboard data');
		} finally {
			setLoading(false);
		}
	}, [projectId]);

	// Load data on mount and project change
	useEffect(() => {
		loadDashboardData();
	}, [loadDashboardData]);

	// Auto-refresh every 5 seconds for real-time updates
	useEffect(() => {
		const interval = setInterval(loadDashboardData, 5000);
		return () => clearInterval(interval);
	}, [loadDashboardData]);

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
				<button onClick={loadDashboardData}>Retry</button>
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
}

function AttentionSection({ items, onTaskClick }: AttentionSectionProps) {
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
				/>
			))}
		</div>
	);
}

// AttentionItemCard component
interface AttentionItemCardProps {
	item: AttentionItem;
	onTaskClick: (taskId: string) => void;
}

function AttentionItemCard({ item, onTaskClick }: AttentionItemCardProps) {
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
					>
						View
					</button>
				)}

				{item.availableActions.includes(AttentionAction.SKIP) && (
					<button className="action-btn secondary">
						Skip
					</button>
				)}

				{item.availableActions.includes(AttentionAction.FORCE) && (
					<button className="action-btn secondary">
						Force
					</button>
				)}

				{item.availableActions.includes(AttentionAction.APPROVE) && (
					<button className="action-btn primary">
						Approve
					</button>
				)}

				{item.availableActions.includes(AttentionAction.REJECT) && (
					<button className="action-btn secondary">
						Reject
					</button>
				)}

				{item.availableActions.includes(AttentionAction.RETRY) && (
					<button className="action-btn primary">
						Retry
					</button>
				)}

				{item.availableActions.includes(AttentionAction.RESOLVE) && (
					<button className="action-btn primary">
						Resolve
					</button>
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