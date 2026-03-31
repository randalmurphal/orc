import { useState, useEffect, useCallback, useMemo, type Dispatch, type ReactNode, type SetStateAction } from 'react';
import * as Collapsible from '@radix-ui/react-collapsible';
import { ChevronDown, ChevronRight, GripVertical } from 'lucide-react';
import { PromptSource, type WorkflowPhase, type WorkflowWithDetails, type PhaseTemplate } from '@/gen/orc/v1/workflow_pb';
import type { Agent, Hook, Skill } from '@/gen/orc/v1/config_pb';
import type { MCPServerInfo } from '@/gen/orc/v1/mcp_pb';
import {
	parseRuntimeConfig,
	serializeRuntimeConfig,
	hydrateSelectedMCPServers,
	type HookDefinition,
} from '@/lib/runtimeConfigUtils';
import { PROVIDERS, PROVIDER_MODELS } from '@/lib/providerUtils';
import { workflowClient } from '@/lib/client';
import { LibraryPicker } from '@/components/core/LibraryPicker';
import { KeyValueEditor } from '@/components/core/KeyValueEditor';
import { PromptEditor } from '../PromptEditor';
import type { AutoSave, FieldError, FieldErrors } from './shared';
import { fetchMCPServerConfig } from './shared';

interface AlwaysVisibleSectionProps {
	phase: WorkflowPhase;
	template: PhaseTemplate;
	agents: Agent[];
	agentsLoading: boolean;
	readOnly: boolean;
	fieldErrors: FieldErrors;
	setFieldErrors: Dispatch<SetStateAction<FieldErrors>>;
	savingFields: Set<string>;
	autoSave: AutoSave;
	isMobile: boolean;
	workflowDefaultProvider: string;
	workflowDetails: WorkflowWithDetails;
	onWorkflowRefresh?: () => void;
}

