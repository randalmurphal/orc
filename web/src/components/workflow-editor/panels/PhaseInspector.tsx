import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import * as Collapsible from '@radix-ui/react-collapsible';
import { ChevronDown, ChevronRight, GripVertical } from 'lucide-react';
import { workflowClient, configClient, mcpClient } from '@/lib/client';
import {
	GateType,
	VariableSourceType,
	PromptSource,
} from '@/gen/orc/v1/workflow_pb';
import type {
	WorkflowPhase,
	WorkflowWithDetails,
	WorkflowVariable,
	PhaseTemplate,
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
import { ConditionEditor, LoopEditor } from '@/components/workflows';
import './PhaseInspector.css';

interface PhaseInspectorProps {
	phase: WorkflowPhase | null;
	workflowDetails: WorkflowWithDetails | null;
	readOnly: boolean;
	onWorkflowRefresh?: () => void;
	onDeletePhase?: () => void;
}

// Section state management for persistence across phase selections
interface SectionState {
	subAgents: boolean;
	prompt: boolean;
	dataFlow: boolean;
	environment: boolean;
	advanced: boolean;
}

// Field validation state
interface FieldError {
	message: string;
	type: 'validation' | 'save' | 'load';
}

interface FieldErrors {
	[key: string]: FieldError | null;
}

// Debounced save state
interface PendingChanges {
	[key: string]: unknown;
}

const DEFAULT_SECTION_STATE: SectionState = {
	subAgents: false,
	prompt: false,
	dataFlow: false,
	environment: false,
	advanced: false,
};

// Track section state across phase selections
const sectionStateCache = new Map<number, SectionState>();

// Utility to check if we're on mobile viewport
const useMobileViewport = () => {
	const [isMobile, setIsMobile] = useState(false);

	useEffect(() => {
		// Check if window and matchMedia are available (for SSR and test environments)
		if (typeof window === 'undefined' || !window.matchMedia) {
			return;
		}

		const mediaQuery = window.matchMedia('(max-width: 640px)');
		setIsMobile(mediaQuery.matches);

		const handleChange = (e: MediaQueryListEvent) => {
			setIsMobile(e.matches);
		};

		mediaQuery.addEventListener('change', handleChange);
		return () => mediaQuery.removeEventListener('change', handleChange);
	}, []);

	return isMobile;
};

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
	const isMobile = useMobileViewport();

	// Section state management
	const [sectionState, setSectionState] = useState<SectionState>(DEFAULT_SECTION_STATE);
	const prevPhaseIdRef = useRef<number | null>(null);

	// Field values and errors
	const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
	const [savingFields, setSavingFields] = useState(new Set<string>());

	// Auto-save debounce
	const [_pendingChanges, setPendingChanges] = useState<PendingChanges>({});
	const debounceTimeoutRef = useRef<number | null>(null);

	// Data fetching states
	const [agents, setAgents] = useState<Agent[]>([]);
	const [hooks, setHooks] = useState<Hook[]>([]);
	const [skills, setSkills] = useState<Skill[]>([]);
	const [mcpServers, setMcpServers] = useState<MCPServerInfo[]>([]);
	const [agentsLoading, setAgentsLoading] = useState(true);
	const [hooksLoading, setHooksLoading] = useState(true);
	const [skillsLoading, setSkillsLoading] = useState(true);
	const [mcpLoading, setMcpLoading] = useState(true);

	// Track scroll position for maintenance during edits
	const inspectorRef = useRef<HTMLDivElement>(null);
	const lastScrollTopRef = useRef(0);

	// Load section state from cache when phase changes
	useEffect(() => {
		if (phase && phase.id !== prevPhaseIdRef.current) {
			const cached = sectionStateCache.get(phase.id);
			if (cached) {
				setSectionState(cached);
			} else {
				setSectionState(DEFAULT_SECTION_STATE);
			}

			// Clear pending changes and errors for new phase
			setPendingChanges({});
			setFieldErrors({});
			setSavingFields(new Set());

			// Cancel any pending saves
			if (debounceTimeoutRef.current) {
				clearTimeout(debounceTimeoutRef.current);
			}
		}
		prevPhaseIdRef.current = phase?.id ?? null;
	}, [phase]);

	// Save section state to cache when it changes
	useEffect(() => {
		if (phase) {
			sectionStateCache.set(phase.id, sectionState);
		}
	}, [phase, sectionState]);

	// Clear cache on unmount to prevent test pollution
	useEffect(() => {
		return () => {
			sectionStateCache.clear();
		};
	}, []);

	// Load data on mount
	useEffect(() => {
		const loadData = async () => {
			try {
				const [agentsResp, hooksResp, skillsResp, mcpResp] = await Promise.allSettled([
					configClient.listAgents({}),
					configClient.listHooks({}),
					configClient.listSkills({}),
					mcpClient.listMCPServers({}),
				]);

				if (agentsResp.status === 'fulfilled') {
					setAgents(agentsResp.value.agents);
				}
				if (hooksResp.status === 'fulfilled') {
					setHooks(hooksResp.value.hooks);
				}
				if (skillsResp.status === 'fulfilled') {
					setSkills(skillsResp.value.skills);
				}
				if (mcpResp.status === 'fulfilled') {
					setMcpServers(mcpResp.value.servers);
				}
			} catch (error) {
				console.error('Failed to load inspector data:', error);
			} finally {
				setAgentsLoading(false);
				setHooksLoading(false);
				setSkillsLoading(false);
				setMcpLoading(false);
			}
		};

		loadData();
	}, []);

	// Maintain scroll position during auto-saves
	useEffect(() => {
		const inspector = inspectorRef.current;
		if (inspector) {
			lastScrollTopRef.current = inspector.scrollTop;
		}
	});

	// Auto-save implementation with 500ms debounce
	const autoSave = useCallback(
		async (fieldName: string, value: unknown, immediate = false) => {
			if (!phase || !workflowDetails?.workflow?.id) return;

			// Update pending changes
			setPendingChanges(prev => ({ ...prev, [fieldName]: value }));

			// Clear existing timeout
			if (debounceTimeoutRef.current) {
				clearTimeout(debounceTimeoutRef.current);
			}

			const saveFunction = async () => {
				try {
					setSavingFields(prev => new Set(prev).add(fieldName));
					setFieldErrors(prev => ({ ...prev, [fieldName]: null }));

					// Restore scroll position before API call
					const inspector = inspectorRef.current;
					const scrollTop = lastScrollTopRef.current;

					await workflowClient.updatePhase({
						workflowId: workflowDetails.workflow!.id,
						phaseId: phase.id,
						[fieldName]: value,
					});

					// Restore scroll position after API call
					if (inspector && inspector.scrollTop !== scrollTop) {
						inspector.scrollTop = scrollTop;
					}

					// Clear from pending changes
					setPendingChanges(prev => {
						const next = { ...prev };
						delete next[fieldName];
						return next;
					});

					onWorkflowRefresh?.();
				} catch (error) {
					const errorMessage = error instanceof Error ? error.message : 'Save failed';

					// Set error and revert field value
					setFieldErrors(prev => ({
						...prev,
						[fieldName]: { message: errorMessage, type: 'save' }
					}));

					// Remove from pending changes (field will revert to original)
					setPendingChanges(prev => {
						const next = { ...prev };
						delete next[fieldName];
						return next;
					});
				} finally {
					setSavingFields(prev => {
						const next = new Set(prev);
						next.delete(fieldName);
						return next;
					});
				}
			};

			if (immediate) {
				// Cancel debounced save and save immediately
				if (debounceTimeoutRef.current) {
					clearTimeout(debounceTimeoutRef.current);
				}
				await saveFunction();
			} else {
				// Debounce the save
				debounceTimeoutRef.current = window.setTimeout(saveFunction, 500);
			}
		},
		[phase, workflowDetails, onWorkflowRefresh]
	);

	// Helper to toggle section state
	const toggleSection = useCallback((section: keyof SectionState) => {
		setSectionState(prev => ({
			...prev,
			[section]: !prev[section],
		}));
	}, []);

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

	return (
		<div
			ref={inspectorRef}
			className={`phase-inspector ${isMobile ? 'phase-inspector--mobile' : ''} ${isMobile ? 'inspector--compact-spacing' : ''}`}
			data-testid="phase-inspector"
		>
			{isBuiltin && readOnly && (
				<div className="phase-inspector-readonly-notice">
					Built-in template - clone to customize
				</div>
			)}

			{/* Always Visible Section */}
			<div
				className={`always-visible-section ${isMobile ? 'always-visible--mobile-stack' : ''}`}
				data-testid="always-visible-section"
			>
				<AlwaysVisibleSection
					phase={phase}
					template={template}
					agents={agents}
					agentsLoading={agentsLoading}
					readOnly={readOnly}
					fieldErrors={fieldErrors}
					setFieldErrors={setFieldErrors}
					savingFields={savingFields}
					autoSave={autoSave}
					isMobile={isMobile}
				/>
			</div>

			{/* Collapsible Sections */}
			<div className="collapsible-sections">
				{/* Sub-Agents Section */}
				<CollapsibleSection
					title="Sub-Agents"
					isOpen={sectionState.subAgents}
					onToggle={() => toggleSection('subAgents')}
					testId="sub-agents"
					isMobile={isMobile}
				>
					<SubAgentsSection
						phase={phase}
						agents={agents}
						agentsLoading={agentsLoading}
						readOnly={readOnly}
						fieldErrors={fieldErrors}
						savingFields={savingFields}
						autoSave={autoSave}
					/>
				</CollapsibleSection>

				{/* Prompt Section */}
				<CollapsibleSection
					title="Prompt"
					isOpen={sectionState.prompt}
					onToggle={() => toggleSection('prompt')}
					testId="prompt"
					isMobile={isMobile}
				>
					<PromptSection
						phase={phase}
						template={template}
						readOnly={isBuiltin}
						fieldErrors={fieldErrors}
					/>
				</CollapsibleSection>

				{/* Data Flow Section */}
				<CollapsibleSection
					title="Data Flow"
					isOpen={sectionState.dataFlow}
					onToggle={() => toggleSection('dataFlow')}
					testId="data-flow"
					isMobile={isMobile}
				>
					<DataFlowSection
						phase={phase}
						template={template}
						workflowDetails={workflowDetails}
						readOnly={readOnly}
						fieldErrors={fieldErrors}
						autoSave={autoSave}
					/>
				</CollapsibleSection>

				{/* Environment Section */}
				<CollapsibleSection
					title="Environment"
					isOpen={sectionState.environment}
					onToggle={() => toggleSection('environment')}
					testId="environment"
					isMobile={isMobile}
				>
					<EnvironmentSection
						phase={phase}
						hooks={hooks}
						skills={skills}
						mcpServers={mcpServers}
						hooksLoading={hooksLoading}
						skillsLoading={skillsLoading}
						mcpLoading={mcpLoading}
						readOnly={readOnly}
						fieldErrors={fieldErrors}
						autoSave={autoSave}
					/>
				</CollapsibleSection>

				{/* Advanced Section (positioned last) */}
				<CollapsibleSection
					title="Advanced"
					isOpen={sectionState.advanced}
					onToggle={() => toggleSection('advanced')}
					testId="advanced"
					isMobile={isMobile}
				>
					<AdvancedSection
						phase={phase}
						readOnly={readOnly}
						fieldErrors={fieldErrors}
						autoSave={autoSave}
						onDeletePhase={onDeletePhase}
					/>
				</CollapsibleSection>
			</div>
		</div>
	);
}

