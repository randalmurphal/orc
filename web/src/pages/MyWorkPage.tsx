import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { attentionDashboardClient, projectClient } from '@/lib/client';
import { ProjectCard } from '@/components/dashboard/ProjectCard';
import { AttentionItemCard } from '@/components/dashboard/AttentionItemCard';
import { CommandCenterSection } from '@/components/dashboard/CommandCenterSection';
import { RunningTaskItem } from '@/components/dashboard/RunningTaskItem';
import { useDocumentTitle } from '@/hooks';
import { onAttentionDashboardSignal } from '@/lib/events/attentionDashboardSignals';
import { onRecommendationSignal } from '@/lib/events/recommendationSignals';
import { useProjectStore } from '@/stores/projectStore';
import { toast } from '@/stores';
import type { RecentCompletion } from '@/gen/orc/v1/dashboard_pb';
import type { AttentionItem, GetAttentionDashboardDataResponse } from '@/gen/orc/v1/attention_dashboard_pb';
import {
	AttentionAction,
	AttentionItemType,
} from '@/gen/orc/v1/attention_dashboard_pb';
import type { ProjectStatus } from '@/gen/orc/v1/project_pb';
import './MyWorkPage.css';

const refreshIntervalMs = 15_000;
const recentCompletionsDisplayLimit = 10;

interface ProjectRecentCompletion {
	projectId: string;
	projectName: string;
	completion: RecentCompletion;
}

interface LoadDataOptions {
	background?: boolean;
}

function isDiscussionItem(item: AttentionItem): boolean {
	return item.type === AttentionItemType.PENDING_DECISION
		|| item.signalKind === 'decision_request'
		|| item.signalKind === 'discussion_needed'
		|| item.signalKind === 'verification_summary';
}

function completionTimestamp(completion: RecentCompletion): number {
	if (!completion.completedAt) {
		return 0;
	}

	return Number(completion.completedAt.seconds) * 1000
		+ Math.floor(completion.completedAt.nanos / 1_000_000);
}

function formatCompletionTime(completion: RecentCompletion): string {
	if (!completion.completedAt) {
		return 'Unknown time';
	}

	return new Intl.DateTimeFormat(undefined, {
		hour: 'numeric',
		minute: '2-digit',
	}).format(new Date(completionTimestamp(completion)));
}

