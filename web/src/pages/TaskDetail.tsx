import { useEffect, useState, useCallback } from 'react';
import { useParams, useSearchParams, useNavigate } from 'react-router-dom';
import { TaskHeader } from '@/components/task-detail/TaskHeader';
import { DependencySidebar } from '@/components/task-detail/DependencySidebar';
import { TabNav, type TabId } from '@/components/task-detail/TabNav';
import { TimelineTab } from '@/components/task-detail/TimelineTab';
import { ChangesTab } from '@/components/task-detail/ChangesTab';
import { TranscriptTab } from '@/components/task-detail/TranscriptTab';
import { TestResultsTab } from '@/components/task-detail/TestResultsTab';
import { AttachmentsTab } from '@/components/task-detail/AttachmentsTab';
import { CommentsTab } from '@/components/task-detail/CommentsTab';
import { Icon } from '@/components/ui/Icon';
import { getTask, getTaskPlan } from '@/lib/api';
import { useTaskSubscription } from '@/hooks';
import { useTask as useStoreTask } from '@/stores/taskStore';
import type { Task, Plan } from '@/lib/types';
import './TaskDetail.css';

// Valid tab IDs
const VALID_TABS: TabId[] = ['timeline', 'changes', 'transcript', 'test-results', 'attachments', 'comments'];

/**
 * Task detail page (/tasks/:id)
 *
 * Route params:
 * - id: Task ID
 *
 * URL params:
 * - tab: Active tab (timeline, changes, transcript, test-results, attachments, comments)
 */
export function TaskDetail() {
	const { id } = useParams<{ id: string }>();
	const [searchParams, setSearchParams] = useSearchParams();
	const navigate = useNavigate();

	// Parse and validate tab from URL
	const tabParam = searchParams.get('tab') as TabId | null;
	const activeTab: TabId = tabParam && VALID_TABS.includes(tabParam) ? tabParam : 'timeline';

	// State
	const [task, setTask] = useState<Task | null>(null);
	const [plan, setPlan] = useState<Plan | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [sidebarCollapsed, setSidebarCollapsed] = useState(false);

	// Subscribe to real-time updates - returns state from the store
	const { state: taskState, transcript: streamingTranscript } = useTaskSubscription(id);

	// Get task from store (updated by WebSocket events)
	const storeTask = useStoreTask(id ?? '');

	// Sync local task state with store task when WebSocket updates arrive
	// This ensures the UI reflects real-time status changes (e.g., running -> completed)
	useEffect(() => {
		if (storeTask) {
			setTask((prev) => {
				// Only update if status or current_phase changed
				if (prev && (prev.status !== storeTask.status || prev.current_phase !== storeTask.current_phase)) {
					return { ...prev, status: storeTask.status, current_phase: storeTask.current_phase };
				}
				return prev;
			});
		}
	}, [storeTask]);

	// Load task data
	const loadTask = useCallback(async () => {
		if (!id) return;

		setLoading(true);
		setError(null);

		try {
			const [taskData, planData] = await Promise.all([
				getTask(id),
				getTaskPlan(id).catch(() => null),
			]);

			setTask(taskData);
			setPlan(planData);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load task');
		} finally {
			setLoading(false);
		}
	}, [id]);

	// Initial load
	useEffect(() => {
		loadTask();
	}, [loadTask]);

	// Handle tab change
	const handleTabChange = useCallback((tabId: TabId) => {
		setSearchParams({ tab: tabId }, { replace: true });
	}, [setSearchParams]);

	// Handle task update (from edit)
	const handleTaskUpdate = useCallback((updatedTask: Task) => {
		setTask(updatedTask);
	}, []);

	// Handle task delete
	const handleTaskDelete = useCallback(() => {
		navigate('/board');
	}, [navigate]);

	// Loading state
	if (loading) {
		return (
			<div className="task-detail-page">
				<div className="task-detail-loading">
					<div className="loading-spinner" />
					<span>Loading task...</span>
				</div>
			</div>
		);
	}

	// Error state
	if (error || !task) {
		return (
			<div className="task-detail-page">
				<div className="task-detail-error">
					<Icon name="alert-circle" size={32} />
					<h2>Failed to load task</h2>
					<p>{error || 'Task not found'}</p>
					<button className="retry-btn" onClick={loadTask}>
						Retry
					</button>
				</div>
			</div>
		);
	}

	// Get phases for comments tab
	const phases = plan?.phases.map(p => p.name) ?? [];

	return (
		<div className="task-detail-page">
			<TaskHeader
				task={task}
				onTaskUpdate={handleTaskUpdate}
				onTaskDelete={handleTaskDelete}
			/>

			<div className="task-detail-content">
				{/* Dependency Sidebar */}
				<DependencySidebar
					task={task}
					collapsed={sidebarCollapsed}
					onToggle={() => setSidebarCollapsed(!sidebarCollapsed)}
				/>

				{/* Main content area */}
				<div className="task-detail-main">
					<TabNav
						activeTab={activeTab}
						onTabChange={handleTabChange}
					>
						{(tabId) => {
							switch (tabId) {
								case 'timeline':
									return (
										<TimelineTab
											task={task}
											taskState={taskState ?? null}
											plan={plan}
										/>
									);
								case 'changes':
									return <ChangesTab taskId={task.id} />;
								case 'transcript':
									return <TranscriptTab taskId={task.id} streamingLines={streamingTranscript} />;
								case 'test-results':
									return <TestResultsTab taskId={task.id} />;
								case 'attachments':
									return <AttachmentsTab taskId={task.id} />;
								case 'comments':
									return <CommentsTab taskId={task.id} phases={phases} />;
								default:
									return null;
							}
						}}
					</TabNav>
				</div>
			</div>
		</div>
	);
}