// ─── Always Visible Section ──────────────────────────────────────────────────

interface AlwaysVisibleSectionProps {
	phase: WorkflowPhase;
	template: PhaseTemplate;
	agents: Agent[];
	agentsLoading: boolean;
	readOnly: boolean;
	fieldErrors: FieldErrors;
	setFieldErrors: React.Dispatch<React.SetStateAction<FieldErrors>>;
	savingFields: Set<string>;
	autoSave: (field: string, value: unknown, immediate?: boolean) => void;
	isMobile: boolean;
}

function AlwaysVisibleSection({
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
}: AlwaysVisibleSectionProps) {
	const [phaseName, setPhaseName] = useState(template.name || '');
	const [agentOverride, setAgentOverride] = useState(phase.agentOverride || '');
	const [modelOverride, setModelOverride] = useState(phase.modelOverride || '');

	// Reset local state when phase changes
	useEffect(() => {
		setPhaseName(template.name || '');
		setAgentOverride(phase.agentOverride || '');
		setModelOverride(phase.modelOverride || '');
	}, [phase.id, template, phase.agentOverride, phase.modelOverride]);

	// Validation helpers
	const validatePhaseName = (name: string): FieldError | null => {
		if (!name.trim()) {
			return { message: 'Name cannot be empty', type: 'validation' };
		}
		return null;
	};

	// Handle field changes with validation
	const handlePhaseNameChange = (value: string) => {
		setPhaseName(value);
		const error = validatePhaseName(value);
		if (!error) {
			// Only auto-save if validation passes
			autoSave('templateName', value);
		}
	};

	const handlePhaseNameBlur = () => {
		const error = validatePhaseName(phaseName);
		if (error) {
			// Set error state before reverting (so error message appears)
			setFieldErrors(prev => ({ ...prev, templateName: error }));
			// Revert to original value after a brief delay to allow error display
			setTimeout(() => {
				setPhaseName(template.name || '');
				// Clear error after revert
				setTimeout(() => {
					setFieldErrors(prev => ({ ...prev, templateName: null }));
				}, 100);
			}, 50);
		} else {
			setFieldErrors(prev => ({ ...prev, templateName: null }));
			autoSave('templateName', phaseName, true); // immediate save on blur
		}
	};

	const handleAgentChange = (value: string) => {
		setAgentOverride(value);
		autoSave('agentOverride', value || undefined);
	};

	const handleModelChange = (value: string) => {
		setModelOverride(value);
		autoSave('modelOverride', value || undefined);
	};

	const nameError = fieldErrors.templateName || validatePhaseName(phaseName);

	return (
		<div className={`always-visible-fields ${isMobile ? 'always-visible--mobile-stack' : ''}`}>
			{/* Phase Name */}
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
				{nameError && (
					<span className="field-error">{nameError.message}</span>
				)}
				{savingFields.has('templateName') && (
					<span className="field-saving">Saving...</span>
				)}
			</div>

			{/* Executor */}
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
				{savingFields.has('agentOverride') && (
					<span className="field-saving">Saving...</span>
				)}
				{fieldErrors.agentOverride && (
					<span className="field-error">{fieldErrors.agentOverride.message}</span>
				)}
			</div>

			{/* Model */}
			<div className="field-group">
				<label htmlFor="phase-model" className="field-label">
					Model
				</label>
				<select
					id="phase-model"
					aria-label="Model"
					value={modelOverride}
					onChange={(e) => handleModelChange(e.target.value)}
					disabled={readOnly || savingFields.has('modelOverride')}
					className={`field-input ${isMobile ? 'touch-friendly' : ''}`}
				>
					<option value="">Inherit from workflow</option>
					<option value="sonnet">Sonnet</option>
					<option value="opus">Opus</option>
					<option value="haiku">Haiku</option>
				</select>
				{savingFields.has('modelOverride') && (
					<span className="field-saving">Saving...</span>
				)}
				{fieldErrors.modelOverride && (
					<span className="field-error">{fieldErrors.modelOverride.message}</span>
				)}
			</div>
		</div>
	);
}

