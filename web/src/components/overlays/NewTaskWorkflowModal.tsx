/**
 * NewTaskWorkflowModal - Workflow-first task creation orchestrator
 *
 * Manages the two-step workflow-first task creation process:
 * 1. WorkflowPickerModal - Select workflow
 * 2. TaskDetailsModal - Enter task details
 *
 * Replaces the old weight-first NewTaskModal with the new workflow-first approach.
 */

import { useState, useCallback } from 'react';
import { WorkflowPickerModal } from './WorkflowPickerModal';
import { TaskDetailsModal, type WorkflowWithPhaseCount, type Task } from './TaskDetailsModal';

interface NewTaskWorkflowModalProps {
	open: boolean;
	onClose: () => void;
	onCreate?: (task: Task, wasRun?: boolean) => void;
	defaultWorkflowId?: string;
}

enum ModalStep {
	WORKFLOW_PICKER = 'workflow-picker',
	TASK_DETAILS = 'task-details',
}

export function NewTaskWorkflowModal({
	open,
	onClose,
	onCreate,
	defaultWorkflowId
}: NewTaskWorkflowModalProps) {
	const [currentStep, setCurrentStep] = useState<ModalStep>(ModalStep.WORKFLOW_PICKER);
	const [selectedWorkflow, setSelectedWorkflow] = useState<WorkflowWithPhaseCount | null>(null);

	// Handle workflow selection from Step 1
	const handleWorkflowSelected = useCallback((workflow: WorkflowWithPhaseCount) => {
		setSelectedWorkflow(workflow);
		setCurrentStep(ModalStep.TASK_DETAILS);
	}, []);

	// Handle back from Step 2 to Step 1
	const handleBackToWorkflowPicker = useCallback(() => {
		setCurrentStep(ModalStep.WORKFLOW_PICKER);
	}, []);

	// Handle task creation completion
	const handleTaskCreated = useCallback((task: Task, wasRun: boolean) => {
		onCreate?.(task, wasRun);
		onClose();
		// Reset state for next time
		setCurrentStep(ModalStep.WORKFLOW_PICKER);
		setSelectedWorkflow(null);
	}, [onCreate, onClose]);

	// Handle modal close
	const handleClose = useCallback(() => {
		onClose();
		// Reset state for next time
		setCurrentStep(ModalStep.WORKFLOW_PICKER);
		setSelectedWorkflow(null);
	}, [onClose]);

	// Only render the active step
	if (currentStep === ModalStep.WORKFLOW_PICKER) {
		return (
			<WorkflowPickerModal
				open={open}
				onClose={handleClose}
				onSelectWorkflow={handleWorkflowSelected}
				defaultWorkflowId={defaultWorkflowId}
			/>
		);
	}

	if (currentStep === ModalStep.TASK_DETAILS && selectedWorkflow) {
		return (
			<TaskDetailsModal
				open={open}
				selectedWorkflow={selectedWorkflow}
				onClose={handleClose}
				onBack={handleBackToWorkflowPicker}
				onTaskCreated={handleTaskCreated}
			/>
		);
	}

	// Shouldn't reach here, but return null for safety
	return null;
}