import { useState, useCallback, useEffect, useMemo } from 'react';
import * as Select from '@radix-ui/react-select';
import { Modal } from './Modal';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import { taskClient, workflowClient } from '@/lib/client';
import { create } from '@bufbuild/protobuf';
import { toast } from '@/stores/uiStore';
import { useCurrentProjectId } from '@/stores';
import {
	type Task,
	TaskWeight,
	TaskCategory,
	CreateTaskRequestSchema,
} from '@/gen/orc/v1/task_pb';
import type { Workflow } from '@/gen/orc/v1/workflow_pb';
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

// Map weight enum values to workflow IDs
const WEIGHT_TO_WORKFLOW: Record<TaskWeight, string | undefined> = {
	[TaskWeight.UNSPECIFIED]: undefined,
	[TaskWeight.TRIVIAL]: 'trivial',
	[TaskWeight.SMALL]: 'small',
	[TaskWeight.MEDIUM]: 'medium',
	[TaskWeight.LARGE]: 'large',
};

// Internal value for "no workflow" since Radix Select requires string values
const NO_WORKFLOW_VALUE = '__none__';

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
	const [workflowId, setWorkflowId] = useState<string | undefined>('medium');
	const [manualWorkflowSelection, setManualWorkflowSelection] = useState(false);
	const [saving, setSaving] = useState(false);

	// Workflow loading state
	const [workflows, setWorkflows] = useState<Workflow[]>([]);
	const [workflowsLoading, setWorkflowsLoading] = useState(false);
	const [workflowsError, setWorkflowsError] = useState<string | null>(null);

	const loadWorkflows = useCallback(async () => {
		setWorkflowsLoading(true);
		setWorkflowsError(null);
		try {
			const response = await workflowClient.listWorkflows({
				includeBuiltin: true,
			});
			setWorkflows(response.workflows);
		} catch (e) {
			setWorkflowsError('Failed to load workflows');
			console.error('Failed to load workflows:', e);
		} finally {
			setWorkflowsLoading(false);
		}
	}, []);

	// Load workflows when modal opens
	useEffect(() => {
		if (open) {
			loadWorkflows();
		}
	}, [open, loadWorkflows]);

	// Reset form when modal opens
	useEffect(() => {
		if (open) {
			setTitle('');
			setDescription('');
			setWeight(TaskWeight.MEDIUM);
			setCategory(TaskCategory.FEATURE);
			setWorkflowId('medium');
			setManualWorkflowSelection(false);
		}
	}, [open]);

	// Auto-select workflow when weight changes (unless manually selected)
	useEffect(() => {
		if (!manualWorkflowSelection) {
			const newWorkflowId = WEIGHT_TO_WORKFLOW[weight];
			setWorkflowId(newWorkflowId);
		}
	}, [weight, manualWorkflowSelection]);

	// Convert internal Select value (string) to external workflow ID
	const selectWorkflowValue = workflowId ?? NO_WORKFLOW_VALUE;

	// Handle workflow selection change
	const handleWorkflowChange = useCallback((value: string) => {
		setManualWorkflowSelection(true);
		if (value === NO_WORKFLOW_VALUE) {
			setWorkflowId(undefined);
		} else {
			setWorkflowId(value);
		}
	}, []);

	// Handle weight change
	const handleWeightChange = useCallback((newWeight: TaskWeight) => {
		setWeight(newWeight);
		// Don't reset manualWorkflowSelection here - that's preserved
	}, []);

	// Sort workflows: builtin first, then by name
	const sortedWorkflows = useMemo(() => {
		return [...workflows].sort((a, b) => {
			if (a.isBuiltin && !b.isBuiltin) return -1;
			if (b.isBuiltin && !a.isBuiltin) return 1;
			return a.name.localeCompare(b.name);
		});
	}, [workflows]);

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
					projectId: currentProjectId,
					title: title.trim(),
					description: description.trim() || undefined,
					weight,
					category,
					workflowId: workflowId || undefined,
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
	}, [currentProjectId, title, description, weight, category, workflowId, onCreate, onClose]);

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
							onChange={(e) => handleWeightChange(Number(e.target.value) as TaskWeight)}
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

				{/* Workflow */}
				<div className="form-group">
					<label htmlFor="new-task-workflow">Workflow</label>
					{workflowsError ? (
						<div className="workflow-error">
							<span>Failed to load workflows</span>
							<Button type="button" variant="ghost" size="sm" onClick={loadWorkflows}>
								Retry
							</Button>
						</div>
					) : workflows.length === 0 && !workflowsLoading ? (
						<div className="workflow-empty">No workflows available</div>
					) : (
						<Select.Root value={selectWorkflowValue} onValueChange={handleWorkflowChange}>
							<Select.Trigger
								id="new-task-workflow"
								className="initiative-select-trigger"
								aria-label="Workflow"
								disabled={workflowsLoading}
							>
								<Select.Value placeholder={workflowsLoading ? 'Loading...' : 'None'} />
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
										{/* No workflow option */}
										<Select.Item value={NO_WORKFLOW_VALUE} className="initiative-select-item">
											<Select.ItemText>None</Select.ItemText>
										</Select.Item>

										{sortedWorkflows.length > 0 && (
											<Select.Separator className="initiative-select-separator" />
										)}

										{/* Workflow list */}
										{sortedWorkflows.map((wf) => (
											<Select.Item
												key={wf.id}
												value={wf.id}
												className="initiative-select-item"
											>
												<Select.ItemText>{wf.name}</Select.ItemText>
												{!wf.isBuiltin && (
													<span className="initiative-status-badge">custom</span>
												)}
											</Select.Item>
										))}
									</Select.Viewport>
								</Select.Content>
							</Select.Portal>
						</Select.Root>
					)}
					<span className="form-hint">
						Workflow controls which phases the task executes
					</span>
				</div>

				{/* Actions */}
				<div className="form-actions">
					<Button type="button" variant="secondary" onClick={onClose}>
						Cancel
					</Button>
					<Button
						type="button"
						variant="primary"
						onClick={handleSave}
						disabled={!title.trim()}
						loading={saving}
					>
						Create Task
					</Button>
				</div>
			</div>
		</Modal>
	);
}