// ─── Collapsible Section Component ───────────────────────────────────────────

interface CollapsibleSectionProps {
	title: string;
	isOpen: boolean;
	onToggle: () => void;
	testId: string;
	isMobile: boolean;
	children: React.ReactNode;
}

function CollapsibleSection({
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
			{/* Conditionally render content to unmount when closed (tests expect this) */}
			{isOpen && (
				<div
					className="collapsible-content"
					data-testid={`${testId}-content`}
				>
					{children}
				</div>
			)}
		</Collapsible.Root>
	);
}

// ─── Sub-Agents Section ──────────────────────────────────────────────────────

interface SubAgentsSectionProps {
	phase: WorkflowPhase;
	agents: Agent[];
	agentsLoading: boolean;
	readOnly: boolean;
	fieldErrors: FieldErrors;
	savingFields: Set<string>;
	autoSave: (field: string, value: unknown, immediate?: boolean) => void;
}

function SubAgentsSection({
	phase,
	agents,
	agentsLoading,
	readOnly,
	fieldErrors,
	savingFields,
	autoSave,
}: SubAgentsSectionProps) {
	const [subAgentsOverride, setSubAgentsOverride] = useState<string[]>(
		phase.subAgentsOverride ?? []
	);
	const [draggedAgent, setDraggedAgent] = useState<string | null>(null);

	useEffect(() => {
		setSubAgentsOverride(phase.subAgentsOverride ?? []);
	}, [phase.id, phase.subAgentsOverride]);

	const handleAddAgent = (agentName: string) => {
		const newSubAgents = [...subAgentsOverride, agentName];
		setSubAgentsOverride(newSubAgents);
		autoSave('subAgentsOverride', newSubAgents);
	};

	const handleRemoveAgent = (agentName: string) => {
		const newSubAgents = subAgentsOverride.filter(name => name !== agentName);
		setSubAgentsOverride(newSubAgents);
		autoSave('subAgentsOverride', newSubAgents);
	};

	const handleDragStart = (agentName: string) => {
		setDraggedAgent(agentName);
	};

	const handleDragOver = (e: React.DragEvent<HTMLDivElement>) => {
		e.preventDefault();
	};

	const handleDrop = (targetAgentName: string) => {
		if (!draggedAgent || draggedAgent === targetAgentName) return;

		const currentOrder = [...subAgentsOverride];
		const draggedIndex = currentOrder.indexOf(draggedAgent);
		const targetIndex = currentOrder.indexOf(targetAgentName);

		if (draggedIndex === -1 || targetIndex === -1) return;

		// Remove dragged item and insert at target position
		currentOrder.splice(draggedIndex, 1);
		currentOrder.splice(targetIndex, 0, draggedAgent);

		setSubAgentsOverride(currentOrder);
		autoSave('subAgentsOverride', currentOrder);
	};

	const handleDragEnd = () => {
		setDraggedAgent(null);
	};

	if (agentsLoading) {
		return <span className="field-loading">Loading agents...</span>;
	}

	if (agents.length === 0) {
		return <span className="field-error">No agents available</span>;
	}

	const assignedAgents = subAgentsOverride.filter(name =>
		agents.some(agent => agent.name === name)
	);
	const availableAgents = agents.filter(agent =>
		!subAgentsOverride.includes(agent.name)
	);

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
							onDragStart={() => handleDragStart(agentName)}
							onDragOver={handleDragOver}
							onDrop={() => handleDrop(agentName)}
							onDragEnd={handleDragEnd}
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
								e.target.value = ''; // Reset selection
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

			{fieldErrors.subAgentsOverride && (
				<span className="field-error">{fieldErrors.subAgentsOverride.message}</span>
			)}
			{savingFields.has('subAgentsOverride') && (
				<span className="field-saving">Saving...</span>
			)}
		</div>
	);
}

