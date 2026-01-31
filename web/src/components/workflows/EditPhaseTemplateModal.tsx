/**
 * EditPhaseTemplateModal - Modal for editing phase template metadata.
 *
 * Features:
 * - Edit phase template name, description
 * - Edit execution settings (agent, gate type, max iterations)
 * - Edit checkpoint and thinking settings
 * - 7 collapsible settings sections for claude_config (hooks, MCP servers, skills,
 *   allowed/disallowed tools, env vars, JSON override)
 * - Built-in templates cannot be edited (shows clone suggestion)
 *
 * Note: Model is now on the Agent, not the PhaseTemplate.
 * Agent assignment is done via agentId.
 */

import { useState, useCallback, useEffect, useRef } from 'react';
import { Modal } from '@/components/overlays/Modal';
import { Button, Icon } from '@/components/ui';
import { workflowClient, configClient, mcpClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';
import type { PhaseTemplate } from '@/gen/orc/v1/workflow_pb';
import type { Agent, Hook, Skill } from '@/gen/orc/v1/config_pb';
import type { MCPServerInfo } from '@/gen/orc/v1/mcp_pb';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import { CollapsibleSettingsSection } from '@/components/core/CollapsibleSettingsSection';
import { LibraryPicker } from '@/components/core/LibraryPicker';
import { TagInput } from '@/components/core/TagInput';
import { KeyValueEditor } from '@/components/core/KeyValueEditor';
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

const GATE_TYPE_OPTIONS = [
	{ value: GateType.AUTO, label: 'Auto' },
	{ value: GateType.HUMAN, label: 'Human' },
	{ value: GateType.SKIP, label: 'Skip' },
];

/** Parse claude_config JSON string into structured state */
function parseClaudeConfig(configStr: string | undefined): {
	hooks: string[];
	skillRefs: string[];
	mcpServers: string[];
	allowedTools: string[];
	disallowedTools: string[];
	env: Record<string, string>;
	extra: Record<string, unknown>;
} {
	const defaults = {
		hooks: [] as string[],
		skillRefs: [] as string[],
		mcpServers: [] as string[],
		allowedTools: [] as string[],
		disallowedTools: [] as string[],
		env: {} as Record<string, string>,
		extra: {} as Record<string, unknown>,
	};

	if (!configStr) return defaults;

	try {
		const parsed = JSON.parse(configStr);
		if (typeof parsed !== 'object' || parsed === null) return defaults;

		const {
			hooks,
			skill_refs,
			mcp_servers,
			allowed_tools,
			disallowed_tools,
			env,
			...rest
		} = parsed;

		return {
			hooks: Array.isArray(hooks) ? hooks : [],
			skillRefs: Array.isArray(skill_refs) ? skill_refs : [],
			mcpServers: mcp_servers && typeof mcp_servers === 'object'
				? Object.keys(mcp_servers)
				: [],
			allowedTools: Array.isArray(allowed_tools) ? allowed_tools : [],
			disallowedTools: Array.isArray(disallowed_tools) ? disallowed_tools : [],
			env: env && typeof env === 'object' ? env : {},
			extra: rest,
		};
	} catch {
		console.warn('Failed to parse claude_config JSON:', configStr);
		return defaults;
	}
}

/** Serialize structured state back to claude_config JSON */
function serializeClaudeConfig(state: {
	hooks: string[];
	skillRefs: string[];
	mcpServers: string[];
	allowedTools: string[];
	disallowedTools: string[];
	env: Record<string, string>;
	extra: Record<string, unknown>;
	mcpServerData: Record<string, unknown>;
}): string {
	const config: Record<string, unknown> = { ...state.extra };

	if (state.hooks.length > 0) config.hooks = state.hooks;
	if (state.skillRefs.length > 0) config.skill_refs = state.skillRefs;
	if (Object.keys(state.mcpServers).length > 0) {
		const servers: Record<string, unknown> = {};
		for (const name of state.mcpServers) {
			servers[name] = state.mcpServerData[name] || {};
		}
		config.mcp_servers = servers;
	}
	if (state.allowedTools.length > 0) config.allowed_tools = state.allowedTools;
	if (state.disallowedTools.length > 0) config.disallowed_tools = state.disallowedTools;
	if (Object.keys(state.env).length > 0) config.env = state.env;

	return JSON.stringify(config, null, 2);
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
	const [maxIterations, setMaxIterations] = useState(50);
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

	// JSON override state
	const [jsonOverride, setJsonOverride] = useState('');
	const [jsonError, setJsonError] = useState('');

	// Library data
	const [hooks, setHooks] = useState<Hook[]>([]);
	const [skills, setSkills] = useState<Skill[]>([]);
	const [mcpServers, setMcpServers] = useState<MCPServerInfo[]>([]);
	const [hooksError, setHooksError] = useState('');
	const [skillsError, setSkillsError] = useState('');
	const [mcpError, setMcpError] = useState('');
	const [hooksLoading, setHooksLoading] = useState(true);
	const [skillsLoading, setSkillsLoading] = useState(true);
	const [mcpLoading, setMcpLoading] = useState(true);

	// Agents list for dropdown
	const [agents, setAgents] = useState<Agent[]>([]);
	const [agentsLoading, setAgentsLoading] = useState(true);

	// Loading state
	const [saving, setSaving] = useState(false);

	// Track whether JSON override was the last edit source
	const jsonOverrideActiveRef = useRef(false);

	// Fetch agents, hooks, skills, MCP servers on mount
	useEffect(() => {
		let mounted = true;

		configClient.listAgents({}).then((response) => {
			if (mounted) {
				setAgents(response.agents);
				setAgentsLoading(false);
			}
		}).catch(() => {
			if (mounted) setAgentsLoading(false);
		});

		configClient.listHooks({}).then((response) => {
			if (mounted) {
				setHooks(response.hooks);
				setHooksLoading(false);
			}
		}).catch(() => {
			if (mounted) {
				setHooksError('Failed to load hooks');
				setHooksLoading(false);
			}
		});

		configClient.listSkills({}).then((response) => {
			if (mounted) {
				setSkills(response.skills);
				setSkillsLoading(false);
			}
		}).catch(() => {
			if (mounted) {
				setSkillsError('Failed to load skills');
				setSkillsLoading(false);
			}
		});

		mcpClient.listMCPServers({}).then((response) => {
			if (mounted) {
				setMcpServers(response.servers);
				setMcpLoading(false);
			}
		}).catch(() => {
			if (mounted) {
				setMcpError('Failed to load MCP servers');
				setMcpLoading(false);
			}
		});

		return () => { mounted = false; };
	}, []);

	// Reset form when template changes or modal opens
	useEffect(() => {
		if (open && template) {
			setName(template.name || '');
			setDescription(template.description || '');
			setAgentId(template.agentId || '');
			setMaxIterations(template.maxIterations || 50);
			setGateType(template.gateType || GateType.AUTO);
			setThinkingEnabled(template.thinkingEnabled || false);
			setCheckpoint(template.checkpoint || false);

			// Parse claude_config
			const config = parseClaudeConfig(template.claudeConfig);
			setSelectedHooks(config.hooks);
			setSelectedSkills(config.skillRefs);
			setSelectedMCPServers(config.mcpServers);
			setAllowedTools(config.allowedTools);
			setDisallowedTools(config.disallowedTools);
			setEnvVars(config.env);
			setExtraFields(config.extra);

			// Parse MCP server data for serialization
			if (template.claudeConfig) {
				try {
					const parsed = JSON.parse(template.claudeConfig);
					if (parsed.mcp_servers && typeof parsed.mcp_servers === 'object') {
						setMcpServerData(parsed.mcp_servers);
					} else {
						setMcpServerData({});
					}
				} catch {
					setMcpServerData({});
				}
			} else {
				setMcpServerData({});
			}

			// Reset JSON override state
			setJsonError('');
			jsonOverrideActiveRef.current = false;
		}
	}, [open, template]);

	// Update JSON override when structured fields change (but not when JSON override is being edited)
	useEffect(() => {
		if (!jsonOverrideActiveRef.current) {
			const json = serializeClaudeConfig({
				hooks: selectedHooks,
				skillRefs: selectedSkills,
				mcpServers: selectedMCPServers,
				allowedTools,
				disallowedTools,
				env: envVars,
				extra: extraFields,
				mcpServerData,
			});
			setJsonOverride(json);
		}
	}, [selectedHooks, selectedSkills, selectedMCPServers, allowedTools, disallowedTools, envVars, extraFields, mcpServerData]);

	// Handle JSON override blur (apply changes)
	const handleJsonBlur = useCallback(() => {
		try {
			const parsed = JSON.parse(jsonOverride);
			if (typeof parsed !== 'object' || parsed === null) {
				setJsonError('Invalid JSON');
				return;
			}

			// Re-parse into structured fields
			const config = parseClaudeConfig(jsonOverride);
			setSelectedHooks(config.hooks);
			setSelectedSkills(config.skillRefs);
			setSelectedMCPServers(config.mcpServers);
			setAllowedTools(config.allowedTools);
			setDisallowedTools(config.disallowedTools);
			setEnvVars(config.env);
			setExtraFields(config.extra);

			// Update MCP server data
			if (parsed.mcp_servers && typeof parsed.mcp_servers === 'object') {
				setMcpServerData(parsed.mcp_servers);
			}

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
			const claudeConfig = serializeClaudeConfig({
				hooks: selectedHooks,
				skillRefs: selectedSkills,
				mcpServers: selectedMCPServers,
				allowedTools,
				disallowedTools,
				env: envVars,
				extra: extraFields,
				mcpServerData,
			});

			const response = await workflowClient.updatePhaseTemplate({
				id: template.id,
				name: name.trim() || undefined,
				description: description.trim() || undefined,
				agentId: agentId.trim() || undefined,
				maxIterations: maxIterations,
				gateType: gateType,
				thinkingEnabled: thinkingEnabled,
				checkpoint: checkpoint,
				claudeConfig: claudeConfig,
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
		maxIterations,
		gateType,
		thinkingEnabled,
		checkpoint,
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

				{/* Claude Config Settings Sections */}
				<div className="edit-template-section">
					<h3 className="edit-template-section-title">Claude Config</h3>

					<CollapsibleSettingsSection title="Hooks" badgeCount={selectedHooks.length}>
						<LibraryPicker
							type="hooks"
							items={hooks}
							selectedNames={selectedHooks}
							onSelectionChange={(names) => {
								setSelectedHooks(names);
								jsonOverrideActiveRef.current = false;
							}}
							error={hooksError}
							loading={hooksLoading}
						/>
					</CollapsibleSettingsSection>

					<CollapsibleSettingsSection title="MCP Servers" badgeCount={selectedMCPServers.length}>
						<LibraryPicker
							type="mcpServers"
							items={mcpServers}
							selectedNames={selectedMCPServers}
							onSelectionChange={(names) => {
								setSelectedMCPServers(names);
								jsonOverrideActiveRef.current = false;
							}}
							error={mcpError}
							loading={mcpLoading}
						/>
					</CollapsibleSettingsSection>

					<CollapsibleSettingsSection title="Skills" badgeCount={selectedSkills.length}>
						<LibraryPicker
							type="skills"
							items={skills}
							selectedNames={selectedSkills}
							onSelectionChange={(names) => {
								setSelectedSkills(names);
								jsonOverrideActiveRef.current = false;
							}}
							error={skillsError}
							loading={skillsLoading}
						/>
					</CollapsibleSettingsSection>

					<CollapsibleSettingsSection title="Allowed Tools" badgeCount={allowedTools.length}>
						<TagInput
							tags={allowedTools}
							onChange={(tags) => {
								setAllowedTools(tags);
								jsonOverrideActiveRef.current = false;
							}}
							placeholder="Add tool name..."
						/>
					</CollapsibleSettingsSection>

					<CollapsibleSettingsSection title="Disallowed Tools" badgeCount={disallowedTools.length}>
						<TagInput
							tags={disallowedTools}
							onChange={(tags) => {
								setDisallowedTools(tags);
								jsonOverrideActiveRef.current = false;
							}}
							placeholder="Add tool name..."
						/>
					</CollapsibleSettingsSection>

					<CollapsibleSettingsSection title="Env Vars" badgeCount={Object.keys(envVars).length}>
						<KeyValueEditor
							entries={envVars}
							onChange={(entries) => {
								setEnvVars(entries);
								jsonOverrideActiveRef.current = false;
							}}
						/>
					</CollapsibleSettingsSection>

					<CollapsibleSettingsSection title="JSON Override" badgeCount={0}>
						<div className="edit-template-json-override">
							<textarea
								className={`edit-template-json-textarea ${jsonError ? 'edit-template-json-textarea--error' : ''}`}
								value={jsonOverride}
								onChange={(e) => {
									setJsonOverride(e.target.value);
									jsonOverrideActiveRef.current = true;
									setJsonError('');
								}}
								onBlur={handleJsonBlur}
								rows={8}
								aria-label="JSON override"
							/>
							{jsonError && (
								<span className="edit-template-json-error">{jsonError}</span>
							)}
						</div>
					</CollapsibleSettingsSection>
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
