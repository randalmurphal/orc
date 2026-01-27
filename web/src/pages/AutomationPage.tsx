/**
 * Automation page (/automation)
 *
 * Displays automation triggers, pending tasks, and execution history.
 * Provides controls for manual trigger execution and configuration.
 *
 * URL params:
 * - project: Project filter (handled by UrlParamSync)
 */

import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { useWsStatus, useTaskStore } from '@/stores';
import { Button, Icon, StatusIndicator } from '@/components/ui';
import { automationClient } from '@/lib/client';
import { useDocumentTitle } from '@/hooks';
import { create } from '@bufbuild/protobuf';
import {
	ListTriggersRequestSchema,
	GetTriggerHistoryRequestSchema,
	RunTriggerRequestSchema,
	SetTriggerEnabledRequestSchema,
	ResetTriggerRequestSchema,
	TriggerType,
	type Trigger,
	type TriggerExecution,
} from '@/gen/orc/v1/automation_pb';
import type { Task } from '@/gen/orc/v1/task_pb';
import { TaskStatus } from '@/gen/orc/v1/task_pb';
import { timestampToRelative } from '@/lib/time';
import './AutomationPage.css';

import {
	GetAutomationStatsRequestSchema,
	ListAutomationTasksRequestSchema,
	type AutomationStats,
} from '@/gen/orc/v1/automation_pb';

// Helpers
function getTriggerTypeLabel(type: TriggerType): string {
	switch (type) {
		case TriggerType.SCHEDULE:
			return 'Scheduled';
		case TriggerType.WEBHOOK:
			return 'Webhook';
		case TriggerType.FILE_WATCH:
			return 'File Watch';
		case TriggerType.GIT_HOOK:
			return 'Git Hook';
		case TriggerType.MANUAL:
			return 'Manual';
		default:
			return 'Unknown';
	}
}

function getTriggerTypeIcon(type: TriggerType): 'clock' | 'target' | 'zap' | 'activity' | 'calendar' {
	switch (type) {
		case TriggerType.SCHEDULE:
			return 'calendar';
		case TriggerType.WEBHOOK:
			return 'zap';
		case TriggerType.FILE_WATCH:
			return 'activity';
		case TriggerType.GIT_HOOK:
			return 'target';
		case TriggerType.MANUAL:
			return 'clock';
		default:
			return 'clock';
	}
}

// Get execution status string from proto boolean
function getExecutionStatus(exec: TriggerExecution): string {
	if (exec.success) return 'success';
	if (exec.error) return 'failed';
	return 'pending';
}

// Build a config object for display from proto trigger fields
function buildTriggerConfig(trigger: Trigger): object {
	return {
		condition: trigger.condition,
		action: trigger.action,
		cooldown: trigger.cooldown,
	};
}

