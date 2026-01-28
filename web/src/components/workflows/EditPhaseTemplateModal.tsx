/**
 * EditPhaseTemplateModal - Modal for editing phase template metadata.
 *
 * Features:
 * - Edit phase template name, description
 * - Edit execution settings (model, gate type, max iterations)
 * - Edit checkpoint and thinking settings
 * - Built-in templates cannot be edited (shows clone suggestion)
 */

import { useState, useCallback, useEffect } from 'react';
import { Modal } from '@/components/overlays/Modal';
import { Button, Icon } from '@/components/ui';
import { workflowClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';
import type { PhaseTemplate } from '@/gen/orc/v1/workflow_pb';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import './EditPhaseTemplateModal.css';

export interface EditPhaseTemplateModalProps {
	/** Whether the modal is open */
	open: boolean;
	/** The template to edit */
	template: PhaseTemplate;
	/** Whether this is a built-in template (read-only) */
	isBuiltin?: boolean;
	/** Callback when modal should close */
	onClose: () => void;
	/** Callback when template is successfully updated */
	onUpdated: (template: PhaseTemplate) => void;
}

const MODEL_OPTIONS = [
	{ value: '', label: 'Default (inherit)' },
	{ value: 'sonnet', label: 'Sonnet' },
	{ value: 'opus', label: 'Opus' },
	{ value: 'haiku', label: 'Haiku' },
];

const GATE_TYPE_OPTIONS = [
	{ value: GateType.AUTO, label: 'Auto' },
	{ value: GateType.HUMAN, label: 'Human' },
	{ value: GateType.SKIP, label: 'Skip' },
];

/**
 * EditPhaseTemplateModal allows editing phase template metadata.
 */
export function EditPhaseTemplateModal({
	open,
	template,
	isBuiltin = false,
	onClose,
	onUpdated,
}: EditPhaseTemplateModalProps) {
	// Form state
	const [name, setName] = useState('');
	const [description, setDescription] = useState('');
	const [modelOverride, setModelOverride] = useState('');
	const [maxIterations, setMaxIterations] = useState(50);
	const [gateType, setGateType] = useState<GateType>(GateType.AUTO);
	const [thinkingEnabled, setThinkingEnabled] = useState(false);
	const [checkpoint, setCheckpoint] = useState(false);

	// Loading state
	const [saving, setSaving] = useState(false);

	// Reset form when template changes or modal opens
	useEffect(() => {
		if (open && template) {
			setName(template.name || '');
			setDescription(template.description || '');
			setModelOverride(template.modelOverride || '');
			setMaxIterations(template.maxIterations || 50);
			setGateType(template.gateType || GateType.AUTO);
			setThinkingEnabled(template.thinkingEnabled || false);
			setCheckpoint(template.checkpoint || false);
		}
	}, [open, template]);

	// Handle save
	const handleSave = useCallback(async () => {
		if (!template) return;

		setSaving(true);
		try {
			const response = await workflowClient.updatePhaseTemplate({
				id: template.id,
				name: name.trim() || undefined,
				description: description.trim() || undefined,
				modelOverride: modelOverride || undefined,
				maxIterations: maxIterations,
				gateType: gateType,
				thinkingEnabled: thinkingEnabled,
				checkpoint: checkpoint,
			});
			if (response.template) {
				toast.success('Phase template updated successfully');
				onUpdated(response.template);
				onClose();
			}
		} catch (e) {
			const errorMsg = e instanceof Error ? e.message : 'Unknown error';
			toast.error(`Failed to update template: ${errorMsg}`);
		} finally {
			setSaving(false);
		}
	}, [
		template,
		name,
		description,
		modelOverride,
		maxIterations,
		gateType,
		thinkingEnabled,
		checkpoint,
		onUpdated,
		onClose,
	]);

	// Handle close
	const handleClose = useCallback(() => {
		onClose();
	}, [onClose]);

	// Built-in template message
	if (isBuiltin) {
		return (
			<Modal
				open={open}
				onClose={handleClose}
				title="Built-in Template"
				size="sm"
				ariaLabel="Built-in template dialog"
			>
				<div className="edit-template-builtin">
					<Icon name="shield" size={24} />
					<p>
						Cannot edit built-in template. Clone to customize this template.
					</p>
					<Button variant="primary" onClick={handleClose}>
						OK
					</Button>
				</div>
			</Modal>
		);
	}

	return (
		<Modal
			open={open}
			onClose={handleClose}
			title="Edit Phase Template"
			size="md"
			ariaLabel="Edit phase template dialog"
		>
			<form
				onSubmit={(e) => {
					e.preventDefault();
					handleSave();
				}}
				className="edit-template-form"
			>
				{/* Metadata Section */}
				<div className="edit-template-section">
					<h3 className="edit-template-section-title">Metadata</h3>

					{/* Name */}
					<div className="form-group">
						<label htmlFor="edit-template-name" className="form-label">
							Name
						</label>
						<input
							id="edit-template-name"
							type="text"
							className="form-input"
							value={name}
							onChange={(e) => setName(e.target.value)}
							placeholder="Phase name"
						/>
					</div>

					{/* Description */}
					<div className="form-group">
						<label htmlFor="edit-template-description" className="form-label">
							Description
						</label>
						<textarea
							id="edit-template-description"
							className="form-textarea"
							value={description}
							onChange={(e) => setDescription(e.target.value)}
							placeholder="Describe what this phase does..."
							rows={2}
						/>
					</div>
				</div>

				{/* Execution Section */}
				<div className="edit-template-section">
					<h3 className="edit-template-section-title">Execution</h3>

					{/* Model and Gate Type */}
					<div className="form-row">
						<div className="form-group form-group-half">
							<label htmlFor="edit-template-model" className="form-label">
								LLM Model
							</label>
							<select
								id="edit-template-model"
								className="form-select"
								value={modelOverride}
								onChange={(e) => setModelOverride(e.target.value)}
							>
								{MODEL_OPTIONS.map((option) => (
									<option key={option.value} value={option.value}>
										{option.label}
									</option>
								))}
							</select>
						</div>

						<div className="form-group form-group-half">
							<label htmlFor="edit-template-gate" className="form-label">
								Gate Type
							</label>
							<select
								id="edit-template-gate"
								className="form-select"
								value={gateType}
								onChange={(e) => setGateType(Number(e.target.value) as GateType)}
							>
								{GATE_TYPE_OPTIONS.map((option) => (
									<option key={option.value} value={option.value}>
										{option.label}
									</option>
								))}
							</select>
						</div>
					</div>

					{/* Max iterations */}
					<div className="form-group">
						<label htmlFor="edit-template-iterations" className="form-label">
							Max Iterations
						</label>
						<input
							id="edit-template-iterations"
							type="number"
							className="form-input"
							value={maxIterations}
							onChange={(e) => setMaxIterations(Number(e.target.value))}
							min={1}
							max={1000}
						/>
						<span className="form-help">
							Maximum number of LLM iterations for this phase
						</span>
					</div>

					{/* Options */}
					<div className="form-group">
						<label className="form-label">Options</label>
						<div className="edit-template-options">
							<label className="form-checkbox">
								<input
									type="checkbox"
									checked={thinkingEnabled}
									onChange={(e) => setThinkingEnabled(e.target.checked)}
								/>
								<span className="form-checkbox-label">Enable deep reasoning (thinking)</span>
							</label>
							<label className="form-checkbox">
								<input
									type="checkbox"
									checked={checkpoint}
									onChange={(e) => setCheckpoint(e.target.checked)}
								/>
								<span className="form-checkbox-label">Create checkpoint after phase</span>
							</label>
						</div>
					</div>
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
		</Modal>
	);
}
