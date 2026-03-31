import { Link } from 'react-router-dom';
import type { Timestamp } from '@bufbuild/protobuf/wkt';
import type { Initiative, InitiativeNote, TaskRef } from '@/gen/orc/v1/initiative_pb';
import { InitiativeStatus } from '@/gen/orc/v1/initiative_pb';
import type { DependencyGraph as DependencyGraphData } from '@/gen/orc/v1/task_pb';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import { DependencyGraph } from '@/components/initiatives';
import {
	getInitiativeStatusDisplay,
	getNoteTypeIcon,
	getNoteTypeLabel,
	getTaskStatusClass,
	getTaskStatusDisplay,
	getTaskStatusIcon,
	type TaskFilter,
} from './utils';

interface InitiativeProgress {
	completed: number;
	total: number;
	percentage: number;
}

interface InitiativeHeaderSectionProps {
	initiative: Initiative;
	emoji: string;
	titleWithoutEmoji: string;
	statusActionLoading: boolean;
	onActivate: () => void;
	onComplete: () => void;
	onEdit: () => void;
	onArchive: () => void;
}

export function InitiativeHeaderSection({
	initiative,
	emoji,
	titleWithoutEmoji,
	statusActionLoading,
	onActivate,
	onComplete,
	onEdit,
	onArchive,
}: InitiativeHeaderSectionProps) {
	const statusLabel = getInitiativeStatusDisplay(initiative.status);

	return (
		<header className="initiative-header">
			<div className="header-top">
				<div className="title-row">
					<span className="initiative-emoji">{emoji}</span>
					<h1 className="initiative-title">{titleWithoutEmoji}</h1>
				</div>
				<div className="header-actions">
					<span className={`status-badge status-${statusLabel}`}>{statusLabel}</span>
					{initiative.status === InitiativeStatus.DRAFT && (
						<Button
							variant="primary"
							onClick={onActivate}
							loading={statusActionLoading}
							leftIcon={<Icon name="play" size={16} />}
						>
							Activate
						</Button>
					)}
					{initiative.status === InitiativeStatus.ACTIVE && (
						<Button
							variant="success"
							onClick={onComplete}
							loading={statusActionLoading}
							leftIcon={<Icon name="check" size={16} />}
						>
							Complete
						</Button>
					)}
					{initiative.status === InitiativeStatus.COMPLETED && (
						<Button
							variant="secondary"
							onClick={onActivate}
							loading={statusActionLoading}
							leftIcon={<Icon name="rotate-ccw" size={16} />}
						>
							Reopen
						</Button>
					)}
					<Button variant="secondary" onClick={onEdit} leftIcon={<Icon name="edit" size={16} />}>
						Edit
					</Button>
					{initiative.status !== InitiativeStatus.ARCHIVED && (
						<Button
							variant="ghost"
							className="btn-danger-hover"
							onClick={onArchive}
							leftIcon={<Icon name="archive" size={16} />}
						>
							Archive
						</Button>
					)}
				</div>
			</div>
			{initiative.vision && <p className="initiative-vision">{initiative.vision}</p>}
		</header>
	);
}

export function InitiativeProgressSection({ progress }: { progress: InitiativeProgress }) {
	return (
		<div className="progress-section">
			<div className="progress-label">
				<span>Progress</span>
				<span className="progress-count">
					{progress.completed}/{progress.total} tasks ({progress.percentage}%)
				</span>
			</div>
			<div className="progress-bar">
				<div className="progress-fill" style={{ width: `${progress.percentage}%` }}></div>
			</div>
		</div>
	);
}

interface InitiativeStatsSectionProps {
	progress: InitiativeProgress;
	totalCost: number;
	formatCost: (cost: number) => string;
}

export function InitiativeStatsSection({
	progress,
	totalCost,
	formatCost,
}: InitiativeStatsSectionProps) {
	return (
		<div className="stats-row">
			<div className="stat-card">
				<span className="stat-label">Total Tasks</span>
				<span className="stat-value">{progress.total}</span>
			</div>
			<div className="stat-card">
				<span className="stat-label">Completed</span>
				<span className="stat-value stat-success">{progress.completed}</span>
			</div>
			<div className="stat-card">
				<span className="stat-label">Total Cost</span>
				<span className="stat-value stat-primary">{formatCost(totalCost)}</span>
			</div>
		</div>
	);
}

