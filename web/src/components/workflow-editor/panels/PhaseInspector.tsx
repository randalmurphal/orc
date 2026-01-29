import { useState, useEffect, useCallback } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { workflowClient } from '@/lib/client';
import {
	GateType,
	VariableSourceType,
} from '@/gen/orc/v1/workflow_pb';
import type {
	WorkflowPhase,
	WorkflowWithDetails,
} from '@/gen/orc/v1/workflow_pb';
import { PromptEditor } from './PromptEditor';
import './PhaseInspector.css';

type InspectorTab = 'prompt' | 'variables' | 'settings';

interface PhaseInspectorProps {
	phase: WorkflowPhase | null;
	workflowDetails: WorkflowWithDetails | null;
	readOnly: boolean;
	onWorkflowRefresh?: () => void;
}

function formatGateType(gt: GateType): string {
	switch (gt) {
		case GateType.HUMAN:
			return 'Human';
		case GateType.SKIP:
			return 'Skip';
		case GateType.AUTO:
			return 'Auto';
		default:
			return 'Auto';
	}
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
}: PhaseInspectorProps) {
	const [activeTab, setActiveTab] = useState<InspectorTab>('prompt');
	const [settingsError, setSettingsError] = useState<string | null>(null);
	const prevPhaseIdRef = useState<number | null>(null);

	// Reset to Prompt tab when selected phase changes
	useEffect(() => {
		if (phase && phase.id !== prevPhaseIdRef[0]) {
			setActiveTab('prompt');
			setSettingsError(null);
		}
		prevPhaseIdRef[0] = phase?.id ?? null;
	}, [phase, prevPhaseIdRef]);

	if (!phase) {
		return null;
	}

	if (!workflowDetails) {
		return (
			<div className="phase-inspector phase-inspector-loading">
				<span>Loading...</span>
			</div>
		);
	}

	const template = phase.template;
	const isBuiltin = template?.isBuiltin ?? false;
	const workflowIsBuiltin = workflowDetails.workflow?.isBuiltin ?? false;

	return (
		<div className="phase-inspector">
			<div className="phase-inspector-header">
				<h3 className="phase-inspector-title">
					{template?.name ?? phase.phaseTemplateId}
				</h3>
				<span className="phase-inspector-id">{phase.phaseTemplateId}</span>
				<span className={`phase-inspector-badge phase-inspector-badge--${isBuiltin ? 'builtin' : 'custom'}`}>
					{isBuiltin ? 'Built-in' : 'Custom'}
				</span>
			</div>

			<Tabs.Root
				value={activeTab}
				onValueChange={(v) => setActiveTab(v as InspectorTab)}
				className="phase-inspector-tabs"
			>
				<Tabs.List className="phase-inspector-tab-list" aria-label="Phase inspector tabs">
					<Tabs.Trigger value="prompt" className="phase-inspector-tab-trigger">
						Prompt
					</Tabs.Trigger>
					<Tabs.Trigger value="variables" className="phase-inspector-tab-trigger">
						Variables
					</Tabs.Trigger>
					<Tabs.Trigger value="settings" className="phase-inspector-tab-trigger">
						Settings
					</Tabs.Trigger>
				</Tabs.List>

				<Tabs.Content value="prompt" className="phase-inspector-tab-content">
					{template ? (
						<PromptEditor
							phaseTemplateId={template.id}
							promptSource={template.promptSource}
							promptContent={template.promptContent}
							readOnly={readOnly}
						/>
					) : (
						<div className="phase-inspector-empty">Phase not found</div>
					)}
				</Tabs.Content>

				<Tabs.Content value="variables" className="phase-inspector-tab-content">
					<VariablesTab
						phase={phase}
						workflowDetails={workflowDetails}
						readOnly={readOnly}
						workflowIsBuiltin={workflowIsBuiltin}
					/>
				</Tabs.Content>

				<Tabs.Content value="settings" className="phase-inspector-tab-content">
					<SettingsTab
						phase={phase}
						workflowDetails={workflowDetails}
						readOnly={readOnly}
						error={settingsError}
						onError={setSettingsError}
						onWorkflowRefresh={onWorkflowRefresh}
					/>
				</Tabs.Content>
			</Tabs.Root>
		</div>
	);
}

// ─── Variables Tab ──────────────────────────────────────────────────────────

interface VariablesTabProps {
	phase: WorkflowPhase;
	workflowDetails: WorkflowWithDetails;
	readOnly: boolean;
	workflowIsBuiltin: boolean;
}

function VariablesTab({ phase, workflowDetails, readOnly, workflowIsBuiltin }: VariablesTabProps) {
	const template = phase.template;
	const inputVariables = template?.inputVariables ?? [];
	const workflowVariables = workflowDetails.variables;
	const workflowVariableNames = new Set(workflowVariables.map((v) => v.name));

	return (
		<div className="phase-inspector-variables">
			{/* Input Variables Section */}
			<div className="phase-inspector-section">
				<h4 className="phase-inspector-section-title">Input Variables</h4>
				{inputVariables.length === 0 ? (
					<div className="phase-inspector-empty">No input variables defined</div>
				) : (
					<ul className="phase-inspector-var-list">
						{inputVariables.map((varName) => {
							const satisfied = workflowVariableNames.has(varName);
							return (
								<li key={varName} className="phase-inspector-var-item">
									<span className="phase-inspector-var-name">{varName}</span>
									<span
										className={`phase-inspector-var-status phase-inspector-var-status--${satisfied ? 'satisfied' : 'missing'}`}
										data-testid={`var-status-${varName}`}
										data-satisfied={String(satisfied)}
									>
										{satisfied ? '✓' : '⚠'}
									</span>
								</li>
							);
						})}
					</ul>
				)}
			</div>

			{/* Available Workflow Variables Section */}
			<div className="phase-inspector-section">
				<h4 className="phase-inspector-section-title">Available Variables</h4>
				{workflowVariables.length === 0 ? (
					<div className="phase-inspector-empty">No workflow variables</div>
				) : (
					<ul className="phase-inspector-var-list">
						{workflowVariables.map((wv) => (
							<li key={wv.id} className="phase-inspector-var-item">
								<span className="phase-inspector-var-name">{wv.name}</span>
								<span className="phase-inspector-var-source-badge">
									{formatSourceType(wv.sourceType)}
								</span>
								{wv.required && (
									<span
										className="phase-inspector-var-required"
										data-testid={`var-required-${wv.name}`}
									>
										required
									</span>
								)}
							</li>
						))}
					</ul>
				)}
				{!readOnly && !workflowIsBuiltin && (
					<button className="phase-inspector-add-var-btn">
						+ Add Variable
					</button>
				)}
			</div>
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
}

function SettingsTab({
	phase,
	workflowDetails,
	readOnly,
	error,
	onError,
	onWorkflowRefresh,
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

	// Reset state when phase changes
	useEffect(() => {
		setMaxIterations(phase.maxIterationsOverride ?? phase.template?.maxIterations ?? 3);
		setModelOverride(phase.modelOverride ?? '');
		setThinkingOverride(phase.thinkingOverride ?? false);
		setGateTypeOverride(phase.gateTypeOverride ?? GateType.UNSPECIFIED);
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
		</div>
	);
}
