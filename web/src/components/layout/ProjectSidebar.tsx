import { useCallback } from 'react';
import { useCurrentProject, useCurrentProjectId, useRunningTasks } from '@/stores';
import { useThreadStore } from '@/stores/threadStore';
import './ProjectSidebar.css';

interface ProjectSidebarProps {
	onProjectChange?: () => void;
}

export function ProjectSidebar({ onProjectChange }: ProjectSidebarProps) {
	const project = useCurrentProject();
	const projectId = useCurrentProjectId();
	const runningTasks = useRunningTasks();
	const threads = useThreadStore((state) => state.threads);
	const selectedThreadId = useThreadStore((state) => state.selectedThreadId);
	const error = useThreadStore((state) => state.error);
	const selectThread = useThreadStore((state) => state.selectThread);
	const createThread = useThreadStore((state) => state.createThread);
	const loadThreads = useThreadStore((state) => state.loadThreads);

	const handleNewThread = useCallback(() => {
		if (projectId) {
			createThread(projectId, 'New Thread');
		}
	}, [projectId, createThread]);

	const handleRetry = useCallback(() => {
		if (projectId) {
			loadThreads(projectId);
		}
	}, [projectId, loadThreads]);

	if (!project) {
		return (
			<nav className="project-sidebar" role="navigation" aria-label="Main navigation">
				<div className="project-sidebar__empty">
					Select a project
				</div>
			</nav>
		);
	}

	return (
		<nav className="project-sidebar" role="navigation" aria-label="Main navigation">
			<div className="project-sidebar__header">
				<button
					className="project-sidebar__project-button"
					onClick={onProjectChange}
				>
					{project.name}
				</button>
			</div>

			<div className="project-sidebar__running-tasks">
				<span className="project-sidebar__running-label">Running</span>
				<span className="project-sidebar__running-count">{runningTasks.length}</span>
			</div>

			<div className="project-sidebar__section">
				{(threads.length > 0 || error) && (
					<div className="project-sidebar__threads-header">
						<span>Threads</span>
						<button
							className="project-sidebar__new-thread"
							onClick={handleNewThread}
							aria-label="New Thread"
						>
							+
						</button>
					</div>
				)}

				{error && (
					<div className="project-sidebar__error">
						<span>{error}</span>
						<button
							className="project-sidebar__retry"
							onClick={handleRetry}
							aria-label="Retry"
						>
							Retry
						</button>
					</div>
				)}

				{!error && threads.length === 0 && (
					<div className="project-sidebar__empty-threads">
						<span>No threads yet</span>
						<button
							className="project-sidebar__new-thread"
							onClick={handleNewThread}
							aria-label="New Thread"
						>
							+
						</button>
					</div>
				)}

				<div className="project-sidebar__thread-list">
					{threads.map((thread) => (
						<div
							key={thread.id}
							className={`project-sidebar__thread-item project-sidebar__thread-item--${thread.status}${
								thread.id === selectedThreadId ? ' project-sidebar__thread-item--active' : ''
							}`}
							onClick={() => selectThread(thread.id)}
							role="button"
							tabIndex={0}
						>
							<span className="project-sidebar__status-dot" />
							<span className="project-sidebar__item-label">
								{thread.title}
							</span>
						</div>
					))}
				</div>
			</div>
		</nav>
	);
}