// ─── Prompt Section ──────────────────────────────────────────────────────────

interface PromptSectionProps {
	phase: WorkflowPhase;
	template: PhaseTemplate;
	readOnly: boolean;
	fieldErrors: FieldErrors;
}

function PromptSection({ phase: _phase, template, readOnly, fieldErrors: _fieldErrors }: PromptSectionProps) {
	const [promptSource, setPromptSource] = useState<PromptSource>(
		template.promptSource || PromptSource.EMBEDDED
	);
	const [filePath, setFilePath] = useState('');

	const handleSourceChange = (source: PromptSource) => {
		setPromptSource(source);
	};

	const validateFilePath = (path: string): FieldError | null => {
		if (path && !path.match(/\.(md|txt)$/i)) {
			return { message: 'Invalid file path - must end in .md or .txt', type: 'validation' };
		}
		return null;
	};

	const filePathError = validateFilePath(filePath);

	return (
		<div className="prompt-section">
			{/* Source Toggle */}
			<div className="prompt-source-toggle">
				<button
					type="button"
					className={`source-button ${promptSource === PromptSource.EMBEDDED ? 'active' : ''}`}
					onClick={() => handleSourceChange(PromptSource.EMBEDDED)}
					aria-pressed={promptSource === PromptSource.EMBEDDED}
				>
					Template
				</button>
				<button
					type="button"
					className={`source-button ${promptSource === PromptSource.DB ? 'active' : ''}`}
					onClick={() => handleSourceChange(PromptSource.DB)}
					aria-pressed={promptSource === PromptSource.DB}
				>
					Custom
				</button>
				<button
					type="button"
					className={`source-button ${promptSource === PromptSource.FILE ? 'active' : ''}`}
					onClick={() => handleSourceChange(PromptSource.FILE)}
					aria-pressed={promptSource === PromptSource.FILE}
				>
					File
				</button>
			</div>

			{/* Content based on source */}
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
					{_fieldErrors.promptContent && (
						<span className="field-error">Failed to load prompt content</span>
					)}
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
					{filePathError && (
						<span className="field-error">{filePathError.message}</span>
					)}
				</div>
			)}
		</div>
	);
}

