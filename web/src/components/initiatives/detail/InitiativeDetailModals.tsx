import type { Dispatch, FormEvent, SetStateAction } from 'react';
import { Button } from '@/components/ui/Button';
import { Modal } from '@/components/overlays/Modal';
import { InitiativeStatus } from '@/gen/orc/v1/initiative_pb';
import type { Task } from '@/gen/orc/v1/task_pb';
import { getTaskStatusDisplay } from './utils';

interface EditInitiativeModalProps {
	open: boolean;
	title: string;
	vision: string;
	status: InitiativeStatus;
	branchBase: string;
	branchPrefix: string;
	onClose: () => void;
	onSave: () => void;
	setTitle: Dispatch<SetStateAction<string>>;
	setVision: Dispatch<SetStateAction<string>>;
	setStatus: Dispatch<SetStateAction<InitiativeStatus>>;
	setBranchBase: Dispatch<SetStateAction<string>>;
	setBranchPrefix: Dispatch<SetStateAction<string>>;
}

export function EditInitiativeModal({
	open,
	title,
	vision,
	status,
	branchBase,
	branchPrefix,
	onClose,
	onSave,
	setTitle,
	setVision,
	setStatus,
	setBranchBase,
	setBranchPrefix,
}: EditInitiativeModalProps) {
	const handleSubmit = (e: FormEvent<HTMLFormElement>) => {
		e.preventDefault();
		onSave();
	};

	return (
		<Modal open={open} onClose={onClose} title="Edit Initiative">
			<form onSubmit={handleSubmit}>
				<div className="form-group">
					<label htmlFor="edit-title">Title</label>
					<input id="edit-title" type="text" value={title} onChange={(e) => setTitle(e.target.value)} required />
				</div>

				<div className="form-group">
					<label htmlFor="edit-vision">Vision</label>
					<textarea
						id="edit-vision"
						value={vision}
						onChange={(e) => setVision(e.target.value)}
						rows={3}
						placeholder="What is the goal of this initiative?"
					></textarea>
				</div>

				<div className="form-group">
					<label htmlFor="edit-status">Status</label>
					<select id="edit-status" value={status} onChange={(e) => setStatus(Number(e.target.value) as InitiativeStatus)}>
						<option value={InitiativeStatus.DRAFT}>Draft</option>
						<option value={InitiativeStatus.ACTIVE}>Active</option>
						<option value={InitiativeStatus.COMPLETED}>Completed</option>
						<option value={InitiativeStatus.ARCHIVED}>Archived</option>
					</select>
				</div>

				<div className="form-section-divider">
					<span className="divider-label">Branch Configuration</span>
				</div>

				<div className="form-group">
					<label htmlFor="edit-branch-base">Target Branch</label>
					<input
						id="edit-branch-base"
						type="text"
						value={branchBase}
						onChange={(e) => setBranchBase(e.target.value)}
						placeholder="e.g., feature/user-auth"
					/>
					<span className="form-hint">Tasks in this initiative will target this branch instead of main</span>
				</div>

				<div className="form-group">
					<label htmlFor="edit-branch-prefix">Task Branch Prefix</label>
					<input
						id="edit-branch-prefix"
						type="text"
						value={branchPrefix}
						onChange={(e) => setBranchPrefix(e.target.value)}
						placeholder="e.g., feature/auth-"
					/>
					<span className="form-hint">Task branches will be named: {branchPrefix || 'feature/auth-'}TASK-XXX</span>
				</div>

				<div className="modal-actions">
					<Button variant="secondary" onClick={onClose}>
						Cancel
					</Button>
					<Button variant="primary" type="submit">
						Save Changes
					</Button>
				</div>
			</form>
		</Modal>
	);
}

interface LinkTaskModalProps {
	open: boolean;
	search: string;
	loading: boolean;
	tasks: Task[];
	onClose: () => void;
	onSearchChange: Dispatch<SetStateAction<string>>;
	onLinkTask: (taskId: string) => void;
}

