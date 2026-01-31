import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import * as Collapsible from '@radix-ui/react-collapsible';
import { ChevronDown, ChevronRight } from 'lucide-react';
import { workflowClient, configClient, mcpClient } from '@/lib/client';
import {
	GateType,
	VariableSourceType,
} from '@/gen/orc/v1/workflow_pb';
import type {
	WorkflowPhase,
	WorkflowWithDetails,
	WorkflowVariable,
} from '@/gen/orc/v1/workflow_pb';
import type { Agent, Hook, Skill } from '@/gen/orc/v1/config_pb';
import type { MCPServerInfo } from '@/gen/orc/v1/mcp_pb';
import { mergeClaudeConfigs, parseClaudeConfig, serializeClaudeConfig } from '@/lib/claudeConfigUtils';
import type { ClaudeConfigState } from '@/lib/claudeConfigUtils';
import { CollapsibleSettingsSection } from '@/components/core/CollapsibleSettingsSection';
import { LibraryPicker } from '@/components/core/LibraryPicker';
import { TagInput } from '@/components/core/TagInput';
import { KeyValueEditor } from '@/components/core/KeyValueEditor';
import { PromptEditor } from './PromptEditor';
import { VariableModal } from '../VariableModal';
import './PhaseInspector.css';

type InspectorTab = 'input' | 'prompt' | 'criteria' | 'settings';

interface PhaseInspectorProps {
	phase: WorkflowPhase | null;
	workflowDetails: WorkflowWithDetails | null;
	readOnly: boolean;
	onWorkflowRefresh?: () => void;
	onDeletePhase?: () => void;
}

function formatSourceType(st: VariableSourceType): string {
	switch (st) {
		case VariableSourceType.STATIC:
			return 'static';
		case VariableSourceType.ENV:
			return 'env';
		case VariableSourceType.SCRIPT:
			return 'script';
		case VariableSourceType.API:
			return 'api';
		case VariableSourceType.PHASE_OUTPUT:
			return 'phase_output';
		case VariableSourceType.PROMPT_FRAGMENT:
			return 'prompt_fragment';
		default:
			return 'unknown';
	}
}

