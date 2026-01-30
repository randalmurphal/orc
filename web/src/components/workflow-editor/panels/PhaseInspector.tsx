import { useState, useEffect, useCallback, useRef } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import * as Collapsible from '@radix-ui/react-collapsible';
import { ChevronDown, ChevronRight } from 'lucide-react';
import { workflowClient, configClient } from '@/lib/client';
import {
	GateType,
	VariableSourceType,
} from '@/gen/orc/v1/workflow_pb';
import type {
	WorkflowPhase,
	WorkflowWithDetails,
	WorkflowVariable,
} from '@/gen/orc/v1/workflow_pb';
import type { Agent } from '@/gen/orc/v1/config_pb';
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
					<option value={GateType.SKIP}>Skip</option>
				</select>
			</div>

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

			{/* Executor Agent */}
			<div className="phase-inspector-setting">
				<label htmlFor="inspector-agent" className="phase-inspector-setting-label">
					Executor Agent
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
