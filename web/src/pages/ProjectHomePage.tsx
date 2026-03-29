import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { attentionDashboardClient, dashboardClient } from '@/lib/client';
import { AttentionItemCard } from '@/components/dashboard/AttentionItemCard';
import { RunningTaskItem } from '@/components/dashboard/RunningTaskItem';
import { HandoffActions } from '@/components/handoff/HandoffActions';
import { useDocumentTitle } from '@/hooks';
import { onAttentionDashboardSignal } from '@/lib/events/attentionDashboardSignals';
import { onRecommendationSignal } from '@/lib/events/recommendationSignals';
import { listRecommendations } from '@/lib/api/recommendation';
import { recommendationKindLabel } from '@/lib/recommendations';
import { useCurrentProject, useCurrentProjectId, useProjectStore } from '@/stores/projectStore';
import { useThreadStore } from '@/stores/threadStore';
import { toast } from '@/stores';
import type { DashboardStats, RecentCompletion } from '@/gen/orc/v1/dashboard_pb';
import type { AttentionItem, GetAttentionDashboardDataResponse } from '@/gen/orc/v1/attention_dashboard_pb';
import {
	AttentionAction,
	AttentionItemType,
} from '@/gen/orc/v1/attention_dashboard_pb';
import type { Recommendation } from '@/gen/orc/v1/recommendation_pb';
import { RecommendationStatus } from '@/gen/orc/v1/recommendation_pb';
import { HandoffSourceType } from '@/gen/orc/v1/handoff_pb';
import './ProjectHomePage.css';

interface SectionState<T> {
	data: T;
	error: string | null;
	loading: boolean;
	hasLoaded: boolean;
}

interface LoadProjectHomeOptions {
	background?: boolean;
}

interface ProjectHomeSectionProps {
	title: string;
	count: number;
	emptyState: string;
	loading?: boolean;
	error?: string | null;
	onRetry?: () => void;
	action?: ReactNode;
	children?: ReactNode;
}

function createSectionState<T>(data: T, loading: boolean): SectionState<T> {
	return {
		data,
		error: null,
		loading,
		hasLoaded: false,
	};
}

function ProjectHomeSection({
	title,
	count,
	emptyState,
	loading = false,
	error = null,
	onRetry,
	action,
	children,
}: ProjectHomeSectionProps) {
	let body = children;
	if (loading) {
		body = <div className="project-home__section-state">Loading {title.toLowerCase()}...</div>;
	} else if (error) {
		body = (
			<div className="project-home__section-state project-home__section-state--error">
				<p>{error}</p>
				{onRetry ? (
					<button
						type="button"
						className="project-home__retry"
						onClick={onRetry}
					>
						Retry
					</button>
				) : null}
			</div>
		);
	} else if (count === 0) {
		body = <div className="project-home__section-state">{emptyState}</div>;
	}

	return (
		<section className="project-home__section" aria-label={title}>
			<header className="project-home__section-header">
				<div className="project-home__section-heading">
					<h2 className="project-home__section-title">{title}</h2>
					<span className="project-home__section-count">{count}</span>
				</div>
				{action ? <div className="project-home__section-action">{action}</div> : null}
			</header>
			<div className="project-home__section-body">{body}</div>
		</section>
	);
}

function isDiscussionAttentionItem(item: AttentionItem): boolean {
	return item.type === AttentionItemType.PENDING_DECISION
		|| item.signalKind === 'decision_request'
		|| item.signalKind === 'discussion_needed'
		|| item.signalKind === 'verification_summary';
}

function formatTimestamp(timestamp?: { seconds?: bigint; nanos?: number }): string {
	if (!timestamp?.seconds) {
		return 'Unknown time';
	}

	return new Intl.DateTimeFormat(undefined, {
		month: 'short',
		day: 'numeric',
		hour: 'numeric',
		minute: '2-digit',
	}).format(new Date(Number(timestamp.seconds) * 1000 + Math.floor((timestamp.nanos ?? 0) / 1_000_000)));
}

function completionBadge(completion: RecentCompletion): string {
	return completion.success ? 'Succeeded' : 'Needs review';
}