export function LinkTaskModal({
	open,
	search,
	loading,
	tasks,
	onClose,
	onSearchChange,
	onLinkTask,
}: LinkTaskModalProps) {
	return (
		<Modal open={open} onClose={onClose} title="Link Existing Task">
			<div className="link-task-content">
				<div className="form-group">
					<label htmlFor="task-search">Search Tasks</label>
					<input
						id="task-search"
						type="text"
						value={search}
						onChange={(e) => onSearchChange(e.target.value)}
						placeholder="Search by ID or title..."
					/>
				</div>

				{loading ? (
					<div className="loading-inline">
						<div className="spinner-sm"></div>
						<span>Loading tasks...</span>
					</div>
				) : tasks.length > 0 ? (
					<div className="available-tasks">
						{tasks.map((task) => (
							<Button
								key={task.id}
								variant="ghost"
								className="available-task-item"
								onClick={() => onLinkTask(task.id)}
							>
								<span className="task-id">{task.id}</span>
								<span className="task-title">{task.title}</span>
								<span className={`task-status-badge status-${getTaskStatusDisplay(task.status)}`}>
									{getTaskStatusDisplay(task.status)}
								</span>
							</Button>
						))}
					</div>
				) : (
					<p className="no-tasks-message">No available tasks to link</p>
				)}
			</div>
		</Modal>
	);
}

interface AddDecisionModalProps {
	open: boolean;
	decisionText: string;
	decisionRationale: string;
	decisionBy: string;
	addingDecision: boolean;
	onClose: () => void;
	onAddDecision: () => void;
	setDecisionText: Dispatch<SetStateAction<string>>;
	setDecisionRationale: Dispatch<SetStateAction<string>>;
	setDecisionBy: Dispatch<SetStateAction<string>>;
}

export function AddDecisionModal({
	open,
	decisionText,
	decisionRationale,
	decisionBy,
	addingDecision,
	onClose,
	onAddDecision,
	setDecisionText,
	setDecisionRationale,
	setDecisionBy,
}: AddDecisionModalProps) {
	const handleSubmit = (e: FormEvent<HTMLFormElement>) => {
		e.preventDefault();
		onAddDecision();
	};

	return (
		<Modal open={open} onClose={onClose} title="Add Decision">
			<form onSubmit={handleSubmit}>
				<div className="form-group">
					<label htmlFor="decision-text">Decision</label>
					<textarea
						id="decision-text"
						value={decisionText}
						onChange={(e) => setDecisionText(e.target.value)}
						rows={2}
						required
						placeholder="What was decided?"
					></textarea>
				</div>

				<div className="form-group">
					<label htmlFor="decision-rationale">Rationale (optional)</label>
					<textarea
						id="decision-rationale"
						value={decisionRationale}
						onChange={(e) => setDecisionRationale(e.target.value)}
						rows={2}
						placeholder="Why was this decision made?"
					></textarea>
				</div>

				<div className="form-group">
					<label htmlFor="decision-by">Decided By (optional)</label>
					<input
						id="decision-by"
						type="text"
						value={decisionBy}
						onChange={(e) => setDecisionBy(e.target.value)}
						placeholder="Name or initials"
					/>
				</div>

				<div className="modal-actions">
					<Button variant="secondary" onClick={onClose}>
						Cancel
					</Button>
					<Button variant="primary" type="submit" disabled={!decisionText.trim()} loading={addingDecision}>
						Add Decision
					</Button>
				</div>
			</form>
		</Modal>
	);
}

interface ArchiveInitiativeModalProps {
	open: boolean;
	title: string;
	loading: boolean;
	onClose: () => void;
	onArchive: () => void;
}

export function ArchiveInitiativeModal({
	open,
	title,
	loading,
	onClose,
	onArchive,
}: ArchiveInitiativeModalProps) {
	return (
		<Modal open={open} onClose={onClose} title="Archive Initiative">
			<div className="confirm-dialog">
				<p className="confirm-message">
					Are you sure you want to archive <strong>"{title}"</strong>?
				</p>
				<p className="confirm-hint">Archived initiatives are hidden from most views but can be restored later.</p>
				<div className="modal-actions">
					<Button variant="secondary" onClick={onClose}>
						Cancel
					</Button>
					<Button variant="danger" onClick={onArchive} loading={loading}>
						Archive Initiative
					</Button>
				</div>
			</div>
		</Modal>
	);
}
