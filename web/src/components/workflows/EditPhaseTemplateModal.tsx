/**
 * EditPhaseTemplateModal - Modal for editing phase template metadata.
 *
 * Features:
 * - Edit phase template name, description
 * - Edit execution settings (agent, gate type, max iterations)
 * - Edit checkpoint and thinking settings
 * - 7 collapsible settings sections for runtime_config (hooks, MCP servers, skills,
 *   allowed/disallowed tools, env vars, JSON override)
 * - Built-in templates cannot be edited (shows clone suggestion)
 *
 * Note: Model is now on the Agent, not the PhaseTemplate.
 * Agent assignment is done via agentId.
 */

import { useState, useCallback, useEffect, useRef } from 'react';
import { Modal } from '@/components/overlays/Modal';
import { Button, Icon } from '@/components/ui';
import { workflowClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';
import type { PhaseTemplate } from '@/gen/orc/v1/workflow_pb';
import { GateType, PromptSource } from '@/gen/orc/v1/workflow_pb';
import { TagInput } from '@/components/core/TagInput';
import {
	fetchMCPServerConfig,
	parseRuntimeConfig,
	serializeRuntimeConfig,
	hydrateSelectedMCPServers,
	type HookDefinition,
} from '@/lib/runtimeConfigUtils';
import {
	GATE_TYPE_TEMPLATE_OPTIONS,
	VARIABLE_SUGGESTIONS,
} from './phase-template-modal/constants';
import { RuntimeConfigSections } from './phase-template-modal/RuntimeConfigSections';
import { useLibraryData } from '@/hooks/useLibraryData';
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
	const [agentId, setAgentId] = useState('');
	const [gateType, setGateType] = useState<GateType>(GateType.AUTO);
	const [thinkingEnabled, setThinkingEnabled] = useState(false);
	const [checkpoint, setCheckpoint] = useState(false);

	// Data flow state
	const [inputVariables, setInputVariables] = useState<string[]>([]);
	const [outputVarName, setOutputVarName] = useState('');
	const [promptSourceState, setPromptSourceState] = useState<'inline' | 'file'>('inline');
	const [promptContent, setPromptContent] = useState('');
	const [promptPath, setPromptPath] = useState('');
	const [showSwitchWarning, setShowSwitchWarning] = useState(false);

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
	const [jsonOverride, setJsonOverride] = useState('');
	const [jsonError, setJsonError] = useState('');

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
	} = useLibraryData();

	// Loading state
	const [saving, setSaving] = useState(false);

	// Track whether JSON override was the last edit source
	const jsonOverrideActiveRef = useRef(false);

	// Reset form when template changes or modal opens
	useEffect(() => {
		if (open && template) {
			setName(template.name || '');
			setDescription(template.description || '');
			setAgentId(template.agentId || '');
			setGateType(template.gateType || GateType.AUTO);
			setThinkingEnabled(template.thinkingEnabled || false);
			setCheckpoint(template.checkpoint || false);

			// Data flow fields
			setInputVariables([...template.inputVariables]);
			setOutputVarName(template.outputVarName || '');
			setPromptSourceState(template.promptSource === PromptSource.FILE ? 'file' : 'inline');
			setPromptContent(template.promptContent || '');
			setPromptPath(template.promptPath || '');
			setShowSwitchWarning(false);

			// Parse runtime_config
			const config = parseRuntimeConfig(template.runtimeConfig);
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

			// Reset JSON override state
			setJsonError('');
			jsonOverrideActiveRef.current = false;
		}
	}, [open, template]);

	useEffect(() => {
		let mounted = true;
		hydrateSelectedMCPServers(
			selectedMCPServers,
			mcpServerData,
			fetchMCPServerConfig,
		).then((hydrated) => {
			if (mounted) {
				setMcpServerData(hydrated);
			}
		}).catch(() => {});

		return () => {
			mounted = false;
		};
	}, [selectedMCPServers]);

	// Update JSON override when structured fields change (but not when JSON override is being edited)
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

	// Handle JSON override blur (apply changes)
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

	// Handle save
	const handleSave = useCallback(async () => {
		if (!template) return;

		setSaving(true);
		try {
			const hydratedMcpServerData = await hydrateSelectedMCPServers(
				selectedMCPServers,
				mcpServerData,
				fetchMCPServerConfig,
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

			const trimmedOutputVar = outputVarName.trim();

			const response = await workflowClient.updatePhaseTemplate({
				id: template.id,
				name: name.trim() || undefined,
				description: description.trim() || undefined,
				agentId: agentId.trim() || undefined,
				gateType: gateType,
				thinkingEnabled: thinkingEnabled,
				checkpoint: checkpoint,
				runtimeConfig,
				inputVariables: inputVariables,
				outputVarName: trimmedOutputVar || undefined,
				promptSource: promptSourceState === 'file' ? PromptSource.FILE : PromptSource.DB,
				promptContent: promptSourceState === 'inline' ? promptContent || undefined : undefined,
				promptPath: promptSourceState === 'file' ? promptPath || undefined : undefined,
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
		agentId,
		gateType,
		thinkingEnabled,
		checkpoint,
		inputVariables,
		outputVarName,
		promptSourceState,
		promptContent,
		promptPath,
		selectedHooks,
		selectedSkills,
		selectedMCPServers,
		allowedTools,
		disallowedTools,
		envVars,
		extraFields,
		mcpServerData,
		onUpdated,
		onClose,
	]);

	// Handle close
	const handleClose = useCallback(() => {
		onClose();
	}, [onClose]);

	const suggestedInputVariables = VARIABLE_SUGGESTIONS.filter(
		(varName) => !inputVariables.includes(varName),
	);

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

					{/* Agent ID and Gate Type */}
					<div className="form-row">
						<div className="form-group form-group-half">
							<label htmlFor="edit-template-agent" className="form-label">
								Executor Agent
							</label>
							<select
								id="edit-template-agent"
								className="form-select"
								value={agentId}
								onChange={(e) => setAgentId(e.target.value)}
								disabled={agentsLoading}
							>
								<option value="">None (no agent)</option>
								{agents.map((agent) => (
									<option key={agent.name} value={agent.name}>
										{agent.name}
										{agent.model ? ` (${agent.model})` : ''}
									</option>
								))}
							</select>
							<span className="form-help">Agent that runs this phase</span>
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
								{GATE_TYPE_TEMPLATE_OPTIONS.map((option) => (
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

				{/* Runtime Config Settings Sections */}
				<div className="edit-template-section">
					<h3 className="edit-template-section-title">Runtime Config</h3>

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
						jsonWrapperClassName="edit-template-json-override"
						jsonTextareaClassName="edit-template-json-textarea"
						jsonErrorClassName="edit-template-json-error"
					/>
				</div>

				{/* Data Flow Section */}
				<div className="edit-template-section">
					<h3 className="edit-template-section-title">Data Flow</h3>

					{/* Input Variables */}
					<div className="form-group">
						<label className="form-label">Input Variables</label>
						<TagInput
							tags={inputVariables}
							onChange={setInputVariables}
							placeholder="Add variable name..."
						/>
						<div className="edit-template-suggestions">
							{suggestedInputVariables.map((varName) => (
								<button
									key={varName}
									type="button"
									className="edit-template-suggestion-btn"
									onClick={() => {
										setInputVariables([...inputVariables, varName]);
									}}
								>
									{varName}
								</button>
							))}
						</div>
					</div>

					{/* Output Variable Name */}
					<div className="form-group">
						<label htmlFor="edit-template-output-var" className="form-label">
							Output Variable
						</label>
						<input
							id="edit-template-output-var"
							type="text"
							className="form-input"
							value={outputVarName}
							onChange={(e) => setOutputVarName(e.target.value)}
							placeholder="e.g. SPEC_CONTENT"
						/>
					</div>
				</div>

				{/* Prompt Section */}
				<div className="edit-template-section">
					<h3 className="edit-template-section-title">Prompt</h3>

					<div className="form-group">
						<div className="edit-template-toggle">
							<button
								type="button"
								className={`edit-template-toggle-btn ${promptSourceState === 'inline' ? 'edit-template-toggle-btn--active' : ''}`}
								onClick={() => {
									// Warn when switching from file to inline
									if (promptSourceState === 'file' && promptPath.trim()) {
										setShowSwitchWarning(true);
									} else {
										setPromptSourceState('inline');
									}
								}}
							>
								Inline
							</button>
							<button
								type="button"
								className={`edit-template-toggle-btn ${promptSourceState === 'file' ? 'edit-template-toggle-btn--active' : ''}`}
								onClick={() => setPromptSourceState('file')}
							>
								File
							</button>
						</div>
					</div>

					{/* File to inline switch warning */}
					{showSwitchWarning && (
						<div className="edit-template-switch-warning">
							<Icon name="alert-triangle" size={16} />
							<span>Switching to inline will clear the file reference. Continue?</span>
							<div className="edit-template-switch-warning-actions">
								<button
									type="button"
									className="edit-template-switch-warning-btn"
									onClick={() => setShowSwitchWarning(false)}
								>
									Cancel
								</button>
								<button
									type="button"
									className="edit-template-switch-warning-btn edit-template-switch-warning-btn--confirm"
									onClick={() => {
										setPromptPath('');
										setPromptSourceState('inline');
										setShowSwitchWarning(false);
									}}
								>
									Continue
								</button>
							</div>
						</div>
					)}

					{promptSourceState === 'inline' && (
						<div className="form-group">
							<div className="edit-template-prompt-editor">
								<textarea
									className="edit-template-prompt-textarea"
									value={promptContent}
									onChange={(e) => setPromptContent(e.target.value)}
									placeholder="Enter your prompt template..."
									aria-label="Prompt Content"
									rows={8}
								/>
							</div>
							<span className="form-help">
								Use {'{{VARIABLE_NAME}}'} for template variables
							</span>
						</div>
					)}

					{promptSourceState === 'file' && (
						<div className="form-group">
							<div className="edit-template-path-input">
								<span className="edit-template-path-prefix">.orc/prompts/</span>
								<input
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
