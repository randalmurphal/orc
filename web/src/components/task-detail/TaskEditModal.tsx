import { useState, useCallback, useMemo, useEffect } from 'react';
import * as Select from '@radix-ui/react-select';
import { Modal } from '@/components/overlays/Modal';
import { Icon } from '@/components/ui/Icon';
import { taskClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';
import { useInitiatives } from '@/stores';
import {
	type Task,
	TaskWeight,
	TaskPriority,
	TaskCategory,
	TaskQueue,
} from '@/gen/orc/v1/task_pb';
import { InitiativeStatus } from '@/gen/orc/v1/initiative_pb';
import './TaskEditModal.css';

const WEIGHTS: TaskWeight[] = [
	TaskWeight.TRIVIAL,
	TaskWeight.SMALL,
	TaskWeight.MEDIUM,
	TaskWeight.LARGE,
];
const PRIORITIES: TaskPriority[] = [
	TaskPriority.CRITICAL,
	TaskPriority.HIGH,
	TaskPriority.NORMAL,
	TaskPriority.LOW,
];
const CATEGORIES: TaskCategory[] = [
	TaskCategory.FEATURE,
	TaskCategory.BUG,
	TaskCategory.REFACTOR,
	TaskCategory.CHORE,
	TaskCategory.DOCS,
	TaskCategory.TEST,
];
const QUEUES: TaskQueue[] = [TaskQueue.ACTIVE, TaskQueue.BACKLOG];

// Labels for enum values
const WEIGHT_LABELS: Record<TaskWeight, string> = {
	[TaskWeight.UNSPECIFIED]: 'unspecified',
	[TaskWeight.TRIVIAL]: 'trivial',
	[TaskWeight.SMALL]: 'small',
	[TaskWeight.MEDIUM]: 'medium',
	[TaskWeight.LARGE]: 'large',
};
const PRIORITY_LABELS: Record<TaskPriority, string> = {
	[TaskPriority.UNSPECIFIED]: 'unspecified',
	[TaskPriority.CRITICAL]: 'critical',
	[TaskPriority.HIGH]: 'high',
	[TaskPriority.NORMAL]: 'normal',
	[TaskPriority.LOW]: 'low',
};
const CATEGORY_LABELS: Record<TaskCategory, string> = {
	[TaskCategory.UNSPECIFIED]: 'unspecified',
	[TaskCategory.FEATURE]: 'feature',
	[TaskCategory.BUG]: 'bug',
	[TaskCategory.REFACTOR]: 'refactor',
	[TaskCategory.CHORE]: 'chore',
	[TaskCategory.DOCS]: 'docs',
	[TaskCategory.TEST]: 'test',
};
const QUEUE_LABELS: Record<TaskQueue, string> = {
	[TaskQueue.UNSPECIFIED]: 'unspecified',
	[TaskQueue.ACTIVE]: 'active',
	[TaskQueue.BACKLOG]: 'backlog',
};
const INITIATIVE_STATUS_LABELS: Record<InitiativeStatus, string> = {
	[InitiativeStatus.UNSPECIFIED]: 'unspecified',
	[InitiativeStatus.DRAFT]: 'draft',
	[InitiativeStatus.ACTIVE]: 'active',
	[InitiativeStatus.COMPLETED]: 'completed',
	[InitiativeStatus.ARCHIVED]: 'archived',
};

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
	const [priority, setPriority] = useState<TaskPriority>(task.priority || TaskPriority.NORMAL);
	const [category, setCategory] = useState<TaskCategory>(task.category || TaskCategory.FEATURE);
	const [queue, setQueue] = useState<TaskQueue>(task.queue || TaskQueue.ACTIVE);
	const [initiativeId, setInitiativeId] = useState<string | undefined>(task.initiativeId);
	const [targetBranch, setTargetBranch] = useState(task.targetBranch ?? '');
	const [saving, setSaving] = useState(false);

	const initiatives = useInitiatives();

	// Reset form when modal opens or task changes
	useEffect(() => {
		if (open) {
			setTitle(task.title);
			setDescription(task.description ?? '');
			setWeight(task.weight);
			setPriority(task.priority || TaskPriority.NORMAL);
			setCategory(task.category || TaskCategory.FEATURE);
			setQueue(task.queue || TaskQueue.ACTIVE);
			setInitiativeId(task.initiativeId);
			setTargetBranch(task.targetBranch ?? '');
		}
	}, [open, task]);

	// Sort initiatives: active first, then by title
	const sortedInitiatives = useMemo(() => {
		return [...initiatives].sort((a, b) => {
			// Active first
			if (a.status === InitiativeStatus.ACTIVE && b.status !== InitiativeStatus.ACTIVE) return -1;
			if (b.status === InitiativeStatus.ACTIVE && a.status !== InitiativeStatus.ACTIVE) return 1;
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
			const response = await taskClient.updateTask({
				id: task.id,
				title: title.trim(),
				description: description.trim() || undefined,
				weight,
				priority,
				category,
				queue,
				initiativeId: initiativeId || undefined,
				targetBranch: targetBranch.trim() || undefined,
			});
			toast.success('Task updated');
			if (response.task) {
				onUpdate(response.task);
			}
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
							value={String(weight)}
							onChange={(e) => setWeight(Number(e.target.value) as TaskWeight)}
						>
							{WEIGHTS.map((w) => (
								<option key={w} value={String(w)}>
									{WEIGHT_LABELS[w]}
								</option>
							))}
						</select>
					</div>

					<div className="form-group">
						<label htmlFor="task-priority">Priority</label>
						<select
							id="task-priority"
							value={String(priority)}
							onChange={(e) => setPriority(Number(e.target.value) as TaskPriority)}
						>
							{PRIORITIES.map((p) => (
								<option key={p} value={String(p)}>
									{PRIORITY_LABELS[p]}
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
							value={String(category)}
							onChange={(e) => setCategory(Number(e.target.value) as TaskCategory)}
						>
							{CATEGORIES.map((c) => (
								<option key={c} value={String(c)}>
									{CATEGORY_LABELS[c]}
								</option>
							))}
						</select>
					</div>

					<div className="form-group">
						<label htmlFor="task-queue">Queue</label>
						<select
							id="task-queue"
							value={String(queue)}
							onChange={(e) => setQueue(Number(e.target.value) as TaskQueue)}
						>
							{QUEUES.map((q) => (
								<option key={q} value={String(q)}>
									{QUEUE_LABELS[q]}
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
											{init.status !== InitiativeStatus.ACTIVE && (
												<span className="initiative-status-badge">{INITIATIVE_STATUS_LABELS[init.status]}</span>
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