export function PhaseInspector({
	phase,
	workflowDetails,
	readOnly,
	onWorkflowRefresh,
	onDeletePhase,
}: PhaseInspectorProps) {
	const [activeTab, setActiveTab] = useState<InspectorTab>('prompt');
	const [settingsError, setSettingsError] = useState<string | null>(null);
	const [varsOpen, setVarsOpen] = useState(true);
	const prevPhaseIdRef = useRef<number | null>(null);

	// Reset to Prompt tab when selected phase changes
	useEffect(() => {
		if (phase && phase.id !== prevPhaseIdRef.current) {
			setActiveTab('prompt');
			setSettingsError(null);
		}
		prevPhaseIdRef.current = phase?.id ?? null;
	}, [phase]);

	if (!phase) {
		return null;
	}

	if (!workflowDetails) {
		return (
			<div className="phase-inspector phase-inspector--loading">
				<span>Loading...</span>
			</div>
		);
	}

	const template = phase.template;

	// If no template, show error state
	if (!template) {
		return (
			<div className="phase-inspector">
				<div className="phase-inspector__header">
					<h3 className="phase-inspector__title">{phase.phaseTemplateId}</h3>
					<span className="phase-inspector__subtitle">Template not found</span>
				</div>
			</div>
		);
	}

	const isBuiltin = template.isBuiltin ?? false;
	const workflowIsBuiltin = workflowDetails.workflow?.isBuiltin ?? false;
	const workflowVariables = workflowDetails.variables ?? [];

	return (
		<div className="phase-inspector">
			{/* Header */}
			<div className="phase-inspector__header">
				<div className="phase-inspector__header-row">
					<h3 className="phase-inspector__title">
						{template.name ?? phase.phaseTemplateId} Phase
					</h3>
					{isBuiltin && (
						<span className="phase-inspector__badge phase-inspector__badge--builtin">
							Built-in
						</span>
					)}
				</div>
				<span className="phase-inspector__subtitle">{phase.phaseTemplateId}</span>
			</div>

			{/* Tabs */}
			<Tabs.Root
				value={activeTab}
				onValueChange={(v) => setActiveTab(v as InspectorTab)}
				className="phase-inspector__tabs"
			>
				<Tabs.List className="phase-inspector__tab-list" aria-label="Phase inspector tabs">
					<Tabs.Trigger value="input" className="phase-inspector__tab">
						Phase Input
					</Tabs.Trigger>
					<Tabs.Trigger value="prompt" className="phase-inspector__tab">
						Prompt
					</Tabs.Trigger>
					<Tabs.Trigger value="criteria" className="phase-inspector__tab">
						Completion
					</Tabs.Trigger>
					<Tabs.Trigger value="settings" className="phase-inspector__tab">
						Settings
					</Tabs.Trigger>
				</Tabs.List>

				<Tabs.Content value="input" className="phase-inspector__content">
					<PhaseInputTab
						phase={phase}
						workflowDetails={workflowDetails}
						readOnly={readOnly}
						workflowIsBuiltin={workflowIsBuiltin}
						onWorkflowRefresh={onWorkflowRefresh}
					/>
				</Tabs.Content>

				<Tabs.Content value="prompt" className="phase-inspector__content">
					<PromptEditor
						phaseTemplateId={template.id}
						promptSource={template.promptSource}
						promptContent={template.promptContent}
						readOnly={isBuiltin}
					/>
				</Tabs.Content>

				<Tabs.Content value="criteria" className="phase-inspector__content">
					<CompletionCriteriaTab phase={phase} />
				</Tabs.Content>

				<Tabs.Content value="settings" className="phase-inspector__content">
					<SettingsTab
						phase={phase}
						workflowDetails={workflowDetails}
						readOnly={readOnly}
						error={settingsError}
						onError={setSettingsError}
						onWorkflowRefresh={onWorkflowRefresh}
						onDeletePhase={onDeletePhase}
					/>
				</Tabs.Content>
			</Tabs.Root>

			{/* Available Variables - Collapsible Section */}
			<Collapsible.Root
				open={varsOpen}
				onOpenChange={setVarsOpen}
				className="phase-inspector__variables"
			>
				<Collapsible.Trigger className="phase-inspector__variables-trigger">
					{varsOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
					<span>Available Variables</span>
					<span className="phase-inspector__variables-count">{workflowVariables.length}</span>
				</Collapsible.Trigger>
				<Collapsible.Content className="phase-inspector__variables-content">
					<AvailableVariablesList
						variables={workflowVariables}
						workflowDetails={workflowDetails}
						workflowIsBuiltin={workflowIsBuiltin}
						onWorkflowRefresh={onWorkflowRefresh}
					/>
				</Collapsible.Content>
			</Collapsible.Root>
		</div>
	);
}

// ─── Phase Input Tab ──────────────────────────────────────────────────────────

interface PhaseInputTabProps {
	phase: WorkflowPhase;
	workflowDetails: WorkflowWithDetails;
	readOnly: boolean;
	workflowIsBuiltin: boolean;
	onWorkflowRefresh?: () => void;
}

function PhaseInputTab({ phase, workflowDetails }: PhaseInputTabProps) {
	const template = phase.template;
	const inputVariables = template?.inputVariables ?? [];
	const workflowVariables = workflowDetails.variables;
	const workflowVariableNames = new Set(workflowVariables.map((v) => v.name));

	return (
		<div className="phase-inspector__input">
			<p className="phase-inspector__input-desc">
				Variables this phase requires to execute:
			</p>
			{inputVariables.length === 0 ? (
				<div className="phase-inspector__empty">No input variables required</div>
			) : (
				<ul className="phase-inspector__input-list">
					{inputVariables.map((varName) => {
						const satisfied = workflowVariableNames.has(varName);
						const varDef = workflowVariables.find((v) => v.name === varName);
						return (
							<li key={varName} className="phase-inspector__input-item">
								<div className="phase-inspector__input-item-header">
									<code className="phase-inspector__input-name">{`{{${varName}}}`}</code>
									<span
										className={`phase-inspector__input-status phase-inspector__input-status--${satisfied ? 'satisfied' : 'missing'}`}
									>
										{satisfied ? '✓ Provided' : '⚠ Missing'}
									</span>
								</div>
								{varDef?.description && (
									<p className="phase-inspector__input-hint">{varDef.description}</p>
								)}
							</li>
						);
					})}
				</ul>
			)}
		</div>
	);
}

// ─── Completion Criteria Tab ─────────────────────────────────────────────────

interface CompletionCriteriaTabProps {
	phase: WorkflowPhase;
}

function CompletionCriteriaTab({ phase }: CompletionCriteriaTabProps) {
	const template = phase.template;
	const gateType = phase.gateTypeOverride || template?.gateType || GateType.AUTO;
	const maxIterations = phase.maxIterationsOverride ?? template?.maxIterations ?? 3;

	const getGateLabel = (gt: GateType): string => {
		switch (gt) {
			case GateType.AUTO: return 'Automatic';
			case GateType.HUMAN: return 'Human Approval';
			case GateType.AI: return 'AI Gate';
			case GateType.SKIP: return 'Skip';
			default: return 'Automatic';
		}
	};

	return (
		<div className="phase-inspector__criteria">
			<div className="phase-inspector__criteria-section">
				<h4 className="phase-inspector__criteria-label">Gate Type</h4>
				<p className="phase-inspector__criteria-value">{getGateLabel(gateType)}</p>
				<p className="phase-inspector__criteria-hint">
					{gateType === GateType.AUTO && 'Proceeds automatically when complete'}
					{gateType === GateType.HUMAN && 'Requires human approval to proceed'}
					{gateType === GateType.AI && 'AI agent evaluates the gate'}
					{gateType === GateType.SKIP && 'Phase is skipped entirely'}
				</p>
			</div>

			<div className="phase-inspector__criteria-section">
				<h4 className="phase-inspector__criteria-label">Max Iterations</h4>
				<p className="phase-inspector__criteria-value">{maxIterations}</p>
				<p className="phase-inspector__criteria-hint">
					Maximum attempts before phase fails
				</p>
			</div>

			<div className="phase-inspector__criteria-section">
				<h4 className="phase-inspector__criteria-label">Output Format</h4>
				<p className="phase-inspector__criteria-hint">
					Phase completes when Claude outputs JSON with{' '}
					<code>{`{"status": "complete", ...}`}</code>
				</p>
			</div>
		</div>
	);
}

// ─── Available Variables List ────────────────────────────────────────────────

interface AvailableVariablesListProps {
	variables: WorkflowVariable[];
	workflowDetails: WorkflowWithDetails;
	workflowIsBuiltin: boolean;
	onWorkflowRefresh?: () => void;
}

function AvailableVariablesList({
	variables,
	workflowDetails,
	workflowIsBuiltin,
	onWorkflowRefresh,
}: AvailableVariablesListProps) {
	const [modalOpen, setModalOpen] = useState(false);
	const [editingVariable, setEditingVariable] = useState<WorkflowVariable | undefined>(undefined);

	const availablePhases = workflowDetails.phases?.map((p) => p.phaseTemplateId) ?? [];

	const handleAddVariable = useCallback(() => {
		setEditingVariable(undefined);
		setModalOpen(true);
	}, []);

	const handleEditVariable = useCallback((wv: WorkflowVariable) => {
		setEditingVariable(wv);
		setModalOpen(true);
	}, []);

	const handleModalSuccess = useCallback(() => {
		onWorkflowRefresh?.();
	}, [onWorkflowRefresh]);

	if (variables.length === 0) {
		return (
			<div className="phase-inspector__variables-empty">
				<p>No variables defined</p>
				{!workflowIsBuiltin && (
					<button className="phase-inspector__add-btn" onClick={handleAddVariable}>
						+ Add Variable
					</button>
				)}
				<VariableModal
					open={modalOpen}
					onOpenChange={setModalOpen}
					workflowId={workflowDetails.workflow?.id ?? ''}
					variable={editingVariable}
					availablePhases={availablePhases}
					onSuccess={handleModalSuccess}
				/>
			</div>
		);
	}

	return (
		<div className="phase-inspector__variables-list">
			{variables.map((wv) => (
				<button
					key={wv.id}
					className="phase-inspector__var-item"
					onClick={!workflowIsBuiltin ? () => handleEditVariable(wv) : undefined}
					disabled={workflowIsBuiltin}
				>
					<code className="phase-inspector__var-name">{wv.name}</code>
					<span className="phase-inspector__var-type">{formatSourceType(wv.sourceType)}</span>
				</button>
			))}
			{!workflowIsBuiltin && (
				<button className="phase-inspector__add-btn" onClick={handleAddVariable}>
					+ Add Variable
				</button>
			)}
			<VariableModal
				open={modalOpen}
				onOpenChange={setModalOpen}
				workflowId={workflowDetails.workflow?.id ?? ''}
				variable={editingVariable}
				availablePhases={availablePhases}
				onSuccess={handleModalSuccess}
			/>
		</div>
	);
}

// ─── Settings Tab ───────────────────────────────────────────────────────────

interface SettingsTabProps {
	phase: WorkflowPhase;
	workflowDetails: WorkflowWithDetails;
	readOnly: boolean;
	error: string | null;
	onError: (err: string | null) => void;
	onWorkflowRefresh?: () => void;
	onDeletePhase?: () => void;
}

function SettingsTab({
	phase,
	workflowDetails,
	readOnly,
	error,
	onError,
	onWorkflowRefresh,
	onDeletePhase,
}: SettingsTabProps) {
	const [maxIterations, setMaxIterations] = useState<number>(
		phase.maxIterationsOverride ?? phase.template?.maxIterations ?? 3,
	);
	const [modelOverride, setModelOverride] = useState<string>(
		phase.modelOverride ?? '',
	);
	const [thinkingOverride, setThinkingOverride] = useState<boolean>(
		phase.thinkingOverride ?? false,
	);
	const [gateTypeOverride, setGateTypeOverride] = useState<GateType>(
		phase.gateTypeOverride ?? GateType.UNSPECIFIED,
	);

	// Agent state
	const [agents, setAgents] = useState<Agent[]>([]);
	const [agentsLoading, setAgentsLoading] = useState(true);
	const [agentOverride, setAgentOverride] = useState<string>(
		phase.agentOverride ?? '',
	);
	const [subAgentsOverride, setSubAgentsOverride] = useState<string[]>(
		phase.subAgentsOverride ?? [],
	);

	// Fetch agents list on mount
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
		return () => { mounted = false; };
	}, []);

	// Reset state when phase changes
	useEffect(() => {
		setMaxIterations(phase.maxIterationsOverride ?? phase.template?.maxIterations ?? 3);
		setModelOverride(phase.modelOverride ?? '');
		setThinkingOverride(phase.thinkingOverride ?? false);
		setGateTypeOverride(phase.gateTypeOverride ?? GateType.UNSPECIFIED);
		setAgentOverride(phase.agentOverride ?? '');
		setSubAgentsOverride(phase.subAgentsOverride ?? []);
		onError(null);
	}, [phase, onError]);

	const updatePhase = useCallback(
		async (updates: Record<string, unknown>) => {
			const workflowId = workflowDetails.workflow?.id;
			if (!workflowId) return;
			onError(null);
			try {
				await workflowClient.updatePhase({
					workflowId,
					phaseId: phase.id,
					...updates,
				});
				// Refresh workflow data
				if (onWorkflowRefresh) {
					onWorkflowRefresh();
				} else {
					await workflowClient.getWorkflow({ id: workflowId });
				}
			} catch (err) {
				const message = err instanceof Error ? err.message : 'Update failed';
				onError(message);
				// Revert to previous values
				setMaxIterations(phase.maxIterationsOverride ?? phase.template?.maxIterations ?? 3);
				setModelOverride(phase.modelOverride ?? '');
				setThinkingOverride(phase.thinkingOverride ?? false);
				setGateTypeOverride(phase.gateTypeOverride ?? GateType.UNSPECIFIED);
				setAgentOverride(phase.agentOverride ?? '');
				setSubAgentsOverride(phase.subAgentsOverride ?? []);
			}
		},
		[workflowDetails, phase, onError, onWorkflowRefresh],
	);

	const handleMaxIterationsBlur = () => {
		const currentValue = phase.maxIterationsOverride ?? phase.template?.maxIterations ?? 3;
		if (maxIterations !== currentValue) {
			updatePhase({ maxIterationsOverride: maxIterations });
		}
	};

	const handleModelChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
		const value = e.target.value;
		setModelOverride(value);
		updatePhase({ modelOverride: value || undefined });
	};

	const handleThinkingChange = (e: React.ChangeEvent<HTMLInputElement>) => {
		const value = e.target.checked;
		setThinkingOverride(value);
		updatePhase({ thinkingOverride: value });
	};

	const handleGateTypeChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
		const value = Number(e.target.value) as GateType;
		setGateTypeOverride(value);
		updatePhase({ gateTypeOverride: value || undefined });
	};

	const handleAgentChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
		const value = e.target.value;
		setAgentOverride(value);
		updatePhase({ agentOverride: value || undefined });
	};

	const handleSubAgentToggle = (agentName: string, checked: boolean) => {
		const newValue = checked
			? [...subAgentsOverride, agentName]
			: subAgentsOverride.filter((a) => a !== agentName);
		setSubAgentsOverride(newValue);
		updatePhase({ subAgentsOverride: newValue.length > 0 ? newValue : undefined });
	};

	const disabled = readOnly;

	return (
		<div className="phase-inspector-settings">
			{readOnly && (
				<div className="phase-inspector-readonly-notice">
					Clone to customize
				</div>
			)}

			{error && (
				<div className="phase-inspector-settings-error">{error}</div>
			)}

			<div className="phase-inspector-setting">
				<label htmlFor="inspector-model" className="phase-inspector-setting-label">
					Model
				</label>
				<select
					id="inspector-model"
					className="phase-inspector-setting-select"
					value={modelOverride}
					onChange={handleModelChange}
					disabled={disabled}
				>
					<option value="">Inherit from workflow</option>
					<option value="claude-sonnet-4-20250514">Sonnet</option>
					<option value="claude-opus-4">Opus</option>
					<option value="claude-haiku-35-20241022">Haiku</option>
				</select>
			</div>

			<div className="phase-inspector-setting">
				<label htmlFor="inspector-thinking" className="phase-inspector-setting-label">
					Thinking
				</label>
				<input
					id="inspector-thinking"
					type="checkbox"
					className="phase-inspector-setting-checkbox"
					checked={thinkingOverride}
					onChange={handleThinkingChange}
					disabled={disabled}
				/>
			</div>

			<div className="phase-inspector-setting">
				<label htmlFor="inspector-gate-type" className="phase-inspector-setting-label">
					Gate Type
				</label>
				<select
					id="inspector-gate-type"
					className="phase-inspector-setting-select"
					value={gateTypeOverride}
					onChange={handleGateTypeChange}
					disabled={disabled}
				>
					<option value={GateType.UNSPECIFIED}>Inherit from template</option>
					<option value={GateType.AUTO}>Auto</option>
					<option value={GateType.HUMAN}>Human</option>
					<option value={GateType.AI}>AI</option>
					<option value={GateType.SKIP}>Skip</option>
				</select>
			</div>

			{/* AI Gate Agent picker - only shown when AI gate type is selected */}
			{gateTypeOverride === GateType.AI && (
				<div className="phase-inspector-setting">
					<label htmlFor="inspector-ai-gate-agent" className="phase-inspector-setting-label">
						AI Gate Agent
					</label>
					{agents.length === 0 && !agentsLoading ? (
						<select
							id="inspector-ai-gate-agent"
							className="phase-inspector-setting-select"
							disabled
						>
							<option>No agents available</option>
						</select>
					) : (
						<select
							id="inspector-ai-gate-agent"
							className="phase-inspector-setting-select"
							disabled={disabled || agentsLoading}
						>
							<option value="">Select agent...</option>
							{agents.map((agent) => (
								<option key={agent.id} value={agent.id}>
									{agent.name}
								</option>
							))}
						</select>
					)}
				</div>
			)}

			<div className="phase-inspector-setting">
				<label htmlFor="inspector-max-iterations" className="phase-inspector-setting-label">
					Max Iterations
				</label>
				<input
					id="inspector-max-iterations"
					type="number"
					className="phase-inspector-setting-input"
					value={maxIterations}
					onChange={(e) => setMaxIterations(Number(e.target.value))}
					onBlur={handleMaxIterationsBlur}
					min={1}
					max={20}
					disabled={disabled}
				/>
			</div>

			{/* Executor */}
			<div className="phase-inspector-setting">
				<label htmlFor="inspector-agent" className="phase-inspector-setting-label">
					Executor
				</label>
				<select
					id="inspector-agent"
					className="phase-inspector-setting-select"
					value={agentOverride}
					onChange={handleAgentChange}
					disabled={disabled || agentsLoading}
				>
					<option value="">
						{phase.template?.agentId
							? `Inherit (${phase.template.agentId})`
							: 'Inherit from template'}
					</option>
					{agents.map((agent) => (
						<option key={agent.name} value={agent.name}>
							{agent.name}
							{agent.model ? ` (${agent.model})` : ''}
						</option>
					))}
				</select>
				<span className="phase-inspector-setting-hint">
					Agent that executes this phase
				</span>
			</div>

			{/* Sub-Agents */}
			<div className="phase-inspector-setting">
				<label className="phase-inspector-setting-label">
					Sub-Agents
				</label>
				<div className="phase-inspector-sub-agents">
					{agentsLoading ? (
						<span className="phase-inspector-loading">Loading agents...</span>
					) : agents.length === 0 ? (
						<span className="phase-inspector-empty">No agents available</span>
					) : (
						agents.map((agent) => (
							<label key={agent.name} className="phase-inspector-checkbox-label">
								<input
									type="checkbox"
									checked={subAgentsOverride.includes(agent.name)}
									onChange={(e) => handleSubAgentToggle(agent.name, e.target.checked)}
									disabled={disabled}
								/>
								<span>{agent.name}</span>
							</label>
						))
					)}
				</div>
				<span className="phase-inspector-setting-hint">
					Agents available for delegation during execution
				</span>
			</div>

			{/* Claude Config Override (editable) */}
			<ClaudeConfigEditor
				phase={phase}
				disabled={readOnly}
				onSave={(json) => updatePhase({ claudeConfigOverride: json || undefined })}
			/>

			{/* Danger Zone - Remove Phase */}
			{!readOnly && onDeletePhase && (
				<div className="phase-inspector-danger-zone">
					<button
						type="button"
						className="phase-inspector-delete-btn"
						onClick={onDeletePhase}
					>
						Remove Phase
					</button>
				</div>
			)}
		</div>
	);
}

