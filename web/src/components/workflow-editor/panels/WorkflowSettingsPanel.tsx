import { useState } from 'react';
import type { Workflow } from '@/gen/orc/v1/workflow_pb';
import { workflowClient } from '@/lib/client';
import './WorkflowSettingsPanel.css';

interface WorkflowSettingsPanelProps {
	workflow: Workflow;
	onWorkflowUpdate: (workflow: Workflow) => void;
}

export function WorkflowSettingsPanel({ workflow, onWorkflowUpdate }: WorkflowSettingsPanelProps) {
	const [isLoading, setIsLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);

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

	const handleFieldChange = (field: string, value: any) => {
		handleUpdate({ [field]: value });
	};

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
					{/* Basic Information */}
					<div className="form-section">
						<h4>Basic Information</h4>

						<div className="form-field">
							<label htmlFor="workflow-name">Name</label>
							<input
								id="workflow-name"
								type="text"
								value={workflow.name}
								onChange={(e) => handleFieldChange('name', e.target.value)}
								onBlur={(e) => handleFieldChange('name', e.target.value)}
								disabled={workflow.isBuiltin || isLoading}
							/>
						</div>

						<div className="form-field">
							<label htmlFor="workflow-description">Description</label>
							<textarea
								id="workflow-description"
								value={workflow.description || ''}
								onChange={(e) => handleFieldChange('description', e.target.value)}
								onBlur={(e) => handleFieldChange('description', e.target.value)}
								disabled={workflow.isBuiltin || isLoading}
								rows={3}
							/>
						</div>
					</div>

					{/* Execution Defaults */}
					<div className="form-section">
						<h4>Execution Defaults</h4>

						<div className="form-field">
							<label htmlFor="default-model">Default Model</label>
							<select
								id="default-model"
								value={workflow.defaultModel || ''}
								onChange={(e) => handleFieldChange('defaultModel', e.target.value)}
								disabled={workflow.isBuiltin || isLoading}
							>
								<option value="">Select a model...</option>
								<option value="claude-sonnet-3-5">claude-sonnet-3-5</option>
								<option value="claude-opus-3">claude-opus-3</option>
								<option value="claude-haiku-3">claude-haiku-3</option>
								<option value="claude-sonnet-4">claude-sonnet-4</option>
								<option value="claude-opus-4">claude-opus-4</option>
							</select>
						</div>

						<div className="form-field">
							<div className="checkbox-field">
								<input
									id="default-thinking"
									type="checkbox"
									checked={workflow.defaultThinking}
									onChange={(e) => handleFieldChange('defaultThinking', e.target.checked)}
									disabled={workflow.isBuiltin || isLoading}
								/>
								<label htmlFor="default-thinking">Enable Thinking by Default</label>
							</div>
						</div>

						<div className="form-field">
							<label htmlFor="default-max-iterations">Default Max Iterations</label>
							<input
								id="default-max-iterations"
								type="number"
								value={workflow.defaultMaxIterations || ''}
								onChange={(e) => {
									const value = parseInt(e.target.value, 10);
									if (!isNaN(value)) {
										handleFieldChange('defaultMaxIterations', value);
									}
								}}
								onBlur={(e) => {
									const value = parseInt(e.target.value, 10);
									if (!isNaN(value)) {
										handleFieldChange('defaultMaxIterations', value);
									}
								}}
								disabled={workflow.isBuiltin || isLoading}
								min="1"
								max="100"
							/>
						</div>
					</div>

					{/* Completion Settings */}
					<div className="form-section">
						<h4>Completion Settings</h4>

						<div className="form-field">
							<label htmlFor="completion-action">On Complete</label>
							<select
								id="completion-action"
								value={workflow.completionAction || ''}
								onChange={(e) => handleFieldChange('completionAction', e.target.value)}
								disabled={workflow.isBuiltin || isLoading}
							>
								<option value="">Inherit from config</option>
								<option value="pr">Create PR</option>
								<option value="commit">Commit only</option>
								<option value="none">None</option>
							</select>
						</div>

						<div className="form-field">
							<label htmlFor="target-branch">Target Branch</label>
							<input
								id="target-branch"
								type="text"
								value={workflow.targetBranch || ''}
								onChange={(e) => handleFieldChange('targetBranch', e.target.value)}
								onBlur={(e) => handleFieldChange('targetBranch', e.target.value)}
								disabled={workflow.isBuiltin || isLoading}
								placeholder="main"
							/>
						</div>
					</div>
				</div>
			</div>
		</div>
	);
}