import { useState, useCallback, useMemo, useEffect } from 'react';
import * as Select from '@radix-ui/react-select';
import { Modal } from '@/components/overlays/Modal';
import { Icon } from '@/components/ui/Icon';
import { updateTask } from '@/lib/api';
import { toast } from '@/stores/uiStore';
import { useInitiatives } from '@/stores';
import type { Task, TaskWeight, TaskPriority, TaskCategory, TaskQueue } from '@/lib/types';
import './TaskEditModal.css';

const WEIGHTS: TaskWeight[] = ['trivial', 'small', 'medium', 'large'];
const PRIORITIES: TaskPriority[] = ['critical', 'high', 'normal', 'low'];
const CATEGORIES: TaskCategory[] = ['feature', 'bug', 'refactor', 'chore', 'docs', 'test'];
const QUEUES: TaskQueue[] = ['active', 'backlog'];

// Internal value for "no initiative" since Radix Select requires string values
const NO_INITIATIVE_VALUE = '__none__';

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
	const [initiativeId, setInitiativeId] = useState<string | undefined>(task.initiative_id);
	const [targetBranch, setTargetBranch] = useState(task.target_branch ?? '');
	const [saving, setSaving] = useState(false);

	const initiatives = useInitiatives();

	// Reset form when modal opens or task changes
	useEffect(() => {
		if (open) {
			setTitle(task.title);
			setDescription(task.description ?? '');
			setWeight(task.weight);
			setPriority(task.priority ?? 'normal');
			setCategory(task.category ?? 'feature');
			setQueue(task.queue ?? 'active');
			setInitiativeId(task.initiative_id);
			setTargetBranch(task.target_branch ?? '');
		}
	}, [open, task]);

	// Sort initiatives: active first, then by title
	const sortedInitiatives = useMemo(() => {
		return [...initiatives].sort((a, b) => {
			// Active first
			if (a.status === 'active' && b.status !== 'active') return -1;
			if (b.status === 'active' && a.status !== 'active') return 1;
			// Then by title
			return a.title.localeCompare(b.title);
		});
	}, [initiatives]);

	// Convert external value (undefined for none) to internal Select value
	const selectInitiativeValue = initiativeId ?? NO_INITIATIVE_VALUE;

	// Handle initiative selection change
	const handleInitiativeChange = (value: string) => {
		if (value === NO_INITIATIVE_VALUE) {
			setInitiativeId(undefined);
		} else {
			setInitiativeId(value);
		}
	};

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
				initiative_id: initiativeId || '', // Empty string to clear initiative
				target_branch: targetBranch.trim() || undefined,
			});
			toast.success('Task updated');
			onUpdate(updated);
			onClose();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to update task');
		} finally {
			setSaving(false);
		}
	}, [task.id, title, description, weight, priority, category, queue, initiativeId, targetBranch, onUpdate, onClose]);

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

				{/* Initiative */}
				<div className="form-group">
					<label htmlFor="task-initiative">Initiative</label>
					<Select.Root value={selectInitiativeValue} onValueChange={handleInitiativeChange}>
						<Select.Trigger
							id="task-initiative"
							className="initiative-select-trigger"
							aria-label="Select initiative"
						>
							<Select.Value placeholder="None" />
							<Select.Icon className="initiative-select-icon">
								<Icon name="chevron-down" size={14} />
							</Select.Icon>
						</Select.Trigger>

						<Select.Portal>
							<Select.Content
								className="initiative-select-content"
								position="popper"
								sideOffset={4}
							>
								<Select.Viewport className="initiative-select-viewport">
									{/* No initiative option */}
									<Select.Item value={NO_INITIATIVE_VALUE} className="initiative-select-item">
										<Select.ItemText>None</Select.ItemText>
									</Select.Item>

									{sortedInitiatives.length > 0 && (
										<Select.Separator className="initiative-select-separator" />
									)}

									{/* Initiative list */}
									{sortedInitiatives.map((init) => (
										<Select.Item
											key={init.id}
											value={init.id}
											className="initiative-select-item"
										>
											<Select.ItemText>{init.title}</Select.ItemText>
											{init.status !== 'active' && (
												<span className="initiative-status-badge">{init.status}</span>
											)}
										</Select.Item>
									))}
								</Select.Viewport>
							</Select.Content>
						</Select.Portal>
					</Select.Root>
					<span className="form-hint">
						Assign task to an initiative for grouping and branch targeting
					</span>
				</div>

				{/* Target Branch */}
				<div className="form-group">
					<label htmlFor="task-target-branch">Target Branch</label>
					<input
						id="task-target-branch"
						type="text"
						value={targetBranch}
						onChange={(e) => setTargetBranch(e.target.value)}
						placeholder="Override PR target branch (e.g., hotfix/v2.1)"
					/>
					<span className="form-hint">
						Leave empty to use initiative branch or project default
					</span>
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
						disabled={saving || !title.trim()}
					>
						{saving ? 'Saving...' : 'Save Changes'}
					</button>
				</div>
			</div>
		</Modal>
	);
}
