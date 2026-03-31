import { useState, useEffect, useCallback, useRef } from 'react';
import { workflowClient } from '@/lib/client';
import type { WorkflowPhase, WorkflowWithDetails } from '@/gen/orc/v1/workflow_pb';
import type { FieldErrors, SectionState } from './phase-inspector/shared';
import { useMobileViewport } from './phase-inspector/hooks';
import { useLibraryData } from '@/hooks/useLibraryData';
import {
	AlwaysVisibleSection,
	CollapsibleSection,
	SubAgentsSection,
	PromptSection,
	DataFlowSection,
	EnvironmentSection,
	AdvancedSection,
} from './phase-inspector/sections';
import './PhaseInspector.css';

interface PhaseInspectorProps {
	phase: WorkflowPhase | null;
	workflowDetails: WorkflowWithDetails | null;
	readOnly: boolean;
	onWorkflowRefresh?: () => void;
	onDeletePhase?: () => void;
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
	const debounceTimeoutRef = useRef<number | null>(null);

	const {
		agents,
		hooks,
		skills,
		mcpServers,
		agentsLoading,
		hooksLoading,
		skillsLoading,
		mcpLoading,
	} = useLibraryData();

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

			// Clear errors for new phase
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

					onWorkflowRefresh?.();
				} catch (error) {
					const errorMessage = error instanceof Error ? error.message : 'Save failed';

					// Set error and revert field value
					setFieldErrors(prev => ({
						...prev,
						[fieldName]: { message: errorMessage, type: 'save' }
					}));

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
					workflowDefaultProvider={workflowDetails.workflow?.defaultProvider ?? ''}
					workflowDetails={workflowDetails}
					onWorkflowRefresh={onWorkflowRefresh}
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
						autoSave={autoSave}
						onDeletePhase={onDeletePhase}
					/>
				</CollapsibleSection>
			</div>
		</div>
	);
}
