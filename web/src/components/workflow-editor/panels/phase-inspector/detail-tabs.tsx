import { useCallback, useEffect, useMemo, useState } from 'react';
import { GateType, type WorkflowPhase, type WorkflowVariable, type WorkflowWithDetails } from '@/gen/orc/v1/workflow_pb';
import type { Agent } from '@/gen/orc/v1/config_pb';
import { configClient, workflowClient } from '@/lib/client';
import { CollapsibleSettingsSection } from '@/components/core/CollapsibleSettingsSection';
import { VariableModal } from '@/components/workflow-editor/VariableModal';
import { ConditionEditor, LoopEditor } from '@/components/workflows';
import type { FieldErrors } from './shared';
import { formatSourceType } from './shared';
import { RuntimeConfigEditor } from './runtime-config-editor';

interface CompletionCriteriaTabProps {
	phase: WorkflowPhase;
}

export function CompletionCriteriaTab({ phase }: CompletionCriteriaTabProps) {
	const template = phase.template;
	const gateType = phase.gateTypeOverride || template?.gateType || GateType.AUTO;

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
				<h4 className="phase-inspector__criteria-label">Output Format</h4>
				<p className="phase-inspector__criteria-hint">
					Phase completes when Claude outputs JSON with <code>{`{"status": "complete", ...}`}</code>
				</p>
			</div>
		</div>
	);
}

interface AvailableVariablesListProps {
	variables: WorkflowVariable[];
	workflowDetails: WorkflowWithDetails;
	workflowIsBuiltin: boolean;
	onWorkflowRefresh?: () => void;
}