// ─── Claude Config Editor (editable in Settings tab) ────────────────────────

interface ClaudeConfigEditorProps {
	phase: WorkflowPhase;
	disabled: boolean;
	onSave: (json: string) => void;
}

function ClaudeConfigEditor({ phase, disabled, onSave }: ClaudeConfigEditorProps) {
	// Structured override fields
	const [selectedHooks, setSelectedHooks] = useState<string[]>([]);
	const [selectedSkills, setSelectedSkills] = useState<string[]>([]);
	const [selectedMCPServers, setSelectedMCPServers] = useState<string[]>([]);
	const [allowedTools, setAllowedTools] = useState<string[]>([]);
	const [disallowedTools, setDisallowedTools] = useState<string[]>([]);
	const [envVars, setEnvVars] = useState<Record<string, string>>({});
	const [extraFields, setExtraFields] = useState<Record<string, unknown>>({});

	// JSON override textarea
	const [jsonText, setJsonText] = useState('');
	const [jsonError, setJsonError] = useState('');
	const jsonActiveRef = useRef(false);

	// Library data
	const [hooks, setHooks] = useState<Hook[]>([]);
	const [skills, setSkills] = useState<Skill[]>([]);
	const [mcpServers, setMcpServers] = useState<MCPServerInfo[]>([]);
	const [hooksLoading, setHooksLoading] = useState(true);
	const [skillsLoading, setSkillsLoading] = useState(true);
	const [mcpLoading, setMcpLoading] = useState(true);
	const [hooksError, setHooksError] = useState('');
	const [skillsError, setSkillsError] = useState('');
	const [mcpError, setMcpError] = useState('');

	// Fetch library data on mount
	useEffect(() => {
		let mounted = true;
		configClient.listHooks({}).then((r) => {
			if (mounted) { setHooks(r.hooks); setHooksLoading(false); }
		}).catch(() => {
			if (mounted) { setHooksError('Failed to load hooks'); setHooksLoading(false); }
		});
		configClient.listSkills({}).then((r) => {
			if (mounted) { setSkills(r.skills); setSkillsLoading(false); }
		}).catch(() => {
			if (mounted) { setSkillsError('Failed to load skills'); setSkillsLoading(false); }
		});
		mcpClient.listMCPServers({}).then((r) => {
			if (mounted) { setMcpServers(r.servers); setMcpLoading(false); }
		}).catch(() => {
			if (mounted) { setMcpError('Failed to load MCP servers'); setMcpLoading(false); }
		});
		return () => { mounted = false; };
	}, []);

	// Parse override when phase changes
	useEffect(() => {
		const config = parseClaudeConfig(phase.claudeConfigOverride);
		setSelectedHooks(config.hooks);
		setSelectedSkills(config.skillRefs);
		setSelectedMCPServers(config.mcpServers);
		setAllowedTools(config.allowedTools);
		setDisallowedTools(config.disallowedTools);
		setEnvVars(config.env);
		setExtraFields(config.extra);
		jsonActiveRef.current = false;
	}, [phase.id, phase.claudeConfigOverride]);

	// Sync structured fields -> JSON text (when not editing JSON directly)
	useEffect(() => {
		if (!jsonActiveRef.current) {
			setJsonText(serializeClaudeConfig({
				hooks: selectedHooks,
				skillRefs: selectedSkills,
				mcpServers: selectedMCPServers,
				allowedTools,
				disallowedTools,
				env: envVars,
				extra: extraFields,
			}));
		}
	}, [selectedHooks, selectedSkills, selectedMCPServers, allowedTools, disallowedTools, envVars, extraFields]);

	// Save helper - serializes all current fields with an override for the changed field
	const saveConfig = useCallback(
		(overrides: Partial<ClaudeConfigState>) => {
			const json = serializeClaudeConfig({
				hooks: overrides.hooks ?? selectedHooks,
				skillRefs: overrides.skillRefs ?? selectedSkills,
				mcpServers: overrides.mcpServers ?? selectedMCPServers,
				allowedTools: overrides.allowedTools ?? allowedTools,
				disallowedTools: overrides.disallowedTools ?? disallowedTools,
				env: overrides.env ?? envVars,
				extra: overrides.extra ?? extraFields,
			});
			onSave(json);
		},
		[selectedHooks, selectedSkills, selectedMCPServers, allowedTools, disallowedTools, envVars, extraFields, onSave],
	);

	// Handle JSON override blur
	const handleJsonBlur = useCallback(() => {
		try {
			const parsed = JSON.parse(jsonText);
			if (typeof parsed !== 'object' || parsed === null) {
				setJsonError('Invalid JSON');
				return;
			}
			const config = parseClaudeConfig(jsonText);
			setSelectedHooks(config.hooks);
			setSelectedSkills(config.skillRefs);
			setSelectedMCPServers(config.mcpServers);
			setAllowedTools(config.allowedTools);
			setDisallowedTools(config.disallowedTools);
			setEnvVars(config.env);
			setExtraFields(config.extra);
			setJsonError('');
			jsonActiveRef.current = false;
			onSave(jsonText);
		} catch {
			setJsonError('Invalid JSON');
		}
	}, [jsonText, onSave]);

	// Merged config for reference display
	const template = phase.template;
	const templateConfigStr = (template as Record<string, unknown> | undefined)?.claudeConfig as string | undefined;
	const merged = useMemo(
		() => mergeClaudeConfigs(templateConfigStr, phase.claudeConfigOverride),
		[templateConfigStr, phase.claudeConfigOverride],
	);

	const inheritedCount =
		(templateConfigStr ? parseClaudeConfig(templateConfigStr) : null);

	return (
		<div className="claude-config-summary">
			<h4 className="claude-config-summary__title">Claude Config</h4>

			{inheritedCount && (
				(inheritedCount.hooks.length > 0 ||
				 inheritedCount.skillRefs.length > 0 ||
				 inheritedCount.mcpServers.length > 0 ||
				 inheritedCount.allowedTools.length > 0 ||
				 inheritedCount.disallowedTools.length > 0 ||
				 Object.keys(inheritedCount.env).length > 0) && (
					<div className="phase-inspector-setting-hint" style={{ marginBottom: '8px' }}>
						Inherited from template: {[
							inheritedCount.hooks.length > 0 && `${inheritedCount.hooks.length} hooks`,
							inheritedCount.skillRefs.length > 0 && `${inheritedCount.skillRefs.length} skills`,
							inheritedCount.mcpServers.length > 0 && `${inheritedCount.mcpServers.length} MCP servers`,
							inheritedCount.allowedTools.length > 0 && `${inheritedCount.allowedTools.length} allowed tools`,
							inheritedCount.disallowedTools.length > 0 && `${inheritedCount.disallowedTools.length} disallowed tools`,
							Object.keys(inheritedCount.env).length > 0 && `${Object.keys(inheritedCount.env).length} env vars`,
						].filter(Boolean).join(', ')}
					</div>
				)
			)}

			<CollapsibleSettingsSection title="Hooks" badgeCount={merged.hooks.length}>
				<LibraryPicker
					type="hooks"
					items={hooks}
					selectedNames={selectedHooks}
					onSelectionChange={(names) => {
						setSelectedHooks(names);
						jsonActiveRef.current = false;
						saveConfig({ hooks: names });
					}}
					error={hooksError}
					loading={hooksLoading}
					disabled={disabled}
				/>
			</CollapsibleSettingsSection>

			<CollapsibleSettingsSection title="MCP Servers" badgeCount={merged.mcpServers.length}>
				<LibraryPicker
					type="mcpServers"
					items={mcpServers}
					selectedNames={selectedMCPServers}
					onSelectionChange={(names) => {
						setSelectedMCPServers(names);
						jsonActiveRef.current = false;
						saveConfig({ mcpServers: names });
					}}
					error={mcpError}
					loading={mcpLoading}
					disabled={disabled}
				/>
			</CollapsibleSettingsSection>

			<CollapsibleSettingsSection title="Skills" badgeCount={merged.skillRefs.length}>
				<LibraryPicker
					type="skills"
					items={skills}
					selectedNames={selectedSkills}
					onSelectionChange={(names) => {
						setSelectedSkills(names);
						jsonActiveRef.current = false;
						saveConfig({ skillRefs: names });
					}}
					error={skillsError}
					loading={skillsLoading}
					disabled={disabled}
				/>
			</CollapsibleSettingsSection>

			<CollapsibleSettingsSection title="Allowed Tools" badgeCount={merged.allowedTools.length}>
				<TagInput
					tags={allowedTools}
					onChange={(tags) => {
						setAllowedTools(tags);
						jsonActiveRef.current = false;
						saveConfig({ allowedTools: tags });
					}}
					placeholder="Add tool name..."
					disabled={disabled}
				/>
			</CollapsibleSettingsSection>

			<CollapsibleSettingsSection title="Disallowed Tools" badgeCount={merged.disallowedTools.length}>
				<TagInput
					tags={disallowedTools}
					onChange={(tags) => {
						setDisallowedTools(tags);
						jsonActiveRef.current = false;
						saveConfig({ disallowedTools: tags });
					}}
					placeholder="Add tool name..."
					disabled={disabled}
				/>
			</CollapsibleSettingsSection>

			<CollapsibleSettingsSection title="Env Vars" badgeCount={Object.keys(merged.env).length}>
				<KeyValueEditor
					entries={envVars}
					onChange={(entries) => {
						setEnvVars(entries);
						jsonActiveRef.current = false;
						saveConfig({ env: entries });
					}}
					disabled={disabled}
				/>
			</CollapsibleSettingsSection>

			<CollapsibleSettingsSection title="JSON Override" badgeCount={0}>
				<div className="claude-config-json-override">
					<textarea
						className={`claude-config-json-textarea ${jsonError ? 'claude-config-json-textarea--error' : ''}`}
						value={jsonText}
						onChange={(e) => {
							setJsonText(e.target.value);
							jsonActiveRef.current = true;
							setJsonError('');
						}}
						onBlur={handleJsonBlur}
						rows={6}
						disabled={disabled}
						aria-label="Claude config JSON override"
					/>
					{jsonError && (
						<span className="claude-config-json-error">{jsonError}</span>
					)}
				</div>
			</CollapsibleSettingsSection>
		</div>
	);
}
