import { useState, useEffect } from 'react';
import type { Workflow } from '@/gen/orc/v1/workflow_pb';
import { workflowClient } from '@/lib/client';
import { PROVIDERS, PROVIDER_MODELS } from '@/lib/providerUtils';
import './WorkflowSettingsPanel.css';

interface WorkflowSettingsPanelProps {
	workflow: Workflow;
	onWorkflowUpdate: (workflow: Workflow) => void;
}

export function WorkflowSettingsPanel({ workflow, onWorkflowUpdate }: WorkflowSettingsPanelProps) {
	const [isLoading, setIsLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [validationError, setValidationError] = useState<string | null>(null);

	// Local form state for controlled inputs
	const [formData, setFormData] = useState({
		name: workflow.name || '',
		description: workflow.description || '',
		defaultProvider: workflow.defaultProvider || '',
		defaultModel: workflow.defaultModel || '',
		defaultThinking: workflow.defaultThinking || false,
		completionAction: workflow.completionAction || '',
		targetBranch: workflow.targetBranch || '',
	});

	// Sync form data when workflow changes
	useEffect(() => {
		setFormData({
			name: workflow.name || '',
			description: workflow.description || '',
			defaultProvider: workflow.defaultProvider || '',
			defaultModel: workflow.defaultModel || '',
			defaultThinking: workflow.defaultThinking || false,
			completionAction: workflow.completionAction || '',
			targetBranch: workflow.targetBranch || '',
		});
		setValidationError(null);
	}, [workflow]);

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

	const handleFieldInputChange = (field: string, value: string | boolean) => {
		setFormData((prev) => ({ ...prev, [field]: value }));
		// Clear validation error on any input change
		if (validationError) {
			setValidationError(null);
		}
	};

	const handleFieldBlur = (field: string, value: string | boolean) => {
		// Validate name field
		if (field === 'name' && typeof value === 'string' && value.trim() === '') {
			setValidationError('Name is required');
			return;
		}
		handleUpdate({ [field]: value });
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

	const handleCheckboxChange = (field: string, checked: boolean) => {
		setFormData((prev) => ({ ...prev, [field]: checked }));
		handleUpdate({ [field]: checked });
	};

	const isDisabled = workflow.isBuiltin || isLoading;
	const showTargetBranch = formData.completionAction !== 'none';

	return (
		<div className="workflow-settings-panel" data-testid="workflow-settings-panel">
			<div className="workflow-settings-section">
				<div className="workflow-settings-header">
					<h3>Workflow Settings</h3>
					{workflow.isBuiltin && (
						<span className="builtin-badge">Built-in</span>
					)}
				</div>

				{workflow.isBuiltin && (
					<div className="readonly-message">
						Clone to customize
					</div>
				)}

				{error && (
					<div className="error-message">
						{error}
					</div>
				)}

				<div className="settings-form">
					{/* Identity Section */}
					<div className="form-section">
						<h4>Identity</h4>

						<div className="form-field">
							<label htmlFor="workflow-name">Name</label>
							<input
								id="workflow-name"
								type="text"
								value={formData.name}
								onChange={(e) => handleFieldInputChange('name', e.target.value)}
								onBlur={(e) => handleFieldBlur('name', e.target.value)}
								disabled={isDisabled}
							/>
							{validationError && (
								<span className="field-error">{validationError}</span>
							)}
						</div>

						<div className="form-field">
							<label htmlFor="workflow-description">Description</label>
							<textarea
								id="workflow-description"
								value={formData.description}
								onChange={(e) => handleFieldInputChange('description', e.target.value)}
								onBlur={(e) => handleFieldBlur('description', e.target.value)}
								disabled={isDisabled}
								rows={3}
							/>
						</div>
					</div>

					{/* Defaults Section */}
					<div className="form-section">
						<h4>Defaults</h4>

						<div className="form-field">
							<label htmlFor="default-provider">Default Provider</label>
							<select
								id="default-provider"
								value={formData.defaultProvider}
								onChange={(e) => handleSelectChange('defaultProvider', e.target.value)}
								disabled={isDisabled}
							>
								<option value="">Claude (default)</option>
								{PROVIDERS.map(p => (
									<option key={p.value} value={p.value}>{p.label}</option>
								))}
							</select>
						</div>

						<div className="form-field">
							<label htmlFor="default-model">Default Model</label>
							{(PROVIDER_MODELS[formData.defaultProvider || 'claude'] ?? []).length > 0 ? (
								<select
									id="default-model"
									value={formData.defaultModel}
									onChange={(e) => handleSelectChange('defaultModel', e.target.value)}
									disabled={isDisabled}
								>
									<option value="">Select a model...</option>
									{(PROVIDER_MODELS[formData.defaultProvider || 'claude'] ?? []).map(m => (
										<option key={m.value} value={m.value}>{m.label}</option>
									))}
								</select>
							) : (
								<input
									id="default-model"
									type="text"
									value={formData.defaultModel}
									onChange={(e) => handleFieldInputChange('defaultModel', e.target.value)}
									onBlur={(e) => handleFieldBlur('defaultModel', e.target.value)}
									disabled={isDisabled}
									placeholder="Type model name..."
								/>
							)}
						</div>

						<div className="form-field">
							<div className="checkbox-field">
								<input
									id="default-thinking"
									type="checkbox"
									checked={formData.defaultThinking}
									onChange={(e) => handleCheckboxChange('defaultThinking', e.target.checked)}
									disabled={isDisabled}
								/>
								<label htmlFor="default-thinking">Enable Thinking by Default</label>
							</div>
						</div>
					</div>

					{/* Completion Section */}
					<div className="form-section">
						<h4>Completion</h4>

						<div className="form-field">
							<label htmlFor="completion-action">On Complete</label>
							<select
								id="completion-action"
								value={formData.completionAction}
								onChange={(e) => handleSelectChange('completionAction', e.target.value)}
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
								<label htmlFor="target-branch">Target Branch</label>
								<input
									id="target-branch"
									type="text"
									value={formData.targetBranch}
									onChange={(e) => handleFieldInputChange('targetBranch', e.target.value)}
									onBlur={(e) => handleFieldBlur('targetBranch', e.target.value)}
									disabled={isDisabled}
									placeholder="main"
								/>
							</div>
						)}
					</div>
				</div>
			</div>
		</div>
	);
}