export function AutomationPage() {
	useDocumentTitle('Automation');
	const navigate = useNavigate();
	const wsStatus = useWsStatus();
	const allTasks = useTaskStore((state) => state.tasks);

	// State
	const [triggers, setTriggers] = useState<Trigger[]>([]);
	const [automationTasks, setAutomationTasks] = useState<Task[]>([]);
	const [stats, setStats] = useState<AutomationStats | null>(null);
	const [selectedTrigger, setSelectedTrigger] = useState<string | null>(null);
	const [triggerHistory, setTriggerHistory] = useState<TriggerExecution[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [actionLoading, setActionLoading] = useState<string | null>(null);

	// Tab state
	const [activeTab, setActiveTab] = useState<'triggers' | 'tasks' | 'history'>('triggers');

	// Load data
	const loadData = useCallback(async () => {
		try {
			const [triggersResponse, tasksResponse, statsResponse] = await Promise.all([
				automationClient.listTriggers(create(ListTriggersRequestSchema, {})),
				automationClient.listAutomationTasks(create(ListAutomationTasksRequestSchema, {})),
				automationClient.getAutomationStats(create(GetAutomationStatsRequestSchema, {})),
			]);
			setTriggers(triggersResponse.triggers);
			// Filter allTasks by returned taskIds
			const taskIdSet = new Set(tasksResponse.taskIds);
			setAutomationTasks(allTasks.filter((t) => taskIdSet.has(t.id)));
			if (statsResponse.stats) {
				setStats(statsResponse.stats);
			}
			setLoading(false);
			setError(null);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load automation data');
			setLoading(false);
		}
	}, [allTasks]);

	// Load trigger history
	const loadHistory = useCallback(async (triggerId: string) => {
		try {
			const response = await automationClient.getTriggerHistory(
				create(GetTriggerHistoryRequestSchema, { triggerId })
			);
			setTriggerHistory(response.executions);
		} catch {
			// Silent fail for history
		}
	}, []);

	// Initial load
	useEffect(() => {
		loadData();
	}, [loadData]);

	// Reload on WebSocket reconnect
	useEffect(() => {
		if (wsStatus === 'connected') {
			loadData();
		}
	}, [wsStatus, loadData]);

	// Subscribe to task events
	// Subscribe to task changes via store subscription to refresh data
	useEffect(() => {
		const unsubscribe = useTaskStore.subscribe(
			(state) => state.tasks,
			() => {
				// Refresh automation data when tasks change
				loadData();
			}
		);
		return unsubscribe;
	}, [loadData]);

	// Load history when trigger selected
	useEffect(() => {
		if (selectedTrigger) {
			loadHistory(selectedTrigger);
		}
	}, [selectedTrigger, loadHistory]);

	// Handlers
	const handleRunTrigger = async (triggerId: string) => {
		setActionLoading(triggerId);
		try {
			const response = await automationClient.runTrigger(
				create(RunTriggerRequestSchema, { id: triggerId })
			);
			// Navigate to the created task
			if (response.execution?.taskId) {
				navigate(`/tasks/${response.execution.taskId}`);
			} else {
				loadData();
			}
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to run trigger');
		} finally {
			setActionLoading(null);
		}
	};

	const handleToggleTrigger = async (triggerId: string, enabled: boolean) => {
		setActionLoading(triggerId);
		try {
			await automationClient.setTriggerEnabled(
				create(SetTriggerEnabledRequestSchema, { id: triggerId, enabled })
			);
			setTriggers((prev: Trigger[]) =>
				prev.map((t: Trigger) => (t.id === triggerId ? { ...t, enabled } : t))
			);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to toggle trigger');
		} finally {
			setActionLoading(null);
		}
	};

	const handleResetTrigger = async (triggerId: string) => {
		setActionLoading(triggerId);
		try {
			await automationClient.resetTrigger(
				create(ResetTriggerRequestSchema, { id: triggerId })
			);
			loadData();
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to reset trigger');
		} finally {
			setActionLoading(null);
		}
	};

	// Filter automation tasks from all tasks (AUTO-* prefix)
	const autoTasks = automationTasks.length > 0
		? automationTasks
		: allTasks.filter((t) => t.id.startsWith('AUTO-'));

	const pendingTasks = autoTasks.filter((t) => t.status === TaskStatus.CREATED || t.status === TaskStatus.PLANNED);
	const runningTasks = autoTasks.filter((t) => t.status === TaskStatus.RUNNING);
	const completedTasks = autoTasks.filter((t) => t.status === TaskStatus.COMPLETED);
	const failedTasks = autoTasks.filter((t) => t.status === TaskStatus.FAILED);

	// Loading state
	if (loading) {
		return (
			<div className="automation-page">
				<div className="loading">
					<div className="spinner" />
					<span>Loading automation...</span>
				</div>
			</div>
		);
	}

	// Error state
	if (error && !stats) {
		return (
			<div className="automation-page">
				<div className="error-state">
					<Icon name="error" size={32} />
					<p>{error}</p>
					<Button onClick={loadData}>Retry</Button>
				</div>
			</div>
		);
	}

	return (
		<div className="automation-page">
			{/* Header */}
			<header className="automation-header">
				<div className="header-title">
					<Icon name="zap" size={24} />
					<h1>Automation</h1>
				</div>
				<div className="header-stats">
					{stats && (
						<>
							<div className="stat">
								<span className="stat-value">{stats.enabledTriggers}</span>
								<span className="stat-label">Active Triggers</span>
							</div>
							<div className="stat">
								<span className="stat-value">{pendingTasks.length}</span>
								<span className="stat-label">Pending</span>
							</div>
							<div className="stat">
								<span className="stat-value">{runningTasks.length}</span>
								<span className="stat-label">Running</span>
							</div>
							<div className="stat">
								<span className="stat-value">{stats.tasksCreated}</span>
								<span className="stat-label">Created</span>
							</div>
						</>
					)}
				</div>
			</header>

			{/* Error banner */}
			{error && (
				<div className="error-banner">
					<Icon name="error" size={16} />
					<span>{error}</span>
					<Button variant="ghost" size="sm" onClick={() => setError(null)}>
						Dismiss
					</Button>
				</div>
			)}

			{/* Tab navigation */}
			<nav className="automation-tabs">
				<button
					className={`tab-btn ${activeTab === 'triggers' ? 'active' : ''}`}
					onClick={() => setActiveTab('triggers')}
				>
					<Icon name="zap" size={16} />
					Triggers
					<span className="tab-count">{triggers.length}</span>
				</button>
				<button
					className={`tab-btn ${activeTab === 'tasks' ? 'active' : ''}`}
					onClick={() => setActiveTab('tasks')}
				>
					<Icon name="tasks" size={16} />
					Tasks
					<span className="tab-count">{autoTasks.length}</span>
				</button>
				<button
					className={`tab-btn ${activeTab === 'history' ? 'active' : ''}`}
					onClick={() => setActiveTab('history')}
				>
					<Icon name="clock" size={16} />
					History
				</button>
			</nav>

			{/* Tab content */}
			<div className="automation-content">
				{activeTab === 'triggers' && (
					<div className="triggers-section">
						{triggers.length === 0 ? (
							<div className="empty-state">
								<Icon name="zap" size={48} />
								<h3>No triggers configured</h3>
								<p>Configure automation triggers in your .orc/config.yaml file.</p>
							</div>
						) : (
							<div className="triggers-list">
								{triggers.map((trigger) => (
									<div
										key={trigger.id}
										className={`trigger-card ${trigger.enabled ? '' : 'disabled'} ${selectedTrigger === trigger.id ? 'selected' : ''}`}
										onClick={() => setSelectedTrigger(selectedTrigger === trigger.id ? null : trigger.id)}
									>
										<div className="trigger-header">
											<div className="trigger-icon">
												<Icon name={getTriggerTypeIcon(trigger.type)} size={20} />
											</div>
											<div className="trigger-info">
												<h3 className="trigger-title">{trigger.id}</h3>
												<p className="trigger-description">{trigger.description}</p>
											</div>
											<div className="trigger-actions">
												<Button
													variant="ghost"
													size="sm"
													iconOnly
													aria-label={trigger.enabled ? 'Disable trigger' : 'Enable trigger'}
													onClick={(e) => {
														e.stopPropagation();
														handleToggleTrigger(trigger.id, !trigger.enabled);
													}}
													loading={actionLoading === trigger.id}
												>
													<Icon name={trigger.enabled ? 'pause' : 'play'} size={16} />
												</Button>
												<Button
													variant="primary"
													size="sm"
													onClick={(e) => {
														e.stopPropagation();
														handleRunTrigger(trigger.id);
													}}
													loading={actionLoading === trigger.id}
													disabled={!trigger.enabled}
												>
													Run Now
												</Button>
											</div>
										</div>

										<div className="trigger-meta">
											<span className="trigger-type">
												<Icon name={getTriggerTypeIcon(trigger.type)} size={12} />
												{getTriggerTypeLabel(trigger.type)}
											</span>
											<span className="trigger-stat">
												<Icon name="target" size={12} />
												{trigger.triggerCount} executions
											</span>
											<span className="trigger-stat">
												<Icon name="clock" size={12} />
												Last: {timestampToRelative(trigger.lastTriggeredAt)}
											</span>
										</div>

										{/* Expanded details */}
										{selectedTrigger === trigger.id && (
											<div className="trigger-details">
												<div className="detail-section">
													<h4>Configuration</h4>
													<pre className="config-preview">
														{JSON.stringify(buildTriggerConfig(trigger), null, 2)}
													</pre>
												</div>
												<div className="detail-actions">
													<Button
														variant="secondary"
														size="sm"
														onClick={(e) => {
															e.stopPropagation();
															handleResetTrigger(trigger.id);
														}}
														loading={actionLoading === trigger.id}
													>
														<Icon name="refresh" size={14} />
														Reset Counter
													</Button>
												</div>
												{triggerHistory.length > 0 && (
													<div className="detail-section">
														<h4>Recent Executions</h4>
														<div className="history-list">
															{triggerHistory.slice(0, 5).map((exec) => (
																<div key={exec.id} className="history-item">
																	<span className={`execution-status status-${getExecutionStatus(exec)}`}>
																		{getExecutionStatus(exec)}
																	</span>
																	<span className="execution-task">
																		{exec.taskId ? (
																			<a
																				href={`/tasks/${exec.taskId}`}
																				onClick={(e) => {
																					e.preventDefault();
																					e.stopPropagation();
																					navigate(`/tasks/${exec.taskId}`);
																				}}
																			>
																				{exec.taskId}
																			</a>
																		) : (
																			'—'
																		)}
																	</span>
																	<span className="execution-time">
																		{timestampToRelative(exec.executedAt)}
																	</span>
																</div>
															))}
														</div>
													</div>
												)}
											</div>
										)}
									</div>
								))}
							</div>
						)}
					</div>
				)}

				{activeTab === 'tasks' && (
					<div className="tasks-section">
						{/* Pending tasks */}
						{pendingTasks.length > 0 && (
							<div className="task-group">
								<h3 className="group-title">
									<Icon name="clock" size={16} />
									Pending Approval
									<span className="group-count">{pendingTasks.length}</span>
								</h3>
								<div className="task-list">
									{pendingTasks.map((task) => (
										<div
											key={task.id}
											className="task-item"
											onClick={() => navigate(`/tasks/${task.id}`)}
										>
											<StatusIndicator status={task.status} size="sm" />
											<span className="task-id">{task.id}</span>
											<span className="task-title">{task.title}</span>
											<span className="task-time">{timestampToRelative(task.createdAt)}</span>
										</div>
									))}
								</div>
							</div>
						)}

						{/* Running tasks */}
						{runningTasks.length > 0 && (
							<div className="task-group">
								<h3 className="group-title">
									<Icon name="play" size={16} />
									Running
									<span className="group-count">{runningTasks.length}</span>
								</h3>
								<div className="task-list">
									{runningTasks.map((task) => (
										<div
											key={task.id}
											className="task-item running"
											onClick={() => navigate(`/tasks/${task.id}`)}
										>
											<StatusIndicator status={task.status} size="sm" />
											<span className="task-id">{task.id}</span>
											<span className="task-title">{task.title}</span>
											<span className="task-phase">{task.currentPhase}</span>
										</div>
									))}
								</div>
							</div>
						)}

						{/* Completed tasks */}
						{completedTasks.length > 0 && (
							<div className="task-group">
								<h3 className="group-title">
									<Icon name="check" size={16} />
									Completed
									<span className="group-count">{completedTasks.length}</span>
								</h3>
								<div className="task-list">
									{completedTasks.slice(0, 10).map((task) => (
										<div
											key={task.id}
											className="task-item completed"
											onClick={() => navigate(`/tasks/${task.id}`)}
										>
											<StatusIndicator status={task.status} size="sm" />
											<span className="task-id">{task.id}</span>
											<span className="task-title">{task.title}</span>
											<span className="task-time">{timestampToRelative(task.updatedAt)}</span>
										</div>
									))}
								</div>
							</div>
						)}

						{/* Failed tasks */}
						{failedTasks.length > 0 && (
							<div className="task-group">
								<h3 className="group-title failed">
									<Icon name="error" size={16} />
									Failed
									<span className="group-count">{failedTasks.length}</span>
								</h3>
								<div className="task-list">
									{failedTasks.map((task) => (
										<div
											key={task.id}
											className="task-item failed"
											onClick={() => navigate(`/tasks/${task.id}`)}
										>
											<StatusIndicator status={task.status} size="sm" />
											<span className="task-id">{task.id}</span>
											<span className="task-title">{task.title}</span>
											<span className="task-time">{timestampToRelative(task.updatedAt)}</span>
										</div>
									))}
								</div>
							</div>
						)}

						{/* Empty state */}
						{autoTasks.length === 0 && (
							<div className="empty-state">
								<Icon name="tasks" size={48} />
								<h3>No automation tasks</h3>
								<p>Automation tasks will appear here when triggers fire.</p>
							</div>
						)}
					</div>
				)}

				{activeTab === 'history' && (
					<div className="history-section">
						<p className="history-hint">
							Select a trigger from the Triggers tab to view its execution history.
						</p>
						{selectedTrigger && triggerHistory.length > 0 && (
							<div className="history-table">
								<table>
									<thead>
										<tr>
											<th>Status</th>
											<th>Task</th>
											<th>Error</th>
											<th>Executed</th>
										</tr>
									</thead>
									<tbody>
										{triggerHistory.map((exec) => (
											<tr key={exec.id}>
												<td>
													<span className={`execution-status status-${getExecutionStatus(exec)}`}>
														{getExecutionStatus(exec)}
													</span>
												</td>
												<td>
													{exec.taskId ? (
														<a
															href={`/tasks/${exec.taskId}`}
															onClick={(e) => {
																e.preventDefault();
																navigate(`/tasks/${exec.taskId}`);
															}}
														>
															{exec.taskId}
														</a>
													) : (
														'—'
													)}
												</td>
												<td className="reason-cell">{exec.error || '—'}</td>
												<td>{timestampToRelative(exec.executedAt)}</td>
											</tr>
										))}
									</tbody>
								</table>
							</div>
						)}
						{selectedTrigger && triggerHistory.length === 0 && (
							<div className="empty-state">
								<Icon name="clock" size={48} />
								<h3>No execution history</h3>
								<p>This trigger hasn't been executed yet.</p>
							</div>
						)}
						{!selectedTrigger && (
							<div className="empty-state">
								<Icon name="clock" size={48} />
								<h3>No trigger selected</h3>
								<p>Select a trigger from the Triggers tab to view its history.</p>
							</div>
						)}
					</div>
				)}
			</div>
		</div>
	);
}
