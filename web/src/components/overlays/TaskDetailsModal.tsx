/**
 * TaskDetailsModal - Step 2 of workflow-first task creation
 *
 * Allows users to enter task details after selecting a workflow.
 * Includes basic fields (title, description) and advanced options
 * (category, priority, queue, initiative, PR settings, etc.)
 */

import { useState, useEffect, useRef, useCallback } from 'react';
import { Modal } from './Modal';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import { taskClient } from '@/lib/client';
import { useCurrentProjectId, useInitiatives, toast } from '@/stores';
import { TaskWeight, TaskCategory, TaskPriority, TaskQueue, type Task as ProtobufTask } from '@/gen/orc/v1/task_pb';

export type Task = ProtobufTask;

export interface WorkflowWithPhaseCount {
	id: string;
	name: string;
	description?: string;
	isBuiltin: boolean;
	phaseCount: number;
}

export interface TaskDetailsModalProps {
	open: boolean;
	selectedWorkflow: WorkflowWithPhaseCount;
	onClose: () => void;
	onBack: () => void;
	onTaskCreated: (task: Task, wasRun: boolean) => void;
}

// Helper function to derive weight from workflow ID
function deriveWeightFromWorkflow(workflowId: string): TaskWeight {
	const lowerWorkflowId = workflowId.toLowerCase();
	if (lowerWorkflowId.includes('trivial')) return TaskWeight.TRIVIAL;
	if (lowerWorkflowId.includes('small')) return TaskWeight.SMALL;
	if (lowerWorkflowId.includes('medium')) return TaskWeight.MEDIUM;
	if (lowerWorkflowId.includes('large')) return TaskWeight.LARGE;
	// Default fallback
	return TaskWeight.SMALL;
}

