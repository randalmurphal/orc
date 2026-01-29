/**
 * ClonePhaseTemplateModal - Modal for cloning an existing phase template.
 *
 * Features:
 * - Pre-filled with source template info
 * - Custom ID (required, auto-generated from name)
 * - Custom name and description (optional)
 * - Shows source template as reference
 */

import { useState, useCallback, useEffect } from 'react';
import { Modal } from '@/components/overlays/Modal';
import { Button, Icon } from '@/components/ui';
import { workflowClient } from '@/lib/client';
import type { PhaseTemplate } from '@/gen/orc/v1/workflow_pb';
import './ClonePhaseTemplateModal.css';

export interface ClonePhaseTemplateModalProps {
	/** Whether the modal is open */
	open: boolean;
	/** The template to clone */
	template: PhaseTemplate | null;
	/** Callback when modal should close */
	onClose: () => void;
	/** Callback when template is successfully cloned */
	onCloned: (template: PhaseTemplate) => void;
}

/**
 * Generates a slug from a string (for template IDs).
 */
function slugify(str: string): string {
	return str
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '')
		.slice(0, 50);
}

/**
 * ClonePhaseTemplateModal allows cloning an existing phase template.
 */
export function ClonePhaseTemplateModal({
	open,
	template,
	onClose,
	onCloned,
}: ClonePhaseTemplateModalProps) {
	const [newId, setNewId] = useState('');
	const [name, setName] = useState('');
	const [description, setDescription] = useState('');
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [idManuallySet, setIdManuallySet] = useState(false);

	// Reset form when template changes
	useEffect(() => {
		if (template) {
			const baseName = template.name.replace(/\s*\(copy\)$/i, '');
			setName(`${baseName} (copy)`);
			setDescription(template.description || '');
			setNewId(slugify(`${baseName}-copy`));
			setIdManuallySet(false);
		}
	}, [template]);

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
			if (!template || !newId.trim()) return;

			setSaving(true);
			setError(null);

			try {
				const response = await workflowClient.clonePhaseTemplate({
					sourceId: template.id,
					newId: newId.trim(),
					newName: name.trim() || undefined,
				});
				if (response.template) {
					onCloned(response.template);
				}
				handleClose();
			} catch (err) {
				setError(err instanceof Error ? err.message : 'Failed to clone phase template');
			} finally {
				setSaving(false);
			}
		},
		[template, newId, name, onCloned, handleClose]
	);

	if (!template) {
		return null;
	}

	return (
		<Modal
			open={open}
			onClose={handleClose}
			title="Clone Phase Template"
			size="md"
			ariaLabel="Clone phase template dialog"
		>
			<form onSubmit={handleSubmit} className="clone-template-form">
				{/* Source template reference */}
				<div className="clone-template-source">
					<div className="clone-template-source-label">Cloning from:</div>
					<div className="clone-template-source-info">
						<Icon name="file-text" size={14} />
						<span className="clone-template-source-name">{template.name}</span>
						<code className="clone-template-source-id">{template.id}</code>
						{template.isBuiltin && (
							<span className="clone-template-source-badge">Built-in</span>
						)}
					</div>
				</div>

				{/* New template ID */}
				<div className="form-group">
					<label htmlFor="template-id" className="form-label">
						Template ID <span className="form-required">*</span>
					</label>
					<input
						id="template-id"
						type="text"
						className="form-input"
						value={newId}
						onChange={(e) => handleIdChange(e.target.value)}
						placeholder="my-custom-phase"
						required
						pattern="[a-z0-9\\-]+"
						title="Lowercase letters, numbers, and hyphens only"
					/>
					<span className="form-help">
						Unique identifier (lowercase letters, numbers, hyphens)
					</span>
				</div>

				{/* New name */}
				<div className="form-group">
					<label htmlFor="template-name" className="form-label">
						Name
					</label>
					<input
						id="template-name"
						type="text"
						className="form-input"
						value={name}
						onChange={(e) => setName(e.target.value)}
						placeholder="My Custom Phase"
					/>
				</div>

				{/* New description */}
				<div className="form-group">
					<label htmlFor="template-description" className="form-label">
						Description
					</label>
					<textarea
						id="template-description"
						className="form-textarea"
						value={description}
						onChange={(e) => setDescription(e.target.value)}
						placeholder="Describe what this phase template does..."
						rows={3}
					/>
				</div>

				{/* Error message */}
				{error && (
					<div className="clone-template-error" role="alert">
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
						{saving ? 'Cloning...' : 'Clone Template'}
					</Button>
				</div>
			</form>
		</Modal>
	);
}
