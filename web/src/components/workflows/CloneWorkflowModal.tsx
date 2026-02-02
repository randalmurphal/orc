/**
 * CloneWorkflowModal - Modal for cloning an existing workflow.
 *
 * Features:
 * - Pre-filled with source workflow info
 * - Custom ID (required, auto-generated from name)
 * - Custom name and description (optional)
 * - Shows source workflow as reference
 */

import { useState, useCallback, useEffect } from 'react';
import { Modal } from '@/components/overlays/Modal';
import { Button, Icon } from '@/components/ui';
import { workflowClient } from '@/lib/client';
import type { Workflow } from '@/gen/orc/v1/workflow_pb';
import './CloneWorkflowModal.css';

export interface CloneWorkflowModalProps {
	/** Whether the modal is open */
	open: boolean;
	/** The workflow to clone */
	workflow: Workflow | null;
	/** Callback when modal should close */
	onClose: () => void;
	/** Callback when workflow is successfully cloned */
	onCloned: (workflow: Workflow) => void;
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

/**
 * CloneWorkflowModal allows cloning an existing workflow.
 */
export function CloneWorkflowModal({
	open,
	workflow,
	onClose,
	onCloned,
}: CloneWorkflowModalProps) {
	const [newId, setNewId] = useState('');
	const [name, setName] = useState('');
	const [description, setDescription] = useState('');
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [idManuallySet, setIdManuallySet] = useState(false);

	// Reset form when workflow changes
	useEffect(() => {
		if (workflow) {
			const baseName = workflow.name.replace(/\s*\(copy\)$/i, '');
			setName(`${baseName} (copy)`);
			setDescription(workflow.description || '');
			setNewId(slugify(`${baseName}-copy`));
			setIdManuallySet(false);
		}
	}, [workflow]);

	// Auto-generate ID from name unless manually set
	useEffect(() => {
		if (!idManuallySet && name) {
			setNewId(slugify(name));
		}
	}, [name, idManuallySet]);

	const handleIdChange = useCallback((value: string) => {
		setNewId(slugify(value));
		setIdManuallySet(true);
	}, []);

	const handleClose = useCallback(() => {
		setError(null);
		onClose();
	}, [onClose]);

	const handleSubmit = useCallback(
		async (e: React.FormEvent) => {
			e.preventDefault();
			if (!workflow || !newId.trim()) return;

			setSaving(true);
			setError(null);

			try {
				const response = await workflowClient.cloneWorkflow({
					sourceId: workflow.id,
					newId: newId.trim(),
					newName: name.trim() || undefined,
				});
				if (response.workflow) {
					onCloned(response.workflow);
				}
				handleClose();
			} catch (err) {
				setError(err instanceof Error ? err.message : 'Failed to clone workflow');
			} finally {
				setSaving(false);
			}
		},
		[workflow, newId, name, onCloned, handleClose]
	);

	if (!workflow) {
		return null;
	}

	return (
		<Modal
			open={open}
			onClose={handleClose}
			title="Clone Workflow"
			size="md"
			ariaLabel="Clone workflow dialog"
		>
			<form onSubmit={handleSubmit} className="clone-workflow-form">
				{/* Source workflow reference */}
				<div className="clone-workflow-source">
					<div className="clone-workflow-source-label">Cloning from:</div>
					<div className="clone-workflow-source-info">
						<Icon name="workflow" size={14} />
						<span className="clone-workflow-source-name">{workflow.name}</span>
						<code className="clone-workflow-source-id">{workflow.id}</code>
						{workflow.isBuiltin && (
							<span className="clone-workflow-source-badge">Built-in</span>
						)}
					</div>
				</div>

				{/* New workflow ID */}
				<div className="form-group">
					<label htmlFor="workflow-id" className="form-label">
						Workflow ID <span className="form-required">*</span>
					</label>
					<input
						id="workflow-id"
						type="text"
						className="form-input"
						value={newId}
						onChange={(e) => handleIdChange(e.target.value)}
						placeholder="my-custom-workflow"
						required
						pattern="[-a-z0-9]+"
						title="Lowercase letters, numbers, and hyphens only"
					/>
					<span className="form-help">
						Unique identifier (lowercase letters, numbers, hyphens)
					</span>
				</div>

				{/* New name */}
				<div className="form-group">
					<label htmlFor="workflow-name" className="form-label">
						Name
					</label>
					<input
						id="workflow-name"
						type="text"
						className="form-input"
						value={name}
						onChange={(e) => setName(e.target.value)}
						placeholder="My Custom Workflow"
					/>
				</div>

				{/* New description */}
				<div className="form-group">
					<label htmlFor="workflow-description" className="form-label">
						Description
					</label>
					<textarea
						id="workflow-description"
						className="form-textarea"
						value={description}
						onChange={(e) => setDescription(e.target.value)}
						placeholder="Describe what this workflow does..."
						rows={3}
					/>
				</div>

				{/* Error message */}
				{error && (
					<div className="clone-workflow-error" role="alert">
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
						disabled={saving || !newId.trim()}
						leftIcon={<Icon name="copy" size={12} />}
					>
						{saving ? 'Cloning...' : 'Clone Workflow'}
					</Button>
				</div>
			</form>
		</Modal>
	);
}
