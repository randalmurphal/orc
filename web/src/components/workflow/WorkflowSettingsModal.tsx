/**
 * WorkflowSettingsModal - Modal for editing workflow settings
 *
 * Displays the same form as WorkflowSettingsPanel but in a modal overlay.
 * Used when editing workflow settings from places other than the workflow editor.
 *
 * Features:
 * - Three sections: Identity, Defaults, Completion
 * - Auto-saves on field blur
 * - Validation for required fields
 * - Read-only mode for built-in workflows
 */

import { useState, useEffect } from 'react';
import type { Workflow } from '@/gen/orc/v1/workflow_pb';
import { workflowClient } from '@/lib/client';
import { PROVIDERS, PROVIDER_MODELS } from '@/lib/providerUtils';
import { Modal } from '@/components/overlays/Modal';
import './WorkflowSettingsModal.css';

interface WorkflowSettingsModalProps {
	open: boolean;
	workflow: Workflow | null;
	onClose: () => void;
	onWorkflowUpdate: (workflow: Workflow) => void;
}

export function WorkflowSettingsModal({
	open,
	workflow,
	onClose,
	onWorkflowUpdate,
}: WorkflowSettingsModalProps) {
	const [isLoading, setIsLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [validationError, setValidationError] = useState<string | null>(null);

	// Local form state for controlled inputs
	const [formData, setFormData] = useState({
		name: '',
		description: '',
		defaultProvider: '',
		defaultModel: '',
		defaultThinking: false,
		completionAction: '',
		targetBranch: '',
	});

	// Sync form data when workflow changes
	useEffect(() => {
		if (workflow) {
			setFormData({
				name: workflow.name || '',
				description: workflow.description || '',
				defaultProvider: workflow.defaultProvider || '',
				defaultModel: workflow.defaultModel || '',
				defaultThinking: workflow.defaultThinking || false,
				completionAction: workflow.completionAction || '',
				targetBranch: workflow.targetBranch || '',
			});
			setError(null);
			setValidationError(null);
		}
	}, [workflow]);

	// Don't render if not open or no workflow
	if (!open || !workflow) {
		return null;
	}

	const handleUpdate = async (updates: Record<string, unknown>) => {
		if (workflow.isBuiltin) return;

		setIsLoading(true);
		setError(null);

		try {
			const response = await workflowClient.updateWorkflow({
				id: workflow.id,
				...updates,
			} as Parameters<typeof workflowClient.updateWorkflow>[0]);

			if (response.workflow) {
				onWorkflowUpdate(response.workflow);
			}
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Unknown error';
			setError(`Failed to update workflow: ${message}`);
			console.error('Failed to update workflow:', err);
		} finally {
			setIsLoading(false);
		}
	};

	const handleFieldChange = (field: string, value: string | boolean) => {
		setFormData((prev) => ({ ...prev, [field]: value }));
	};

	const handleFieldBlur = (field: string, value: string | boolean) => {
		// Validate name field
		if (field === 'name' && typeof value === 'string' && value.trim() === '') {
			setValidationError('Name is required');
			return;
		}
		setValidationError(null);
		handleUpdate({ [field]: value });
	};

	const handleCheckboxChange = (field: string, checked: boolean) => {
		setFormData((prev) => ({ ...prev, [field]: checked }));
		handleUpdate({ [field]: checked });
	};

	const handleSelectChange = (field: string, value: string) => {
		if (field === 'defaultProvider') {
			setFormData(prev => ({ ...prev, [field]: value, defaultModel: '' }));
			handleUpdate({ defaultProvider: value, defaultModel: '' });
			return;
		}
		setFormData((prev) => ({ ...prev, [field]: value }));
		handleUpdate({ [field]: value });
	};

	const isDisabled = workflow.isBuiltin || isLoading;
	const showTargetBranch = formData.completionAction !== 'none';

	return (
		<Modal open={open} onClose={onClose} title="Workflow Settings" size="md">
			<div className="workflow-settings-modal">
				{workflow.isBuiltin && (
					<div className="clone-message">Clone to customize</div>
				)}

				{error && <div className="error-message">{error}</div>}

				{validationError && (
					<div className="validation-error">{validationError}</div>
				)}

				<div className="settings-form">
					{/* Identity Section */}
					<div className="form-section">
						<h3>Identity</h3>

						<div className="form-field">
							<label htmlFor="modal-workflow-name">Name</label>
							<input
								id="modal-workflow-name"
								type="text"
								value={formData.name}
								onChange={(e) => handleFieldChange('name', e.target.value)}
								onBlur={(e) => handleFieldBlur('name', e.target.value)}
								disabled={isDisabled}
							/>
						</div>

						<div className="form-field">
							<label htmlFor="modal-workflow-description">Description</label>
							<textarea
								id="modal-workflow-description"
								value={formData.description}
								onChange={(e) =>
									handleFieldChange('description', e.target.value)
								}
								onBlur={(e) => handleFieldBlur('description', e.target.value)}
								disabled={isDisabled}
								rows={3}
							/>
						</div>
					</div>

					{/* Defaults Section */}
					<div className="form-section">
						<h3>Defaults</h3>

						<div className="form-field">
							<label htmlFor="modal-default-provider">Default Provider</label>
							<select
								id="modal-default-provider"
								value={formData.defaultProvider}
								onChange={(e) =>
									handleSelectChange('defaultProvider', e.target.value)
								}
								disabled={isDisabled}
							>
								<option value="">Claude (default)</option>
								{PROVIDERS.map(p => (
									<option key={p.value} value={p.value}>{p.label}</option>
								))}
							</select>
						</div>

						<div className="form-field">
							<label htmlFor="modal-default-model">Default Model</label>
							{(PROVIDER_MODELS[formData.defaultProvider || 'claude'] ?? []).length > 0 ? (
								<select
									id="modal-default-model"
									value={formData.defaultModel}
									onChange={(e) =>
										handleSelectChange('defaultModel', e.target.value)
									}
									disabled={isDisabled}
								>
									<option value="">Select a model...</option>
									{(PROVIDER_MODELS[formData.defaultProvider || 'claude'] ?? []).map(m => (
										<option key={m.value} value={m.value}>{m.label}</option>
									))}
								</select>
							) : (
								<input
									id="modal-default-model"
									type="text"
									value={formData.defaultModel}
									onChange={(e) =>
										handleFieldChange('defaultModel', e.target.value)
									}
									onBlur={(e) =>
										handleFieldBlur('defaultModel', e.target.value)
									}
									disabled={isDisabled}
									placeholder="Type model name..."
								/>
							)}
						</div>

						<div className="form-field">
							<div className="checkbox-field">
								<input
									id="modal-default-thinking"
									type="checkbox"
									checked={formData.defaultThinking}
									onChange={(e) =>
										handleCheckboxChange('defaultThinking', e.target.checked)
									}
									disabled={isDisabled}
								/>
								<label htmlFor="modal-default-thinking">
									Enable Thinking by Default
								</label>
							</div>
						</div>
					</div>

					{/* Completion Section */}
					<div className="form-section">
						<h3>Completion</h3>

						<div className="form-field">
							<label htmlFor="modal-completion-action">On Complete</label>
							<select
								id="modal-completion-action"
								value={formData.completionAction}
								onChange={(e) =>
									handleSelectChange('completionAction', e.target.value)
								}
								disabled={isDisabled}
							>
								<option value="">Inherit from config</option>
								<option value="pr">Create PR</option>
								<option value="commit">Commit only</option>
								<option value="none">None</option>
							</select>
						</div>

						{showTargetBranch && (
							<div className="form-field">
								<label htmlFor="modal-target-branch">Target Branch</label>
								<input
									id="modal-target-branch"
									type="text"
									value={formData.targetBranch}
									onChange={(e) =>
										handleFieldChange('targetBranch', e.target.value)
									}
									onBlur={(e) =>
										handleFieldBlur('targetBranch', e.target.value)
									}
									disabled={isDisabled}
									placeholder="main"
								/>
							</div>
						)}
					</div>
				</div>
			</div>
		</Modal>
	);
}
