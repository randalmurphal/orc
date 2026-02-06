import { useState, useEffect, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { projectClient } from '@/lib/client';
import { useProjectStore } from '@/stores/projectStore';
import { ProjectCard } from '@/components/dashboard/ProjectCard';
import { useDocumentTitle } from '@/hooks';
import type { ProjectStatus } from '@/gen/orc/v1/project_pb';
import { TaskStatus } from '@/gen/orc/v1/task_pb';
import './MyWorkPage.css';

type StatusFilter = 'all' | 'running' | 'blocked' | 'created' | 'paused';

function matchesFilter(status: TaskStatus, filter: StatusFilter): boolean {
	switch (filter) {
		case 'running':
			return status === TaskStatus.RUNNING || status === TaskStatus.FINALIZING;
		case 'blocked':
			return status === TaskStatus.BLOCKED;
		case 'created':
			return status === TaskStatus.CREATED;
		case 'paused':
			return status === TaskStatus.PAUSED;
		default:
			return true;
	}
}

export function MyWorkPage() {
	useDocumentTitle('My Work');
	const navigate = useNavigate();
	const [projects, setProjects] = useState<ProjectStatus[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [filter, setFilter] = useState<StatusFilter>('all');

	const fetchData = useCallback(async () => {
		setLoading(true);
		setError(null);
		try {
			const response = await projectClient.getAllProjectsStatus({});
			setProjects(response.projects);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load projects');
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		fetchData();
	}, [fetchData]);

	const handleTaskClick = useCallback(
		(projectId: string, taskId: string) => {
			useProjectStore.getState().selectProject(projectId);
			navigate(`/tasks/${taskId}`);
		},
		[navigate],
	);

	const handleViewAll = useCallback(
		(projectId: string) => {
			useProjectStore.getState().selectProject(projectId);
			navigate('/board');
		},
		[navigate],
	);

	const filteredProjects: ProjectStatus[] = useMemo(() => {
		if (filter === 'all') return projects;
		return projects
			.map((p) => {
				const filtered = p.activeTasks.filter((t) => matchesFilter(t.status, filter));
				return { ...p, activeTasks: filtered } as ProjectStatus;
			})
			.filter((p) => p.activeTasks.length > 0);
	}, [projects, filter]);

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
				<p>Error loading projects</p>
				<button onClick={fetchData}>Retry</button>
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
			<div className="my-work-page__header">
				<h1 className="my-work-page__title">My Work</h1>
				<select
					className="my-work-page__filter"
					value={filter}
					onChange={(e) => setFilter(e.target.value as StatusFilter)}
					aria-label="Filter by status"
				>
					<option value="all">All</option>
					<option value="running">Running</option>
					<option value="blocked">Blocked</option>
					<option value="paused">Paused</option>
					<option value="created">Created</option>
				</select>
			</div>
			<div className="my-work-page__projects">
				{filteredProjects.length === 0 ? (
					<div className="my-work-page__no-match">
						No matching tasks
					</div>
				) : (
					filteredProjects.map((project) => (
						<ProjectCard
							key={project.projectId}
							project={project}
							onTaskClick={handleTaskClick}
							onViewAll={handleViewAll}
						/>
					))
				)}
			</div>
		</div>
	);
}
