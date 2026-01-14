/**
 * TaskEditModal - Edit task properties
 *
 * Features:
 * - Edit title, description, weight, category, queue, priority
 * - Detects changes from original task
 * - Keyboard shortcut: Cmd/Ctrl+Enter to save
 */

import { useState, useEffect, useCallback, useMemo } from 'react';
import { Modal } from './Modal';
import { isMac } from '@/lib/platform';
import type { Task, TaskWeight, TaskQueue, TaskPriority, TaskCategory } from '@/lib/types';
import { CATEGORY_CONFIG } from '@/lib/types';
import './TaskEditModal.css';

interface TaskEditModalProps {
	task: Task;
	open: boolean;
	onClose: () => void;
	onSave: (update: {
		title?: string;
		description?: string;
		weight?: TaskWeight;
		queue?: TaskQueue;
		priority?: TaskPriority;
		category?: TaskCategory;
	}) => Promise<void>;
}

const WEIGHT_OPTIONS: { value: TaskWeight; label: string; description: string }[] = [
	{ value: 'trivial', label: 'Trivial', description: 'One-liner fix' },
	{ value: 'small', label: 'Small', description: 'Bug fix, small feature' },
	{ value: 'medium', label: 'Medium', description: 'Feature with tests' },
	{ value: 'large', label: 'Large', description: 'Complex feature' },
	{ value: 'greenfield', label: 'Greenfield', description: 'New system' },
];

const QUEUE_OPTIONS: { value: TaskQueue; label: string }[] = [
	{ value: 'active', label: 'Active' },
	{ value: 'backlog', label: 'Backlog' },
];

const PRIORITY_OPTIONS: { value: TaskPriority; label: string; color: string }[] = [
	{ value: 'critical', label: 'Critical', color: 'var(--status-error)' },
	{ value: 'high', label: 'High', color: 'var(--status-warning)' },
	{ value: 'normal', label: 'Normal', color: 'var(--text-muted)' },
	{ value: 'low', label: 'Low', color: 'var(--text-disabled)' },
];

const CATEGORY_OPTIONS: { value: TaskCategory; label: string; icon: string; color: string }[] = [
	{ value: 'feature', label: CATEGORY_CONFIG.feature.label, icon: CATEGORY_CONFIG.feature.icon, color: CATEGORY_CONFIG.feature.color },
	{ value: 'bug', label: CATEGORY_CONFIG.bug.label, icon: CATEGORY_CONFIG.bug.icon, color: CATEGORY_CONFIG.bug.color },
	{ value: 'refactor', label: CATEGORY_CONFIG.refactor.label, icon: CATEGORY_CONFIG.refactor.icon, color: CATEGORY_CONFIG.refactor.color },
	{ value: 'chore', label: CATEGORY_CONFIG.chore.label, icon: CATEGORY_CONFIG.chore.icon, color: CATEGORY_CONFIG.chore.color },
	{ value: 'docs', label: CATEGORY_CONFIG.docs.label, icon: CATEGORY_CONFIG.docs.icon, color: CATEGORY_CONFIG.docs.color },
	{ value: 'test', label: CATEGORY_CONFIG.test.label, icon: CATEGORY_CONFIG.test.icon, color: CATEGORY_CONFIG.test.color },
];