export function AlwaysVisibleSection({
	phase,
	template,
	agents,
	agentsLoading,
	readOnly,
	fieldErrors,
	setFieldErrors,
	savingFields,
	autoSave,
	isMobile,
	workflowDefaultProvider,
	workflowDetails,
	onWorkflowRefresh,
}: AlwaysVisibleSectionProps) {
	const [phaseName, setPhaseName] = useState(template.name || '');
	const [agentOverride, setAgentOverride] = useState(phase.agentOverride || '');
	const [modelOverride, setModelOverride] = useState(phase.modelOverride || '');
	const [providerOverride, setProviderOverride] = useState(phase.providerOverride || '');

	useEffect(() => {
		setPhaseName(template.name || '');
		setAgentOverride(phase.agentOverride || '');
		setModelOverride(phase.modelOverride || '');
		setProviderOverride(phase.providerOverride || '');
	}, [phase.id, template, phase.agentOverride, phase.modelOverride, phase.providerOverride]);

	const validatePhaseName = (name: string): FieldError | null => {
		if (!name.trim()) {
			return { message: 'Name cannot be empty', type: 'validation' };
		}
		return null;
	};

	const handlePhaseNameChange = (value: string) => {
		setPhaseName(value);
		const error = validatePhaseName(value);
		if (!error) {
			void autoSave('templateName', value);
		}
	};

	const handlePhaseNameBlur = () => {
		const error = validatePhaseName(phaseName);
		if (error) {
			setFieldErrors(prev => ({ ...prev, templateName: error }));
			setTimeout(() => {
				setPhaseName(template.name || '');
				setTimeout(() => {
					setFieldErrors(prev => ({ ...prev, templateName: null }));
				}, 100);
			}, 50);
		} else {
			setFieldErrors(prev => ({ ...prev, templateName: null }));
			void autoSave('templateName', phaseName, true);
		}
	};

	const handleAgentChange = (value: string) => {
		setAgentOverride(value);
		void autoSave('agentOverride', value || undefined);
	};

	const handleModelChange = (value: string) => {
		setModelOverride(value);
		void autoSave('modelOverride', value || undefined);
	};

	const handleProviderChange = async (value: string) => {
		const providerChanged = value !== providerOverride;
		setProviderOverride(value);

		if (providerChanged) {
			setModelOverride('');
			if (!workflowDetails?.workflow?.id) return;
			try {
				await workflowClient.updatePhase({
					workflowId: workflowDetails.workflow.id,
					phaseId: phase.id,
					providerOverride: value || undefined,
					modelOverride: undefined,
				});
				onWorkflowRefresh?.();
			} catch (error) {
				const errorMessage = error instanceof Error ? error.message : 'Save failed';
				setFieldErrors(prev => ({
					...prev,
					providerOverride: { message: errorMessage, type: 'save' },
				}));
			}
		} else {
			void autoSave('providerOverride', value || undefined);
		}
	};

	const activeProvider = providerOverride || workflowDefaultProvider || 'claude';
	const activeModels = PROVIDER_MODELS[activeProvider] ?? [];
	const nameError = fieldErrors.templateName || validatePhaseName(phaseName);

	return (
		<div className={`always-visible-fields ${isMobile ? 'always-visible--mobile-stack' : ''}`}>
			<div className="field-group">
				<label htmlFor="phase-name" className="field-label">
					Phase Name
				</label>
				<input
					id="phase-name"
					data-testid="phase-name"
					type="text"
					value={phaseName}
					onChange={(e) => handlePhaseNameChange(e.target.value)}
					onBlur={handlePhaseNameBlur}
					disabled={readOnly || savingFields.has('templateName')}
					className={`field-input ${nameError ? 'field-error' : ''} ${isMobile ? 'touch-friendly' : ''}`}
					title={phaseName.length > 50 ? phaseName : undefined}
				/>
				{nameError && <span className="field-error">{nameError.message}</span>}
				{savingFields.has('templateName') && <span className="field-saving">Saving...</span>}
			</div>

			<div className="field-group">
				<label htmlFor="phase-executor" className="field-label">
					Executor
				</label>
				<select
					id="phase-executor"
					aria-label="Executor"
					value={agentOverride}
					onChange={(e) => handleAgentChange(e.target.value)}
					disabled={agentsLoading || readOnly || savingFields.has('agentOverride') || agents.length === 0}
					className={`field-input ${isMobile ? 'touch-friendly' : ''}`}
				>
					{agentsLoading ? (
						<option value="">Loading agents...</option>
					) : agents.length === 0 ? (
						<option value="">No agents available</option>
					) : (
						<>
							<option value="">
								{template.agentId ? `Inherit (${template.agentId})` : 'Inherit from template'}
							</option>
							{agents.map((agent) => (
								<option key={agent.name} value={agent.name}>
									{agent.name}{agent.description ? ` (${agent.description})` : ''}
								</option>
							))}
						</>
					)}
				</select>
				{savingFields.has('agentOverride') && <span className="field-saving">Saving...</span>}
				{fieldErrors.agentOverride && <span className="field-error">{fieldErrors.agentOverride.message}</span>}
			</div>

			<div className="field-group">
				<label htmlFor="phase-provider" className="field-label">
					Provider
				</label>
				<select
					id="phase-provider"
					aria-label="Provider"
					value={providerOverride}
					onChange={(e) => void handleProviderChange(e.target.value)}
					disabled={readOnly || savingFields.has('providerOverride')}
					className={`field-input ${isMobile ? 'touch-friendly' : ''}`}
				>
					<option value="">Inherit from workflow</option>
					{PROVIDERS.map((p) => (
						<option key={p.value} value={p.value}>{p.label}</option>
					))}
				</select>
				{savingFields.has('providerOverride') && <span className="field-saving">Saving...</span>}
				{fieldErrors.providerOverride && <span className="field-error">{fieldErrors.providerOverride.message}</span>}
			</div>

			<div className="field-group">
				<label htmlFor="phase-model" className="field-label">
					Model
				</label>
				{activeModels.length > 0 ? (
					<select
						id="phase-model"
						aria-label="Model"
						value={modelOverride}
						onChange={(e) => handleModelChange(e.target.value)}
						disabled={readOnly || savingFields.has('modelOverride')}
						className={`field-input ${isMobile ? 'touch-friendly' : ''}`}
					>
						<option value="">Inherit from workflow</option>
						{activeModels.map((m) => (
							<option key={m.value} value={m.value}>{m.label}</option>
						))}
					</select>
				) : (
					<input
						id="phase-model"
						aria-label="Model"
						type="text"
						value={modelOverride}
						onChange={(e) => handleModelChange(e.target.value)}
						onBlur={() => void autoSave('modelOverride', modelOverride || undefined, true)}
						disabled={readOnly || savingFields.has('modelOverride')}
						className={`field-input ${isMobile ? 'touch-friendly' : ''}`}
						placeholder="Type model name..."
					/>
				)}
				{savingFields.has('modelOverride') && <span className="field-saving">Saving...</span>}
				{fieldErrors.modelOverride && <span className="field-error">{fieldErrors.modelOverride.message}</span>}
			</div>
		</div>
	);
}

