import { useState, useCallback, useEffect } from 'react';
import * as Dialog from '@radix-ui/react-dialog';
import { X } from 'lucide-react';
import { workflowClient } from '@/lib/client';
import { VariableSourceType, type WorkflowVariable } from '@/gen/orc/v1/workflow_pb';
import './VariableModal.css';

// Source config type definitions
interface StaticConfig {
	value: string;
}

interface EnvConfig {
	var: string;
	default?: string;
}

interface ScriptConfig {
	path: string;
	args?: string[];
	workDir?: string;
	timeout?: number;
}

interface ApiConfig {
	url: string;
	method?: string;
	headers?: Record<string, string>;
	jqFilter?: string;
	timeout?: number;
}

interface PhaseOutputConfig {
	phase: string;
	field?: string;
}

interface PromptFragmentConfig {
	path: string;
}

type SourceConfig = StaticConfig | EnvConfig | ScriptConfig | ApiConfig | PhaseOutputConfig | PromptFragmentConfig;

interface VariableModalProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	workflowId: string;
	variable?: WorkflowVariable; // If editing existing variable
	availablePhases?: string[]; // For phase_output dropdown
	onSuccess?: () => void;
}

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
				setSourceConfig({ value: '' });
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
		// Reset source config to appropriate default
		switch (newType) {
			case VariableSourceType.STATIC:
				setSourceConfig({ value: '' });
				break;
			case VariableSourceType.ENV:
				setSourceConfig({ var: '' });
				break;
			case VariableSourceType.SCRIPT:
				setSourceConfig({ path: '' });
				break;
			case VariableSourceType.API:
				setSourceConfig({ url: '', method: 'GET' });
				break;
			case VariableSourceType.PHASE_OUTPUT:
				setSourceConfig({ phase: '' });
				break;
			case VariableSourceType.PROMPT_FRAGMENT:
				setSourceConfig({ path: '' });
				break;
		}
	}, []);

	const handleSubmit = useCallback(async (e: React.FormEvent) => {
		e.preventDefault();
		setError(null);
		setSaving(true);

		try {
			const trimmedName = name.trim().toUpperCase().replace(/[^A-Z0-9_]/g, '_');

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

// ─── Source Type Radio ─────────────────────────────────────────────────────

interface SourceTypeRadioProps {
	value: VariableSourceType;
	selected: VariableSourceType;
	onChange: (type: VariableSourceType) => void;
	label: string;
}

function SourceTypeRadio({ value, selected, onChange, label }: SourceTypeRadioProps) {
	return (
		<label className={`variable-modal-source-radio ${selected === value ? 'variable-modal-source-radio--selected' : ''}`}>
			<input
				type="radio"
				name="source-type"
				checked={selected === value}
				onChange={() => onChange(value)}
				className="sr-only"
			/>
			<span>{label}</span>
		</label>
	);
}

// ─── Source Config Fields ──────────────────────────────────────────────────

interface SourceConfigFieldsProps {
	sourceType: VariableSourceType;
	config: SourceConfig;
	onChange: (config: SourceConfig) => void;
	availablePhases: string[];
}

function SourceConfigFields({ sourceType, config, onChange, availablePhases }: SourceConfigFieldsProps) {
	switch (sourceType) {
		case VariableSourceType.STATIC:
			return (
				<StaticSourceForm
					config={config as StaticConfig}
					onChange={onChange}
				/>
			);
		case VariableSourceType.ENV:
			return (
				<EnvSourceForm
					config={config as EnvConfig}
					onChange={onChange}
				/>
			);
		case VariableSourceType.SCRIPT:
			return (
				<ScriptSourceForm
					config={config as ScriptConfig}
					onChange={onChange}
				/>
			);
		case VariableSourceType.API:
			return (
				<ApiSourceForm
					config={config as ApiConfig}
					onChange={onChange}
				/>
			);
		case VariableSourceType.PHASE_OUTPUT:
			return (
				<PhaseOutputSourceForm
					config={config as PhaseOutputConfig}
					onChange={onChange}
					availablePhases={availablePhases}
				/>
			);
		case VariableSourceType.PROMPT_FRAGMENT:
			return (
				<PromptFragmentSourceForm
					config={config as PromptFragmentConfig}
					onChange={onChange}
				/>
			);
		default:
			return null;
	}
}

// ─── Static Source Form ────────────────────────────────────────────────────

interface StaticSourceFormProps {
	config: StaticConfig;
	onChange: (config: StaticConfig) => void;
}

function StaticSourceForm({ config, onChange }: StaticSourceFormProps) {
	return (
		<div className="variable-modal-source-fields">
			<div className="variable-modal-field">
				<label htmlFor="static-value" className="variable-modal-label">
					Value <span className="variable-modal-required">*</span>
				</label>
				<textarea
					id="static-value"
					className="variable-modal-textarea"
					value={config.value}
					onChange={(e) => onChange({ ...config, value: e.target.value })}
					placeholder="The static value"
					rows={3}
				/>
			</div>
		</div>
	);
}

// ─── Environment Source Form ───────────────────────────────────────────────

interface EnvSourceFormProps {
	config: EnvConfig;
	onChange: (config: EnvConfig) => void;
}

function EnvSourceForm({ config, onChange }: EnvSourceFormProps) {
	return (
		<div className="variable-modal-source-fields">
			<div className="variable-modal-field">
				<label htmlFor="env-var" className="variable-modal-label">
					Environment Variable <span className="variable-modal-required">*</span>
				</label>
				<input
					id="env-var"
					type="text"
					className="variable-modal-input"
					value={config.var}
					onChange={(e) => onChange({ ...config, var: e.target.value })}
					placeholder="MY_ENV_VAR"
				/>
			</div>
			<div className="variable-modal-field">
				<label htmlFor="env-default" className="variable-modal-label">
					Default (if not set)
				</label>
				<input
					id="env-default"
					type="text"
					className="variable-modal-input"
					value={config.default ?? ''}
					onChange={(e) => onChange({ ...config, default: e.target.value || undefined })}
					placeholder="fallback value"
				/>
			</div>
		</div>
	);
}

// ─── Script Source Form ────────────────────────────────────────────────────

interface ScriptSourceFormProps {
	config: ScriptConfig;
	onChange: (config: ScriptConfig) => void;
}

function ScriptSourceForm({ config, onChange }: ScriptSourceFormProps) {
	const [argsText, setArgsText] = useState((config.args ?? []).join(' '));

	return (
		<div className="variable-modal-source-fields">
			<div className="variable-modal-field">
				<label htmlFor="script-path" className="variable-modal-label">
					Script Path <span className="variable-modal-required">*</span>
				</label>
				<input
					id="script-path"
					type="text"
					className="variable-modal-input"
					value={config.path}
					onChange={(e) => onChange({ ...config, path: e.target.value })}
					placeholder=".orc/scripts/fetch-token.sh"
				/>
			</div>
			<div className="variable-modal-field">
				<label htmlFor="script-args" className="variable-modal-label">
					Arguments
				</label>
				<input
					id="script-args"
					type="text"
					className="variable-modal-input"
					value={argsText}
					onChange={(e) => {
						setArgsText(e.target.value);
						const args = e.target.value.split(/\s+/).filter(Boolean);
						onChange({ ...config, args: args.length > 0 ? args : undefined });
					}}
					placeholder="--env {{TASK_ID}}"
				/>
				<span className="variable-modal-hint">
					Space-separated. Supports {'{{VAR}}'} interpolation.
				</span>
			</div>
			<div className="variable-modal-field">
				<label htmlFor="script-timeout" className="variable-modal-label">
					Timeout (ms)
				</label>
				<input
					id="script-timeout"
					type="number"
					className="variable-modal-input variable-modal-input--narrow"
					value={config.timeout ?? ''}
					onChange={(e) => onChange({ ...config, timeout: parseInt(e.target.value) || undefined })}
					placeholder="5000"
					min={0}
				/>
			</div>
		</div>
	);
}

// ─── API Source Form ───────────────────────────────────────────────────────

interface ApiSourceFormProps {
	config: ApiConfig;
	onChange: (config: ApiConfig) => void;
}

function ApiSourceForm({ config, onChange }: ApiSourceFormProps) {
	const [headersText, setHeadersText] = useState(
		config.headers ? JSON.stringify(config.headers, null, 2) : ''
	);

	return (
		<div className="variable-modal-source-fields">
			<div className="variable-modal-field">
				<label htmlFor="api-url" className="variable-modal-label">
					URL <span className="variable-modal-required">*</span>
				</label>
				<input
					id="api-url"
					type="text"
					className="variable-modal-input"
					value={config.url}
					onChange={(e) => onChange({ ...config, url: e.target.value })}
					placeholder="https://api.example.com/data"
				/>
				<span className="variable-modal-hint">
					Supports {'{{VAR}}'} interpolation.
				</span>
			</div>
			<div className="variable-modal-field">
				<label htmlFor="api-method" className="variable-modal-label">
					Method
				</label>
				<select
					id="api-method"
					className="variable-modal-select"
					value={config.method ?? 'GET'}
					onChange={(e) => onChange({ ...config, method: e.target.value })}
				>
					<option value="GET">GET</option>
					<option value="POST">POST</option>
					<option value="PUT">PUT</option>
				</select>
			</div>
			<div className="variable-modal-field">
				<label htmlFor="api-headers" className="variable-modal-label">
					Headers (JSON)
				</label>
				<textarea
					id="api-headers"
					className="variable-modal-textarea"
					value={headersText}
					onChange={(e) => {
						setHeadersText(e.target.value);
						try {
							const headers = e.target.value.trim() ? JSON.parse(e.target.value) : undefined;
							onChange({ ...config, headers });
						} catch {
							// Invalid JSON, don't update
						}
					}}
					placeholder='{"Authorization": "Bearer {{API_TOKEN}}"}'
					rows={3}
				/>
			</div>
			<div className="variable-modal-field">
				<label htmlFor="api-jq" className="variable-modal-label">
					JQ Filter (JSONPath)
				</label>
				<input
					id="api-jq"
					type="text"
					className="variable-modal-input"
					value={config.jqFilter ?? ''}
					onChange={(e) => onChange({ ...config, jqFilter: e.target.value || undefined })}
					placeholder="data.result"
				/>
			</div>
			<div className="variable-modal-field">
				<label htmlFor="api-timeout" className="variable-modal-label">
					Timeout (ms)
				</label>
				<input
					id="api-timeout"
					type="number"
					className="variable-modal-input variable-modal-input--narrow"
					value={config.timeout ?? ''}
					onChange={(e) => onChange({ ...config, timeout: parseInt(e.target.value) || undefined })}
					placeholder="10000"
					min={0}
				/>
			</div>
		</div>
	);
}

// ─── Phase Output Source Form ──────────────────────────────────────────────

interface PhaseOutputSourceFormProps {
	config: PhaseOutputConfig;
	onChange: (config: PhaseOutputConfig) => void;
	availablePhases: string[];
}

function PhaseOutputSourceForm({ config, onChange, availablePhases }: PhaseOutputSourceFormProps) {
	return (
		<div className="variable-modal-source-fields">
			<div className="variable-modal-field">
				<label htmlFor="phase-output-phase" className="variable-modal-label">
					Phase <span className="variable-modal-required">*</span>
				</label>
				{availablePhases.length > 0 ? (
					<select
						id="phase-output-phase"
						className="variable-modal-select"
						value={config.phase}
						onChange={(e) => onChange({ ...config, phase: e.target.value })}
					>
						<option value="">Select a phase</option>
						{availablePhases.map((p) => (
							<option key={p} value={p}>{p}</option>
						))}
					</select>
				) : (
					<input
						id="phase-output-phase"
						type="text"
						className="variable-modal-input"
						value={config.phase}
						onChange={(e) => onChange({ ...config, phase: e.target.value })}
						placeholder="spec"
					/>
				)}
			</div>
			<div className="variable-modal-field">
				<label htmlFor="phase-output-field" className="variable-modal-label">
					Field (optional)
				</label>
				<input
					id="phase-output-field"
					type="text"
					className="variable-modal-input"
					value={config.field ?? ''}
					onChange={(e) => onChange({ ...config, field: e.target.value || undefined })}
					placeholder="content"
				/>
				<span className="variable-modal-hint">
					Extract a specific field from the phase output JSON
				</span>
			</div>
		</div>
	);
}

// ─── Prompt Fragment Source Form ───────────────────────────────────────────

interface PromptFragmentSourceFormProps {
	config: PromptFragmentConfig;
	onChange: (config: PromptFragmentConfig) => void;
}

function PromptFragmentSourceForm({ config, onChange }: PromptFragmentSourceFormProps) {
	return (
		<div className="variable-modal-source-fields">
			<div className="variable-modal-field">
				<label htmlFor="fragment-path" className="variable-modal-label">
					Fragment Path <span className="variable-modal-required">*</span>
				</label>
				<input
					id="fragment-path"
					type="text"
					className="variable-modal-input"
					value={config.path}
					onChange={(e) => onChange({ ...config, path: e.target.value })}
					placeholder=".orc/prompts/fragments/code-style.md"
				/>
			</div>
		</div>
	);
}

// ─── Helper Functions ──────────────────────────────────────────────────────

function parseSourceConfig(sourceType: VariableSourceType, configJson: string): SourceConfig {
	try {
		const parsed = JSON.parse(configJson);
		return parsed;
	} catch {
		// Return appropriate default based on source type
		switch (sourceType) {
			case VariableSourceType.STATIC:
				return { value: '' };
			case VariableSourceType.ENV:
				return { var: '' };
			case VariableSourceType.SCRIPT:
				return { path: '' };
			case VariableSourceType.API:
				return { url: '' };
			case VariableSourceType.PHASE_OUTPUT:
				return { phase: '' };
			case VariableSourceType.PROMPT_FRAGMENT:
				return { path: '' };
			default:
				return { value: '' };
		}
	}
}
