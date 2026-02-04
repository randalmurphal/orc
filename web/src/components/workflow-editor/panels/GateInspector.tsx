import type { Edge } from '@xyflow/react';
import { useState, useCallback, useMemo } from 'react';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import type { WorkflowWithDetails } from '@/gen/orc/v1/workflow_pb';
import type { GateEdgeData } from '../utils/layoutWorkflow';
import { workflowClient } from '@/lib/client';
import './GateInspector.css';

// Enhanced gate configuration data structure
interface GateConfigData extends Record<string, unknown> {
	// Base gate fields (matching GateEdgeData)
	gateType: GateType;
	gateStatus?: 'pending' | 'passed' | 'blocked' | 'failed';
	phaseId?: number;
	position?: 'entry' | 'exit' | 'between';
	maxRetries?: number;
	failureAction?: 'retry' | 'retry_from' | 'fail' | 'pause';

	// Enhanced configuration fields
	// Auto gate configuration
	autoCriteria?: {
		hasOutput?: boolean;
		noErrors?: boolean;
		completionMarker?: boolean;
		customPattern?: string;
	};

	// Human gate configuration
	humanConfig?: {
		reviewPrompt?: string;
	};

	// AI gate configuration
	aiConfig?: {
		reviewerAgentId?: string;
		contextSources?: ('phase_outputs' | 'task_details' | 'vars')[];
	};

	// Failure handling
	retryFromPhaseId?: number;

	// Advanced configuration
	advancedConfig?: {
		beforeScript?: string;
		afterScript?: string;
		storeResultAs?: string;
	};
}

interface GateInspectorProps {
	edge: Edge<GateEdgeData> | null | undefined;
	workflowDetails: WorkflowWithDetails | null;
	readOnly: boolean;
}

/**
 * Get position label for gate edge
 */
function getPositionLabel(position: GateEdgeData['position']): string {
	switch (position) {
		case 'entry':
			return 'Entry Gate';
		case 'exit':
			return 'Exit Gate';
		case 'between':
			return 'Gate';
		default:
			return 'Gate';
	}
}

/**
 * Enhanced GateInspector - Panel for inspecting and editing gate configurations.
 *
 * Features:
 * - Gate type selector (Auto, Human, AI, Skip) with edit functionality
 * - Type-specific configuration sections (Auto criteria, Human prompts, AI agents)
 * - Failure handling with retry options
 * - Collapsible advanced settings
 * - API integration for saving changes
 * - Read-only mode for built-in workflows
 */
