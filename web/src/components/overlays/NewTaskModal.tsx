import { useState, useCallback, useEffect } from 'react';
import { Modal } from './Modal';
import { taskClient } from '@/lib/client';
import { create } from '@bufbuild/protobuf';
import { toast } from '@/stores/uiStore';
import { useCurrentProjectId } from '@/stores';
import {
	type Task,
	TaskWeight,
	TaskCategory,
	CreateTaskRequestSchema,
} from '@/gen/orc/v1/task_pb';
import '../task-detail/TaskEditModal.css';

// Weight options with enum values and display labels
const WEIGHT_OPTIONS = [
	{ value: TaskWeight.TRIVIAL, label: 'trivial' },
	{ value: TaskWeight.SMALL, label: 'small' },
	{ value: TaskWeight.MEDIUM, label: 'medium' },
	{ value: TaskWeight.LARGE, label: 'large' },
] as const;

// Category options with enum values and display labels
const CATEGORY_OPTIONS = [
	{ value: TaskCategory.FEATURE, label: 'feature' },
	{ value: TaskCategory.BUG, label: 'bug' },
	{ value: TaskCategory.REFACTOR, label: 'refactor' },
	{ value: TaskCategory.CHORE, label: 'chore' },
	{ value: TaskCategory.DOCS, label: 'docs' },
	{ value: TaskCategory.TEST, label: 'test' },
] as const;

interface NewTaskModalProps {
	open: boolean;
	onClose: () => void;
	onCreate?: (task: Task) => void;
}

export function NewTaskModal({ open, onClose, onCreate }: NewTaskModalProps) {
	const currentProjectId = useCurrentProjectId();
	const [title, setTitle] = useState('');
	const [description, setDescription] = useState('');
	const [weight, setWeight] = useState<TaskWeight>(TaskWeight.MEDIUM);
	const [category, setCategory] = useState<TaskCategory>(TaskCategory.FEATURE);
	const [saving, setSaving] = useState(false);

	// Reset form when modal opens
	useEffect(() => {
		if (open) {
			setTitle('');
			setDescription('');
			setWeight(TaskWeight.MEDIUM);
			setCategory(TaskCategory.FEATURE);
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
			const response = await taskClient.createTask(
				create(CreateTaskRequestSchema, {
					title: title.trim(),
					description: description.trim() || undefined,
					weight,
					category,
				})
			);
			if (response.task) {
				toast.success(`Task ${response.task.id} created`);
				onCreate?.(response.task);
			}
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
							onChange={(e) => setWeight(Number(e.target.value) as TaskWeight)}
						>
							{WEIGHT_OPTIONS.map((w) => (
								<option key={w.value} value={w.value}>
									{w.label}
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
							onChange={(e) => setCategory(Number(e.target.value) as TaskCategory)}
						>
							{CATEGORY_OPTIONS.map((c) => (
								<option key={c.value} value={c.value}>
									{c.label}
								</option>
							))}
						</select>
						<span className="form-hint">Affects how Claude approaches work</span>
					</div>
				</div>

				{/* Actions */}
				<div className="form-actions">
					<button type="button" onClick={onClose} className="btn-secondary">
						Cancel
					</button>
					<button
						type="button"
						onClick={handleSave}
						disabled={saving || !title.trim()}
						className="btn-primary"
					>
						{saving ? 'Creating...' : 'Create Task'}
					</button>
				</div>
			</div>
		</Modal>
	);
}
