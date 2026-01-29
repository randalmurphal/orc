/**
 * InitiativesView container component - assembles the complete initiatives
 * overview page with aggregate statistics, initiative cards grid, and
 * proper empty/loading/error states.
 */

import { useState, useEffect, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTaskStore } from '@/stores';
import { initiativeClient } from '@/lib/client';
import { create } from '@bufbuild/protobuf';
import type { Initiative } from '@/gen/orc/v1/initiative_pb';
import { InitiativeStatus, ListInitiativesRequestSchema } from '@/gen/orc/v1/initiative_pb';
import { TaskStatus } from '@/gen/orc/v1/task_pb';
import { StatsRow, type InitiativeStats } from './StatsRow';
import { InitiativeCard } from './InitiativeCard';
import { NewInitiativeModal } from '@/components/overlays/NewInitiativeModal';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import './InitiativesView.css';

// =============================================================================
// Types
// =============================================================================

export interface InitiativesViewProps {
	className?: string;
}

interface ProgressData {
	completed: number;
	total: number;
}

// =============================================================================
// Skeleton Components
// =============================================================================

function InitiativeCardSkeleton() {
	return (
		<article className="initiatives-view-card-skeleton" aria-hidden="true">
			<div className="initiatives-view-card-skeleton-header">
				<div className="initiatives-view-card-skeleton-icon" />
				<div className="initiatives-view-card-skeleton-info">
					<div className="initiatives-view-card-skeleton-title" />
					<div className="initiatives-view-card-skeleton-desc" />
				</div>
				<div className="initiatives-view-card-skeleton-badge" />
			</div>
			<div className="initiatives-view-card-skeleton-progress">
				<div className="initiatives-view-card-skeleton-progress-header">
					<div className="initiatives-view-card-skeleton-label" />
					<div className="initiatives-view-card-skeleton-value" />
				</div>
				<div className="initiatives-view-card-skeleton-bar" />
			</div>
			<div className="initiatives-view-card-skeleton-meta">
				<div className="initiatives-view-card-skeleton-meta-item" />
				<div className="initiatives-view-card-skeleton-meta-item" />
				<div className="initiatives-view-card-skeleton-meta-item" />
			</div>
		</article>
	);
}

function InitiativesViewSkeleton() {
	return (
		<div className="initiatives-view-grid" aria-busy="true" aria-label="Loading initiatives">
			<InitiativeCardSkeleton />
			<InitiativeCardSkeleton />
			<InitiativeCardSkeleton />
			<InitiativeCardSkeleton />
		</div>
	);
}

// =============================================================================
// Empty State
// =============================================================================

function InitiativesViewEmpty() {
	return (
		<div className="initiatives-view-empty" role="status">
			<div className="initiatives-view-empty-icon">
				<Icon name="layers" size={32} />
			</div>
			<h2 className="initiatives-view-empty-title">Create your first initiative</h2>
			<p className="initiatives-view-empty-desc">
				Initiatives help you organize related tasks into cohesive projects with shared vision and
				decisions.
			</p>
		</div>
	);
}

// =============================================================================
// Error State
// =============================================================================

interface InitiativesViewErrorProps {
	error: string;
	onRetry: () => void;
}

function InitiativesViewError({ error, onRetry }: InitiativesViewErrorProps) {
	return (
		<div className="initiatives-view-error" role="alert">
			<div className="initiatives-view-error-icon">
				<Icon name="alert-circle" size={24} />
			</div>
			<h2 className="initiatives-view-error-title">Failed to load initiatives</h2>
			<p className="initiatives-view-error-desc">{error}</p>
			<Button variant="secondary" onClick={onRetry}>
				Retry
			</Button>
		</div>
	);
}

// =============================================================================
// InitiativesView Component
// =============================================================================

/**
 * InitiativesView displays all initiatives with aggregate statistics.
 *
 * @example
 * <InitiativesView />
 */