export function TaskDetailsModal({
	open,
	selectedWorkflow,
	onClose,
	onBack,
	onTaskCreated
}: TaskDetailsModalProps) {
	const [title, setTitle] = useState('');
	const [description, setDescription] = useState('');
	const [isAdvancedOpen, setIsAdvancedOpen] = useState(false);

	// Advanced fields
	const [category, setCategory] = useState(TaskCategory.FEATURE);
	const [priority, setPriority] = useState(TaskPriority.NORMAL);
	const [queue, setQueue] = useState(TaskQueue.ACTIVE);
	const [initiativeId, setInitiativeId] = useState<string | undefined>(undefined);
	const [targetBranch, setTargetBranch] = useState('');
	const [branchName, setBranchName] = useState('');
	const [prDraft, setPrDraft] = useState<boolean | undefined>(undefined);
	const [prLabels, setPrLabels] = useState('');
	const [prReviewers, setPrReviewers] = useState('');

	// Loading states
	const [isCreating, setIsCreating] = useState(false);
	const [isCreatingAndRunning, setIsCreatingAndRunning] = useState(false);

	// Get data from stores
	const currentProjectId = useCurrentProjectId();
	const initiatives = useInitiatives();

	// Refs for focus management
	const titleInputRef = useRef<HTMLInputElement>(null);

	const isFormValid = title.trim().length > 0;

	const handleCreate = useCallback(async () => {
		if (!isFormValid || isCreating) return;

		try {
			setIsCreating(true);

			const parsedPrLabels = prLabels.trim() ? prLabels.split(',').map(s => s.trim()).filter(Boolean) : [];
			const parsedPrReviewers = prReviewers.trim() ? prReviewers.split(',').map(s => s.trim()).filter(Boolean) : [];

			const response = await taskClient.createTask({
				projectId: currentProjectId || '',
				title: title.trim(),
				description: description.trim(),
				workflowId: selectedWorkflow.id,
				weight: deriveWeightFromWorkflow(selectedWorkflow.id),
				category,
				priority,
				queue,
				initiativeId: initiativeId || undefined,
				targetBranch: targetBranch.trim(),
				blockedBy: [],
				relatedTo: [],
				metadata: {},
				branchName: branchName.trim(),
				prDraft,
				prLabels: parsedPrLabels,
				prReviewers: parsedPrReviewers,
				prLabelsSet: prLabels.trim().length > 0,
				prReviewersSet: prReviewers.trim().length > 0,
			});

			if (response.task) {
				onTaskCreated(response.task, false);
			}
		} catch (error) {
			const message = error instanceof Error ? error.message : 'Unknown error';
			toast.error(message);
		} finally {
			setIsCreating(false);
		}
	}, [isFormValid, isCreating, prLabels, prReviewers, currentProjectId, title, description, selectedWorkflow.id, category, priority, queue, initiativeId, targetBranch, branchName, prDraft, onTaskCreated]);

	const handleCreateAndRun = async () => {
		if (!isFormValid || isCreatingAndRunning) return;

		try {
			setIsCreatingAndRunning(true);

			const parsedPrLabels = prLabels.trim() ? prLabels.split(',').map(s => s.trim()).filter(Boolean) : [];
			const parsedPrReviewers = prReviewers.trim() ? prReviewers.split(',').map(s => s.trim()).filter(Boolean) : [];

			const response = await taskClient.createTask({
				projectId: currentProjectId || '',
				title: title.trim(),
				description: description.trim(),
				workflowId: selectedWorkflow.id,
				weight: deriveWeightFromWorkflow(selectedWorkflow.id),
				category,
				priority,
				queue,
				initiativeId: initiativeId || undefined,
				targetBranch: targetBranch.trim(),
				blockedBy: [],
				relatedTo: [],
				metadata: {},
				branchName: branchName.trim(),
				prDraft,
				prLabels: parsedPrLabels,
				prReviewers: parsedPrReviewers,
				prLabelsSet: prLabels.trim().length > 0,
				prReviewersSet: prReviewers.trim().length > 0,
			});

			if (response.task) {
				// Try to run the task
				try {
					await taskClient.runTask({
						projectId: currentProjectId || '',
						taskId: response.task.id,
					});
					onTaskCreated(response.task, true);
				} catch (runError) {
					// Task created successfully but run failed
					const runMessage = runError instanceof Error ? runError.message : 'Unknown error';
					toast.error(runMessage);
					onTaskCreated(response.task, false);
				}
			}
		} catch (error) {
			const message = error instanceof Error ? error.message : 'Unknown error';
			toast.error(message);
		} finally {
			setIsCreatingAndRunning(false);
		}
	};

	// Reset form when modal opens
	useEffect(() => {
		if (open) {
			setTitle('');
			setDescription('');
			setIsAdvancedOpen(false);
			setCategory(TaskCategory.FEATURE);
			setPriority(TaskPriority.NORMAL);
			setQueue(TaskQueue.ACTIVE);
			setInitiativeId(undefined);
			setTargetBranch('');
			setBranchName('');
			setPrDraft(undefined);
			setPrLabels('');
			setPrReviewers('');
			setIsCreating(false);
			setIsCreatingAndRunning(false);
		}
	}, [open]);

	// Auto-focus title field when modal opens
	useEffect(() => {
		if (open && titleInputRef.current) {
			titleInputRef.current.focus();
		}
	}, [open]);

	// Handle keyboard navigation
	useEffect(() => {
		if (!open) return;

		const handleKeyDown = (e: KeyboardEvent) => {
			if (e.key === 'Escape') {
				onClose();
			} else if (e.key === 'Enter' && !e.shiftKey) {
				// Only handle Enter if we're not in a textarea
				const target = e.target as HTMLElement;
				if (target.tagName !== 'TEXTAREA' && title.trim()) {
					e.preventDefault();
					handleCreate();
				}
			}
		};

		document.addEventListener('keydown', handleKeyDown);
		return () => document.removeEventListener('keydown', handleKeyDown);
	}, [open, title, onClose, handleCreate]);

	return (
		<Modal open={open} title="New Task" onClose={onClose}>
			<div className="space-y-6">
				{/* Workflow Display */}
				<div className="flex items-center justify-between p-4 bg-gray-50 rounded-lg">
					<div>
						<h3 className="font-medium text-gray-900">{selectedWorkflow.name}</h3>
						{selectedWorkflow.description && (
							<p className="text-sm text-gray-600 mt-1">{selectedWorkflow.description}</p>
						)}
					</div>
					<Button variant="ghost" size="sm" onClick={onBack}>
						Change
					</Button>
				</div>

				{/* Basic Fields */}
				<div className="space-y-4">
					<div>
						<label htmlFor="task-title" className="block text-sm font-medium text-gray-700 mb-1">
							Title *
						</label>
						<input
							id="task-title"
							ref={titleInputRef}
							type="text"
							required
							value={title}
							onChange={(e) => setTitle(e.target.value)}
							className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500"
							placeholder="Enter task title..."
						/>
					</div>

					<div>
						<label htmlFor="task-description" className="block text-sm font-medium text-gray-700 mb-1">
							Description
						</label>
						<textarea
							id="task-description"
							value={description}
							onChange={(e) => setDescription(e.target.value)}
							rows={3}
							className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500"
							placeholder="Enter task description..."
						/>
					</div>
				</div>

				{/* Advanced Section */}
				<div className="border border-gray-200 rounded-lg">
					<button
						type="button"
						onClick={() => setIsAdvancedOpen(!isAdvancedOpen)}
						className="w-full flex items-center justify-between p-4 text-left hover:bg-gray-50"
					>
						<span className="font-medium text-gray-900">Advanced</span>
						<Icon name={isAdvancedOpen ? 'chevron-up' : 'chevron-down'} size={16} />
					</button>

					{isAdvancedOpen && (
						<div className="px-4 pb-4 space-y-4 border-t border-gray-200">
							<div className="grid grid-cols-1 md:grid-cols-3 gap-4">
								{/* Category */}
								<div>
									<label htmlFor="task-category" className="block text-sm font-medium text-gray-700 mb-1">
										Category
									</label>
									<select
										id="task-category"
										value={category}
										onChange={(e) => setCategory(Number(e.target.value) as TaskCategory)}
										className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500"
									>
										<option value={TaskCategory.FEATURE}>Feature</option>
										<option value={TaskCategory.BUG}>Bug</option>
										<option value={TaskCategory.REFACTOR}>Refactor</option>
										<option value={TaskCategory.CHORE}>Chore</option>
										<option value={TaskCategory.DOCS}>Docs</option>
										<option value={TaskCategory.TEST}>Test</option>
									</select>
								</div>

								{/* Priority */}
								<div>
									<label htmlFor="task-priority" className="block text-sm font-medium text-gray-700 mb-1">
										Priority
									</label>
									<select
										id="task-priority"
										value={priority}
										onChange={(e) => setPriority(Number(e.target.value) as TaskPriority)}
										className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500"
									>
										<option value={TaskPriority.CRITICAL}>Critical</option>
										<option value={TaskPriority.HIGH}>High</option>
										<option value={TaskPriority.NORMAL}>Normal</option>
										<option value={TaskPriority.LOW}>Low</option>
									</select>
								</div>

								{/* Queue */}
								<div>
									<label htmlFor="task-queue" className="block text-sm font-medium text-gray-700 mb-1">
										Queue
									</label>
									<select
										id="task-queue"
										value={queue}
										onChange={(e) => setQueue(Number(e.target.value) as TaskQueue)}
										className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500"
									>
										<option value={TaskQueue.ACTIVE}>Active</option>
										<option value={TaskQueue.BACKLOG}>Backlog</option>
									</select>
								</div>
							</div>

							{/* Initiative */}
							<div>
								<label htmlFor="task-initiative" className="block text-sm font-medium text-gray-700 mb-1">
									Initiative
								</label>
								<select
									id="task-initiative"
									value={initiativeId || ''}
									onChange={(e) => setInitiativeId(e.target.value === '' ? undefined : e.target.value)}
									className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500"
								>
									<option value="">None</option>
									{initiatives.map((initiative) => (
										<option key={initiative.id} value={initiative.id}>
											{initiative.title}
										</option>
									))}
								</select>
							</div>

							{/* Git Settings */}
							<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
								<div>
									<label htmlFor="target-branch" className="block text-sm font-medium text-gray-700 mb-1">
										Target Branch
									</label>
									<input
										id="target-branch"
										type="text"
										value={targetBranch}
										onChange={(e) => setTargetBranch(e.target.value)}
										className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500"
										placeholder="e.g., main, develop"
									/>
								</div>

								<div>
									<label htmlFor="branch-name" className="block text-sm font-medium text-gray-700 mb-1">
										Branch Name
									</label>
									<input
										id="branch-name"
										type="text"
										value={branchName}
										onChange={(e) => setBranchName(e.target.value)}
										className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500"
										placeholder="Custom branch name (optional)"
									/>
								</div>
							</div>

							{/* PR Settings */}
							<div className="space-y-4">
								<div className="flex items-center">
									<input
										id="pr-draft"
										type="checkbox"
										checked={prDraft || false}
										onChange={(e) => setPrDraft(e.target.checked)}
										className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
									/>
									<label htmlFor="pr-draft" className="ml-2 block text-sm text-gray-700">
										PR Draft
									</label>
								</div>

								<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
									<div>
										<label htmlFor="pr-labels" className="block text-sm font-medium text-gray-700 mb-1">
											PR Labels
										</label>
										<input
											id="pr-labels"
											type="text"
											value={prLabels}
											onChange={(e) => setPrLabels(e.target.value)}
											className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500"
											placeholder="bug,enhancement (comma-separated)"
										/>
									</div>

									<div>
										<label htmlFor="pr-reviewers" className="block text-sm font-medium text-gray-700 mb-1">
											PR Reviewers
										</label>
										<input
											id="pr-reviewers"
											type="text"
											value={prReviewers}
											onChange={(e) => setPrReviewers(e.target.value)}
											className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500"
											placeholder="user1,user2 (comma-separated)"
										/>
									</div>
								</div>
							</div>
						</div>
					)}
				</div>

				{/* Action Buttons */}
				<div className="flex justify-between">
					<Button variant="ghost" onClick={onBack}>
						Back
					</Button>
					<div className="flex space-x-2">
						<Button
							disabled={!isFormValid || isCreating || isCreatingAndRunning}
							loading={isCreating}
							onClick={handleCreate}
						>
							{isCreating ? 'Creating...' : 'Create'}
						</Button>
						<Button
							variant="primary"
							disabled={!isFormValid || isCreating || isCreatingAndRunning}
							loading={isCreatingAndRunning}
							onClick={handleCreateAndRun}
						>
							{isCreatingAndRunning ? 'Creating & Running...' : 'Create & Run'}
						</Button>
					</div>
				</div>

				{/* Loading Text */}
				{isCreating && <div>Creating...</div>}
				{isCreatingAndRunning && <div>Creating & Running...</div>}
			</div>
		</Modal>
	);
}