interface InitiativeDecisionsSectionProps {
	initiative: Initiative;
	formatDate: (timestamp?: Timestamp) => string;
	onAddDecision: () => void;
}

export function InitiativeDecisionsSection({
	initiative,
	formatDate,
	onAddDecision,
}: InitiativeDecisionsSectionProps) {
	return (
		<section className="decisions-section">
			<div className="section-header">
				<h2>Decisions</h2>
				<Button variant="secondary" size="sm" onClick={onAddDecision} leftIcon={<Icon name="plus" size={14} />}>
					Add Decision
				</Button>
			</div>

			{initiative.decisions && initiative.decisions.length > 0 ? (
				<div className="decision-list">
					{initiative.decisions.map((decision) => (
						<div key={decision.id} className="decision-item">
							<div className="decision-header">
								<span className="decision-date">{formatDate(decision.date)}</span>
								{decision.by && <span className="decision-by">by {decision.by}</span>}
							</div>
							<p className="decision-text">{decision.decision}</p>
							{decision.rationale && <p className="decision-rationale">{decision.rationale}</p>}
						</div>
					))}
				</div>
			) : (
				<div className="empty-state-inline">
					<span>No decisions recorded yet</span>
				</div>
			)}
		</section>
	);
}

interface InitiativeKnowledgeSectionProps {
	notes: InitiativeNote[];
	notesByType: Record<string, InitiativeNote[]>;
	notesLoading: boolean;
	knowledgeExpanded: boolean;
	formatDate: (timestamp?: Timestamp) => string;
	onToggle: () => void;
}

export function InitiativeKnowledgeSection({
	notes,
	notesByType,
	notesLoading,
	knowledgeExpanded,
	formatDate,
	onToggle,
}: InitiativeKnowledgeSectionProps) {
	return (
		<section className="knowledge-section">
			<div className="section-header section-header-collapsible">
				<h2>Knowledge ({notes.length})</h2>
				<Button
					variant="ghost"
					size="sm"
					onClick={onToggle}
					aria-expanded={knowledgeExpanded}
					leftIcon={<Icon name={knowledgeExpanded ? 'chevron-up' : 'chevron-down'} size={16} />}
				>
					{knowledgeExpanded ? 'Collapse' : 'Expand'}
				</Button>
			</div>

			{knowledgeExpanded && (
				<>
					{notesLoading ? (
						<div className="loading-inline">
							<div className="spinner-sm"></div>
							<span>Loading notes...</span>
						</div>
					) : notes.length > 0 ? (
						<div className="notes-by-type">
							{['pattern', 'warning', 'learning', 'handoff'].map((noteType) => {
								const typeNotes = notesByType[noteType];
								if (!typeNotes || typeNotes.length === 0) {
									return null;
								}

								return (
									<div key={noteType} className="note-type-group">
										<div className="note-type-header">
											<span className={`note-type-icon type-${noteType}`}>
												<Icon name={getNoteTypeIcon(noteType)} size={14} />
											</span>
											<span>{getNoteTypeLabel(noteType)}</span>
											<span className="note-type-count">({typeNotes.length})</span>
										</div>
										{typeNotes.map((note) => (
											<div key={note.id} className="note-item">
												<p className="note-content">{note.content}</p>
												<div className="note-meta">
													<span className={`note-author-badge author-${note.authorType}`}>
														{note.authorType === 'agent' ? '🤖 Agent' : '👤 Human'}
													</span>
													{note.sourceTask && (
														<Link to={`/tasks/${note.sourceTask}`} className="note-source-task">
															{note.sourceTask}
														</Link>
													)}
													<span>{formatDate(note.createdAt)}</span>
												</div>
												{note.relevantFiles && note.relevantFiles.length > 0 && (
													<div className="note-relevant-files">
														{note.relevantFiles.map((file, idx) => (
															<span key={idx} className="note-file">
																{file}
															</span>
														))}
													</div>
												)}
											</div>
										))}
									</div>
								);
							})}
						</div>
					) : (
						<div className="empty-state-inline">
							<Icon name="brain" size={24} />
							<span>No knowledge captured yet. Notes will appear as tasks share learnings.</span>
						</div>
					)}
				</>
			)}
		</section>
	);
}

