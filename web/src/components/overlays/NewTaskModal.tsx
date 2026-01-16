import { useState, useCallback, useEffect } from 'react';
import { Modal } from './Modal';
import { createProjectTask } from '@/lib/api';
import { toast } from '@/stores/uiStore';
import { useCurrentProjectId } from '@/stores';
import type { Task, TaskWeight, TaskCategory } from '@/lib/types';
import '../task-detail/TaskEditModal.css';

const WEIGHTS: TaskWeight[] = ['trivial', 'small', 'medium', 'large', 'greenfield'];
const CATEGORIES: TaskCategory[] = ['feature', 'bug', 'refactor', 'chore', 'docs', 'test'];

interface NewTaskModalProps {
	open: boolean;
	onClose: () => void;
	onCreate?: (task: Task) => void;
}

export function NewTaskModal({ open, onClose, onCreate }: NewTaskModalProps) {
	const currentProjectId = useCurrentProjectId();
	const [title, setTitle] = useState('');
	const [description, setDescription] = useState('');
	const [weight, setWeight] = useState<TaskWeight>('medium');
	const [category, setCategory] = useState<TaskCategory>('feature');
	const [saving, setSaving] = useState(false);

	// Reset form when modal opens
	useEffect(() => {
		if (open) {
			setTitle('');
			setDescription('');
			setWeight('medium');
			setCategory('feature');
		}
	}, [open]);

	const handleSave = useCallback(async () => {
		if (!title.trim()) {
			toast.error('Title is required');
			return;
		}

		if (!currentProjectId) {
			toast.error('No project selected');
			return;
		}

		setSaving(true);
		try {
			const task = await createProjectTask(
				currentProjectId,
				title.trim(),
				description.trim() || undefined,
				weight,
				category
			);
			toast.success(`Task ${task.id} created`);
			onCreate?.(task);
			onClose();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to create task');
		} finally {
			setSaving(false);
		}
	}, [currentProjectId, title, description, weight, category, onCreate, onClose]);

	// Handle Enter key to submit
	const handleKeyDown = useCallback(
		(e: React.KeyboardEvent) => {
			if (e.key === 'Enter' && !e.shiftKey && title.trim()) {
				e.preventDefault();
				handleSave();
			}
		},
		[handleSave, title]
	);

	return (
		<Modal open={open} title="New Task" onClose={onClose}>
			<div className="task-edit-form">
				{/* Title */}
				<div className="form-group">
					<label htmlFor="new-task-title">Title</label>
					<input
						id="new-task-title"
						type="text"
						value={title}
						onChange={(e) => setTitle(e.target.value)}
						onKeyDown={handleKeyDown}
						placeholder="What needs to be done?"
						autoFocus
					/>
				</div>

				{/* Description */}
				<div className="form-group">
					<label htmlFor="new-task-description">Description</label>
					<textarea
						id="new-task-description"
						value={description}
						onChange={(e) => setDescription(e.target.value)}
						placeholder="Optional description or context"
						rows={3}
					/>
				</div>

				{/* Weight & Category Row */}
				<div className="form-row">
					<div className="form-group">
						<label htmlFor="new-task-weight">Weight</label>
						<select
							id="new-task-weight"
							value={weight}
							onChange={(e) => setWeight(e.target.value as TaskWeight)}
						>
							{WEIGHTS.map((w) => (
								<option key={w} value={w}>
									{w}
								</option>
							))}
						</select>
						<span className="form-hint">Determines execution phases</span>
					</div>

					<div className="form-group">
						<label htmlFor="new-task-category">Category</label>
						<select
							id="new-task-category"
							value={category}
							onChange={(e) => setCategory(e.target.value as TaskCategory)}
						>
							{CATEGORIES.map((c) => (
								<option key={c} value={c}>
									{c}
								</option>
							))}
						</select>
					</div>
				</div>

				{/* Actions */}
				<div className="form-actions">
					<button
						type="button"
						className="cancel-btn"
						onClick={onClose}
						disabled={saving}
					>
						Cancel
					</button>
					<button
						type="button"
						className="save-btn"
						onClick={handleSave}
						disabled={saving || !title.trim() || !currentProjectId}
					>
						{saving ? 'Creating...' : 'Create Task'}
					</button>
				</div>
			</div>
		</Modal>
	);
}