export function ProjectHomePage() {
	useDocumentTitle('Project Home');

	const navigate = useNavigate();
	const project = useCurrentProject();
	const projectId = useCurrentProjectId() ?? '';
	const threads = useThreadStore((state) => state.threads);
	const threadLoading = useThreadStore((state) => state.loading);
	const threadError = useThreadStore((state) => state.error);
	const selectThread = useThreadStore((state) => state.selectThread);
	const refreshThreadList = useThreadStore((state) => state.refreshThreadList);
	const [attentionState, setAttentionState] = useState<SectionState<GetAttentionDashboardDataResponse | null>>(
		() => createSectionState<GetAttentionDashboardDataResponse | null>(null, projectId !== '')
	);
	const [recommendationState, setRecommendationState] = useState<SectionState<Recommendation[]>>(
		() => createSectionState<Recommendation[]>([], projectId !== '')
	);
	const [completionState, setCompletionState] = useState<SectionState<DashboardStats | null>>(
		() => createSectionState<DashboardStats | null>(null, projectId !== '')
	);
	const [pendingActions, setPendingActions] = useState<Record<string, AttentionAction>>({});

	const currentProjectIdRef = useRef(projectId);
	const latestRequestIdRef = useRef(0);
	const backgroundRefreshInFlightRef = useRef(false);
	const backgroundRefreshQueuedRef = useRef(false);
	currentProjectIdRef.current = projectId;

	const isCurrentRequest = useCallback((requestId: number, requestProjectId: string) => {
		return latestRequestIdRef.current === requestId && currentProjectIdRef.current === requestProjectId;
	}, []);

	const loadProjectHomeData = useCallback(async (options: LoadProjectHomeOptions = {}): Promise<boolean> => {
		if (!projectId) {
			return false;
		}

		const requestId = latestRequestIdRef.current + 1;
		latestRequestIdRef.current = requestId;
		const requestProjectId = projectId;
		const isBackgroundRefresh = options.background ?? false;

		const markLoading = <T,>(setState: React.Dispatch<React.SetStateAction<SectionState<T>>>) => {
			setState((current) => ({
				...current,
				error: null,
				loading: !isBackgroundRefresh || !current.hasLoaded,
			}));
		};

		markLoading(setAttentionState);
		markLoading(setRecommendationState);
		markLoading(setCompletionState);

		const [attentionResult, recommendationResult, completionResult] = await Promise.allSettled([
			attentionDashboardClient.getAttentionDashboardData({ projectId: requestProjectId }),
			listRecommendations(requestProjectId, RecommendationStatus.PENDING),
			dashboardClient.getStats({ projectId: requestProjectId }),
		]);

		if (!isCurrentRequest(requestId, requestProjectId)) {
			return false;
		}

		setAttentionState((current) => {
			if (attentionResult.status === 'fulfilled') {
				return {
					data: attentionResult.value,
					error: null,
					loading: false,
					hasLoaded: true,
				};
			}

			console.error('Failed to load project attention data', attentionResult.reason);
			return {
				...current,
				error: attentionResult.reason instanceof Error ? attentionResult.reason.message : 'Failed to load running tasks and attention items',
				loading: false,
				hasLoaded: true,
			};
		});

		setRecommendationState((current) => {
			if (recommendationResult.status === 'fulfilled') {
				return {
					data: recommendationResult.value.recommendations,
					error: null,
					loading: false,
					hasLoaded: true,
				};
			}

			console.error('Failed to load project recommendations', recommendationResult.reason);
			return {
				...current,
				error: recommendationResult.reason instanceof Error ? recommendationResult.reason.message : 'Failed to load recommendations',
				loading: false,
				hasLoaded: true,
			};
		});

		setCompletionState((current) => {
			if (completionResult.status === 'fulfilled') {
				return {
					data: completionResult.value.stats ?? null,
					error: null,
					loading: false,
					hasLoaded: true,
				};
			}

			console.error('Failed to load recent completions', completionResult.reason);
			return {
				...current,
				error: completionResult.reason instanceof Error ? completionResult.reason.message : 'Failed to load recently completed tasks',
				loading: false,
				hasLoaded: true,
			};
		});

		return true;
	}, [isCurrentRequest, projectId]);

	const requestBackgroundRefresh = useCallback(() => {
		if (!projectId) {
			return;
		}

		if (backgroundRefreshInFlightRef.current) {
			backgroundRefreshQueuedRef.current = true;
			return;
		}

		backgroundRefreshInFlightRef.current = true;
		void loadProjectHomeData({ background: true }).finally(() => {
			backgroundRefreshInFlightRef.current = false;
			if (!backgroundRefreshQueuedRef.current) {
				return;
			}

			backgroundRefreshQueuedRef.current = false;
			requestBackgroundRefresh();
		});
	}, [loadProjectHomeData, projectId]);

	useEffect(() => {
		backgroundRefreshInFlightRef.current = false;
		backgroundRefreshQueuedRef.current = false;
		latestRequestIdRef.current += 1;
		setPendingActions({});
		setAttentionState(createSectionState<GetAttentionDashboardDataResponse | null>(null, projectId !== ''));
		setRecommendationState(createSectionState<Recommendation[]>([], projectId !== ''));
		setCompletionState(createSectionState<DashboardStats | null>(null, projectId !== ''));

		if (!projectId) {
			return;
		}

		void loadProjectHomeData();
	}, [loadProjectHomeData, projectId]);

	useEffect(() => {
		if (!projectId) {
			return;
		}

		const unsubscribeRecommendationSignals = onRecommendationSignal((signal) => {
			if (signal.projectId !== projectId) {
				return;
			}
			requestBackgroundRefresh();
		});

		const unsubscribeAttentionSignals = onAttentionDashboardSignal((signal) => {
			if (signal.projectId !== projectId) {
				return;
			}
			requestBackgroundRefresh();
		});

		return () => {
			unsubscribeRecommendationSignals();
			unsubscribeAttentionSignals();
		};
	}, [projectId, requestBackgroundRefresh]);

	const selectProject = useCallback((nextProjectId: string) => {
		useProjectStore.getState().selectProject(nextProjectId);
	}, []);

	const openTask = useCallback((nextProjectId: string, taskId: string) => {
		selectProject(nextProjectId);
		navigate(`/tasks/${taskId}`);
	}, [navigate, selectProject]);

	const handleRetryThreads = useCallback(() => {
		if (!projectId) {
			return;
		}
		void refreshThreadList(projectId);
	}, [projectId, refreshThreadList]);

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
				projectId,
				attentionItemId: item.id,
				action,
				decisionOptionId: decisionOptionId ?? '',
			});

			if (!response.success) {
				throw new Error(response.errorMessage || 'Attention action failed');
			}

			const refreshed = await loadProjectHomeData({ background: true });
			if (!refreshed) {
				toast.warning('Action succeeded, but the project home did not refresh.');
			}
		} catch (error) {
			toast.error(error instanceof Error ? error.message : 'Attention action failed');
		} finally {
			setPendingActions((current) => {
				const next = { ...current };
				delete next[item.id];
				return next;
			});
		}
	}, [loadProjectHomeData, projectId]);

	const runningTasks = attentionState.data?.runningSummary?.tasks ?? [];
	const attentionItems = useMemo(() => {
		return (attentionState.data?.attentionItems ?? []).filter((item) => !isDiscussionAttentionItem(item));
	}, [attentionState.data]);
	const recentCompletions = completionState.data?.recentCompletions ?? [];
	const projectThreads = threads.filter((thread) => thread.status !== 'archived');
	const projectRecommendations = recommendationState.data;

	if (!projectId) {
		return (
			<div className="project-home project-home--empty">
				<p>Select a project to open the project home.</p>
			</div>
		);
	}

	return (
		<div className="project-home">
			<header className="project-home__hero">
				<div className="project-home__hero-copy">
					<span className="project-home__eyebrow">Project home</span>
					<h1 className="project-home__title">{project?.name ?? projectId}</h1>
				</div>
				<div className="project-home__hero-stats">
					<span className="project-home__stat">{runningTasks.length} <span>running</span></span>
					<span className="project-home__stat">{attentionItems.length} <span>attention</span></span>
					<span className="project-home__stat">{projectRecommendations.length} <span>recommendations</span></span>
				</div>
			</header>

			<div className="project-home__grid">
				<ProjectHomeSection
					title="Running Tasks"
					count={runningTasks.length}
					emptyState="No tasks running"
					loading={attentionState.loading}
					error={attentionState.error}
					onRetry={() => void loadProjectHomeData()}
				>
					<div className="project-home__list">
						{runningTasks.map((task) => (
							<div key={task.id} className="project-home__row-with-actions">
								<RunningTaskItem task={task} onOpen={openTask} />
								<HandoffActions
									projectId={projectId}
									sourceType={HandoffSourceType.TASK}
									sourceId={task.id}
								/>
							</div>
						))}
					</div>
				</ProjectHomeSection>

				<ProjectHomeSection
					title="Needs Attention"
					count={attentionItems.length}
					emptyState="Nothing needs attention"
					loading={attentionState.loading}
					error={attentionState.error}
					onRetry={() => void loadProjectHomeData()}
				>
					<div className="project-home__list">
						{attentionItems.map((item) => (
							<AttentionItemCard
								key={item.id}
								item={item}
								projectName={project?.name ?? projectId}
								pendingAction={pendingActions[item.id]}
								onOpen={(attentionItem) => {
									if (!attentionItem.taskId) {
										return;
									}
									openTask(projectId, attentionItem.taskId);
								}}
								onAction={handleAttentionAction}
							/>
						))}
					</div>
				</ProjectHomeSection>

				<ProjectHomeSection
					title="Recommendations"
					count={projectRecommendations.length}
					emptyState="No pending recommendations"
					loading={recommendationState.loading}
					error={recommendationState.error}
					onRetry={() => void loadProjectHomeData()}
					action={
						<Link className="project-home__section-link" to="/recommendations">
							View all
						</Link>
					}
				>
					<div className="project-home__list">
						{projectRecommendations.map((recommendation) => (
							<article key={recommendation.id} className="project-home__recommendation">
								<div className="project-home__recommendation-body">
									<div className="project-home__recommendation-header">
										<span className="project-home__badge">{recommendationKindLabel(recommendation.kind)}</span>
										<span className="project-home__meta">{formatTimestamp(recommendation.createdAt)}</span>
									</div>
									<h3 className="project-home__item-title">{recommendation.title}</h3>
									<p className="project-home__item-copy">{recommendation.summary || 'No summary provided.'}</p>
									<p className="project-home__recommendation-action">
										{recommendation.proposedAction || 'Open the inbox for the next operator step.'}
									</p>
								</div>
								<HandoffActions
									projectId={projectId}
									sourceType={HandoffSourceType.RECOMMENDATION}
									sourceId={recommendation.id}
								/>
							</article>
						))}
					</div>
				</ProjectHomeSection>

				<ProjectHomeSection
					title="Discussions"
					count={projectThreads.length}
					emptyState="No active discussions"
					loading={threadLoading}
					error={threadError}
					onRetry={handleRetryThreads}
				>
					<div className="project-home__list">
						{projectThreads.map((thread) => (
							<button
								key={thread.id}
								type="button"
								className="project-home__thread-row"
								onClick={() => selectThread(thread.id)}
							>
								<span className="project-home__thread-title">{thread.title}</span>
								<span className="project-home__thread-meta">
									{thread.taskId ? `${thread.taskId} · ` : ''}
									{formatTimestamp(thread.updatedAt ?? thread.createdAt)}
								</span>
							</button>
						))}
					</div>
				</ProjectHomeSection>

				<ProjectHomeSection
					title="Recently Completed"
					count={recentCompletions.length}
					emptyState="No recent completions"
					loading={completionState.loading}
					error={completionState.error}
					onRetry={() => void loadProjectHomeData()}
				>
					<div className="project-home__list">
						{recentCompletions.map((completion) => (
							<button
								key={completion.id}
								type="button"
								className="project-home__completion-row"
								onClick={() => openTask(projectId, completion.id)}
							>
								<div>
									<div className="project-home__completion-title">{completion.title}</div>
									<div className="project-home__thread-meta">{completion.id}</div>
								</div>
								<div className="project-home__completion-meta">
									<span className="project-home__badge project-home__badge--success">{completionBadge(completion)}</span>
									<span className="project-home__meta">{formatTimestamp(completion.completedAt)}</span>
								</div>
							</button>
						))}
					</div>
				</ProjectHomeSection>
			</div>
		</div>
	);
}
