/**
 * CreateWorkflowModal - Modal for creating a new workflow from scratch.
 *
 * Features:
 * - ID, name, and description inputs
 * - Default model and thinking options
 * - Validation and error handling
 */

import { useState, useCallback, useEffect } from 'react';
import { Modal } from '@/components/overlays/Modal';
import { Button, Icon } from '@/components/ui';
import { workflowClient } from '@/lib/client';
import { type Workflow } from '@/gen/orc/v1/workflow_pb';
import './CreateWorkflowModal.css';

export interface CreateWorkflowModalProps {
	/** Whether the modal is open */
	open: boolean;
	/** Callback when modal should close */
	onClose: () => void;
	/** Callback when workflow is successfully created */
	onCreated: (workflow: Workflow) => void;
}

/**
 * Generates a slug from a string (for workflow IDs).
 */
function slugify(str: string): string {
	return str
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '')
		.slice(0, 50);
}

const MODEL_OPTIONS = [
	{ value: '', label: 'Default (inherit)' },
	{ value: 'sonnet', label: 'Sonnet' },
	{ value: 'opus', label: 'Opus' },
	{ value: 'haiku', label: 'Haiku' },
];

const COMPLETION_ACTION_OPTIONS = [
	{ value: '', label: 'Inherit from config' },
	{ value: 'pr', label: 'Create PR' },
	{ value: 'commit', label: 'Commit only' },
	{ value: 'none', label: 'No action' },
];

/**
 * CreateWorkflowModal allows creating a new workflow from scratch.
 */
export function CreateWorkflowModal({
	open,
	onClose,
	onCreated,
}: CreateWorkflowModalProps) {
	const [id, setId] = useState('');
	const [name, setName] = useState('');
	const [description, setDescription] = useState('');
	const [defaultModel, setDefaultModel] = useState('');
	const [defaultThinking, setDefaultThinking] = useState(false);
	const [completionAction, setCompletionAction] = useState('');
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [idManuallySet, setIdManuallySet] = useState(false);

	// Reset form when modal opens
	useEffect(() => {
		if (open) {
			setId('');
			setName('');
			setDescription('');
			setDefaultModel('');
			setDefaultThinking(false);
			setCompletionAction('');
			setError(null);
			setIdManuallySet(false);
		}
	}, [open]);

	// Auto-generate ID from name unless manually set
	useEffect(() => {
		if (!idManuallySet && name) {
			setId(slugify(name));
		}
	}, [name, idManuallySet]);

	const handleIdChange = useCallback((value: string) => {
		setId(slugify(value));
		setIdManuallySet(true);
	}, []);

	const handleClose = useCallback(() => {
		setError(null);
		onClose();
	}, [onClose]);

	const handleSubmit = useCallback(
		async (e: React.FormEvent) => {
			e.preventDefault();
			if (!id.trim()) return;

			setSaving(true);
			setError(null);

			try {
				const response = await workflowClient.createWorkflow({
					id: id.trim(),
					name: name.trim() || undefined,
					description: description.trim() || undefined,
					defaultModel: defaultModel || undefined,
					defaultThinking: defaultThinking,
					completionAction: completionAction,
				});
				if (response.workflow) {
					onCreated(response.workflow);
				}
				handleClose();
			} catch (err) {
				setError(err instanceof Error ? err.message : 'Failed to create workflow');
			} finally {
				setSaving(false);
			}
		},
		[id, name, description, defaultModel, defaultThinking, completionAction, onCreated, handleClose]
	);

	return (
		<Modal
			open={open}
			onClose={handleClose}
			title="Create Workflow"
			size="md"
			ariaLabel="Create workflow dialog"
		>
			<form onSubmit={handleSubmit} className="create-workflow-form">
				{/* Workflow ID */}
				<div className="form-group">
					<label htmlFor="new-workflow-id" className="form-label">
						Workflow ID <span className="form-required">*</span>
					</label>
					<input
						id="new-workflow-id"
						type="text"
						className="form-input"
						value={id}
						onChange={(e) => handleIdChange(e.target.value)}
						placeholder="my-custom-workflow"
						required
						pattern="[a-z0-9-]+"
						title="Lowercase letters, numbers, and hyphens only"
					/>
					<span className="form-help">
						Unique identifier (lowercase letters, numbers, hyphens)
					</span>
				</div>

				{/* Name */}
				<div className="form-group">
					<label htmlFor="new-workflow-name" className="form-label">
						Name
					</label>
					<input
						id="new-workflow-name"
						type="text"
						className="form-input"
						value={name}
						onChange={(e) => setName(e.target.value)}
						placeholder="My Custom Workflow"
					/>
				</div>

				{/* Description */}
				<div className="form-group">
					<label htmlFor="new-workflow-description" className="form-label">
						Description
					</label>
					<textarea
						id="new-workflow-description"
						className="form-textarea"
						value={description}
						onChange={(e) => setDescription(e.target.value)}
						placeholder="Describe what this workflow does..."
						rows={3}
					/>
				</div>

				{/* Default Model */}
				<div className="form-row">
					<div className="form-group form-group-half">
						<label htmlFor="new-workflow-model" className="form-label">
							Default Model
						</label>
						<select
							id="new-workflow-model"
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
							<span className="form-checkbox-label">Enable thinking mode</span>
						</label>
					</div>
				</div>

				{/* Completion Action */}
				<div className="form-group">
					<label htmlFor="new-workflow-completion-action" className="form-label">
						Completion Action
					</label>
					<select
						id="new-workflow-completion-action"
						className="form-select"
						value={completionAction}
						onChange={(e) => setCompletionAction(e.target.value)}
					>
						{COMPLETION_ACTION_OPTIONS.map((option) => (
							<option key={option.value} value={option.value}>
								{option.label}
							</option>
						))}
					</select>
					<span className="form-help">
						What happens when the workflow completes successfully
					</span>
				</div>

				{/* Error message */}
				{error && (
					<div className="create-workflow-error" role="alert">
						<Icon name="alert-circle" size={14} />
						<span>{error}</span>
					</div>
				)}

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
						disabled={saving || !id.trim()}
						leftIcon={<Icon name="plus" size={12} />}
					>
						{saving ? 'Creating...' : 'Create Workflow'}
					</Button>
				</div>
			</form>
		</Modal>
	);
}