export function GateInspector({
	edge,
	workflowDetails,
	readOnly,
}: GateInspectorProps) {
	// State for managing configuration changes
	const [isLoading, setIsLoading] = useState(false);
	const [isAdvancedExpanded, setIsAdvancedExpanded] = useState(false);
	const [localGateType, setLocalGateType] = useState<GateType | null>(null);
	const [localMaxRetries, setLocalMaxRetries] = useState<number | null>(null);
	const [localFailureAction, setLocalFailureAction] = useState<GateConfigData['failureAction'] | null>(null);
	const [localConfig, setLocalConfig] = useState<Partial<GateConfigData>>({});

	// Extract edge data safely
	const edgeData = edge?.data as GateEdgeData | undefined;
	const gateType = localGateType ?? edgeData?.gateType ?? GateType.AUTO;
	const gateStatus = edgeData?.gateStatus;
	// Use local state if set (including 0 for "cleared" state), otherwise fall back to edge data or default
	const maxRetries = localMaxRetries !== null ? localMaxRetries : (edgeData?.maxRetries ?? 3);
	const failureAction = localFailureAction ?? edgeData?.failureAction ?? 'retry';
	const phaseId = edgeData?.phaseId;

	// Configuration data with local state overrides - memoized for performance
	const enhancedData = edgeData as GateConfigData | undefined;
	const autoCriteria = useMemo(() =>
		({ ...(enhancedData?.autoCriteria ?? {}), ...(localConfig.autoCriteria ?? {}) }),
		[enhancedData?.autoCriteria, localConfig.autoCriteria]
	);
	const humanConfig = useMemo(() =>
		({ ...(enhancedData?.humanConfig ?? {}), ...(localConfig.humanConfig ?? {}) }),
		[enhancedData?.humanConfig, localConfig.humanConfig]
	);
	const aiConfig = useMemo(() =>
		({ ...(enhancedData?.aiConfig ?? {}), ...(localConfig.aiConfig ?? {}) }),
		[enhancedData?.aiConfig, localConfig.aiConfig]
	);
	const advancedConfig = useMemo(() =>
		({ ...(enhancedData?.advancedConfig ?? {}), ...(localConfig.advancedConfig ?? {}) }),
		[enhancedData?.advancedConfig, localConfig.advancedConfig]
	);
	const retryFromPhaseId = localConfig.retryFromPhaseId ?? enhancedData?.retryFromPhaseId;

	// API call to save configuration changes
	const saveConfiguration = useCallback(async (updates: Partial<GateConfigData>) => {
		if (!phaseId || readOnly) return;

		setIsLoading(true);
		try {
			await workflowClient.updatePhaseTemplate({
				id: phaseId.toString(),
				gateType: updates.gateType ?? gateType,
				maxIterations: updates.maxRetries ?? maxRetries,
				...(updates.autoCriteria !== undefined && { autoCriteria: updates.autoCriteria }),
				...(updates.humanConfig !== undefined && { humanConfig: updates.humanConfig }),
				...(updates.aiConfig !== undefined && { aiConfig: updates.aiConfig }),
				...(updates.failureAction !== undefined && { failureAction: updates.failureAction }),
				...(updates.advancedConfig !== undefined && { advancedConfig: updates.advancedConfig }),
				...(updates.retryFromPhaseId !== undefined && { retryFromPhaseId: updates.retryFromPhaseId }),
			});
		} catch (_error) {
			// Only log in development/non-test environments
			if (process.env.NODE_ENV !== 'test') {
				console.error('Failed to save gate configuration:', _error);
			}
			// Re-throw to allow caller to handle (e.g., revert state)
			throw _error;
		} finally {
			setIsLoading(false);
		}
	}, [phaseId, readOnly, gateType, maxRetries]);

	// Event handlers
	const handleGateTypeChange = useCallback(async (event: React.ChangeEvent<HTMLSelectElement>) => {
		const newGateType = parseInt(event.target.value) as GateType;
		// Update local state immediately for UI responsiveness
		setLocalGateType(newGateType);
		try {
			await saveConfiguration({ gateType: newGateType });
		} catch (_error) {
			// Revert local state on API failure
			setLocalGateType(null);
		}
	}, [saveConfiguration]);

	const handleMaxRetriesChange = useCallback(async (event: React.ChangeEvent<HTMLInputElement>) => {
		const rawValue = event.target.value;
		const parsedValue = parseInt(rawValue);
		// Allow empty input during editing (user clearing the field)
		// Store the parsed value if valid, otherwise treat empty as "clearing in progress"
		const newMaxRetries = isNaN(parsedValue) ? 0 : parsedValue;
		// Update local state immediately for UI responsiveness
		setLocalMaxRetries(newMaxRetries);
		// Only save if we have a valid non-zero value
		if (newMaxRetries > 0) {
			try {
				await saveConfiguration({ maxRetries: newMaxRetries });
			} catch (_error) {
				// Revert local state on API failure
				setLocalMaxRetries(null);
			}
		}
	}, [saveConfiguration]);

	const handleFailureActionChange = useCallback(async (event: React.ChangeEvent<HTMLSelectElement>) => {
		const newFailureAction = event.target.value as GateConfigData['failureAction'];
		// Update local state immediately for UI responsiveness
		setLocalFailureAction(newFailureAction);
		try {
			await saveConfiguration({ failureAction: newFailureAction });
		} catch (_error) {
			// Revert local state on API failure
			setLocalFailureAction(null);
		}
	}, [saveConfiguration]);

	const handleRetryFromChange = useCallback(async (event: React.ChangeEvent<HTMLSelectElement>) => {
		const newRetryFromPhaseId = parseInt(event.target.value) || undefined;
		await saveConfiguration({ retryFromPhaseId: newRetryFromPhaseId });
	}, [saveConfiguration]);

	// Auto criteria handlers
	const handleAutoCriteriaChange = useCallback((key: keyof NonNullable<GateConfigData['autoCriteria']>, value: boolean | string) => {
		const newAutoCriteria = { ...autoCriteria, [key]: value };
		setLocalConfig(prev => ({ ...prev, autoCriteria: newAutoCriteria }));
		saveConfiguration({ autoCriteria: newAutoCriteria });
	}, [saveConfiguration, autoCriteria]);

	// Human config handlers
	const handleHumanConfigChange = useCallback((key: keyof NonNullable<GateConfigData['humanConfig']>, value: string) => {
		const newHumanConfig = { ...humanConfig, [key]: value };
		setLocalConfig(prev => ({ ...prev, humanConfig: newHumanConfig }));
		saveConfiguration({ humanConfig: newHumanConfig });
	}, [saveConfiguration, humanConfig]);

	// AI config handlers
	const handleAIConfigChange = useCallback((key: keyof NonNullable<GateConfigData['aiConfig']>, value: string | string[]) => {
		const newAIConfig = { ...aiConfig, [key]: value };
		setLocalConfig(prev => ({ ...prev, aiConfig: newAIConfig }));
		saveConfiguration({ aiConfig: newAIConfig });
	}, [saveConfiguration, aiConfig]);

	// Advanced config handlers
	const handleAdvancedConfigChange = useCallback((key: keyof NonNullable<GateConfigData['advancedConfig']>, value: string) => {
		const newAdvancedConfig = { ...advancedConfig, [key]: value };
		setLocalConfig(prev => ({ ...prev, advancedConfig: newAdvancedConfig }));
		saveConfiguration({ advancedConfig: newAdvancedConfig });
	}, [saveConfiguration, advancedConfig]);

	// Return nothing if no edge is selected (after all hooks)
	if (!edge || !edge.data) {
		return null;
	}

	// Extract position and build UI state after early return
	const position = edgeData?.position ?? 'between';

	// Find the target phase name for between gates
	const targetPhase = phaseId
		? workflowDetails?.phases?.find((p) => p.id === phaseId)
		: null;
	const targetPhaseName = targetPhase?.template?.name ?? targetPhase?.phaseTemplateId;

	// Build header text based on position
	let headerText: string;
	if (position === 'entry') {
		headerText = 'Entry Gate';
	} else if (position === 'exit') {
		headerText = 'Exit Gate';
	} else if (targetPhaseName) {
		headerText = `Gate → ${targetPhaseName}`;
	} else {
		headerText = getPositionLabel(position);
	}

	// Get status CSS class
	const statusClass = gateStatus ? `gate-inspector__status--${gateStatus}` : '';

	return (
		<div className="gate-inspector">
			<div className="gate-inspector__header">
				<h3>{headerText}</h3>
			</div>

			{readOnly && (
				<div className="gate-inspector__readonly-notice">
					Clone to customize
				</div>
			)}

			<div className="gate-inspector__settings">
				{/* Gate Type */}
				<div className="gate-inspector__field">
					<label className="gate-inspector__label" htmlFor="gate-type">Gate Type</label>
					<select
						id="gate-type"
						className="gate-inspector__select"
						value={gateType}
						disabled={readOnly || isLoading}
						onChange={handleGateTypeChange}
					>
						<option value={GateType.AUTO}>Auto</option>
						<option value={GateType.HUMAN}>Human</option>
						<option value={GateType.AI}>AI</option>
						<option value={GateType.SKIP}>Skip</option>
					</select>
				</div>

				{/* Auto Gate Configuration */}
				{gateType === GateType.AUTO && (
					<div className="gate-inspector__section">
						<h4>Auto Configuration</h4>
						<div className="gate-inspector__field">
							<label>
								<input
									type="checkbox"
									checked={autoCriteria.hasOutput || false}
									disabled={readOnly}
									onChange={(e) => handleAutoCriteriaChange('hasOutput', e.target.checked)}
								/>
								Has Output
							</label>
						</div>
						<div className="gate-inspector__field">
							<label>
								<input
									type="checkbox"
									checked={autoCriteria.noErrors || false}
									disabled={readOnly}
									onChange={(e) => handleAutoCriteriaChange('noErrors', e.target.checked)}
								/>
								No Errors
							</label>
						</div>
						<div className="gate-inspector__field">
							<label>
								<input
									type="checkbox"
									checked={autoCriteria.completionMarker || false}
									disabled={readOnly}
									onChange={(e) => handleAutoCriteriaChange('completionMarker', e.target.checked)}
								/>
								Completion Marker
							</label>
						</div>
						<div className="gate-inspector__field">
							<label className="gate-inspector__label" htmlFor="custom-pattern">Custom Pattern</label>
							<input
								id="custom-pattern"
								type="text"
								className="gate-inspector__input"
								value={autoCriteria.customPattern || ''}
								disabled={readOnly}
								placeholder="Regex pattern for custom validation"
								onChange={(e) => handleAutoCriteriaChange('customPattern', e.target.value)}
							/>
						</div>
					</div>
				)}

				{/* Human Gate Configuration */}
				{gateType === GateType.HUMAN && (
					<div className="gate-inspector__section">
						<h4>Human Configuration</h4>
						<div className="gate-inspector__field">
							<label className="gate-inspector__label" htmlFor="review-prompt">Review Prompt</label>
							<textarea
								id="review-prompt"
								className="gate-inspector__input"
								value={humanConfig.reviewPrompt || ''}
								disabled={readOnly}
								placeholder="Instructions for the human reviewer..."
								rows={3}
								onChange={(e) => handleHumanConfigChange('reviewPrompt', e.target.value)}
							/>
						</div>
					</div>
				)}

				{/* AI Gate Configuration */}
				{gateType === GateType.AI && (
					<div className="gate-inspector__section">
						<h4>AI Configuration</h4>
						<div className="gate-inspector__field">
							<label className="gate-inspector__label" htmlFor="reviewer-agent">Reviewer Agent</label>
							<select
								id="reviewer-agent"
								className="gate-inspector__select"
								value={aiConfig.reviewerAgentId || ''}
								disabled={readOnly}
								onChange={(e) => handleAIConfigChange('reviewerAgentId', e.target.value)}
							>
								<option value="">Select agent...</option>
								<option value="security-reviewer">Security Reviewer</option>
								<option value="code-reviewer">Code Reviewer</option>
							</select>
						</div>
						<div className="gate-inspector__field">
							<label className="gate-inspector__label">Context Sources</label>
							<label>
								<input
									type="checkbox"
									checked={(aiConfig.contextSources || []).includes('phase_outputs')}
									disabled={readOnly}
									onChange={(e) => {
										const sources = aiConfig.contextSources || [];
										const newSources = e.target.checked
											? [...sources, 'phase_outputs' as const]
											: sources.filter(s => s !== 'phase_outputs');
										handleAIConfigChange('contextSources', newSources);
									}}
								/>
								Phase Outputs
							</label>
							<label>
								<input
									type="checkbox"
									checked={(aiConfig.contextSources || []).includes('task_details')}
									disabled={readOnly}
									onChange={(e) => {
										const sources = aiConfig.contextSources || [];
										const newSources = e.target.checked
											? [...sources, 'task_details' as const]
											: sources.filter(s => s !== 'task_details');
										handleAIConfigChange('contextSources', newSources);
									}}
								/>
								Task Details
							</label>
							<label>
								<input
									type="checkbox"
									checked={(aiConfig.contextSources || []).includes('vars')}
									disabled={readOnly}
									onChange={(e) => {
										const sources = aiConfig.contextSources || [];
										const newSources = e.target.checked
											? [...sources, 'vars' as const]
											: sources.filter(s => s !== 'vars');
										handleAIConfigChange('contextSources', newSources);
									}}
								/>
								Variables
							</label>
						</div>
					</div>
				)}

				{/* Failure Handling */}
				<div className="gate-inspector__section">
					<h4>Failure Handling</h4>
					<div className="gate-inspector__field">
						<label className="gate-inspector__label" htmlFor="on-fail">On Fail</label>
						<select
							id="on-fail"
							className="gate-inspector__select"
							value={failureAction}
							disabled={readOnly || isLoading}
							onChange={handleFailureActionChange}
						>
							<option value="retry">Retry</option>
							<option value="retry_from">Retry From</option>
							<option value="fail">Fail</option>
							<option value="pause">Pause</option>
						</select>
					</div>

					{failureAction === 'retry_from' && (
						<div className="gate-inspector__field">
							<label className="gate-inspector__label" htmlFor="retry-from">Retry From</label>
							<select
								id="retry-from"
								className="gate-inspector__select"
								value={retryFromPhaseId || ''}
								disabled={readOnly || isLoading}
								onChange={handleRetryFromChange}
							>
								<option value="">Select phase...</option>
								{workflowDetails?.phases?.map((phase) => (
									<option key={phase.id} value={phase.id}>
										{phase.template?.name || phase.phaseTemplateId}
									</option>
								))}
							</select>
						</div>
					)}
				</div>

				{/* Max Retries */}
				<div className="gate-inspector__field">
					<label className="gate-inspector__label" htmlFor="max-retries">Max Retries</label>
					<input
						id="max-retries"
						type="number"
						className="gate-inspector__input"
						value={maxRetries}
						disabled={readOnly || isLoading}
						min="0"
						max="10"
						onChange={handleMaxRetriesChange}
					/>
				</div>

				{/* Advanced Section (Collapsible) */}
				<div className="gate-inspector__section">
					<button
						type="button"
						className="gate-inspector__section-toggle"
						onClick={() => setIsAdvancedExpanded(!isAdvancedExpanded)}
					>
						Advanced {isAdvancedExpanded ? '▼' : '▶'}
					</button>

					{isAdvancedExpanded && (
						<div className="gate-inspector__advanced">
							<div className="gate-inspector__field">
								<label className="gate-inspector__label" htmlFor="before-script">Before Script</label>
								<input
									id="before-script"
									type="text"
									className="gate-inspector__input"
									value={advancedConfig.beforeScript || ''}
									disabled={readOnly}
									placeholder="Script to run before gate evaluation"
									onChange={(e) => handleAdvancedConfigChange('beforeScript', e.target.value)}
								/>
							</div>
							<div className="gate-inspector__field">
								<label className="gate-inspector__label" htmlFor="after-script">After Script</label>
								<input
									id="after-script"
									type="text"
									className="gate-inspector__input"
									value={advancedConfig.afterScript || ''}
									disabled={readOnly}
									placeholder="Script to run after gate evaluation"
									onChange={(e) => handleAdvancedConfigChange('afterScript', e.target.value)}
								/>
							</div>
							<div className="gate-inspector__field">
								<label className="gate-inspector__label" htmlFor="store-result-as">Store Result As</label>
								<input
									id="store-result-as"
									type="text"
									className="gate-inspector__input"
									value={advancedConfig.storeResultAs || ''}
									disabled={readOnly}
									placeholder="Variable name to store gate result"
									onChange={(e) => handleAdvancedConfigChange('storeResultAs', e.target.value)}
								/>
							</div>
						</div>
					)}
				</div>

				{/* Gate Status (during execution) */}
				{gateStatus && (
					<div className="gate-inspector__field">
						<label className="gate-inspector__label">Status</label>
						<span className={`gate-inspector__status ${statusClass}`}>
							{gateStatus.charAt(0).toUpperCase() + gateStatus.slice(1)}
						</span>
					</div>
				)}
			</div>
		</div>
	);
}
