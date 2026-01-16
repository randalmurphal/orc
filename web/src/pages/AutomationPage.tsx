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
import { useWebSocket } from '@/hooks/useWebSocket';
import { Button, Icon, StatusIndicator } from '@/components/ui';
import type { Task } from '@/lib/types';
import './AutomationPage.css';

// Types for automation data
interface Trigger {
	id: string;
	type: string;
	description: string;
	enabled: boolean;
	config: TriggerConfig;
	last_triggered_at: string | null;
	trigger_count: number;
	created_at: string;
}

interface TriggerConfig {
	metric?: string;
	threshold?: number;
	event?: string;
	operator?: string;
	value?: number;
	weights?: string[];
	categories?: string[];
	filter?: Record<string, unknown>;
}

interface TriggerExecution {
	id: number;
	trigger_id: string;
	task_id: string | null;
	triggered_at: string;
	trigger_reason: string;
	status: string;
	completed_at: string | null;
	error_message: string | null;
}

interface AutomationStats {
	total_triggers: number;
	enabled_triggers: number;
	pending_tasks: number;
	running_tasks: number;
	completed_today: number;
	failed_today: number;
}

// API functions
async function listTriggers(): Promise<Trigger[]> {
	const res = await fetch('/api/automation/triggers');
	if (!res.ok) throw new Error('Failed to fetch triggers');
	const data = await res.json();
	return data.triggers || [];
}

async function getTriggerHistory(triggerId: string, limit = 10): Promise<TriggerExecution[]> {
	const res = await fetch(`/api/automation/triggers/${triggerId}/history?limit=${limit}`);
	if (!res.ok) throw new Error('Failed to fetch history');
	const data = await res.json();
	return data.executions || [];
}

async function runTrigger(triggerId: string): Promise<{ task_id: string }> {
	const res = await fetch(`/api/automation/triggers/${triggerId}/run`, {
		method: 'POST',
	});
	if (!res.ok) {
		const error = await res.json().catch(() => ({ error: 'Failed to run trigger' }));
		throw new Error(error.error || 'Failed to run trigger');
	}
	return res.json();
}

async function toggleTrigger(triggerId: string, enabled: boolean): Promise<void> {
	const res = await fetch(`/api/automation/triggers/${triggerId}`, {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ enabled }),
	});
	if (!res.ok) throw new Error('Failed to update trigger');
}

async function resetTrigger(triggerId: string): Promise<void> {
	const res = await fetch(`/api/automation/triggers/${triggerId}/reset`, {
		method: 'POST',
	});
	if (!res.ok) throw new Error('Failed to reset trigger');
}

async function getAutomationStats(): Promise<AutomationStats> {
	const res = await fetch('/api/automation/stats');
	if (!res.ok) throw new Error('Failed to fetch stats');
	return res.json();
}

async function listAutomationTasks(): Promise<Task[]> {
	const res = await fetch('/api/automation/tasks');
	if (!res.ok) throw new Error('Failed to fetch automation tasks');
	const data = await res.json();
	return data.tasks || [];
}

// Helpers
function formatRelativeTime(dateStr: string | null): string {
	if (!dateStr) return 'Never';
	const date = new Date(dateStr);
	const now = new Date();
	const diffMs = now.getTime() - date.getTime();
	const diffMins = Math.floor(diffMs / 60000);
	const diffHours = Math.floor(diffMs / 3600000);
	const diffDays = Math.floor(diffMs / 86400000);

	if (diffMins < 1) return 'just now';
	if (diffMins < 60) return `${diffMins}m ago`;
	if (diffHours < 24) return `${diffHours}h ago`;
	return `${diffDays}d ago`;
}

function getTriggerTypeLabel(type: string): string {
	switch (type) {
		case 'count':
			return 'Count-based';
		case 'initiative':
			return 'Initiative';
		case 'event':
			return 'Event';
		case 'threshold':
			return 'Threshold';
		case 'schedule':
			return 'Scheduled';
		default:
			return type;
	}
}

function getTriggerTypeIcon(type: string): 'clock' | 'target' | 'zap' | 'activity' | 'calendar' {
	switch (type) {
		case 'count':
			return 'target';
		case 'initiative':
			return 'target';
		case 'event':
			return 'zap';
		case 'threshold':
			return 'activity';
		case 'schedule':
			return 'calendar';
		default:
			return 'clock';
	}
}