// ─── Data Flow Section ───────────────────────────────────────────────────────

interface DataFlowSectionProps {
	phase: WorkflowPhase;
	template: PhaseTemplate;
	workflowDetails: WorkflowWithDetails;
	readOnly: boolean;
	fieldErrors: FieldErrors;
	autoSave: (field: string, value: unknown, immediate?: boolean) => void;
}

function DataFlowSection({
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

	const handleProducesArtifactChange = (checked: boolean) => {
		setProducesArtifact(checked);
		autoSave('producesArtifact', checked);
	};

	const handleArtifactTypeChange = (type: string) => {
		setArtifactType(type);
		autoSave('artifactType', type);
	};

	const handleOutputVariableChange = (variable: string) => {
		setOutputVariable(variable);
		autoSave('outputVariable', variable);
	};

	return (
		<div className="data-flow-section">
			{/* Input Variables */}
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
									{varDef?.description && (
										<p className="variable-description">{varDef.description}</p>
									)}
								</li>
							);
						})}
					</ul>
				)}
			</div>

			{/* Output Variable */}
			<div className="output-variable">
				<label htmlFor="output-variable" className="field-label">
					Output Variable
				</label>
				<input
					id="output-variable"
					type="text"
					value={outputVariable}
					onChange={(e) => handleOutputVariableChange(e.target.value)}
					disabled={readOnly}
					className="field-input"
					placeholder="Variable name to store output"
				/>
			</div>

			{/* Artifact Production */}
			<div className="artifact-section">
				<label className="checkbox-label">
					<input
						type="checkbox"
						checked={producesArtifact}
						onChange={(e) => handleProducesArtifactChange(e.target.checked)}
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
							onChange={(e) => handleArtifactTypeChange(e.target.value)}
							disabled={readOnly}
							className="field-input"
						>
							<option value="spec">spec</option>
							<option value="tests">tests</option>
							<option value="docs">docs</option>
							<option value="code">code</option>
						</select>
						{fieldErrors.artifactType && (
							<span className="field-error">Failed to load artifact types</span>
						)}
					</div>
				)}
			</div>
		</div>
	);
}