interface InitiativeTasksSectionProps {
	tasks: TaskRef[];
	allTaskCount: number;
	taskFilter: TaskFilter;
	onTaskFilterChange: (filter: TaskFilter) => void;
	onLinkTask: () => void;
	onUnlinkTask: (taskId: string) => void;
}

export function InitiativeTasksSection({
	tasks,
	allTaskCount,
	taskFilter,
	onTaskFilterChange,
	onLinkTask,
	onUnlinkTask,
}: InitiativeTasksSectionProps) {
	return (
		<section className="tasks-section">
			<div className="section-header">
				<h2>Tasks</h2>
				<div className="section-actions">
					<select
						className="filter-select"
						value={taskFilter}
						onChange={(e) => onTaskFilterChange(e.target.value as TaskFilter)}
						aria-label="Filter tasks"
					>
						<option value="all">All</option>
						<option value="completed">Completed</option>
						<option value="running">In Progress</option>
						<option value="planned">Planned</option>
					</select>
					<Button variant="secondary" size="sm" onClick={onLinkTask} leftIcon={<Icon name="link" size={14} />}>
						Link Existing
					</Button>
				</div>
			</div>

			{tasks.length > 0 ? (
				<div className="task-list">
					{tasks.map((task) => (
						<div key={task.id} className="task-item">
							<Link to={`/tasks/${task.id}`} className="task-link">
								<span className={`task-status ${getTaskStatusClass(task.status)}`}>
									<Icon name={getTaskStatusIcon(task.status)} size={16} />
								</span>
								<span className="task-id">{task.id}</span>
								<span className="task-title">{task.title}</span>
								<span className="task-status-text">{getTaskStatusDisplay(task.status)}</span>
							</Link>
							<Button
								variant="ghost"
								iconOnly
								size="sm"
								className="btn-icon btn-remove"
								onClick={() => onUnlinkTask(task.id)}
								title="Remove from initiative"
								aria-label="Remove from initiative"
							>
								<Icon name="x" size={14} />
							</Button>
						</div>
					))}
				</div>
			) : allTaskCount > 0 ? (
				<div className="empty-state-inline">
					<span>No tasks match the current filter</span>
				</div>
			) : (
				<div className="empty-state">
					<Icon name="clipboard" size={32} />
					<p>No tasks in this initiative yet</p>
					<Button variant="primary" onClick={onLinkTask}>
						Link a Task
					</Button>
				</div>
			)}
		</section>
	);
}

interface InitiativeDependencyGraphSectionProps {
	graphExpanded: boolean;
	graphLoading: boolean;
	graphError: string | null;
	graphData: DependencyGraphData | null;
	onToggle: () => void;
	onRetry: () => void;
}

export function InitiativeDependencyGraphSection({
	graphExpanded,
	graphLoading,
	graphError,
	graphData,
	onToggle,
	onRetry,
}: InitiativeDependencyGraphSectionProps) {
	return (
		<section className="graph-section">
			<div className="section-header section-header-collapsible">
				<h2>Dependency Graph</h2>
				<Button
					variant="ghost"
					size="sm"
					onClick={onToggle}
					aria-expanded={graphExpanded}
					leftIcon={<Icon name={graphExpanded ? 'chevron-up' : 'chevron-down'} size={16} />}
				>
					{graphExpanded ? 'Collapse' : 'Expand'}
				</Button>
			</div>

			{graphExpanded && (
				<div className="graph-content">
					{graphLoading ? (
						<div className="graph-loading">
							<div className="spinner"></div>
							<span>Loading graph...</span>
						</div>
					) : graphError ? (
						<div className="graph-error">
							<p>{graphError}</p>
							<Button variant="secondary" onClick={onRetry}>
								Retry
							</Button>
						</div>
					) : graphData && graphData.nodes.length > 0 ? (
						<div className="graph-container-wrapper">
							<DependencyGraph nodes={graphData.nodes} edges={graphData.edges} />
						</div>
					) : (
						<div className="empty-state-inline">
							<Icon name="git-branch" size={24} />
							<span>No tasks with dependencies to visualize</span>
						</div>
					)}
				</div>
			)}
		</section>
	);
}
