import { memo } from 'react';
import type { ProjectStatus } from '@/gen/orc/v1/project_pb';
import { TaskRow } from './TaskRow';
import './ProjectCard.css';

export interface ProjectCardProps {
	project: ProjectStatus;
	onTaskClick: (projectId: string, taskId: string) => void;
	onViewAll: (projectId: string) => void;
}

export const ProjectCard = memo(function ProjectCard({
	project,
	onTaskClick,
	onViewAll,
}: ProjectCardProps) {
	return (
		<div className="project-card">
			<div className="project-card__header">
				<div className="project-card__info">
					<button
						type="button"
						className="project-card__name-button"
						onClick={() => onViewAll(project.projectId)}
					>
						<h3 className="project-card__name">{project.projectName}</h3>
					</button>
					<div className="project-card__stats">
						<span className="project-card__stat">
							{project.totalTasks} total
						</span>
						<span className="project-card__stat">
							{project.completedToday} done today
						</span>
						<span className="project-card__stat">
							{project.activeThreadCount} active threads
						</span>
						<span className="project-card__stat">
							{project.pendingRecommendations} pending recommendations
						</span>
					</div>
				</div>
				<button
					className="project-card__view-all"
					onClick={() => onViewAll(project.projectId)}
				>
					View all
				</button>
			</div>
			<div className="project-card__tasks">
				{project.activeTasks.length === 0 ? (
					<div className="project-card__empty">No active tasks</div>
				) : (
					project.activeTasks.map((task) => (
						<TaskRow
							key={task.id}
							task={task}
							onClick={() => onTaskClick(project.projectId, task.id)}
						/>
					))
				)}
			</div>
		</div>
	);
});