// ─── Environment Section ─────────────────────────────────────────────────────

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
	autoSave: (field: string, value: unknown, immediate?: boolean) => void;
}

function EnvironmentSection({
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

	// Parse current claudeConfigOverride to get selections
	const currentConfig = useMemo(
		() => parseClaudeConfig(phase.claudeConfigOverride),
		[phase.claudeConfigOverride]
	);

	const [selectedMCPServers, setSelectedMCPServers] = useState<string[]>(currentConfig.mcpServers);
	const [selectedSkills, setSelectedSkills] = useState<string[]>(currentConfig.skillRefs);
	const [selectedHooks, setSelectedHooks] = useState<string[]>(currentConfig.hooks);

	// Update selections when phase changes
	useEffect(() => {
		const config = parseClaudeConfig(phase.claudeConfigOverride);
		setSelectedMCPServers(config.mcpServers);
		setSelectedSkills(config.skillRefs);
		setSelectedHooks(config.hooks);
	}, [phase.id, phase.claudeConfigOverride]);

	const handleWorkingDirectoryChange = (directory: string) => {
		setWorkingDirectory(directory);
		autoSave('workingDirectory', directory);
	};

	const handleEnvVarsChange = (vars: Record<string, string>) => {
		setEnvVars(vars);
		autoSave('envVars', vars);
	};

	// Helper to save updated config
	const saveConfigUpdate = useCallback(
		(update: Partial<{ mcpServers: string[]; skillRefs: string[]; hooks: string[] }>) => {
			const newConfig = serializeClaudeConfig({
				hooks: update.hooks ?? selectedHooks,
				skillRefs: update.skillRefs ?? selectedSkills,
				mcpServers: update.mcpServers ?? selectedMCPServers,
				allowedTools: currentConfig.allowedTools,
				disallowedTools: currentConfig.disallowedTools,
				env: currentConfig.env,
				extra: currentConfig.extra,
			});
			autoSave('claudeConfigOverride', newConfig);
		},
		[selectedHooks, selectedSkills, selectedMCPServers, currentConfig, autoSave]
	);

	const handleMCPServersChange = (names: string[]) => {
		setSelectedMCPServers(names);
		saveConfigUpdate({ mcpServers: names });
	};

	const handleSkillsChange = (names: string[]) => {
		setSelectedSkills(names);
		saveConfigUpdate({ skillRefs: names });
	};

	const handleHooksChange = (names: string[]) => {
		setSelectedHooks(names);
		saveConfigUpdate({ hooks: names });
	};

	const isLoading = hooksLoading || skillsLoading || mcpLoading;
	const hasNoData = hooks.length === 0 && skills.length === 0 && mcpServers.length === 0;

	return (
		<div className="environment-section">
			{/* Working Directory */}
			<div className="working-directory">
				<label htmlFor="working-directory" className="field-label">
					Working Directory
				</label>
				<select
					id="working-directory"
					value={workingDirectory}
					onChange={(e) => handleWorkingDirectoryChange(e.target.value)}
					disabled={readOnly}
					className="field-input"
				>
					<option value="inherit">Inherit from workflow</option>
					<option value="project-root">Project Root</option>
					<option value="task-specific">Task-specific</option>
				</select>
			</div>

			{/* Environment Variables */}
			<div className="env-vars">
				<h4 className="section-title">Environment Variables</h4>
				<KeyValueEditor
					entries={envVars}
					onChange={handleEnvVarsChange}
					disabled={readOnly}
				/>
			</div>

			{/* MCP Servers, Skills, Hooks */}
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
								onSelectionChange={handleMCPServersChange}
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
								onSelectionChange={handleSkillsChange}
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
								onSelectionChange={handleHooksChange}
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

// ─── Advanced Section ────────────────────────────────────────────────────────

interface AdvancedSectionProps {
	phase: WorkflowPhase;
	readOnly: boolean;
	fieldErrors: FieldErrors;
	autoSave: (field: string, value: unknown, immediate?: boolean) => void;
	onDeletePhase?: () => void;
}

function AdvancedSection({
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

	const handleThinkingChange = (checked: boolean) => {
		setThinkingOverride(checked);
		autoSave('thinkingOverride', checked);
	};

	return (
		<div className="advanced-section">
			{/* Thinking Override */}
			<div className="thinking-override">
				<label className="checkbox-label">
					<input
						type="checkbox"
						checked={thinkingOverride}
						onChange={(e) => handleThinkingChange(e.target.checked)}
						disabled={readOnly}
						aria-label="Thinking override"
					/>
					<span>Enable thinking override</span>
				</label>
			</div>

			{/* Delete Phase */}
			{!readOnly && onDeletePhase && (
				<div className="danger-zone">
					<button
						type="button"
						onClick={onDeletePhase}
						className="delete-button"
					>
						Remove Phase
					</button>
				</div>
			)}
		</div>
	);
}

// ─── Legacy Components (keeping for backwards compatibility) ─────────────────

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

export function SettingsTab({
	phase,
	workflowDetails,
	readOnly,
	error,
	onError,
	onWorkflowRefresh,
	onDeletePhase,
}: SettingsTabProps) {
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

	// Claude config draft — updated by ClaudeConfigEditor, saved with the rest
	const [claudeConfigDraft, setClaudeConfigDraft] = useState<string | null>(null);
	const [saving, setSaving] = useState(false);

	// Condition state — tracks pending condition changes
	const [conditionDraft, setConditionDraft] = useState<string | undefined>(undefined);
	const [conditionDirty, setConditionDirty] = useState(false);

	// Loop config state — tracks pending loop configuration changes
	const [loopConfigDraft, setLoopConfigDraft] = useState<string | undefined>(undefined);
	const [loopConfigDirty, setLoopConfigDirty] = useState(false);

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

	// Reset state when phase changes (e.g. after save + refresh, or selecting a different node)
	useEffect(() => {
		setModelOverride(phase.modelOverride ?? '');
		setThinkingOverride(phase.thinkingOverride ?? false);
		setGateTypeOverride(phase.gateTypeOverride ?? GateType.UNSPECIFIED);
		setAgentOverride(phase.agentOverride ?? '');
		setSubAgentsOverride(phase.subAgentsOverride ?? []);
		setClaudeConfigDraft(null);
		setConditionDraft(phase.condition);
		setConditionDirty(false);
		setLoopConfigDraft(phase.loopConfig);
		setLoopConfigDirty(false);
		onError(null);
	}, [phase, onError]);

	// Dirty detection — compare local state vs committed phase values
	const isDirty = useMemo(() => {
		if (modelOverride !== (phase.modelOverride ?? '')) return true;
		if (thinkingOverride !== (phase.thinkingOverride ?? false)) return true;
		if (gateTypeOverride !== (phase.gateTypeOverride ?? GateType.UNSPECIFIED)) return true;
		if (agentOverride !== (phase.agentOverride ?? '')) return true;
		const origSorted = [...(phase.subAgentsOverride ?? [])].sort();
		const currSorted = [...subAgentsOverride].sort();
		if (JSON.stringify(currSorted) !== JSON.stringify(origSorted)) return true;
		if (claudeConfigDraft !== null) return true;
		if (conditionDirty) return true;
		if (loopConfigDirty) return true;
		return false;
	}, [modelOverride, thinkingOverride, gateTypeOverride, agentOverride, subAgentsOverride, claudeConfigDraft, conditionDirty, loopConfigDirty, phase]);

	// Save all pending changes in one API call
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
				...(claudeConfigDraft !== null ? { claudeConfigOverride: claudeConfigDraft || undefined } : {}),
				...(conditionDirty ? { condition: conditionDraft || '' } : {}),
				...(loopConfigDirty ? { loopConfig: loopConfigDraft || '' } : {}),
			});
			setClaudeConfigDraft(null);
			setConditionDirty(false);
			setLoopConfigDirty(false);
			onWorkflowRefresh?.();
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Update failed';
			onError(message);
		} finally {
			setSaving(false);
		}
	}, [workflowDetails, phase.id, modelOverride, thinkingOverride, gateTypeOverride, agentOverride, subAgentsOverride, claudeConfigDraft, conditionDirty, conditionDraft, loopConfigDirty, loopConfigDraft, onError, onWorkflowRefresh]);

	// Discard all pending changes
	const handleDiscard = useCallback(() => {
		setModelOverride(phase.modelOverride ?? '');
		setThinkingOverride(phase.thinkingOverride ?? false);
		setGateTypeOverride(phase.gateTypeOverride ?? GateType.UNSPECIFIED);
		setAgentOverride(phase.agentOverride ?? '');
		setSubAgentsOverride(phase.subAgentsOverride ?? []);
		setClaudeConfigDraft(null);
		setConditionDraft(phase.condition);
		setConditionDirty(false);
		setLoopConfigDraft(phase.loopConfig);
		setLoopConfigDirty(false);
		onError(null);
	}, [phase, onError]);

	const handleSubAgentToggle = (agentName: string, checked: boolean) => {
		setSubAgentsOverride((prev) =>
			checked ? [...prev, agentName] : prev.filter((a) => a !== agentName),
		);
	};

	// Handle condition changes from ConditionEditor
	const handleConditionChange = useCallback((newCondition: string) => {
		setConditionDraft(newCondition || undefined);
		setConditionDirty(true);
	}, []);

	// Handle loop config changes from LoopEditor
	const handleLoopConfigChange = useCallback((newLoopConfig: string) => {
		setLoopConfigDraft(newLoopConfig || undefined);
		setLoopConfigDirty(true);
	}, []);

	// Compute prior phases for loop target selection
	const priorPhases = useMemo(() => {
		const phases = workflowDetails.phases ?? [];
		const currentSequence = phase.sequence;
		return phases
			.filter((p) => p.sequence < currentSequence)
			.map((p) => p.phaseTemplateId);
	}, [workflowDetails.phases, phase.sequence]);

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

			{/* Save / Discard bar */}
			{!readOnly && isDirty && (
				<div className="phase-inspector-save-bar">
					<button
						type="button"
						className="phase-inspector-save-btn"
						onClick={handleSave}
						disabled={saving}
					>
						{saving ? 'Saving...' : 'Save Changes'}
					</button>
					<button
						type="button"
						className="phase-inspector-discard-btn"
						onClick={handleDiscard}
						disabled={saving}
					>
						Discard
					</button>
				</div>
			)}

			<div className="phase-inspector-setting">
				<label htmlFor="inspector-model" className="phase-inspector-setting-label">
					Model
				</label>
				<select
					id="inspector-model"
					className="phase-inspector-setting-select"
					value={modelOverride}
					onChange={(e) => setModelOverride(e.target.value)}
					disabled={disabled}
				>
					<option value="">Inherit from workflow</option>
					<option value="sonnet">Sonnet</option>
					<option value="opus">Opus</option>
					<option value="haiku">Haiku</option>
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
					onChange={(e) => setThinkingOverride(e.target.checked)}
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
					onChange={(e) => setGateTypeOverride(Number(e.target.value) as GateType)}
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

			{/* Executor */}
			<div className="phase-inspector-setting">
				<label htmlFor="inspector-agent" className="phase-inspector-setting-label">
					Executor
				</label>
				<select
					id="inspector-agent"
					className="phase-inspector-setting-select"
					value={agentOverride}
					onChange={(e) => setAgentOverride(e.target.value)}
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

			{/* Condition — conditional phase execution */}
			<CollapsibleSettingsSection title="Condition" badgeCount={conditionDraft || phase.condition ? 1 : 0} defaultExpanded>
				<ConditionEditor
					condition={(conditionDirty ? conditionDraft : phase.condition) || ''}
					onChange={handleConditionChange}
					disabled={readOnly}
				/>
			</CollapsibleSettingsSection>

			{/* Loop — loop back to earlier phase when condition is met */}
			<CollapsibleSettingsSection title="Loop" badgeCount={loopConfigDraft || phase.loopConfig ? 1 : 0}>
				<LoopEditor
					loopConfig={(loopConfigDirty ? loopConfigDraft : phase.loopConfig) || ''}
					onChange={handleLoopConfigChange}
					priorPhases={priorPhases}
					disabled={readOnly}
				/>
			</CollapsibleSettingsSection>

			{/* Claude Config Override (editable) — changes accumulate in claudeConfigDraft */}
			<ClaudeConfigEditor
				phase={phase}
				disabled={readOnly}
				onSave={setClaudeConfigDraft}
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
				<InheritedChips items={inheritedCount?.hooks} />
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
				<InheritedChips items={inheritedCount?.mcpServers} />
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
				<InheritedChips items={inheritedCount?.skillRefs} />
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
				<InheritedChips items={inheritedCount?.allowedTools} />
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
				<InheritedChips items={inheritedCount?.disallowedTools} />
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
				<InheritedChips items={inheritedCount?.env ? Object.keys(inheritedCount.env) : undefined} label="env vars" />
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

/** Read-only chips showing items inherited from the phase template's claude_config. */
function InheritedChips({ items }: { items?: string[]; label?: string }) {
	if (!items || items.length === 0) return null;
	return (
		<div className="inherited-chips">
			<span className="inherited-chips__label">From template:</span>
			{items.map((item) => (
				<span key={item} className="inherited-chips__chip">{item}</span>
			))}
		</div>
	);
}
