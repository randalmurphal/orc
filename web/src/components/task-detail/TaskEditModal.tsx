import { useState, useCallback } from 'react';
import { Button } from '@/components/ui/Button';
import { Modal } from '@/components/overlays/Modal';
import { updateTask } from '@/lib/api';
import { toast } from '@/stores/uiStore';
import type { Task, TaskWeight, TaskPriority, TaskCategory, TaskQueue } from '@/lib/types';
import './TaskEditModal.css';

const WEIGHTS: TaskWeight[] = ['trivial', 'small', 'medium', 'large', 'greenfield'];
const PRIORITIES: TaskPriority[] = ['critical', 'high', 'normal', 'low'];
const CATEGORIES: TaskCategory[] = ['feature', 'bug', 'refactor', 'chore', 'docs', 'test'];
const QUEUES: TaskQueue[] = ['active', 'backlog'];

interface TaskEditModalProps {
	open: boolean;
	task: Task;
	onClose: () => void;
	onUpdate: (task: Task) => void;
}

export function TaskEditModal({ open, task, onClose, onUpdate }: TaskEditModalProps) {
	const [title, setTitle] = useState(task.title);
	const [description, setDescription] = useState(task.description ?? '');
	const [weight, setWeight] = useState<TaskWeight>(task.weight);
	const [priority, setPriority] = useState<TaskPriority>(task.priority ?? 'normal');
	const [category, setCategory] = useState<TaskCategory>(task.category ?? 'feature');
	const [queue, setQueue] = useState<TaskQueue>(task.queue ?? 'active');
	const [saving, setSaving] = useState(false);

	const handleSave = useCallback(async () => {
		if (!title.trim()) {
			toast.error('Title is required');
			return;
		}

		setSaving(true);
		try {
			const updated = await updateTask(task.id, {
				title: title.trim(),
				description: description.trim() || undefined,
				weight,
				priority,
				category,
				queue,
			});
			toast.success('Task updated');
			onUpdate(updated);
			onClose();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to update task');
		} finally {
			setSaving(false);
		}
	}, [task.id, title, description, weight, priority, category, queue, onUpdate, onClose]);

	return (
		<Modal open={open} title="Edit Task" onClose={onClose}>
			<div className="task-edit-form">
				{/* Title */}
				<div className="form-group">
					<label htmlFor="task-title">Title</label>
					<input
						id="task-title"
						type="text"
						value={title}
						onChange={(e) => setTitle(e.target.value)}
						placeholder="Task title"
						autoFocus
					/>
				</div>

				{/* Description */}
				<div className="form-group">
					<label htmlFor="task-description">Description</label>
					<textarea
						id="task-description"
						value={description}
						onChange={(e) => setDescription(e.target.value)}
						placeholder="Optional description"
						rows={3}
					/>
				</div>

				{/* Weight & Priority Row */}
				<div className="form-row">
					<div className="form-group">
						<label htmlFor="task-weight">Weight</label>
						<select
							id="task-weight"
							value={weight}
							onChange={(e) => setWeight(e.target.value as TaskWeight)}
						>
							{WEIGHTS.map((w) => (
								<option key={w} value={w}>
									{w}
								</option>
							))}
						</select>
					</div>

					<div className="form-group">
						<label htmlFor="task-priority">Priority</label>
						<select
							id="task-priority"
							value={priority}
							onChange={(e) => setPriority(e.target.value as TaskPriority)}
						>
							{PRIORITIES.map((p) => (
								<option key={p} value={p}>
									{p}
								</option>
							))}
						</select>
					</div>
				</div>

				{/* Category & Queue Row */}
				<div className="form-row">
					<div className="form-group">
						<label htmlFor="task-category">Category</label>
						<select
							id="task-category"
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

					<div className="form-group">
						<label htmlFor="task-queue">Queue</label>
						<select
							id="task-queue"
							value={queue}
							onChange={(e) => setQueue(e.target.value as TaskQueue)}
						>
							{QUEUES.map((q) => (
								<option key={q} value={q}>
									{q}
								</option>
							))}
						</select>
					</div>
				</div>

				{/* Actions */}
				<div className="form-actions">
					<Button
						variant="secondary"
						size="md"
						onClick={onClose}
						disabled={saving}
					>
						Cancel
					</Button>
					<Button
						variant="primary"
						size="md"
						onClick={handleSave}
						loading={saving}
						disabled={!title.trim()}
					>
						Save Changes
					</Button>
				</div>
			</div>
		</Modal>
	);
}
