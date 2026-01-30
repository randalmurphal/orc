import { useEffect, useState, useCallback } from 'react';
import { useParams, useSearchParams, useNavigate } from 'react-router-dom';
import { create } from '@bufbuild/protobuf';
import { TaskHeader } from '@/components/task-detail/TaskHeader';
import { DependencySidebar } from '@/components/task-detail/DependencySidebar';
import { TabNav, type TabId } from '@/components/task-detail/TabNav';
import { TimelineTab } from '@/components/task-detail/TimelineTab';
import { ChangesTab } from '@/components/task-detail/ChangesTab';
import { TranscriptTab } from '@/components/task-detail/TranscriptTab';
import { TestResultsTab } from '@/components/task-detail/TestResultsTab';
import { AttachmentsTab } from '@/components/task-detail/AttachmentsTab';
import { CommentsTab } from '@/components/task-detail/CommentsTab';
import { ReviewFindingsTab } from '@/components/task-detail/ReviewFindingsTab';
import { Icon } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';
import { taskClient } from '@/lib/client';
import { useTaskSubscription, useDocumentTitle } from '@/hooks';
import { useTask as useStoreTask } from '@/stores/taskStore';
import { useCurrentProjectId } from '@/stores';
import type { Task, TaskPlan } from '@/gen/orc/v1/task_pb';
import { GetTaskRequestSchema, GetTaskPlanRequestSchema } from '@/gen/orc/v1/task_pb';
import './TaskDetail.css';

// Valid tab IDs
const VALID_TABS: TabId[] = ['timeline', 'changes', 'transcript', 'test-results', 'review-findings', 'attachments', 'comments'];

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
	const projectId = useCurrentProjectId();

	// Parse and validate tab from URL
	const tabParam = searchParams.get('tab') as TabId | null;
	const activeTab: TabId = tabParam && VALID_TABS.includes(tabParam) ? tabParam : 'timeline';

	// State
	const [task, setTask] = useState<Task | null>(null);

	// Set document title based on task
	useDocumentTitle(task ? `${task.id}: ${task.title}` : id);
	const [plan, setPlan] = useState<TaskPlan | null>(null);
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
				// Only update if status or currentPhase changed
				if (prev && (prev.status !== storeTask.status || prev.currentPhase !== storeTask.currentPhase)) {
					return { ...prev, status: storeTask.status, currentPhase: storeTask.currentPhase };
				}
				return prev;
			});
		}
	}, [storeTask]);

	// Load task data
	const loadTask = useCallback(async () => {
		if (!id || !projectId) return;

		setLoading(true);
		setError(null);

		try {
			const [taskResponse, planResponse] = await Promise.all([
				taskClient.getTask(create(GetTaskRequestSchema, { projectId, taskId: id })),
				taskClient.getTaskPlan(create(GetTaskPlanRequestSchema, { projectId, taskId: id })).catch(() => null),
			]);

			if (taskResponse.task) {
				setTask(taskResponse.task);
			}
			if (planResponse?.plan) {
				setPlan(planResponse.plan);
			}
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load task');
		} finally {
			setLoading(false);
		}
	}, [id, projectId]);

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
					<Button variant="secondary" onClick={loadTask}>
						Retry
					</Button>
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
				plan={plan ?? undefined}
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
								case 'review-findings':
									return <ReviewFindingsTab taskId={task.id} />;
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