export function AvailableVariablesList({
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

interface SettingsTabProps {
	phase: WorkflowPhase;
	workflowDetails: WorkflowWithDetails;
	readOnly: boolean;
	error: string | null;
	onError: (err: string | null) => void;
	onWorkflowRefresh?: () => void;
	onDeletePhase?: () => void;
}

export function SettingsTab({
	phase,
	workflowDetails,
	readOnly,
	error,
	onError,
	onWorkflowRefresh,
	onDeletePhase,
}: SettingsTabProps) {
	const [modelOverride, setModelOverride] = useState<string>(phase.modelOverride ?? '');
	const [thinkingOverride, setThinkingOverride] = useState<boolean>(phase.thinkingOverride ?? false);
	const [gateTypeOverride, setGateTypeOverride] = useState<GateType>(phase.gateTypeOverride ?? GateType.UNSPECIFIED);
	const [agents, setAgents] = useState<Agent[]>([]);
	const [agentsLoading, setAgentsLoading] = useState(true);
	const [agentOverride, setAgentOverride] = useState<string>(phase.agentOverride ?? '');
	const [subAgentsOverride, setSubAgentsOverride] = useState<string[]>(phase.subAgentsOverride ?? []);
	const [runtimeConfigDraft, setRuntimeConfigDraft] = useState<string | null>(null);
	const [saving, setSaving] = useState(false);
	const [conditionDraft, setConditionDraft] = useState<string | undefined>(undefined);
	const [conditionDirty, setConditionDirty] = useState(false);
	const [loopConfigDraft, setLoopConfigDraft] = useState<string | undefined>(undefined);
	const [loopConfigDirty, setLoopConfigDirty] = useState(false);

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
		return () => {
			mounted = false;
		};
	}, []);

	useEffect(() => {
		setModelOverride(phase.modelOverride ?? '');
		setThinkingOverride(phase.thinkingOverride ?? false);
		setGateTypeOverride(phase.gateTypeOverride ?? GateType.UNSPECIFIED);
		setAgentOverride(phase.agentOverride ?? '');
		setSubAgentsOverride(phase.subAgentsOverride ?? []);
		setRuntimeConfigDraft(null);
		setConditionDraft(phase.condition);
		setConditionDirty(false);
		setLoopConfigDraft(phase.loopConfig);
		setLoopConfigDirty(false);
		onError(null);
	}, [phase, onError]);

	const isDirty = useMemo(() => {
		if (modelOverride !== (phase.modelOverride ?? '')) return true;
		if (thinkingOverride !== (phase.thinkingOverride ?? false)) return true;
		if (gateTypeOverride !== (phase.gateTypeOverride ?? GateType.UNSPECIFIED)) return true;
		if (agentOverride !== (phase.agentOverride ?? '')) return true;
		const origSorted = [...(phase.subAgentsOverride ?? [])].sort();
		const currSorted = [...subAgentsOverride].sort();
		if (JSON.stringify(currSorted) !== JSON.stringify(origSorted)) return true;
		if (runtimeConfigDraft !== null) return true;
		if (conditionDirty) return true;
		if (loopConfigDirty) return true;
		return false;
	}, [modelOverride, thinkingOverride, gateTypeOverride, agentOverride, subAgentsOverride, runtimeConfigDraft, conditionDirty, loopConfigDirty, phase]);

	const handleSave = useCallback(async () => {
		const workflowId = workflowDetails.workflow?.id;
		if (!workflowId) return;
		onError(null);
		setSaving(true);
		try {
			await workflowClient.updatePhase({
				workflowId,
				phaseId: phase.id,
				modelOverride: modelOverride || undefined,
				thinkingOverride,
				gateTypeOverride: gateTypeOverride || undefined,
				agentOverride: agentOverride || undefined,
				subAgentsOverride,
				subAgentsOverrideSet: true,
				...(runtimeConfigDraft !== null ? { runtimeConfigOverride: runtimeConfigDraft || undefined } : {}),
				...(conditionDirty ? { condition: conditionDraft || '' } : {}),
				...(loopConfigDirty ? { loopConfig: loopConfigDraft || '' } : {}),
			});
			setRuntimeConfigDraft(null);
			setConditionDirty(false);
			setLoopConfigDirty(false);
			onWorkflowRefresh?.();
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Update failed';
			onError(message);
		} finally {
			setSaving(false);
		}
	}, [workflowDetails, phase.id, modelOverride, thinkingOverride, gateTypeOverride, agentOverride, subAgentsOverride, runtimeConfigDraft, conditionDirty, conditionDraft, loopConfigDirty, loopConfigDraft, onError, onWorkflowRefresh]);

	const handleDiscard = useCallback(() => {
		setModelOverride(phase.modelOverride ?? '');
		setThinkingOverride(phase.thinkingOverride ?? false);
		setGateTypeOverride(phase.gateTypeOverride ?? GateType.UNSPECIFIED);
		setAgentOverride(phase.agentOverride ?? '');
		setSubAgentsOverride(phase.subAgentsOverride ?? []);
		setRuntimeConfigDraft(null);
		setConditionDraft(phase.condition);
		setConditionDirty(false);
		setLoopConfigDraft(phase.loopConfig);
		setLoopConfigDirty(false);
		onError(null);
	}, [phase, onError]);

	const handleSubAgentToggle = (agentName: string, checked: boolean) => {
		setSubAgentsOverride((prev) => checked ? [...prev, agentName] : prev.filter((a) => a !== agentName));
	};

	const handleConditionChange = useCallback((newCondition: string) => {
		setConditionDraft(newCondition || undefined);
		setConditionDirty(true);
	}, []);

	const handleLoopConfigChange = useCallback((newLoopConfig: string) => {
		setLoopConfigDraft(newLoopConfig || undefined);
		setLoopConfigDirty(true);
	}, []);

	const priorPhases = useMemo(() => {
		const phases = workflowDetails.phases ?? [];
		const currentSequence = phase.sequence;
		return phases.filter((p) => p.sequence < currentSequence).map((p) => p.phaseTemplateId);
	}, [workflowDetails.phases, phase.sequence]);

	const disabled = readOnly;

	return (
		<div className="phase-inspector-settings">
			{readOnly && <div className="phase-inspector-readonly-notice">Clone to customize</div>}
			{error && <div className="phase-inspector-settings-error">{error}</div>}

			{!readOnly && isDirty && (
				<div className="phase-inspector-save-bar">
					<button type="button" className="phase-inspector-save-btn" onClick={handleSave} disabled={saving}>
						{saving ? 'Saving...' : 'Save Changes'}
					</button>
					<button type="button" className="phase-inspector-discard-btn" onClick={handleDiscard} disabled={saving}>
						Discard
					</button>
				</div>
			)}

			<div className="phase-inspector-setting">
				<label htmlFor="inspector-model" className="phase-inspector-setting-label">Model</label>
				<select id="inspector-model" className="phase-inspector-setting-select" value={modelOverride} onChange={(e) => setModelOverride(e.target.value)} disabled={disabled}>
					<option value="">Inherit from workflow</option>
					<option value="sonnet">Sonnet</option>
					<option value="opus">Opus</option>
					<option value="haiku">Haiku</option>
				</select>
			</div>

			<div className="phase-inspector-setting">
				<label htmlFor="inspector-thinking" className="phase-inspector-setting-label">Thinking</label>
				<input id="inspector-thinking" type="checkbox" className="phase-inspector-setting-checkbox" checked={thinkingOverride} onChange={(e) => setThinkingOverride(e.target.checked)} disabled={disabled} />
			</div>

			<div className="phase-inspector-setting">
				<label htmlFor="inspector-gate-type" className="phase-inspector-setting-label">Gate Type</label>
				<select id="inspector-gate-type" className="phase-inspector-setting-select" value={gateTypeOverride} onChange={(e) => setGateTypeOverride(Number(e.target.value) as GateType)} disabled={disabled}>
					<option value={GateType.UNSPECIFIED}>Inherit from template</option>
					<option value={GateType.AUTO}>Auto</option>
					<option value={GateType.HUMAN}>Human</option>
					<option value={GateType.AI}>AI</option>
					<option value={GateType.SKIP}>Skip</option>
				</select>
			</div>

			{gateTypeOverride === GateType.AI && (
				<div className="phase-inspector-setting">
					<label htmlFor="inspector-ai-gate-agent" className="phase-inspector-setting-label">AI Gate Agent</label>
					{agents.length === 0 && !agentsLoading ? (
						<select id="inspector-ai-gate-agent" className="phase-inspector-setting-select" disabled>
							<option>No agents available</option>
						</select>
					) : (
						<select id="inspector-ai-gate-agent" className="phase-inspector-setting-select" disabled={disabled || agentsLoading}>
							<option value="">Select agent...</option>
							{agents.map((agent) => (
								<option key={agent.id} value={agent.id}>{agent.name}</option>
							))}
						</select>
					)}
				</div>
			)}

			<div className="phase-inspector-setting">
				<label htmlFor="inspector-agent" className="phase-inspector-setting-label">Executor</label>
				<select id="inspector-agent" className="phase-inspector-setting-select" value={agentOverride} onChange={(e) => setAgentOverride(e.target.value)} disabled={disabled || agentsLoading}>
					<option value="">
						{phase.template?.agentId ? `Inherit (${phase.template.agentId})` : 'Inherit from template'}
					</option>
					{agents.map((agent) => (
						<option key={agent.name} value={agent.name}>
							{agent.name}{agent.model ? ` (${agent.model})` : ''}
						</option>
					))}
				</select>
				<span className="phase-inspector-setting-hint">Agent that executes this phase</span>
			</div>

			<div className="phase-inspector-setting">
				<label className="phase-inspector-setting-label">Sub-Agents</label>
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
				<span className="phase-inspector-setting-hint">Agents available for delegation during execution</span>
			</div>

			<CollapsibleSettingsSection title="Condition" badgeCount={conditionDraft || phase.condition ? 1 : 0} defaultExpanded>
				<ConditionEditor condition={(conditionDirty ? conditionDraft : phase.condition) || ''} onChange={handleConditionChange} disabled={readOnly} />
			</CollapsibleSettingsSection>

			<CollapsibleSettingsSection title="Loop" badgeCount={loopConfigDraft || phase.loopConfig ? 1 : 0}>
				<LoopEditor loopConfig={(loopConfigDirty ? loopConfigDraft : phase.loopConfig) || ''} onChange={handleLoopConfigChange} priorPhases={priorPhases} disabled={readOnly} />
			</CollapsibleSettingsSection>

			<RuntimeConfigEditor phase={phase} disabled={readOnly} onSave={setRuntimeConfigDraft} />

			{!readOnly && onDeletePhase && (
				<div className="phase-inspector-danger-zone">
					<button type="button" className="phase-inspector-delete-btn" onClick={onDeletePhase}>
						Remove Phase
					</button>
				</div>
			)}
		</div>
	);
}
