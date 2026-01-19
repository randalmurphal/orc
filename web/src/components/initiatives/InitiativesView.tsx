/**
 * InitiativesView container component - assembles the complete initiatives
 * overview page with aggregate statistics, initiative cards grid, and
 * proper empty/loading/error states.
 */

import { useState, useEffect, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTaskStore } from '@/stores';
import { listInitiatives } from '@/lib/api';
import type { Initiative } from '@/lib/types';
import { StatsRow, type InitiativeStats } from './StatsRow';
import { InitiativeCard } from './InitiativeCard';
import { Button } from '@/components/ui/Button';
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
// Icons
// =============================================================================

function PlusIcon() {
	return (
		<svg
			width="12"
			height="12"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2.5"
			aria-hidden="true"
		>
			<path d="M12 4v16m8-8H4" />
		</svg>
	);
}

function LayersIcon() {
	return (
		<svg
			width="32"
			height="32"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			aria-hidden="true"
		>
			<polygon points="12 2 2 7 12 12 22 7 12 2" />
			<polyline points="2 17 12 22 22 17" />
			<polyline points="2 12 12 17 22 12" />
		</svg>
	);
}

function AlertCircleIcon() {
	return (
		<svg
			width="24"
			height="24"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			aria-hidden="true"
		>
			<circle cx="12" cy="12" r="10" />
			<path d="M12 8v4M12 16h.01" />
		</svg>
	);
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
				<LayersIcon />
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
				<AlertCircleIcon />
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
	const tasks = useTaskStore((state) => state.tasks);

	const [initiatives, setInitiatives] = useState<Initiative[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Load initiatives from API
	const loadInitiatives = useCallback(async () => {
		setLoading(true);
		setError(null);
		try {
			const data = await listInitiatives();
			setInitiatives(data);
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

	// Compute progress for each initiative from tasks
	const progressMap = useMemo(() => {
		const map = new Map<string, ProgressData>();

		for (const initiative of initiatives) {
			// Get tasks linked to this initiative
			const initiativeTasks = tasks.filter((t) => t.initiative_id === initiative.id);
			const completed = initiativeTasks.filter((t) => t.status === 'completed').length;
			const total = initiativeTasks.length;
			map.set(initiative.id, { completed, total });
		}

		return map;
	}, [initiatives, tasks]);

	// Compute aggregate stats
	const stats: InitiativeStats = useMemo(() => {
		// Count active initiatives
		const activeInitiatives = initiatives.filter((i) => i.status === 'active').length;

		// Count total tasks linked to initiatives
		const initiativeTaskIds = new Set(
			initiatives.flatMap((i) => i.tasks?.map((t) => t.id) || [])
		);
		const linkedTasks = tasks.filter(
			(t) => t.initiative_id || initiativeTaskIds.has(t.id)
		);
		const totalTasks = linkedTasks.length;

		// Calculate completion rate
		const completedTasks = linkedTasks.filter((t) => t.status === 'completed').length;
		const completionRate = totalTasks > 0 ? (completedTasks / totalTasks) * 100 : 0;

		// Calculate total cost from task states (tokens * rate)
		// For now, we'll use a placeholder since we don't have cost data readily available
		// In a real implementation, this would aggregate from task states
		let totalCost = 0;
		const taskStates = useTaskStore.getState().taskStates;
		for (const [taskId] of taskStates) {
			const task = tasks.find((t) => t.id === taskId);
			if (task?.initiative_id || initiativeTaskIds.has(taskId)) {
				const state = taskStates.get(taskId);
				if (state?.tokens) {
					// Rough estimate: $3/1M input tokens, $15/1M output tokens
					const inputCost = (state.tokens.input_tokens / 1_000_000) * 3;
					const outputCost = (state.tokens.output_tokens / 1_000_000) * 15;
					totalCost += inputCost + outputCost;
				}
			}
		}

		// Count tasks this week
		const oneWeekAgo = new Date();
		oneWeekAgo.setDate(oneWeekAgo.getDate() - 7);
		const tasksThisWeek = linkedTasks.filter(
			(t) => new Date(t.created_at) > oneWeekAgo
		).length;

		return {
			activeInitiatives,
			totalTasks,
			tasksThisWeek,
			completionRate,
			totalCost,
		};
	}, [initiatives, tasks]);

	// Get progress for a specific initiative
	const getProgress = useCallback(
		(initiativeId: string): ProgressData => {
			return progressMap.get(initiativeId) || { completed: 0, total: 0 };
		},
		[progressMap]
	);

	// Handle new initiative button click
	const handleNewInitiative = useCallback(() => {
		window.dispatchEvent(new CustomEvent('orc:new-initiative'));
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
					leftIcon={<PlusIcon />}
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
							return (
								<InitiativeCard
									key={initiative.id}
									initiative={initiative}
									completedTasks={progress.completed}
									totalTasks={progress.total}
									onClick={() => handleCardClick(initiative.id)}
								/>
							);
						})}
					</div>
				)}
			</div>
		</div>
	);
}
