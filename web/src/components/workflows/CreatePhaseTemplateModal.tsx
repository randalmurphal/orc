/**
 * CreatePhaseTemplateModal - Modal for creating a new phase template from scratch.
 *
 * Features:
 * - Create new phase template with ID auto-generated from Name
 * - Prompt source toggle: Inline (DB) or File
 * - Inline prompt editor with {{VARIABLE}} highlighting
 * - Data flow: Input Variables (with suggestions), Output Variable Name
 * - Execution settings: Agent, Gate Type, Thinking, Checkpoint
 * - 7 collapsible runtime config sections (same as EditPhaseTemplateModal)
 */

import { useState, useCallback, useEffect, useRef } from 'react';
import { Modal } from '@/components/overlays/Modal';
import { Button, Icon } from '@/components/ui';
import { workflowClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';
import type { PhaseTemplate } from '@/gen/orc/v1/workflow_pb';
import { GateType, PromptSource } from '@/gen/orc/v1/workflow_pb';
import {
	parseRuntimeConfig,
	serializeRuntimeConfig,
	hydrateSelectedMCPServers,
	type HookDefinition,
} from '@/lib/runtimeConfigUtils';
import { GATE_TYPE_OPTIONS, slugify } from './phase-template-modal/constants';
import { PromptEditor } from './phase-template-modal/PromptEditor';
import { RuntimeConfigSections } from './phase-template-modal/RuntimeConfigSections';
import { VariableTagInput } from './phase-template-modal/VariableTagInput';
import {
	fetchMCPServerDefinition,
	usePhaseTemplateLibraries,
} from './phase-template-modal/usePhaseTemplateLibraries';
import './CreatePhaseTemplateModal.css';

export interface CreatePhaseTemplateModalProps {
	/** Whether the modal is open */
	open: boolean;
	/** Callback when modal should close */
	onClose: () => void;
	/** Callback when template is successfully created */
	onCreated: (template: PhaseTemplate) => void;
}

/**
 * CreatePhaseTemplateModal allows creating a new phase template from scratch.
 */
export function CreatePhaseTemplateModal({
	open,
	onClose,
	onCreated,
}: CreatePhaseTemplateModalProps) {
	// Form state
	const [templateId, setTemplateId] = useState('');
	const [name, setName] = useState('');
	const [description, setDescription] = useState('');
	const [idManuallyEdited, setIdManuallyEdited] = useState(false);

	// Prompt state
	const [promptSource, setPromptSource] = useState<'inline' | 'file'>('inline');
	const [promptContent, setPromptContent] = useState('');
	const [promptPath, setPromptPath] = useState('');

	// Data flow state
	const [inputVariables, setInputVariables] = useState<string[]>([]);
	const [outputVarName, setOutputVarName] = useState('');

	// Execution settings
	const [agentId, setAgentId] = useState('');
	const [gateType, setGateType] = useState<GateType>(GateType.AUTO);
	const [thinkingEnabled, setThinkingEnabled] = useState(false);
	const [checkpoint, setCheckpoint] = useState(false);

	// Claude config structured state
	const [selectedHooks, setSelectedHooks] = useState<string[]>([]);
	const [selectedSkills, setSelectedSkills] = useState<string[]>([]);
	const [selectedMCPServers, setSelectedMCPServers] = useState<string[]>([]);
	const [allowedTools, setAllowedTools] = useState<string[]>([]);
	const [disallowedTools, setDisallowedTools] = useState<string[]>([]);
	const [envVars, setEnvVars] = useState<Record<string, string>>({});
	const [extraFields, setExtraFields] = useState<Record<string, unknown>>({});
	const [mcpServerData, setMcpServerData] = useState<Record<string, unknown>>({});
	const [hookConfig, setHookConfig] = useState<Record<string, unknown>>({});
	const [hookEventTypes, setHookEventTypes] = useState<Record<string, string>>({});

	// JSON override state
	const [jsonOverride, setJsonOverride] = useState('{}');
	const [jsonError, setJsonError] = useState('');
	const jsonOverrideActiveRef = useRef(false);

	// Library data
	const {
		agents,
		agentsLoading,
		hooks,
		hooksError,
		hooksLoading,
		skills,
		skillsError,
		skillsLoading,
		mcpServers,
		mcpError,
		mcpLoading,
	} = usePhaseTemplateLibraries();

	// Saving state
	const [saving, setSaving] = useState(false);

	// Reset form when modal opens
	useEffect(() => {
		if (open) {
			setTemplateId('');
			setName('');
			setDescription('');
			setIdManuallyEdited(false);
			setPromptSource('inline');
			setPromptContent('');
			setPromptPath('');
			setInputVariables([]);
			setOutputVarName('');
			setAgentId('');
			setGateType(GateType.AUTO);
			setThinkingEnabled(false);
			setCheckpoint(false);
			setSelectedHooks([]);
			setSelectedSkills([]);
			setSelectedMCPServers([]);
			setAllowedTools([]);
			setDisallowedTools([]);
			setEnvVars({});
			setMcpServerData({});
			setJsonOverride('{}');
			setJsonError('');
			jsonOverrideActiveRef.current = false;
		}
	}, [open]);

	// Auto-generate ID from name (unless manually edited)
	const handleNameChange = useCallback((newName: string) => {
		setName(newName);
		if (!idManuallyEdited) {
			setTemplateId(slugify(newName));
		}
	}, [idManuallyEdited]);

	// Mark ID as manually edited
	const handleIdChange = useCallback((newId: string) => {
		setTemplateId(newId);
		setIdManuallyEdited(true);
	}, []);

	// Update JSON override when structured fields change
	useEffect(() => {
		if (!jsonOverrideActiveRef.current) {
			const json = serializeRuntimeConfig({
				hooks: selectedHooks,
				skillRefs: selectedSkills,
				mcpServers: selectedMCPServers,
				allowedTools,
				disallowedTools,
				env: envVars,
				extra: extraFields,
				mcpServerData,
				hookConfig,
				hookEventTypes,
			}, {
				hookDefinitions: hooks.map((hook): HookDefinition => ({
					name: hook.name,
					eventType: hook.eventType,
				})),
			});
			setJsonOverride(json);
		}
	}, [selectedHooks, selectedSkills, selectedMCPServers, allowedTools, disallowedTools, envVars, extraFields, mcpServerData, hookConfig, hookEventTypes, hooks]);

	useEffect(() => {
		let mounted = true;
		hydrateSelectedMCPServers(
			selectedMCPServers,
			mcpServerData,
			fetchMCPServerDefinition,
		).then((hydrated) => {
			if (mounted) {
				setMcpServerData(hydrated);
			}
		}).catch(() => {});

		return () => {
			mounted = false;
		};
	}, [selectedMCPServers]);

	// Handle JSON override blur
	const handleJsonBlur = useCallback(() => {
		try {
			const parsed = JSON.parse(jsonOverride);
			if (typeof parsed !== 'object' || parsed === null) {
				setJsonError('Invalid JSON');
				return;
			}

			// Re-parse into structured fields
			const config = parseRuntimeConfig(jsonOverride);
			setSelectedHooks(config.hooks);
			setSelectedSkills(config.skillRefs);
			setSelectedMCPServers(config.mcpServers);
			setAllowedTools(config.allowedTools);
			setDisallowedTools(config.disallowedTools);
			setEnvVars(config.env);
			setExtraFields(config.extra);
			setMcpServerData(config.mcpServerData ?? {});
			setHookConfig(config.hookConfig ?? {});
			setHookEventTypes(config.hookEventTypes ?? {});

			setJsonError('');
			jsonOverrideActiveRef.current = false;
		} catch {
			setJsonError('Invalid JSON');
		}
	}, [jsonOverride]);

	// Validate required fields
	const isValid = templateId.trim() !== '' && name.trim() !== '';

	// Handle create
	const handleCreate = useCallback(async () => {
		if (!isValid) return;

		setSaving(true);
		try {
			const hydratedMcpServerData = await hydrateSelectedMCPServers(
				selectedMCPServers,
				mcpServerData,
				fetchMCPServerDefinition,
			);
			const runtimeConfig = serializeRuntimeConfig({
				hooks: selectedHooks,
				skillRefs: selectedSkills,
				mcpServers: selectedMCPServers,
				allowedTools,
				disallowedTools,
				env: envVars,
				mcpServerData: hydratedMcpServerData,
				hookConfig,
				hookEventTypes,
				extra: extraFields,
			}, {
				hookDefinitions: hooks.map((hook): HookDefinition => ({
					name: hook.name,
					eventType: hook.eventType,
				})),
			});

			const response = await workflowClient.createPhaseTemplate({
				id: templateId.trim(),
				name: name.trim(),
				description: description.trim() || undefined,
				promptSource: promptSource === 'file' ? PromptSource.FILE : PromptSource.DB,
				promptContent: promptSource === 'inline' ? promptContent || undefined : undefined,
				promptPath: promptSource === 'file' ? promptPath || undefined : undefined,
				gateType: gateType,
				thinkingEnabled: thinkingEnabled || undefined,
				checkpoint: checkpoint,
				agentId: agentId || undefined,
				runtimeConfig: runtimeConfig !== '{}' ? runtimeConfig : undefined,
				outputVarName: outputVarName.trim() || undefined,
				producesArtifact: false,
				inputVariables: inputVariables.length > 0 ? inputVariables : undefined,
				subAgentIds: [],
			});

			if (response.template) {
				toast.success(`Phase template created: ${response.template.name}`);
				onCreated(response.template);
				onClose();
			}
		} catch (e) {
			const errorMsg = e instanceof Error ? e.message : 'Unknown error';
			toast.error(`Failed to create template: ${errorMsg}`);
		} finally {
			setSaving(false);
		}
	}, [
		isValid,
		templateId,
		name,
		description,
		promptSource,
		promptContent,
		promptPath,
		gateType,
		thinkingEnabled,
		checkpoint,
		agentId,
		inputVariables,
		selectedHooks,
		selectedSkills,
		selectedMCPServers,
		allowedTools,
		disallowedTools,
		envVars,
		extraFields,
		mcpServerData,
		outputVarName,
		onCreated,
		onClose,
	]);

	return (
		<Modal
			open={open}
			onClose={onClose}
			title="Create Phase Template"
			size="md"
			ariaLabel="Create phase template dialog"
		>
			<form
				onSubmit={(e) => {
					e.preventDefault();
					handleCreate();
				}}
				className="create-template-form"
			>
				{/* Metadata Section */}
				<div className="create-template-section">
					<h3 className="create-template-section-title">Metadata</h3>

					{/* Name */}
					<div className="form-group">
						<label htmlFor="create-template-name" className="form-label">
							Name <span className="form-required">*</span>
						</label>
						<input
							id="create-template-name"
							type="text"
							className="form-input"
							value={name}
							onChange={(e) => handleNameChange(e.target.value)}
							placeholder="e.g. Code Analysis"
						/>
					</div>

					{/* Template ID */}
					<div className="form-group">
						<label htmlFor="create-template-id" className="form-label">
							Template ID <span className="form-required">*</span>
						</label>
						<input
							id="create-template-id"
							type="text"
							className="form-input form-input-mono"
							value={templateId}
							onChange={(e) => handleIdChange(e.target.value)}
							placeholder="e.g. code-analysis"
						/>
						<span className="form-help">
							Unique identifier (auto-generated from name)
						</span>
					</div>

					{/* Description */}
					<div className="form-group">
						<label htmlFor="create-template-description" className="form-label">
							Description
						</label>
						<textarea
							id="create-template-description"
							className="form-textarea"
							value={description}
							onChange={(e) => setDescription(e.target.value)}
							placeholder="Describe what this phase does..."
							rows={2}
						/>
					</div>
				</div>

				{/* Prompt Section */}
				<div className="create-template-section">
					<h3 className="create-template-section-title">Prompt</h3>

					<div className="form-group">
						<div className="create-template-toggle">
							<button
								type="button"
								className={`create-template-toggle-btn ${promptSource === 'inline' ? 'create-template-toggle-btn--active' : ''}`}
								onClick={() => setPromptSource('inline')}
							>
								Inline
							</button>
							<button
								type="button"
								className={`create-template-toggle-btn ${promptSource === 'file' ? 'create-template-toggle-btn--active' : ''}`}
								onClick={() => setPromptSource('file')}
							>
								File
							</button>
						</div>
					</div>

					{promptSource === 'inline' && (
						<div className="form-group">
							<PromptEditor
								value={promptContent}
								onChange={setPromptContent}
								placeholder="Enter your prompt template..."
							/>
							<span className="form-help">
								Use {'{{VARIABLE_NAME}}'} for template variables
							</span>
						</div>
					)}

					{promptSource === 'file' && (
						<div className="form-group">
							<label htmlFor="create-template-prompt-path" className="form-label">
								Prompt Path
							</label>
							<div className="create-template-path-input">
								<span className="create-template-path-prefix">.orc/prompts/</span>
								<input
									id="create-template-prompt-path"
									type="text"
									className="form-input"
									value={promptPath}
									onChange={(e) => setPromptPath(e.target.value)}
									placeholder="path/to/prompt.md"
								/>
							</div>
						</div>
					)}
				</div>

				{/* Data Flow Section */}
				<div className="create-template-section">
					<h3 className="create-template-section-title">Data Flow</h3>

					{/* Input Variables */}
					<div className="form-group">
						<label className="form-label">Input Variables</label>
						<VariableTagInput
							tags={inputVariables}
							onChange={setInputVariables}
							placeholder="Add variable..."
						/>
						<span className="form-help">
							Variables this phase expects to receive
						</span>
					</div>

					{/* Output Variable Name */}
					<div className="form-group">
						<label htmlFor="create-template-output-var" className="form-label">
							Output Variable
						</label>
						<input
							id="create-template-output-var"
							type="text"
							className="form-input form-input-mono"
							value={outputVarName}
							onChange={(e) => setOutputVarName(e.target.value)}
							placeholder="e.g. SPEC_CONTENT"
						/>
						<span className="form-help">
							Downstream phases reference this name
						</span>
					</div>
				</div>

				{/* Execution Section */}
				<div className="create-template-section">
					<h3 className="create-template-section-title">Execution</h3>

					{/* Agent and Gate Type */}
					<div className="form-row">
						<div className="form-group form-group-half">
							<label htmlFor="create-template-agent" className="form-label">
								Agent
							</label>
							<select
								id="create-template-agent"
								className="form-select"
								value={agentId}
								onChange={(e) => setAgentId(e.target.value)}
								disabled={agentsLoading}
							>
								<option value="">Default</option>
								{agents.map((agent) => (
									<option key={agent.name} value={agent.name}>
										{agent.name}
										{agent.model ? ` (${agent.model})` : ''}
									</option>
								))}
							</select>
						</div>

						<div className="form-group form-group-half">
							<label htmlFor="create-template-gate" className="form-label">
								Gate Type
							</label>
							<select
								id="create-template-gate"
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

					{/* Options */}
					<div className="form-group">
						<label className="form-label">Options</label>
						<div className="create-template-options">
							<label className="form-checkbox">
								<input
									type="checkbox"
									checked={thinkingEnabled}
									onChange={(e) => setThinkingEnabled(e.target.checked)}
								/>
								<span className="form-checkbox-label">Enable Thinking</span>
							</label>
							<label className="form-checkbox">
								<input
									type="checkbox"
									checked={checkpoint}
									onChange={(e) => setCheckpoint(e.target.checked)}
								/>
								<span className="form-checkbox-label">Checkpoint</span>
							</label>
						</div>
					</div>
				</div>

				{/* Runtime Config Settings Sections */}
				<div className="create-template-section">
					<h3 className="create-template-section-title">Runtime Config</h3>

					<RuntimeConfigSections
						selectedHooks={selectedHooks}
						onSelectedHooksChange={(names) => {
							setSelectedHooks(names);
							jsonOverrideActiveRef.current = false;
						}}
						selectedMcpServers={selectedMCPServers}
						onSelectedMcpServersChange={(names) => {
							setSelectedMCPServers(names);
							jsonOverrideActiveRef.current = false;
						}}
						selectedSkills={selectedSkills}
						onSelectedSkillsChange={(names) => {
							setSelectedSkills(names);
							jsonOverrideActiveRef.current = false;
						}}
						allowedTools={allowedTools}
						onAllowedToolsChange={(tags) => {
							setAllowedTools(tags);
							jsonOverrideActiveRef.current = false;
						}}
						disallowedTools={disallowedTools}
						onDisallowedToolsChange={(tags) => {
							setDisallowedTools(tags);
							jsonOverrideActiveRef.current = false;
						}}
						envVars={envVars}
						onEnvVarsChange={(entries) => {
							setEnvVars(entries);
							jsonOverrideActiveRef.current = false;
						}}
						jsonOverride={jsonOverride}
						onJsonOverrideChange={(value) => {
							setJsonOverride(value);
							jsonOverrideActiveRef.current = true;
							setJsonError('');
						}}
						onJsonOverrideBlur={handleJsonBlur}
						jsonError={jsonError}
						hooks={hooks}
						hooksError={hooksError}
						hooksLoading={hooksLoading}
						skills={skills}
						skillsError={skillsError}
						skillsLoading={skillsLoading}
						mcpServers={mcpServers}
						mcpError={mcpError}
						mcpLoading={mcpLoading}
						jsonWrapperClassName="create-template-json-override"
						jsonTextareaClassName="create-template-json-textarea"
						jsonErrorClassName="create-template-json-error"
					/>
				</div>

				{/* Actions */}
				<div className="form-actions">
					<Button
						type="button"
						variant="ghost"
						onClick={onClose}
						disabled={saving}
					>
						Cancel
					</Button>
					<Button
						type="submit"
						variant="primary"
						disabled={saving || !isValid}
						leftIcon={<Icon name="plus" size={12} />}
					>
						{saving ? 'Creating...' : 'Create'}
					</Button>
				</div>
			</form>
		</Modal>
	);
}