interface CollapsibleSectionProps {
	title: string;
	isOpen: boolean;
	onToggle: () => void;
	testId: string;
	isMobile: boolean;
	children: ReactNode;
}

export function CollapsibleSection({
	title,
	isOpen,
	onToggle,
	testId,
	isMobile,
	children,
}: CollapsibleSectionProps) {
	return (
		<Collapsible.Root
			open={isOpen}
			onOpenChange={onToggle}
			className={`collapsible-section ${isMobile ? 'section--mobile-stack' : ''}`}
		>
			<Collapsible.Trigger
				className={`collapsible-header ${isMobile ? 'touch-friendly' : ''}`}
				style={{ minHeight: isMobile ? '44px' : undefined }}
			>
				{isOpen ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
				<span>{title}</span>
			</Collapsible.Trigger>
			{isOpen && (
				<div className="collapsible-content" data-testid={`${testId}-content`}>
					{children}
				</div>
			)}
		</Collapsible.Root>
	);
}

interface SubAgentsSectionProps {
	phase: WorkflowPhase;
	agents: Agent[];
	agentsLoading: boolean;
	readOnly: boolean;
	fieldErrors: FieldErrors;
	savingFields: Set<string>;
	autoSave: AutoSave;
}

export function SubAgentsSection({
	phase,
	agents,
	agentsLoading,
	readOnly,
	fieldErrors,
	savingFields,
	autoSave,
}: SubAgentsSectionProps) {
	const [subAgentsOverride, setSubAgentsOverride] = useState<string[]>(
		phase.subAgentsOverride ?? [],
	);
	const [draggedAgent, setDraggedAgent] = useState<string | null>(null);

	useEffect(() => {
		setSubAgentsOverride(phase.subAgentsOverride ?? []);
	}, [phase.id, phase.subAgentsOverride]);

	const handleAddAgent = (agentName: string) => {
		const newSubAgents = [...subAgentsOverride, agentName];
		setSubAgentsOverride(newSubAgents);
		void autoSave('subAgentsOverride', newSubAgents);
	};

	const handleRemoveAgent = (agentName: string) => {
		const newSubAgents = subAgentsOverride.filter(name => name !== agentName);
		setSubAgentsOverride(newSubAgents);
		void autoSave('subAgentsOverride', newSubAgents);
	};

	const handleDrop = (targetAgentName: string) => {
		if (!draggedAgent || draggedAgent === targetAgentName) return;
		const currentOrder = [...subAgentsOverride];
		const draggedIndex = currentOrder.indexOf(draggedAgent);
		const targetIndex = currentOrder.indexOf(targetAgentName);
		if (draggedIndex === -1 || targetIndex === -1) return;
		currentOrder.splice(draggedIndex, 1);
		currentOrder.splice(targetIndex, 0, draggedAgent);
		setSubAgentsOverride(currentOrder);
		void autoSave('subAgentsOverride', currentOrder);
	};

	if (agentsLoading) {
		return <span className="field-loading">Loading agents...</span>;
	}
	if (agents.length === 0) {
		return <span className="field-error">No agents available</span>;
	}

	const assignedAgents = subAgentsOverride.filter(name =>
		agents.some(agent => agent.name === name),
	);
	const availableAgents = agents.filter(agent => !subAgentsOverride.includes(agent.name));

	return (
		<div className="sub-agents-section">
			{assignedAgents.length === 0 ? (
				<p className="sub-agents-empty">None assigned</p>
			) : (
				<div className="sub-agents-list">
					{assignedAgents.map((agentName) => (
						<div
							key={agentName}
							className={`sub-agent-item ${draggedAgent === agentName ? 'sub-agent-item--dragging' : ''}`}
							draggable={!readOnly}
							data-testid={`drag-handle-${agentName}`}
							onDragStart={() => setDraggedAgent(agentName)}
							onDragOver={(e) => e.preventDefault()}
							onDrop={() => handleDrop(agentName)}
							onDragEnd={() => setDraggedAgent(null)}
						>
							{!readOnly && <GripVertical size={14} className="drag-handle" />}
							<span className="agent-name">{agentName}</span>
							{!readOnly && (
								<button
									type="button"
									onClick={() => handleRemoveAgent(agentName)}
									className="remove-button"
									aria-label={`Remove ${agentName}`}
								>
									×
								</button>
							)}
						</div>
					))}
				</div>
			)}

			{!readOnly && availableAgents.length > 0 && (
				<div className="add-agent-section">
					<select
						onChange={(e) => {
							if (e.target.value) {
								handleAddAgent(e.target.value);
								e.target.value = '';
							}
						}}
						className="add-agent-select"
						aria-label="Add agent"
					>
						<option value="">Add agent...</option>
						{availableAgents.map((agent) => (
							<option key={agent.name} value={agent.name}>
								{agent.name}
							</option>
						))}
					</select>
				</div>
			)}

			{fieldErrors.subAgentsOverride && <span className="field-error">{fieldErrors.subAgentsOverride.message}</span>}
			{savingFields.has('subAgentsOverride') && <span className="field-saving">Saving...</span>}
		</div>
	);
}

interface PromptSectionProps {
	phase: WorkflowPhase;
	template: PhaseTemplate;
	readOnly: boolean;
	fieldErrors: FieldErrors;
}

export function PromptSection({ phase: _phase, template, readOnly, fieldErrors: _fieldErrors }: PromptSectionProps) {
	const [promptSource, setPromptSource] = useState<PromptSource>(
		template.promptSource || PromptSource.EMBEDDED,
	);
	const [filePath, setFilePath] = useState('');

	const validateFilePath = (path: string): FieldError | null => {
		if (path && !path.match(/\.(md|txt)$/i)) {
			return { message: 'Invalid file path - must end in .md or .txt', type: 'validation' };
		}
		return null;
	};

	const filePathError = validateFilePath(filePath);

	return (
		<div className="prompt-section">
			<div className="prompt-source-toggle">
				<button
					type="button"
					className={`source-button ${promptSource === PromptSource.EMBEDDED ? 'active' : ''}`}
					onClick={() => setPromptSource(PromptSource.EMBEDDED)}
					aria-pressed={promptSource === PromptSource.EMBEDDED}
				>
					Template
				</button>
				<button
					type="button"
					className={`source-button ${promptSource === PromptSource.DB ? 'active' : ''}`}
					onClick={() => setPromptSource(PromptSource.DB)}
					aria-pressed={promptSource === PromptSource.DB}
				>
					Custom
				</button>
				<button
					type="button"
					className={`source-button ${promptSource === PromptSource.FILE ? 'active' : ''}`}
					onClick={() => setPromptSource(PromptSource.FILE)}
					aria-pressed={promptSource === PromptSource.FILE}
				>
					File
				</button>
			</div>

			{promptSource === PromptSource.EMBEDDED && (
				<div className="prompt-template">
					<p>Using template content: {template.promptContent?.slice(0, 100)}...</p>
				</div>
			)}

			{promptSource === PromptSource.DB && (
				<div className="prompt-custom" data-testid="prompt-editor">
					<PromptEditor
						phaseTemplateId={template.id}
						promptSource={promptSource}
						promptContent={template.promptContent}
						readOnly={readOnly}
					/>
					{_fieldErrors.promptContent && <span className="field-error">Failed to load prompt content</span>}
				</div>
			)}

			{promptSource === PromptSource.FILE && (
				<div className="prompt-file">
					<label htmlFor="prompt-file-path" className="field-label">
						File Path
					</label>
					<input
						id="prompt-file-path"
						aria-label="File path"
						type="text"
						value={filePath}
						onChange={(e) => setFilePath(e.target.value)}
						className={`field-input ${filePathError ? 'field-error' : ''}`}
						placeholder="path/to/prompt.md"
					/>
					{filePathError && <span className="field-error">{filePathError.message}</span>}
				</div>
			)}
		</div>
	);
}

interface DataFlowSectionProps {
	phase: WorkflowPhase;
	template: PhaseTemplate;
	workflowDetails: WorkflowWithDetails;
	readOnly: boolean;
	fieldErrors: FieldErrors;
	autoSave: AutoSave;
}

export function DataFlowSection({
	phase: _phase,
	template,
	workflowDetails,
	readOnly,
	fieldErrors,
	autoSave,
}: DataFlowSectionProps) {
	const [producesArtifact, setProducesArtifact] = useState(false);
	const [artifactType, setArtifactType] = useState('spec');
	const [outputVariable, setOutputVariable] = useState('');

	const inputVariables = template?.inputVariables ?? [];
	const workflowVariables = workflowDetails.variables ?? [];
	const workflowVariableNames = new Set(workflowVariables.map((v) => v.name));

	return (
		<div className="data-flow-section">
			<div className="input-variables">
				<h4 className="section-title">Input Variables</h4>
				{inputVariables.length === 0 ? (
					<p className="empty-state">None defined</p>
				) : (
					<ul className="variable-list">
						{inputVariables.map((varName: string) => {
							const satisfied = workflowVariableNames.has(varName);
							const varDef = workflowVariables.find((v) => v.name === varName);
							return (
								<li key={varName} className="variable-item">
									<code className="variable-name">{`{{${varName}}}`}</code>
									<span className={`variable-status ${satisfied ? 'satisfied' : 'missing'}`}>
										{satisfied ? '✓ Provided' : '⚠ Missing'}
									</span>
									{varDef?.description && <p className="variable-description">{varDef.description}</p>}
								</li>
							);
						})}
					</ul>
				)}
			</div>

			<div className="output-variable">
				<label htmlFor="output-variable" className="field-label">
					Output Variable
				</label>
				<input
					id="output-variable"
					type="text"
					value={outputVariable}
					onChange={(e) => {
						setOutputVariable(e.target.value);
						void autoSave('outputVariable', e.target.value);
					}}
					disabled={readOnly}
					className="field-input"
					placeholder="Variable name to store output"
				/>
			</div>

			<div className="artifact-section">
				<label className="checkbox-label">
					<input
						type="checkbox"
						checked={producesArtifact}
						onChange={(e) => {
							setProducesArtifact(e.target.checked);
							void autoSave('producesArtifact', e.target.checked);
						}}
						disabled={readOnly}
						aria-label="Produces artifact"
					/>
					<span>Produces Artifact</span>
				</label>

				{producesArtifact && (
					<div className="artifact-type">
						<label htmlFor="artifact-type" className="field-label">
							Artifact Type
						</label>
						<select
							id="artifact-type"
							aria-label="Artifact type"
							value={artifactType}
							onChange={(e) => {
								setArtifactType(e.target.value);
								void autoSave('artifactType', e.target.value);
							}}
							disabled={readOnly}
							className="field-input"
						>
							<option value="spec">spec</option>
							<option value="tests">tests</option>
							<option value="docs">docs</option>
							<option value="code">code</option>
						</select>
						{fieldErrors.artifactType && <span className="field-error">Failed to load artifact types</span>}
					</div>
				)}
			</div>
		</div>
	);
}

interface EnvironmentSectionProps {
	phase: WorkflowPhase;
	hooks: Hook[];
	skills: Skill[];
	mcpServers: MCPServerInfo[];
	hooksLoading: boolean;
	skillsLoading: boolean;
	mcpLoading: boolean;
	readOnly: boolean;
	fieldErrors: FieldErrors;
	autoSave: AutoSave;
}

export function EnvironmentSection({
	phase,
	hooks,
	skills,
	mcpServers,
	hooksLoading,
	skillsLoading,
	mcpLoading,
	readOnly,
	fieldErrors: _fieldErrors,
	autoSave,
}: EnvironmentSectionProps) {
	const [workingDirectory, setWorkingDirectory] = useState('inherit');
	const [envVars, setEnvVars] = useState<Record<string, string>>({});
	const currentConfig = useMemo(
		() => parseRuntimeConfig(phase.runtimeConfigOverride),
		[phase.runtimeConfigOverride],
	);
	const [selectedMCPServers, setSelectedMCPServers] = useState<string[]>(currentConfig.mcpServers);
	const [selectedSkills, setSelectedSkills] = useState<string[]>(currentConfig.skillRefs);
	const [selectedHooks, setSelectedHooks] = useState<string[]>(currentConfig.hooks);

	useEffect(() => {
		const config = parseRuntimeConfig(phase.runtimeConfigOverride);
		setSelectedMCPServers(config.mcpServers);
		setSelectedSkills(config.skillRefs);
		setSelectedHooks(config.hooks);
	}, [phase.id, phase.runtimeConfigOverride]);

	const saveConfigUpdate = useCallback(
		async (update: Partial<{ mcpServers: string[]; skillRefs: string[]; hooks: string[] }>) => {
			const nextMcpServers = update.mcpServers ?? selectedMCPServers;
			const mcpServerData = await hydrateSelectedMCPServers(
				nextMcpServers,
				currentConfig.mcpServerData ?? {},
				fetchMCPServerConfig,
			);
			const newConfig = serializeRuntimeConfig({
				hooks: update.hooks ?? selectedHooks,
				skillRefs: update.skillRefs ?? selectedSkills,
				mcpServers: nextMcpServers,
				allowedTools: currentConfig.allowedTools,
				disallowedTools: currentConfig.disallowedTools,
				env: currentConfig.env,
				mcpServerData,
				hookConfig: currentConfig.hookConfig,
				hookEventTypes: currentConfig.hookEventTypes,
				extra: currentConfig.extra,
			}, {
				hookDefinitions: hooks.map((hook): HookDefinition => ({
					name: hook.name,
					eventType: hook.eventType,
				})),
			});
			void autoSave('runtimeConfigOverride', newConfig);
		},
		[selectedHooks, selectedSkills, selectedMCPServers, currentConfig, autoSave, hooks],
	);

	const isLoading = hooksLoading || skillsLoading || mcpLoading;
	const hasNoData = hooks.length === 0 && skills.length === 0 && mcpServers.length === 0;

	return (
		<div className="environment-section">
			<div className="working-directory">
				<label htmlFor="working-directory" className="field-label">
					Working Directory
				</label>
				<select
					id="working-directory"
					value={workingDirectory}
					onChange={(e) => {
						setWorkingDirectory(e.target.value);
						void autoSave('workingDirectory', e.target.value);
					}}
					disabled={readOnly}
					className="field-input"
				>
					<option value="inherit">Inherit from workflow</option>
					<option value="project-root">Project Root</option>
					<option value="task-specific">Task-specific</option>
				</select>
			</div>

			<div className="env-vars">
				<h4 className="section-title">Environment Variables</h4>
				<KeyValueEditor
					entries={envVars}
					onChange={(vars) => {
						setEnvVars(vars);
						void autoSave('envVars', vars);
					}}
					disabled={readOnly}
				/>
			</div>

			{isLoading ? (
				<span className="field-loading">Loading environment options...</span>
			) : hasNoData ? (
				<p className="empty-state">None configured</p>
			) : (
				<div className="environment-tools">
					{mcpServers.length > 0 && (
						<div className="env-tool-section">
							<h4 className="section-title">MCP Servers</h4>
							<LibraryPicker
								type="mcpServers"
								items={mcpServers}
								selectedNames={selectedMCPServers}
								onSelectionChange={(names) => {
									setSelectedMCPServers(names);
									void saveConfigUpdate({ mcpServers: names });
								}}
								loading={mcpLoading}
								disabled={readOnly}
							/>
						</div>
					)}

					{skills.length > 0 && (
						<div className="env-tool-section">
							<h4 className="section-title">Skills</h4>
							<LibraryPicker
								type="skills"
								items={skills}
								selectedNames={selectedSkills}
								onSelectionChange={(names) => {
									setSelectedSkills(names);
									void saveConfigUpdate({ skillRefs: names });
								}}
								loading={skillsLoading}
								disabled={readOnly}
							/>
						</div>
					)}

					{hooks.length > 0 && (
						<div className="env-tool-section">
							<h4 className="section-title">Hooks</h4>
							<LibraryPicker
								type="hooks"
								items={hooks}
								selectedNames={selectedHooks}
								onSelectionChange={(names) => {
									setSelectedHooks(names);
									void saveConfigUpdate({ hooks: names });
								}}
								loading={hooksLoading}
								disabled={readOnly}
							/>
						</div>
					)}
				</div>
			)}
		</div>
	);
}

interface AdvancedSectionProps {
	phase: WorkflowPhase;
	readOnly: boolean;
	fieldErrors: FieldErrors;
	autoSave: AutoSave;
	onDeletePhase?: () => void;
}

export function AdvancedSection({
	phase,
	readOnly,
	fieldErrors: _fieldErrors,
	autoSave,
	onDeletePhase,
}: AdvancedSectionProps) {
	const [thinkingOverride, setThinkingOverride] = useState(phase.thinkingOverride ?? false);

	useEffect(() => {
		setThinkingOverride(phase.thinkingOverride ?? false);
	}, [phase.id, phase.thinkingOverride]);

	return (
		<div className="advanced-section">
			<div className="thinking-override">
				<label className="checkbox-label">
					<input
						type="checkbox"
						checked={thinkingOverride}
						onChange={(e) => {
							setThinkingOverride(e.target.checked);
							void autoSave('thinkingOverride', e.target.checked);
						}}
						disabled={readOnly}
						aria-label="Thinking override"
					/>
					<span>Enable thinking override</span>
				</label>
			</div>

			{!readOnly && onDeletePhase && (
				<div className="danger-zone">
					<button type="button" onClick={onDeletePhase} className="delete-button">
						Remove Phase
					</button>
				</div>
			)}
		</div>
	);
}
