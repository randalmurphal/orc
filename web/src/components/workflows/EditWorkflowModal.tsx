/**
 * EditWorkflowModal - Modal for editing workflow metadata and phases.
 *
 * Features:
 * - Edit workflow name, description, model, and thinking settings
 * - Manage phases through PhaseListEditor sub-component
 * - Built-in workflows cannot be edited (shows clone suggestion)
 * - Load workflow details on open
 * - Error handling with toast notifications
 */

import { useState, useCallback, useEffect } from 'react';
import { Modal } from '@/components/overlays/Modal';
import { Button, Icon } from '@/components/ui';
import { workflowClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';
import { PhaseListEditor, type PhaseOverrides, type AddPhaseRequest } from './PhaseListEditor';
import type { Workflow, WorkflowPhase, PhaseTemplate } from '@/gen/orc/v1/workflow_pb';
import './EditWorkflowModal.css';

export interface EditWorkflowModalProps {
	/** Whether the modal is open */
	open: boolean;
	/** The workflow to edit (basic info) */
	workflow: Workflow;
	/** Callback when modal should close */
	onClose: () => void;
	/** Callback when workflow is successfully updated */
	onUpdated: (workflow: Workflow) => void;
}

const MODEL_OPTIONS = [
	{ value: '', label: 'Default (inherit)' },
	{ value: 'sonnet', label: 'Sonnet' },
	{ value: 'opus', label: 'Opus' },
	{ value: 'haiku', label: 'Haiku' },
];

/**
 * EditWorkflowModal allows editing workflow metadata and phases.
 */
export function EditWorkflowModal({
	open,
	workflow,
	onClose,
	onUpdated,
}: EditWorkflowModalProps) {
	// Form state
	const [name, setName] = useState('');
	const [description, setDescription] = useState('');
	const [defaultModel, setDefaultModel] = useState('');
	const [defaultThinking, setDefaultThinking] = useState(false);

	// Phases state
	const [phases, setPhases] = useState<WorkflowPhase[]>([]);
	const [phaseTemplates, setPhaseTemplates] = useState<PhaseTemplate[]>([]);

	// Loading states
	const [loadingDetails, setLoadingDetails] = useState(false);
	const [loadingTemplates, setLoadingTemplates] = useState(false);
	const [saving, setSaving] = useState(false);
	const [phaseLoading, setPhaseLoading] = useState(false);

	// Error state
	const [loadError, setLoadError] = useState<string | null>(null);
	const [templateError, setTemplateError] = useState<string | null>(null);

	// Check if built-in workflow
	const isBuiltin = workflow.isBuiltin;

	// Reset form when workflow changes or modal opens
	useEffect(() => {
		if (open && workflow && !isBuiltin) {
			// Reset to basic workflow info initially
			setName(workflow.name || '');
			setDescription(workflow.description || '');
			setDefaultModel(workflow.defaultModel || '');
			setDefaultThinking(workflow.defaultThinking || false);
			setPhases([]);
			setLoadError(null);

			// Load full workflow details
			setLoadingDetails(true);
			workflowClient
				.getWorkflow({ id: workflow.id })
				.then((response) => {
					if (response.workflow) {
						const details = response.workflow;
						if (details.workflow) {
							setName(details.workflow.name || '');
							setDescription(details.workflow.description || '');
							setDefaultModel(details.workflow.defaultModel || '');
							setDefaultThinking(details.workflow.defaultThinking || false);
						}
						setPhases(details.phases || []);
					}
				})
				.catch((e) => {
					const errorMsg = e instanceof Error ? e.message : 'Failed to load workflow';
					setLoadError(errorMsg);
				})
				.finally(() => {
					setLoadingDetails(false);
				});

			// Load phase templates
			setLoadingTemplates(true);
			setTemplateError(null);
			workflowClient
				.listPhaseTemplates({ includeBuiltin: true })
				.then((response) => {
					setPhaseTemplates(response.templates || []);
				})
				.catch((e) => {
					const errorMsg = e instanceof Error ? e.message : 'Failed to load templates';
					setTemplateError(errorMsg);
				})
				.finally(() => {
					setLoadingTemplates(false);
				});
		}
	}, [open, workflow, isBuiltin]);

	// Handle save metadata
	const handleSave = useCallback(async () => {
		setSaving(true);
		try {
			const response = await workflowClient.updateWorkflow({
				id: workflow.id,
				name: name.trim() || undefined,
				description: description.trim() || undefined,
				defaultModel: defaultModel || undefined,
				defaultThinking: defaultThinking,
			});
			if (response.workflow) {
				toast.success('Workflow updated successfully');
				onUpdated(response.workflow);
				onClose();
			}
		} catch (e) {
			const errorMsg = e instanceof Error ? e.message : 'Unknown error';
			toast.error(`Failed to update workflow: ${errorMsg}`);
		} finally {
			setSaving(false);
		}
	}, [workflow.id, name, description, defaultModel, defaultThinking, onUpdated, onClose]);

	// Handle add phase
	const handleAddPhase = useCallback(
		async (request: AddPhaseRequest) => {
			setPhaseLoading(true);
			try {
				const response = await workflowClient.addPhase({
					workflowId: workflow.id,
					phaseTemplateId: request.phaseTemplateId,
					sequence: request.sequence,
				});
				if (response.phase) {
					setPhases((prev) => [...prev, response.phase!]);
				}
			} catch (e) {
				const errorMsg = e instanceof Error ? e.message : 'Unknown error';
				toast.error(`Failed to add phase: ${errorMsg}`);
				throw e;
			} finally {
				setPhaseLoading(false);
			}
		},
		[workflow.id]
	);

	// Handle update phase
	const handleUpdatePhase = useCallback(
		async (phaseId: number, overrides: PhaseOverrides) => {
			setPhaseLoading(true);
			try {
				const response = await workflowClient.updatePhase({
					workflowId: workflow.id,
					phaseId: phaseId,
					modelOverride: overrides.modelOverride,
					thinkingOverride: overrides.thinkingOverride,
					gateTypeOverride: overrides.gateTypeOverride,
					maxIterationsOverride: overrides.maxIterationsOverride,
				});
				if (response.phase) {
					setPhases((prev) =>
						prev.map((p) => (p.id === phaseId ? response.phase! : p))
					);
				}
			} catch (e) {
				const errorMsg = e instanceof Error ? e.message : 'Unknown error';
				toast.error(`Failed to update phase: ${errorMsg}`);
				throw e;
			} finally {
				setPhaseLoading(false);
			}
		},
		[workflow.id]
	);

	// Handle remove phase
	const handleRemovePhase = useCallback(
		async (phaseId: number) => {
			setPhaseLoading(true);
			try {
				await workflowClient.removePhase({
					workflowId: workflow.id,
					phaseId: phaseId,
				});
				setPhases((prev) => prev.filter((p) => p.id !== phaseId));
			} catch (e) {
				const errorMsg = e instanceof Error ? e.message : 'Unknown error';
				toast.error(`Failed to remove phase: ${errorMsg}`);
				throw e;
			} finally {
				setPhaseLoading(false);
			}
		},
		[workflow.id]
	);

	// Handle reorder phase
	const handleReorderPhase = useCallback(
		async (phaseId: number, direction: 'up' | 'down') => {
			// Find current phase and its neighbor
			const sortedPhases = [...phases].sort((a, b) => a.sequence - b.sequence);
			const currentIndex = sortedPhases.findIndex((p) => p.id === phaseId);
			if (currentIndex === -1) return;

			const targetIndex = direction === 'up' ? currentIndex - 1 : currentIndex + 1;
			if (targetIndex < 0 || targetIndex >= sortedPhases.length) return;

			const currentPhase = sortedPhases[currentIndex];
			const targetPhase = sortedPhases[targetIndex];

			setPhaseLoading(true);
			try {
				// Swap sequences
				await workflowClient.updatePhase({
					workflowId: workflow.id,
					phaseId: currentPhase.id,
					sequence: targetPhase.sequence,
				});
				await workflowClient.updatePhase({
					workflowId: workflow.id,
					phaseId: targetPhase.id,
					sequence: currentPhase.sequence,
				});

				// Update local state
				setPhases((prev) =>
					prev.map((p) => {
						if (p.id === currentPhase.id) {
							return { ...p, sequence: targetPhase.sequence };
						}
						if (p.id === targetPhase.id) {
							return { ...p, sequence: currentPhase.sequence };
						}
						return p;
					})
				);
			} catch (e) {
				const errorMsg = e instanceof Error ? e.message : 'Unknown error';
				toast.error(`Failed to reorder phases: ${errorMsg}`);
				throw e;
			} finally {
				setPhaseLoading(false);
			}
		},
		[workflow.id, phases]
	);

	// Handle retry load
	const handleRetry = useCallback(() => {
		if (!workflow) return;
		setLoadError(null);
		setLoadingDetails(true);
		workflowClient
			.getWorkflow({ id: workflow.id })
			.then((response) => {
				if (response.workflow) {
					const details = response.workflow;
					if (details.workflow) {
						setName(details.workflow.name || '');
						setDescription(details.workflow.description || '');
						setDefaultModel(details.workflow.defaultModel || '');
						setDefaultThinking(details.workflow.defaultThinking || false);
					}
					setPhases(details.phases || []);
				}
			})
			.catch((e) => {
				const errorMsg = e instanceof Error ? e.message : 'Failed to load workflow';
				setLoadError(errorMsg);
			})
			.finally(() => {
				setLoadingDetails(false);
			});
	}, [workflow]);

	// Handle close
	const handleClose = useCallback(() => {
		onClose();
	}, [onClose]);

	// Built-in workflow message
	if (isBuiltin) {
		return (
			<Modal
				open={open}
				onClose={handleClose}
				title="Built-in Workflow"
				size="sm"
				ariaLabel="Built-in workflow dialog"
			>
				<div className="edit-workflow-builtin">
					<Icon name="shield" size={24} />
					<p>
						Cannot edit built-in workflow. Clone to customize this workflow.
					</p>
					<Button variant="primary" onClick={handleClose}>
						OK
					</Button>
				</div>
			</Modal>
		);
	}

	// Loading/error state
	const isLoading = loadingDetails || loadingTemplates;
	const hasError = loadError || templateError;

	return (
		<Modal
			open={open}
			onClose={handleClose}
			title="Edit Workflow"
			size="lg"
			ariaLabel="Edit workflow dialog"
		>
			{/* Loading state */}
			{isLoading && !hasError && (
				<div className="edit-workflow-loading">
					<Icon name="loader" size={16} className="spinning" />
					<span>
						{loadingDetails && loadingTemplates
							? 'Loading...'
							: loadingDetails
								? 'Loading workflow details...'
								: 'Loading templates...'}
					</span>
				</div>
			)}

			{/* Error state */}
			{loadError && (
				<div className="edit-workflow-error" role="alert">
					<Icon name="alert-circle" size={14} />
					<span>Failed to load: {loadError}</span>
					<Button variant="secondary" size="sm" onClick={handleRetry}>
						Retry
					</Button>
				</div>
			)}

			{/* Template load error */}
			{templateError && !loadError && (
				<div className="edit-workflow-error" role="alert">
					<Icon name="alert-circle" size={14} />
					<span>Failed to load phase templates: {templateError}</span>
				</div>
			)}

			{/* Main form - only show when loaded */}
			{!isLoading && !loadError && (
				<form
					onSubmit={(e) => {
						e.preventDefault();
						handleSave();
					}}
					className="edit-workflow-form"
				>
					{/* Metadata Section */}
					<div className="edit-workflow-section">
						<h3 className="edit-workflow-section-title">Metadata</h3>

						{/* Name */}
						<div className="form-group">
							<label htmlFor="edit-workflow-name" className="form-label">
								Name
							</label>
							<input
								id="edit-workflow-name"
								type="text"
								className="form-input"
								value={name}
								onChange={(e) => setName(e.target.value)}
								placeholder="Workflow name"
							/>
						</div>

						{/* Description */}
						<div className="form-group">
							<label htmlFor="edit-workflow-description" className="form-label">
								Description
							</label>
							<textarea
								id="edit-workflow-description"
								className="form-textarea"
								value={description}
								onChange={(e) => setDescription(e.target.value)}
								placeholder="Describe what this workflow does..."
								rows={2}
							/>
						</div>

						{/* Model and Thinking */}
						<div className="form-row">
							<div className="form-group form-group-half">
								<label htmlFor="edit-workflow-model" className="form-label">
									Default LLM
								</label>
								<select
									id="edit-workflow-model"
									className="form-select"
									value={defaultModel}
									onChange={(e) => setDefaultModel(e.target.value)}
								>
									{MODEL_OPTIONS.map((option) => (
										<option key={option.value} value={option.value}>
											{option.label}
										</option>
									))}
								</select>
							</div>

							<div className="form-group form-group-half">
								<label className="form-label">Options</label>
								<label className="form-checkbox">
									<input
										type="checkbox"
										checked={defaultThinking}
										onChange={(e) => setDefaultThinking(e.target.checked)}
									/>
									<span className="form-checkbox-label">Enable deep reasoning</span>
								</label>
							</div>
						</div>
					</div>

					{/* Phases Section */}
					<div className="edit-workflow-section">
						<h3 className="edit-workflow-section-title">
							Phases
							{phases.length > 0 && (
								<span className="edit-workflow-phase-count">({phases.length})</span>
							)}
						</h3>

						<PhaseListEditor
							workflowId={workflow.id}
							phases={phases}
							phaseTemplates={phaseTemplates}
							loading={phaseLoading}
							onAddPhase={handleAddPhase}
							onUpdatePhase={handleUpdatePhase}
							onRemovePhase={handleRemovePhase}
							onReorderPhase={handleReorderPhase}
						/>
					</div>

					{/* Actions */}
					<div className="form-actions">
						<Button
							type="button"
							variant="ghost"
							onClick={handleClose}
							disabled={saving}
						>
							Cancel
						</Button>
						<Button
							type="submit"
							variant="primary"
							disabled={saving}
							leftIcon={<Icon name="save" size={12} />}
						>
							{saving ? 'Saving...' : 'Save'}
						</Button>
					</div>
				</form>
			)}
		</Modal>
	);
}