export function InitiativesView({ className = '' }: InitiativesViewProps) {
	const navigate = useNavigate();
	const taskStates = useTaskStore((state) => state.taskStates);

	const [initiatives, setInitiatives] = useState<Initiative[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Load initiatives from API
	const loadInitiatives = useCallback(async () => {
		setLoading(true);
		setError(null);
		try {
			const response = await initiativeClient.listInitiatives(create(ListInitiativesRequestSchema, {}));
			setInitiatives(response.initiatives);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load initiatives');
		} finally {
			setLoading(false);
		}
	}, []);

	// Initial load
	useEffect(() => {
		loadInitiatives();
	}, [loadInitiatives]);

	// Compute aggregate task stats from initiative.tasks (from API)
	// This is more accurate than task store since it includes all linked tasks
	const { totalLinkedTasks, completedCount } = useMemo(() => {
		let total = 0;
		let completed = 0;

		for (const initiative of initiatives) {
			const initiativeTasks = initiative.tasks || [];
			total += initiativeTasks.length;
			completed += initiativeTasks.filter((t) => t.status === TaskStatus.COMPLETED).length;
		}

		return { totalLinkedTasks: total, completedCount: completed };
	}, [initiatives]);

	// Compute progress for each initiative using initiative.tasks from API
	// This is more accurate than task store since initiatives return their linked tasks directly
	const progressMap = useMemo(() => {
		const map = new Map<string, ProgressData>();

		for (const initiative of initiatives) {
			// Use initiative.tasks directly from API (TaskRef[] with status field)
			const initiativeTasks = initiative.tasks || [];
			const completed = initiativeTasks.filter((t) => t.status === TaskStatus.COMPLETED).length;
			map.set(initiative.id, { completed, total: initiativeTasks.length });
		}

		return map;
	}, [initiatives]);

	// Compute aggregate stats from initiative.tasks
	const stats: InitiativeStats = useMemo(() => {
		// Count active initiatives
		const activeInitiatives = initiatives.filter((i) => i.status === InitiativeStatus.ACTIVE).length;

		// Use totals computed from initiative.tasks
		const completionRate = totalLinkedTasks > 0 ? (completedCount / totalLinkedTasks) * 100 : 0;

		// Calculate total cost from task states (tokens * rate)
		// Build Set of linked task IDs for O(1) lookup
		const linkedTaskIds = new Set<string>();
		for (const initiative of initiatives) {
			for (const task of initiative.tasks || []) {
				linkedTaskIds.add(task.id);
			}
		}
		let totalCost = 0;
		for (const [taskId, state] of taskStates) {
			if (linkedTaskIds.has(taskId) && state?.tokens) {
				// Rough estimate: $3/1M input tokens, $15/1M output tokens
				const inputCost = (state.tokens?.inputTokens / 1_000_000) * 3;
				const outputCost = (state.tokens?.outputTokens / 1_000_000) * 15;
				totalCost += inputCost + outputCost;
			}
		}

		return {
			activeInitiatives,
			totalTasks: totalLinkedTasks,
			tasksThisWeek: 0, // Not available from initiative.tasks (no createdAt)
			completionRate,
			totalCost,
		};
	}, [initiatives, totalLinkedTasks, completedCount, taskStates]);

	// Compute per-initiative cost and token aggregates from taskStates
	const initiativeMetaMap = useMemo(() => {
		const map = new Map<string, { costSpent: number; tokensUsed: number }>();

		for (const initiative of initiatives) {
			let costSpent = 0;
			let tokensUsed = 0;

			for (const task of initiative.tasks || []) {
				const state = taskStates.get(task.id);
				if (state?.cost) {
					costSpent += state.cost.totalCostUsd;
				}
				if (state?.tokens) {
					tokensUsed += state.tokens.totalTokens;
				}
			}

			if (costSpent > 0 || tokensUsed > 0) {
				map.set(initiative.id, { costSpent, tokensUsed });
			}
		}

		return map;
	}, [initiatives, taskStates]);

	// Get progress for a specific initiative
	const getProgress = useCallback(
		(initiativeId: string): ProgressData => {
			return progressMap.get(initiativeId) || { completed: 0, total: 0 };
		},
		[progressMap]
	);

	// New Initiative modal state
	const [showNewInitiativeModal, setShowNewInitiativeModal] = useState(false);

	// Handle new initiative button click
	const handleNewInitiative = useCallback(() => {
		setShowNewInitiativeModal(true);
	}, []);

	// Handle card click - navigate to initiative detail
	const handleCardClick = useCallback(
		(initiativeId: string) => {
			navigate(`/initiatives/${initiativeId}`);
		},
		[navigate]
	);

	const classes = ['initiatives-view', className].filter(Boolean).join(' ');

	return (
		<div className={classes}>
			<header className="initiatives-view-header">
				<div className="initiatives-view-header-text">
					<h1 className="initiatives-view-title">Initiatives</h1>
					<p className="initiatives-view-subtitle">
						Manage your project epics and milestones
					</p>
				</div>
				<Button
					variant="primary"
					leftIcon={<Icon name="plus" size={12} />}
					onClick={handleNewInitiative}
				>
					New Initiative
				</Button>
			</header>

			<div className="initiatives-view-content">
				<StatsRow stats={stats} loading={loading} />

				{loading && <InitiativesViewSkeleton />}

				{!loading && error && (
					<InitiativesViewError error={error} onRetry={loadInitiatives} />
				)}

				{!loading && !error && initiatives.length === 0 && <InitiativesViewEmpty />}

				{!loading && !error && initiatives.length > 0 && (
					<div className="initiatives-view-grid">
						{initiatives.map((initiative) => {
							const progress = getProgress(initiative.id);
							const meta = initiativeMetaMap.get(initiative.id);
							return (
								<InitiativeCard
									key={initiative.id}
									initiative={initiative}
									completedTasks={progress.completed}
									totalTasks={progress.total}
									costSpent={meta?.costSpent}
									tokensUsed={meta?.tokensUsed}
									onClick={() => handleCardClick(initiative.id)}
								/>
							);
						})}
					</div>
				)}
			</div>

			<NewInitiativeModal
				open={showNewInitiativeModal}
				onClose={() => setShowNewInitiativeModal(false)}
				onCreate={() => loadInitiatives()}
			/>
		</div>
	);
}
