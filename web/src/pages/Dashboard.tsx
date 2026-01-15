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
import { useTaskStore, useWsStatus } from '@/stores';
import { useWebSocket } from '@/hooks/useWebSocket';
import { getDashboardStats, listInitiatives, type DashboardStats } from '@/lib/api';
import type { Initiative, Task } from '@/lib/types';
import {
	DashboardStats as StatsSection,
	DashboardQuickActions,
	DashboardActiveTasks,
	DashboardRecentActivity,
	DashboardSummary,
	DashboardInitiatives,
} from '@/components/dashboard';
import './Dashboard.css';

// Filter tasks by status
const ACTIVE_STATUSES = ['running', 'blocked', 'paused'];
const RECENT_STATUSES = ['completed', 'failed'];

export function Dashboard() {
	const navigate = useNavigate();
	const wsStatus = useWsStatus();
	const { on } = useWebSocket();
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
		.sort((a: Task, b: Task) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())
		.slice(0, 5);

	const loadDashboardData = useCallback(async () => {
		try {
			// Load stats and initiatives in parallel
			const [statsData, initiativesData] = await Promise.all([
				getDashboardStats(),
				listInitiatives({ status: 'active' }),
			]);
			setStats(statsData);
			setInitiatives(initiativesData);
			setLoading(false);
			setError(null);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load dashboard');
			setLoading(false);
		}
	}, []);

	const loadInitiatives = useCallback(async () => {
		try {
			const data = await listInitiatives({ status: 'active' });
			setInitiatives(data);
		} catch {
			// Silently fail - not critical
		}
	}, []);

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

	// Subscribe to task events to refresh initiatives when task status changes
	useEffect(() => {
		const unsubscribe = on('all', (event) => {
			if (
				'event' in event &&
				['task_updated', 'task_created', 'task_deleted'].includes(event.event)
			) {
				// Refresh initiatives to update progress counts
				loadInitiatives();
				// Also refresh stats
				getDashboardStats().then(setStats).catch(() => {});
			}
		});
		return unsubscribe;
	}, [on, loadInitiatives]);

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
					<button onClick={loadDashboardData}>Retry</button>
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
