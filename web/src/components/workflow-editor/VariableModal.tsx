import { useState, useCallback, useEffect } from 'react';
import * as Dialog from '@radix-ui/react-dialog';
import { X } from 'lucide-react';
import { workflowClient } from '@/lib/client';
import { VariableSourceType, type WorkflowVariable } from '@/gen/orc/v1/workflow_pb';
import { parseSourceConfig, normalizeVariableName } from './variable-modal/helpers';
import { SourceConfigFields, SourceTypeRadio } from './variable-modal/SourceConfigFields';
import { defaultSourceConfig, type SourceConfig, type VariableModalProps } from './variable-modal/types';
import './VariableModal.css';

export function VariableModal({
	open,
	onOpenChange,
	workflowId,
	variable,
	availablePhases = [],
	onSuccess,
}: VariableModalProps) {
	const isEditing = !!variable;

	// Form state
	const [name, setName] = useState('');
	const [description, setDescription] = useState('');
	const [sourceType, setSourceType] = useState<VariableSourceType>(VariableSourceType.STATIC);
	const [sourceConfig, setSourceConfig] = useState<SourceConfig>({ value: '' });
	const [extract, setExtract] = useState('');
	const [required, setRequired] = useState(false);
	const [defaultValue, setDefaultValue] = useState('');
	const [cacheTtl, setCacheTtl] = useState(0);

	const [saving, setSaving] = useState(false);
	const [deleting, setDeleting] = useState(false);
	const [confirmingDelete, setConfirmingDelete] = useState(false);
	const [error, setError] = useState<string | null>(null);

	// Reset form when modal opens/closes or variable changes
	useEffect(() => {
		if (open) {
			if (variable) {
				setName(variable.name);
				setDescription(variable.description ?? '');
				setSourceType(variable.sourceType);
				setSourceConfig(parseSourceConfig(variable.sourceType, variable.sourceConfig));
				setExtract(variable.extract ?? '');
				setRequired(variable.required);
				setDefaultValue(variable.defaultValue ?? '');
				setCacheTtl(variable.cacheTtlSeconds);
			} else {
				// Reset to defaults for new variable
				setName('');
				setDescription('');
				setSourceType(VariableSourceType.STATIC);
				setSourceConfig(defaultSourceConfig(VariableSourceType.STATIC));
				setExtract('');
				setRequired(false);
				setDefaultValue('');
				setCacheTtl(0);
			}
			setError(null);
			setConfirmingDelete(false);
		}
	}, [open, variable]);

	const handleSourceTypeChange = useCallback((newType: VariableSourceType) => {
		setSourceType(newType);
		setSourceConfig(defaultSourceConfig(newType));
	}, []);

	const handleSubmit = useCallback(async (e: React.FormEvent) => {
		e.preventDefault();
		setError(null);
		setSaving(true);

		try {
			const trimmedName = normalizeVariableName(name);

			if (!trimmedName) {
				throw new Error('Variable name is required');
			}

			if (isEditing) {
				// Update existing variable
				await workflowClient.updateVariable({
					workflowId,
					name: trimmedName,
					description: description.trim() || undefined,
					sourceType,
					sourceConfig: JSON.stringify(sourceConfig),
					required,
					defaultValue: defaultValue.trim() || undefined,
					cacheTtlSeconds: cacheTtl,
					extract: extract.trim() || undefined,
				});
			} else {
				// Create new variable
				await workflowClient.addVariable({
					workflowId,
					name: trimmedName,
					description: description.trim() || undefined,
					sourceType,
					sourceConfig: JSON.stringify(sourceConfig),
					required,
					defaultValue: defaultValue.trim() || undefined,
					cacheTtlSeconds: cacheTtl,
					extract: extract.trim() || undefined,
				});
			}

			onOpenChange(false);
			onSuccess?.();
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to save variable');
		} finally {
			setSaving(false);
		}
	}, [workflowId, name, description, sourceType, sourceConfig, required, defaultValue, cacheTtl, extract, isEditing, onOpenChange, onSuccess]);

	const handleDelete = useCallback(async () => {
		if (!confirmingDelete) {
			setConfirmingDelete(true);
			return;
		}

		setError(null);
		setDeleting(true);

		try {
			await workflowClient.removeVariable({
				workflowId,
				name: variable?.name ?? '',
			});

			onOpenChange(false);
			onSuccess?.();
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to delete variable');
		} finally {
			setDeleting(false);
			setConfirmingDelete(false);
		}
	}, [workflowId, variable, confirmingDelete, onOpenChange, onSuccess]);

	return (
		<Dialog.Root open={open} onOpenChange={onOpenChange}>
			<Dialog.Portal>
				<Dialog.Overlay className="variable-modal-overlay" />
				<Dialog.Content className="variable-modal-content">
					<Dialog.Title className="variable-modal-title">
						{isEditing ? 'Edit Variable' : 'Add Variable'}
					</Dialog.Title>
					<Dialog.Description className="sr-only">
						Configure a workflow variable
					</Dialog.Description>

					<form onSubmit={handleSubmit} className="variable-modal-form">
						{error && (
							<div className="variable-modal-error">{error}</div>
						)}

						{/* Name */}
						<div className="variable-modal-field">
							<label htmlFor="var-name" className="variable-modal-label">
								Name <span className="variable-modal-required">*</span>
							</label>
							<input
								id="var-name"
								type="text"
								className="variable-modal-input"
								value={name}
								onChange={(e) => setName(e.target.value)}
								placeholder="MY_VARIABLE"
								disabled={isEditing}
							/>
							<span className="variable-modal-hint">
								Used in prompts as <code>{`{{${name.toUpperCase().replace(/[^A-Z0-9_]/g, '_') || 'VAR_NAME'}}}`}</code>
							</span>
						</div>

						{/* Description */}
						<div className="variable-modal-field">
							<label htmlFor="var-description" className="variable-modal-label">
								Description
							</label>
							<input
								id="var-description"
								type="text"
								className="variable-modal-input"
								value={description}
								onChange={(e) => setDescription(e.target.value)}
								placeholder="What this variable is used for"
							/>
						</div>

						{/* Source Type */}
						<div className="variable-modal-field">
							<label className="variable-modal-label">Source Type</label>
							<div className="variable-modal-source-types">
								<SourceTypeRadio
									value={VariableSourceType.STATIC}
									selected={sourceType}
									onChange={handleSourceTypeChange}
									label="Static"
								/>
								<SourceTypeRadio
									value={VariableSourceType.ENV}
									selected={sourceType}
									onChange={handleSourceTypeChange}
									label="Environment"
								/>
								<SourceTypeRadio
									value={VariableSourceType.SCRIPT}
									selected={sourceType}
									onChange={handleSourceTypeChange}
									label="Script"
								/>
								<SourceTypeRadio
									value={VariableSourceType.API}
									selected={sourceType}
									onChange={handleSourceTypeChange}
									label="API"
								/>
								<SourceTypeRadio
									value={VariableSourceType.PHASE_OUTPUT}
									selected={sourceType}
									onChange={handleSourceTypeChange}
									label="Phase Output"
								/>
								<SourceTypeRadio
									value={VariableSourceType.PROMPT_FRAGMENT}
									selected={sourceType}
									onChange={handleSourceTypeChange}
									label="Prompt Fragment"
								/>
							</div>
						</div>

						{/* Source-specific fields */}
						<SourceConfigFields
							sourceType={sourceType}
							config={sourceConfig}
							onChange={setSourceConfig}
							availablePhases={availablePhases}
						/>

						{/* Extraction (optional) */}
						<details className="variable-modal-extraction">
							<summary className="variable-modal-extraction-summary">
								Extraction (optional)
							</summary>
							<div className="variable-modal-field">
								<label htmlFor="var-extract" className="variable-modal-label">
									JSONPath Expression
								</label>
								<input
									id="var-extract"
									type="text"
									className="variable-modal-input"
									value={extract}
									onChange={(e) => setExtract(e.target.value)}
									placeholder="data.items.0.name"
								/>
								<span className="variable-modal-hint">
									Extract a specific field from JSON output using gjson syntax
								</span>
							</div>
						</details>

						{/* Required */}
						<div className="variable-modal-field variable-modal-field--checkbox">
							<input
								id="var-required"
								type="checkbox"
								className="variable-modal-checkbox"
								checked={required}
								onChange={(e) => setRequired(e.target.checked)}
							/>
							<label htmlFor="var-required" className="variable-modal-label">
								Required (fail if resolution fails)
							</label>
						</div>

						{/* Default Value */}
						{!required && (
							<div className="variable-modal-field">
								<label htmlFor="var-default" className="variable-modal-label">
									Default Value
								</label>
								<input
									id="var-default"
									type="text"
									className="variable-modal-input"
									value={defaultValue}
									onChange={(e) => setDefaultValue(e.target.value)}
									placeholder="Fallback value if resolution fails"
								/>
							</div>
						)}

						{/* Cache TTL */}
						<div className="variable-modal-field">
							<label htmlFor="var-cache" className="variable-modal-label">
								Cache TTL (seconds)
							</label>
							<input
								id="var-cache"
								type="number"
								className="variable-modal-input variable-modal-input--narrow"
								value={cacheTtl}
								onChange={(e) => setCacheTtl(parseInt(e.target.value) || 0)}
								min={0}
							/>
							<span className="variable-modal-hint">
								0 = no caching
							</span>
						</div>

						{/* Actions */}
						<div className="variable-modal-actions">
							{isEditing && (
								<button
									type="button"
									className={`variable-modal-btn variable-modal-btn--delete ${confirmingDelete ? 'variable-modal-btn--delete-confirm' : ''}`}
									onClick={handleDelete}
									disabled={saving || deleting}
								>
									{deleting ? 'Deleting...' : confirmingDelete ? 'Click to Confirm' : 'Delete'}
								</button>
							)}
							<div className="variable-modal-actions-right">
								<button
									type="button"
									className="variable-modal-btn variable-modal-btn--cancel"
									onClick={() => {
										if (confirmingDelete) {
											setConfirmingDelete(false);
										} else {
											onOpenChange(false);
										}
									}}
									disabled={saving || deleting}
								>
									Cancel
								</button>
								<button
									type="submit"
									className="variable-modal-btn variable-modal-btn--save"
									disabled={saving || deleting || !name.trim()}
								>
									{saving ? 'Saving...' : isEditing ? 'Save Changes' : 'Add Variable'}
								</button>
							</div>
						</div>
					</form>

					<Dialog.Close asChild>
						<button className="variable-modal-close" aria-label="Close">
							<X size={20} />
						</button>
					</Dialog.Close>
				</Dialog.Content>
			</Dialog.Portal>
		</Dialog.Root>
	);
}
