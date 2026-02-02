/**
 * TaskDetailsModal - Step 2 of workflow-first task creation
 *
 * COMPONENT SKELETON FOR TDD - This is a minimal implementation that allows
 * tests to compile and fail appropriately. Full implementation will be done
 * in the implement phase.
 */

import React from 'react';
import { Modal } from './Modal';

export interface WorkflowWithPhaseCount {
	id: string;
	name: string;
	description?: string;
	isBuiltin: boolean;
	phaseCount: number;
}

export interface Task {
	id: string;
	title: string;
	[key: string]: any;
}

export interface TaskDetailsModalProps {
	open: boolean;
	selectedWorkflow: WorkflowWithPhaseCount;
	onClose: () => void;
	onBack: () => void;
	onTaskCreated: (task: Task, wasRun: boolean) => void;
}

export function TaskDetailsModal({
	open,
	selectedWorkflow,
	onClose,
	onBack,
	onTaskCreated
}: TaskDetailsModalProps) {
	if (!open) return null;

	// This is a skeleton component - tests should fail
	return (
		<Modal open={open} title="New Task" onClose={onClose}>
			<div data-testid="task-details-modal">
				<p>TaskDetailsModal skeleton - not implemented</p>
				<p>Selected workflow: {selectedWorkflow.name}</p>
			</div>
		</Modal>
	);
}