export function TaskEditModal({ task, open, onClose, onSave }: TaskEditModalProps) {
	// Form state
	const [title, setTitle] = useState(task.title);
	const [description, setDescription] = useState(task.description ?? '');
	const [weight, setWeight] = useState<TaskWeight>(task.weight);
	const [queue, setQueue] = useState<TaskQueue>(task.queue ?? 'active');
	const [priority, setPriority] = useState<TaskPriority>(task.priority ?? 'normal');
	const [category, setCategory] = useState<TaskCategory>(task.category ?? 'feature');

	// UI state
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);

	const modifierKey = isMac() ? 'Cmd' : 'Ctrl';

	// Reset form when task changes or modal opens
	useEffect(() => {
		if (open) {
			setTitle(task.title);
			setDescription(task.description ?? '');
			setWeight(task.weight);
			setQueue(task.queue ?? 'active');
			setPriority(task.priority ?? 'normal');
			setCategory(task.category ?? 'feature');
			setError(null);
		}
	}, [open, task]);

	// Detect changes
	const hasChanges = useMemo(() => {
		return (
			title !== task.title ||
			description !== (task.description ?? '') ||
			weight !== task.weight ||
			queue !== (task.queue ?? 'active') ||
			priority !== (task.priority ?? 'normal') ||
			category !== (task.category ?? 'feature')
		);
	}, [title, description, weight, queue, priority, category, task]);

	const canSubmit = title.trim().length > 0 && hasChanges && !loading;

	// Handle form submission
	const handleSubmit = useCallback(async (e?: React.FormEvent) => {
		e?.preventDefault();
		if (!canSubmit) return;

		setLoading(true);
		setError(null);

		try {
			const update: {
				title?: string;
				description?: string;
				weight?: TaskWeight;
				queue?: TaskQueue;
				priority?: TaskPriority;
				category?: TaskCategory;
			} = {};

			if (title !== task.title) update.title = title.trim();
			if (description !== (task.description ?? '')) update.description = description.trim();
			if (weight !== task.weight) update.weight = weight;
			if (queue !== (task.queue ?? 'active')) update.queue = queue;
			if (priority !== (task.priority ?? 'normal')) update.priority = priority;
			if (category !== (task.category ?? 'feature')) update.category = category;

			await onSave(update);
			onClose();
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to update task');
		} finally {
			setLoading(false);
		}
	}, [canSubmit, title, description, weight, queue, priority, category, task, onSave, onClose]);

	// Handle Cmd/Ctrl+Enter shortcut
	const handleKeyDown = useCallback(
		(e: React.KeyboardEvent) => {
			if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
				e.preventDefault();
				handleSubmit();
			}
		},
		[handleSubmit]
	);

	return (
		<Modal open={open} onClose={onClose} title="Edit Task" size="md">
			<form className="task-edit-form" onSubmit={handleSubmit} onKeyDown={handleKeyDown}>
				{error && (
					<div className="error-banner" role="alert">
						{error}
					</div>
				)}

				{/* Title */}
				<div className="form-field">
					<label htmlFor="task-title">Title</label>
					<input
						id="task-title"
						type="text"
						value={title}
						onChange={(e) => setTitle(e.target.value)}
						placeholder="Task title"
						disabled={loading}
						required
					/>
				</div>

				{/* Description */}
				<div className="form-field">
					<label htmlFor="task-description">Description</label>
					<textarea
						id="task-description"
						value={description}
						onChange={(e) => setDescription(e.target.value)}
						placeholder="Describe what needs to be done..."
						rows={4}
						disabled={loading}
					/>
				</div>

				{/* Weight */}
				<div className="form-field">
					<label id="weight-label">Weight</label>
					<div className="weight-options" role="radiogroup" aria-labelledby="weight-label">
						{WEIGHT_OPTIONS.map((option) => (
							<label
								key={option.value}
								className={`weight-option ${weight === option.value ? `selected ${option.value}` : ''}`}
							>
								<input
									type="radio"
									name="weight"
									value={option.value}
									checked={weight === option.value}
									onChange={() => setWeight(option.value)}
									disabled={loading}
								/>
								<span className="weight-label">{option.label}</span>
								<span className="weight-desc">{option.description}</span>
							</label>
						))}
					</div>
				</div>

				{/* Category */}
				<div className="form-field">
					<label id="category-label">Category</label>
					<div className="category-options" role="radiogroup" aria-labelledby="category-label">
						{CATEGORY_OPTIONS.map((option) => (
							<label
								key={option.value}
								className={`category-option ${category === option.value ? 'selected' : ''}`}
								style={{ '--category-color': option.color } as React.CSSProperties}
							>
								<input
									type="radio"
									name="category"
									value={option.value}
									checked={category === option.value}
									onChange={() => setCategory(option.value)}
									disabled={loading}
								/>
								<span className="category-icon">{option.icon}</span>
								<span className="category-label-text">{option.label}</span>
							</label>
						))}
					</div>
				</div>

				{/* Queue and Priority row */}
				<div className="form-row">
					{/* Queue */}
					<div className="form-field flex-1">
						<label id="queue-label">Queue</label>
						<div className="toggle-options" role="radiogroup" aria-labelledby="queue-label">
							{QUEUE_OPTIONS.map((option) => (
								<label
									key={option.value}
									className={`toggle-option ${queue === option.value ? 'selected' : ''} ${queue === 'backlog' && option.value === 'backlog' ? 'backlog' : ''}`}
								>
									<input
										type="radio"
										name="queue"
										value={option.value}
										checked={queue === option.value}
										onChange={() => setQueue(option.value)}
										disabled={loading}
									/>
									<span className="toggle-label">{option.label}</span>
								</label>
							))}
						</div>
					</div>

					{/* Priority */}
					<div className="form-field flex-1">
						<label id="priority-label">Priority</label>
						<div className="priority-options" role="radiogroup" aria-labelledby="priority-label">
							{PRIORITY_OPTIONS.map((option) => (
								<label
									key={option.value}
									className={`priority-option ${priority === option.value ? 'selected' : ''}`}
									style={{ '--priority-color': option.color } as React.CSSProperties}
								>
									<input
										type="radio"
										name="priority"
										value={option.value}
										checked={priority === option.value}
										onChange={() => setPriority(option.value)}
										disabled={loading}
									/>
									<span
										className="priority-indicator"
										style={{ background: option.color }}
									/>
									<span className="priority-label">{option.label}</span>
								</label>
							))}
						</div>
					</div>
				</div>

				{/* Actions */}
				<div className="form-actions">
					<button type="button" className="btn-ghost" onClick={onClose} disabled={loading}>
						Cancel
					</button>
					<button type="submit" className="btn-primary" disabled={!canSubmit}>
						{loading ? 'Saving...' : 'Save Changes'}
					</button>
				</div>

				<div className="keyboard-hint">
					<kbd>{modifierKey}</kbd> + <kbd>Enter</kbd> to save
				</div>
			</form>
		</Modal>
	);
}
