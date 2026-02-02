/**
 * WorkflowPickerModal - Step 1 of workflow-first task creation
 *
 * Replaces weight-based task creation with direct workflow selection.
 * Users select a workflow from available options before proceeding to task details.
 */

import { useState, useCallback, useEffect, useMemo } from 'react';
import { Modal } from './Modal';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import { workflowClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';
import { useCurrentProjectId } from '@/stores';
import type { Workflow } from '@/gen/orc/v1/workflow_pb';

import './WorkflowPickerModal.css';

interface WorkflowWithPhaseCount extends Workflow {
	phaseCount: number;
}

interface WorkflowPickerModalProps {
	open: boolean;
	onClose: () => void;
	onSelectWorkflow: (workflow: WorkflowWithPhaseCount) => void;
	defaultWorkflowId?: string;
}

export function WorkflowPickerModal({
	open,
	onClose,
	onSelectWorkflow,
	defaultWorkflowId
}: WorkflowPickerModalProps) {
	const currentProjectId = useCurrentProjectId();
	const [workflows, setWorkflows] = useState<Workflow[]>([]);
	const [phaseCounts, setPhaseCounts] = useState<Record<string, number>>({});
	const [selectedWorkflowId, setSelectedWorkflowId] = useState<string | undefined>(defaultWorkflowId);
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);

	// Load workflows when modal opens
	const loadWorkflows = useCallback(async () => {
		if (!open) return;

		setLoading(true);
		setError(null);
		try {
			const response = await workflowClient.listWorkflows({
				includeBuiltin: true,
			});
			setWorkflows(response.workflows);
			setPhaseCounts(response.phaseCounts || {});
		} catch (e) {
			setError('Failed to load workflows');
			console.error('Failed to load workflows:', e);
		} finally {
			setLoading(false);
		}
	}, [open]);

	useEffect(() => {
		loadWorkflows();
	}, [loadWorkflows]);

	// Reset selection when modal opens/closes or default changes
	useEffect(() => {
		if (open) {
			setSelectedWorkflowId(defaultWorkflowId);
		}
	}, [open, defaultWorkflowId]);

	// Sort workflows: built-in first, then by name
	const sortedWorkflows = useMemo(() => {
		return [...workflows].sort((a, b) => {
			if (a.isBuiltin && !b.isBuiltin) return -1;
			if (b.isBuiltin && !a.isBuiltin) return 1;
			return a.name.localeCompare(b.name);
		});
	}, [workflows]);

	// Handle workflow selection
	const handleSelectWorkflow = useCallback((workflowId: string) => {
		setSelectedWorkflowId(workflowId);
	}, []);

	// Handle proceeding to next step
	const handleNext = useCallback(() => {
		if (!selectedWorkflowId) return;

		const selectedWorkflow = workflows.find(w => w.id === selectedWorkflowId);
		if (!selectedWorkflow) return;

		const workflowWithPhaseCount: WorkflowWithPhaseCount = {
			...selectedWorkflow,
			phaseCount: phaseCounts[selectedWorkflowId] || 0,
		};

		onSelectWorkflow(workflowWithPhaseCount);
	}, [selectedWorkflowId, workflows, phaseCounts, onSelectWorkflow]);

	// Handle retry
	const handleRetry = useCallback(() => {
		loadWorkflows();
	}, [loadWorkflows]);

	// Keyboard navigation
	const handleCardKeyDown = useCallback((e: React.KeyboardEvent, workflowId: string) => {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			handleSelectWorkflow(workflowId);
		}
	}, [handleSelectWorkflow]);

	if (!open) return null;

	return (
		<Modal
			open={open}
			title="New Task"
			onClose={onClose}
			size="large"
		>
			<div className="workflow-picker-modal">
				<div className="workflow-picker-header">
					<h2>Choose a workflow</h2>
					<p className="workflow-picker-subtitle">
						Select a workflow to continue
					</p>
				</div>

				<div className="workflow-picker-content">
					{loading && (
						<div className="workflow-picker-loading">
							<Icon name="loader" size={24} />
							<span>Loading workflows...</span>
						</div>
					)}

					{error && (
						<div className="workflow-picker-error">
							<div className="error-message">
								<Icon name="alert-circle" size={20} />
								<span>{error}</span>
							</div>
							<Button
								type="button"
								variant="ghost"
								size="sm"
								onClick={handleRetry}
							>
								Retry
							</Button>
						</div>
					)}

					{!loading && !error && sortedWorkflows.length === 0 && (
						<div className="workflow-picker-empty">
							<Icon name="package" size={48} />
							<h3>No workflows available</h3>
							<p>No workflows found for this project.</p>
						</div>
					)}

					{!loading && !error && sortedWorkflows.length > 0 && (
						<div className="workflow-cards-grid">
							{sortedWorkflows.map((workflow) => {
								const isSelected = selectedWorkflowId === workflow.id;
								const isDefault = defaultWorkflowId === workflow.id;
								const phaseCount = phaseCounts[workflow.id] || 0;
								const phaseText = phaseCount === 1 ? '1 phase' : `${phaseCount} phases`;

								return (
									<button
										key={workflow.id}
										type="button"
										className={`workflow-card ${isSelected ? 'selected' : ''}`}
										onClick={() => handleSelectWorkflow(workflow.id)}
										onKeyDown={(e) => handleCardKeyDown(e, workflow.id)}
										aria-pressed={isSelected}
									>
										<div className="workflow-card-header">
											<div className="workflow-card-title">
												{isDefault && <span className="default-indicator">★</span>}
												<span className="workflow-name">{workflow.name}</span>
												{workflow.isBuiltin && (
													<span className="built-in-badge">Built-in</span>
												)}
											</div>
											<div className="workflow-phase-count">
												{phaseText}
											</div>
										</div>

										{workflow.description && (
											<div className="workflow-card-description">
												{workflow.description}
											</div>
										)}

										{isSelected && (
											<div className="selection-indicator">
												<Icon name="check" size={16} />
											</div>
										)}
									</button>
								);
							})}
						</div>
					)}
				</div>

				<div className="workflow-picker-actions">
					<Button type="button" variant="secondary" onClick={onClose}>
						Cancel
					</Button>
					<Button
						type="button"
						variant="primary"
						onClick={handleNext}
						disabled={!selectedWorkflowId || loading || error !== null}
					>
						Next →
					</Button>
				</div>
			</div>
		</Modal>
	);
}