export function MyWorkPage() {
	useDocumentTitle('Command Center');

	const navigate = useNavigate();
	const [projects, setProjects] = useState<ProjectStatus[]>([]);
	const [dashboardData, setDashboardData] = useState<GetAttentionDashboardDataResponse | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [pendingActions, setPendingActions] = useState<Record<string, AttentionAction>>({});

	const hasLoadedRef = useRef(false);
	const requestIdRef = useRef(0);
	const backgroundRefreshInFlightRef = useRef(false);
	const backgroundRefreshQueuedRef = useRef(false);

	const loadData = useCallback(async (options: LoadDataOptions = {}): Promise<boolean> => {
		const background = options.background ?? false;
		const requestId = requestIdRef.current + 1;
		requestIdRef.current = requestId;
		const shouldShowLoading = !background || !hasLoadedRef.current;

		try {
			if (shouldShowLoading) {
				setLoading(true);
			}

			const [projectResponse, attentionResponse] = await Promise.all([
				projectClient.getAllProjectsStatus({}),
				attentionDashboardClient.getAttentionDashboardData({ projectId: '' }),
			]);

			if (requestId !== requestIdRef.current) {
				return false;
			}

			hasLoadedRef.current = true;
			setProjects(projectResponse.projects);
			setDashboardData(attentionResponse);
			setError(null);
			return true;
		} catch (err) {
			console.error('Failed to load command center data', err);
			if (requestId !== requestIdRef.current) {
				return false;
			}

			if (!background || !hasLoadedRef.current) {
				setError(err instanceof Error ? err.message : 'Failed to load command center');
			}
			return false;
		} finally {
			if (requestId === requestIdRef.current && shouldShowLoading) {
				setLoading(false);
			}
		}
	}, []);

	const requestBackgroundRefresh = useCallback(() => {
		if (backgroundRefreshInFlightRef.current) {
			backgroundRefreshQueuedRef.current = true;
			return;
		}

		backgroundRefreshInFlightRef.current = true;
		void loadData({ background: true }).finally(() => {
			backgroundRefreshInFlightRef.current = false;
			if (!backgroundRefreshQueuedRef.current) {
				return;
			}

			backgroundRefreshQueuedRef.current = false;
			requestBackgroundRefresh();
		});
	}, [loadData]);

	useEffect(() => {
		void loadData();
	}, [loadData]);

	useEffect(() => {
		const interval = globalThis.setInterval(() => {
			requestBackgroundRefresh();
		}, refreshIntervalMs);

		const handleFocus = () => requestBackgroundRefresh();
		const handleVisibilityChange = () => {
			if (!document.hidden) {
				requestBackgroundRefresh();
			}
		};

		const unsubscribeRecommendationSignals = onRecommendationSignal(() => {
			requestBackgroundRefresh();
		});
		const unsubscribeAttentionSignals = onAttentionDashboardSignal(() => {
			requestBackgroundRefresh();
		});

		window.addEventListener('focus', handleFocus);
		document.addEventListener('visibilitychange', handleVisibilityChange);

		return () => {
			globalThis.clearInterval(interval);
			window.removeEventListener('focus', handleFocus);
			document.removeEventListener('visibilitychange', handleVisibilityChange);
			unsubscribeRecommendationSignals();
			unsubscribeAttentionSignals();
		};
	}, [requestBackgroundRefresh]);

	const selectProject = useCallback((projectId: string) => {
		useProjectStore.getState().selectProject(projectId);
	}, []);

	const openTask = useCallback((projectId: string, taskId: string) => {
		selectProject(projectId);
		navigate(`/tasks/${taskId}`);
	}, [navigate, selectProject]);

	const openProjectBoard = useCallback((projectId: string) => {
		selectProject(projectId);
		navigate('/board');
	}, [navigate, selectProject]);

	const openAttentionItem = useCallback((item: AttentionItem) => {
		if (!item.projectId) {
			return;
		}

		if (item.taskId) {
			openTask(item.projectId, item.taskId);
			return;
		}

		openProjectBoard(item.projectId);
	}, [openProjectBoard, openTask]);

	const handleAttentionAction = useCallback(async (
		item: AttentionItem,
		action: AttentionAction,
		decisionOptionId?: string,
	) => {
		setPendingActions((current) => ({
			...current,
			[item.id]: action,
		}));

		try {
			const response = await attentionDashboardClient.performAttentionAction({
				projectId: item.projectId,
				attentionItemId: item.id,
				action,
				decisionOptionId: decisionOptionId ?? '',
			});

			if (!response.success) {
				throw new Error(response.errorMessage || 'Attention action failed');
			}

			const refreshed = await loadData({ background: true });
			if (!refreshed) {
				toast.warning('Action succeeded, but the command center did not refresh.');
			}
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Attention action failed');
		} finally {
			setPendingActions((current) => {
				const next = { ...current };
				delete next[item.id];
				return next;
			});
		}
	}, [loadData]);

	const runningTasks = dashboardData?.runningSummary?.tasks ?? [];
	const projectNamesByID = useMemo(() => {
		return new Map(projects.map((project) => [project.projectId, project.projectName]));
	}, [projects]);
	const attentionItems = useMemo(
		() => (dashboardData?.attentionItems ?? []).filter((item) => !isDiscussionItem(item)),
		[dashboardData],
	);
	const discussionItems = useMemo(
		() => (dashboardData?.attentionItems ?? []).filter(isDiscussionItem),
		[dashboardData],
	);

	const recommendationProjects = useMemo(() => {
		return [...projects]
			.filter((project) => project.pendingRecommendations > 0)
			.sort((left, right) => {
				if (left.pendingRecommendations === right.pendingRecommendations) {
					return left.projectName.localeCompare(right.projectName);
				}
				return right.pendingRecommendations - left.pendingRecommendations;
			});
	}, [projects]);

	const totalPendingRecommendations = dashboardData?.pendingRecommendations
		?? recommendationProjects.reduce((total, project) => total + project.pendingRecommendations, 0);

	const recentCompletions = useMemo<ProjectRecentCompletion[]>(() => {
		return projects
			.flatMap((project) => project.recentCompletions.map((completion) => ({
				projectId: project.projectId,
				projectName: project.projectName,
				completion,
			})))
			.sort((left, right) => completionTimestamp(right.completion) - completionTimestamp(left.completion))
			.slice(0, recentCompletionsDisplayLimit);
	}, [projects]);

	if (loading) {
		return (
			<div className="page-loader" role="progressbar">
				<div className="page-loader__spinner" />
			</div>
		);
	}

	if (error) {
		return (
			<div className="my-work-page__error">
				<p>Error loading command center</p>
				<p>{error}</p>
				<button type="button" onClick={() => void loadData()}>
					Retry
				</button>
			</div>
		);
	}

	if (projects.length === 0) {
		return (
			<div className="my-work-page__empty">
				<p>No projects found</p>
				<p>
					Run <code>orc init</code> in a project directory to get started.
				</p>
			</div>
		);
	}

	return (
		<div className="my-work-page">
			<header className="my-work-page__hero">
				<div>
					<p className="my-work-page__eyebrow">Operator control plane</p>
					<h1 className="my-work-page__title">Command Center</h1>
					<p className="my-work-page__subtitle">
						Cross-project triage for what is running, blocked, waiting for discussion, and ready for handoff.
					</p>
				</div>
				<div className="my-work-page__hero-stats">
					<div className="my-work-page__hero-stat">
						<span className="my-work-page__hero-value">{projects.length}</span>
						<span className="my-work-page__hero-label">Projects</span>
					</div>
					<div className="my-work-page__hero-stat">
						<span className="my-work-page__hero-value">{runningTasks.length}</span>
						<span className="my-work-page__hero-label">Running</span>
					</div>
					<div className="my-work-page__hero-stat">
						<span className="my-work-page__hero-value">{attentionItems.length + discussionItems.length}</span>
						<span className="my-work-page__hero-label">Signals</span>
					</div>
				</div>
			</header>

			<div className="my-work-page__sections">
				<CommandCenterSection
					title="Running"
					count={runningTasks.length}
					emptyState="No tasks running"
				>
					<div className="command-center-list">
						{runningTasks.map((task) => (
							<RunningTaskItem
								key={`${task.projectId}-${task.id}`}
								task={task}
								onOpen={openTask}
							/>
						))}
					</div>
				</CommandCenterSection>

				<CommandCenterSection
					title="Attention"
					count={attentionItems.length}
					emptyState="Nothing needs attention"
				>
					<div className="command-center-list">
						{attentionItems.map((item) => (
							<AttentionItemCard
								key={item.id}
								item={item}
								projectName={item.projectId ? projectNamesByID.get(item.projectId) ?? item.projectId : 'Unknown project'}
								pendingAction={pendingActions[item.id]}
								onOpen={openAttentionItem}
								onAction={handleAttentionAction}
							/>
						))}
					</div>
				</CommandCenterSection>

				<CommandCenterSection
					title="Discussions"
					count={discussionItems.length}
					emptyState="No active discussions"
				>
					<div className="command-center-list">
						{discussionItems.map((item) => (
							<AttentionItemCard
								key={item.id}
								item={item}
								projectName={item.projectId ? projectNamesByID.get(item.projectId) ?? item.projectId : 'Unknown project'}
								pendingAction={pendingActions[item.id]}
								onOpen={openAttentionItem}
								onAction={handleAttentionAction}
							/>
						))}
					</div>
				</CommandCenterSection>

				<CommandCenterSection
					title="Recommendations"
					count={totalPendingRecommendations}
					emptyState="No pending recommendations"
				>
					<div className="command-center-list">
						{recommendationProjects.map((project) => (
							<button
								key={project.projectId}
								type="button"
								className="command-center-summary-row"
								onClick={() => openProjectBoard(project.projectId)}
							>
								<span className="command-center-summary-row__title">{project.projectName}</span>
								<span className="command-center-summary-row__meta">
									{project.pendingRecommendations} pending
								</span>
							</button>
						))}
					</div>
				</CommandCenterSection>

				<CommandCenterSection
					title="Recently Completed"
					count={recentCompletions.length}
					emptyState="No recent completions"
				>
					<div className="command-center-list">
						{recentCompletions.map(({ projectId, projectName, completion }) => (
							<button
								key={`${projectId}-${completion.id}`}
								type="button"
								className="command-center-summary-row"
								onClick={() => openTask(projectId, completion.id)}
							>
								<span className="command-center-summary-row__content">
									<span className="command-center-summary-row__title">{completion.title}</span>
									<span className="command-center-summary-row__detail">
										{completion.id} · {projectName}
									</span>
								</span>
								<span className="command-center-summary-row__meta">
									{formatCompletionTime(completion)}
								</span>
							</button>
						))}
					</div>
				</CommandCenterSection>
			</div>

			<section className="my-work-page__projects-block" aria-label="Projects">
				<div className="my-work-page__projects-header">
					<h2 className="my-work-page__projects-title">Projects</h2>
					<p className="my-work-page__projects-copy">Project summaries stay secondary here. Open the board when you want project-scoped execution detail.</p>
				</div>
				<div className="my-work-page__projects">
					{projects.map((project) => (
						<ProjectCard
							key={project.projectId}
							project={project}
							onTaskClick={openTask}
							onViewAll={openProjectBoard}
						/>
					))}
				</div>
			</section>
		</div>
	);
}
