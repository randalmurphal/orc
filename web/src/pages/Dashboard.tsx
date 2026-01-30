/**
 * Dashboard page (/dashboard)
 *
 * Displays quick stats, active tasks, recent activity, and initiatives.
 * Stats update in real-time via WebSocket.
 *
 * URL params:
 * - project: Project filter
 */

import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { create } from '@bufbuild/protobuf';
import { useTaskStore, useWsStatus } from '@/stores';
import { useCurrentProjectId } from '@/stores/projectStore';
import { dashboardClient, initiativeClient } from '@/lib/client';
import { useDocumentTitle } from '@/hooks';
import { GetStatsRequestSchema, type DashboardStats } from '@/gen/orc/v1/dashboard_pb';
import { ListInitiativesRequestSchema, InitiativeStatus } from '@/gen/orc/v1/initiative_pb';
import type { Task, TaskStatus } from '@/gen/orc/v1/task_pb';
import { TaskStatus as TaskStatusEnum } from '@/gen/orc/v1/task_pb';
import type { Initiative } from '@/gen/orc/v1/initiative_pb';
import { timestampToDate } from '@/lib/time';
import {
	DashboardStats as StatsSection,
	DashboardQuickActions,
	DashboardActiveTasks,
	DashboardRecentActivity,
	DashboardSummary,
	DashboardInitiatives,
} from '@/components/dashboard';
import { Button } from '@/components/ui/Button';
import './Dashboard.css';

// Filter tasks by status
const ACTIVE_STATUSES: TaskStatus[] = [TaskStatusEnum.RUNNING, TaskStatusEnum.BLOCKED, TaskStatusEnum.PAUSED];
const RECENT_STATUSES: TaskStatus[] = [TaskStatusEnum.COMPLETED, TaskStatusEnum.FAILED];

export function Dashboard() {
	useDocumentTitle('Dashboard');
	const navigate = useNavigate();
	const wsStatus = useWsStatus();
	const projectId = useCurrentProjectId();
	const tasks = useTaskStore((state) => state.tasks);

	const [stats, setStats] = useState<DashboardStats | null>(null);
	const [initiatives, setInitiatives] = useState<Initiative[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Derive active and recent tasks from store
	const activeTasks = tasks
		.filter((t: Task) => ACTIVE_STATUSES.includes(t.status))
		.slice(0, 5);

	const recentTasks = tasks
		.filter((t: Task) => RECENT_STATUSES.includes(t.status))
		.sort((a: Task, b: Task) => {
			const aTime = timestampToDate(a.updatedAt)?.getTime() ?? 0;
			const bTime = timestampToDate(b.updatedAt)?.getTime() ?? 0;
			return bTime - aTime;
		})
		.slice(0, 5);

	const loadDashboardData = useCallback(async () => {
		try {
			// Load stats and initiatives in parallel
			const [statsResponse, initiativesResponse] = await Promise.all([
				dashboardClient.getStats(create(GetStatsRequestSchema, {})),
				initiativeClient.listInitiatives(create(ListInitiativesRequestSchema, {
					projectId: projectId ?? undefined,
					status: InitiativeStatus.ACTIVE
				})),
			]);
			if (statsResponse.stats) {
				setStats(statsResponse.stats);
			}
			setInitiatives(initiativesResponse.initiatives);
			setLoading(false);
			setError(null);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load dashboard');
			setLoading(false);
		}
	}, [projectId]);

	const loadInitiatives = useCallback(async () => {
		try {
			const response = await initiativeClient.listInitiatives(
				create(ListInitiativesRequestSchema, { projectId: projectId ?? undefined, status: InitiativeStatus.ACTIVE })
			);
			setInitiatives(response.initiatives);
		} catch {
			// Silently fail - not critical
		}
	}, [projectId]);

	// Initial load
	useEffect(() => {
		loadDashboardData();
	}, [loadDashboardData]);

	// Refresh data on WebSocket reconnect
	useEffect(() => {
		if (wsStatus === 'connected') {
			loadDashboardData();
		}
	}, [wsStatus, loadDashboardData]);

	// Subscribe to task changes via store subscription to refresh initiatives
	useEffect(() => {
		// Subscribe to taskStore changes - when tasks change, refresh stats
		const unsubscribe = useTaskStore.subscribe(
			(state) => state.tasks,
			() => {
				// Refresh initiatives to update progress counts
				loadInitiatives();
				// Also refresh stats
				dashboardClient.getStats(create(GetStatsRequestSchema, {}))
					.then((res) => { if (res.stats) setStats(res.stats); })
					.catch(() => {});
			}
		);
		return unsubscribe;
	}, [loadInitiatives]);

	const navigateToFiltered = (status: string) => {
		navigate(`/?status=${status}`);
	};

	const navigateToDependencyFiltered = (status: string) => {
		navigate(`/?dependency_status=${status}`);
	};

	const handleNewTask = () => {
		window.dispatchEvent(new CustomEvent('orc:new-task'));
	};

	const handleViewTasks = () => {
		navigate('/');
	};

	// Loading state
	if (loading && !stats) {
		return (
			<div className="dashboard">
				<div className="loading">
					<div className="spinner" />
					<span>Loading dashboard...</span>
				</div>
			</div>
		);
	}

	// Error state
	if (error) {
		return (
			<div className="dashboard">
				<div className="error">
					<p>{error}</p>
					<Button variant="secondary" onClick={loadDashboardData}>Retry</Button>
				</div>
			</div>
		);
	}

	// Main dashboard content
	if (!stats) {
		return null;
	}

	return (
		<div className="dashboard">
			<StatsSection
				stats={stats}
				wsStatus={wsStatus}
				onFilterClick={navigateToFiltered}
				onDependencyFilterClick={navigateToDependencyFiltered}
			/>
			<DashboardQuickActions onNewTask={handleNewTask} onViewTasks={handleViewTasks} />
			<DashboardInitiatives initiatives={initiatives} />
			<DashboardActiveTasks tasks={activeTasks} />
			<DashboardRecentActivity tasks={recentTasks} />
			<DashboardSummary stats={stats} />
		</div>
	);
}