export function AutomationPage() {
	const navigate = useNavigate();
	const wsStatus = useWsStatus();
	const { on } = useWebSocket();
	const allTasks = useTaskStore((state: { tasks: Task[] }) => state.tasks);

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
			const [triggersData, tasksData, statsData] = await Promise.all([
				listTriggers(),
				listAutomationTasks(),
				getAutomationStats(),
			]);
			setTriggers(triggersData);
			setAutomationTasks(tasksData);
			setStats(statsData);
			setLoading(false);
			setError(null);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load automation data');
			setLoading(false);
		}
	}, []);

	// Load trigger history
	const loadHistory = useCallback(async (triggerId: string) => {
		try {
			const history = await getTriggerHistory(triggerId);
			setTriggerHistory(history);
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
	useEffect(() => {
		const unsubscribe = on('all', (event) => {
			if (
				'event' in event &&
				['task_created', 'task_updated', 'task_deleted'].includes(event.event)
			) {
				loadData();
			}
		});
		return unsubscribe;
	}, [on, loadData]);

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
			const result = await runTrigger(triggerId);
			// Navigate to the created task
			if (result.task_id) {
				navigate(`/tasks/${result.task_id}`);
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
			await toggleTrigger(triggerId, enabled);
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
			await resetTrigger(triggerId);
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
		: allTasks.filter((t: Task) => t.id.startsWith('AUTO-'));

	const pendingTasks = autoTasks.filter((t: Task) => t.status === 'created' || t.status === 'planned');
	const runningTasks = autoTasks.filter((t: Task) => t.status === 'running');
	const completedTasks = autoTasks.filter((t: Task) => t.status === 'completed' || t.status === 'finished');
	const failedTasks = autoTasks.filter((t: Task) => t.status === 'failed');

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
								<span className="stat-value">{stats.enabled_triggers}</span>
								<span className="stat-label">Active Triggers</span>
							</div>
							<div className="stat">
								<span className="stat-value">{stats.pending_tasks}</span>
								<span className="stat-label">Pending</span>
							</div>
							<div className="stat">
								<span className="stat-value">{stats.running_tasks}</span>
								<span className="stat-label">Running</span>
							</div>
							<div className="stat">
								<span className="stat-value">{stats.completed_today}</span>
								<span className="stat-label">Today</span>
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
												{trigger.trigger_count} executions
											</span>
											<span className="trigger-stat">
												<Icon name="clock" size={12} />
												Last: {formatRelativeTime(trigger.last_triggered_at)}
											</span>
										</div>

										{/* Expanded details */}
										{selectedTrigger === trigger.id && (
											<div className="trigger-details">
												<div className="detail-section">
													<h4>Configuration</h4>
													<pre className="config-preview">
														{JSON.stringify(trigger.config, null, 2)}
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
																	<span className={`execution-status status-${exec.status}`}>
																		{exec.status}
																	</span>
																	<span className="execution-task">
																		{exec.task_id ? (
																			<a
																				href={`/tasks/${exec.task_id}`}
																				onClick={(e) => {
																					e.preventDefault();
																					e.stopPropagation();
																					navigate(`/tasks/${exec.task_id}`);
																				}}
																			>
																				{exec.task_id}
																			</a>
																		) : (
																			'—'
																		)}
																	</span>
																	<span className="execution-time">
																		{formatRelativeTime(exec.triggered_at)}
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
											<span className="task-time">{formatRelativeTime(task.created_at)}</span>
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
											<span className="task-phase">{task.current_phase}</span>
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
											<span className="task-time">{formatRelativeTime(task.updated_at)}</span>
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
											<span className="task-time">{formatRelativeTime(task.updated_at)}</span>
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
											<th>Reason</th>
											<th>Started</th>
											<th>Completed</th>
										</tr>
									</thead>
									<tbody>
										{triggerHistory.map((exec) => (
											<tr key={exec.id}>
												<td>
													<span className={`execution-status status-${exec.status}`}>
														{exec.status}
													</span>
												</td>
												<td>
													{exec.task_id ? (
														<a
															href={`/tasks/${exec.task_id}`}
															onClick={(e) => {
																e.preventDefault();
																navigate(`/tasks/${exec.task_id}`);
															}}
														>
															{exec.task_id}
														</a>
													) : (
														'—'
													)}
												</td>
												<td className="reason-cell">{exec.trigger_reason}</td>
												<td>{formatRelativeTime(exec.triggered_at)}</td>
												<td>{exec.completed_at ? formatRelativeTime(exec.completed_at) : '—'}</td>
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
