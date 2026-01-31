/**
 * PhaseListEditor - Sub-component for managing workflow phases.
 *
 * Features:
 * - Display phases in sequence order with visual indicators
 * - Add phases from template selector
 * - Edit phase overrides (model, thinking, gate, iterations)
 * - Edit claude_config overrides (hooks, skills, MCP, tools, env) with inherited/override distinction
 * - Remove phases with confirmation
 * - Reorder phases with up/down buttons
 */

import { useState, useCallback, useMemo } from 'react';
import * as RadixSelect from '@radix-ui/react-select';
import { Button, Icon } from '@/components/ui';
import { GateType, type WorkflowPhase, type PhaseTemplate } from '@/gen/orc/v1/workflow_pb';
import { CollapsibleSettingsSection } from '@/components/core/CollapsibleSettingsSection';
import { TagInput } from '@/components/core/TagInput';
import { KeyValueEditor } from '@/components/core/KeyValueEditor';
import { parseClaudeConfig, serializeClaudeConfig, type ClaudeConfigState } from '@/lib/claudeConfigUtils';
import './PhaseListEditor.css';

export interface PhaseOverrides {
	modelOverride?: string;
	thinkingOverride?: boolean;
	gateTypeOverride?: GateType;
	maxIterationsOverride?: number;
	claudeConfigOverride?: string;
}

export interface AddPhaseRequest {
	phaseTemplateId: string;
	sequence: number;
}

export interface PhaseListEditorProps {
	/** Workflow ID */
	workflowId: string;
	/** List of phases in the workflow */
	phases: WorkflowPhase[];
	/** Available phase templates */
	phaseTemplates: PhaseTemplate[];
	/** Whether the component is in a loading state */
	loading: boolean;
	/** Callback when adding a new phase */
	onAddPhase: (request: AddPhaseRequest) => Promise<void>;
	/** Callback when updating a phase's overrides */
	onUpdatePhase: (phaseId: number, overrides: PhaseOverrides) => Promise<void>;
	/** Callback when removing a phase */
	onRemovePhase: (phaseId: number) => Promise<void>;
	/** Callback when reordering a phase */
	onReorderPhase: (phaseId: number, direction: 'up' | 'down') => Promise<void>;
}

const INHERIT_VALUE = '__inherit__';

const MODEL_OPTIONS = [
	{ value: INHERIT_VALUE, label: 'Inherit (default)' },
	{ value: 'sonnet', label: 'Sonnet' },
	{ value: 'opus', label: 'Opus' },
	{ value: 'haiku', label: 'Haiku' },
];

const GATE_TYPE_OPTIONS = [
	{ value: GateType.UNSPECIFIED, label: 'Inherit (default)' },
	{ value: GateType.AUTO, label: 'Auto' },
	{ value: GateType.HUMAN, label: 'Human' },
	{ value: GateType.SKIP, label: 'Skip' },
];

/** Generate badge text for a section showing inherited/override breakdown. */
function sectionBadgeText(inheritedCount: number, overrideCount: number): string {
	const total = inheritedCount + overrideCount;
	if (total === 0) return '0';
	if (overrideCount === 0) return `${total} inherited`;
	if (inheritedCount === 0) return `${total} override`;
	return `${total} — ${inheritedCount} inherited, ${overrideCount} override`;
}

/** Get unique override items (items in override that aren't in inherited). */
function uniqueOverrides(inherited: string[], overrides: string[]): string[] {
	const inheritedSet = new Set(inherited);
	return overrides.filter((item) => !inheritedSet.has(item));
}

/**
 * PhaseListEditor manages phases within a workflow.
 */
export function PhaseListEditor({
	phases,
	phaseTemplates,
	loading,
	onAddPhase,
	onUpdatePhase,
	onRemovePhase,
	onReorderPhase,
}: PhaseListEditorProps) {
	// Add phase dialog state
	const [addDialogOpen, setAddDialogOpen] = useState(false);
	const [selectedTemplateId, setSelectedTemplateId] = useState('');

	// Edit phase dialog state
	const [editingPhase, setEditingPhase] = useState<WorkflowPhase | null>(null);
	const [editOverrides, setEditOverrides] = useState<PhaseOverrides>({});

	// Claude config override state (for edit dialog)
	const [overrideHooks, setOverrideHooks] = useState<string[]>([]);
	const [overrideSkills, setOverrideSkills] = useState<string[]>([]);
	const [overrideMcpServers, setOverrideMcpServers] = useState<string[]>([]);
	const [overrideAllowedTools, setOverrideAllowedTools] = useState<string[]>([]);
	const [overrideDisallowedTools, setOverrideDisallowedTools] = useState<string[]>([]);
	const [overrideEnv, setOverrideEnv] = useState<Record<string, string>>({});
	const [jsonOverride, setJsonOverride] = useState('');

	// Sort phases by sequence
	const sortedPhases = useMemo(() => {
		return [...phases].sort((a, b) => a.sequence - b.sequence);
	}, [phases]);

	// Get phase template by ID
	const getTemplate = useCallback(
		(templateId: string): PhaseTemplate | undefined => {
			return phaseTemplates.find((t) => t.id === templateId);
		},
		[phaseTemplates]
	);

	// Calculate next sequence number
	const nextSequence = useMemo(() => {
		if (phases.length === 0) return 1;
		return Math.max(...phases.map((p) => p.sequence)) + 1;
	}, [phases]);

	// Parse template claude_config for the editing phase
	const templateConfig = useMemo<ClaudeConfigState>(() => {
		if (!editingPhase) return parseClaudeConfig(undefined);
		const tmpl = editingPhase.template;
		return parseClaudeConfig((tmpl as Record<string, unknown> | undefined)?.claudeConfig as string | undefined);
	}, [editingPhase]);

	// Handle add phase
	const handleAddPhase = useCallback(async () => {
		if (!selectedTemplateId) return;
		try {
			await onAddPhase({
				phaseTemplateId: selectedTemplateId,
				sequence: nextSequence,
			});
		} catch {
			return;
		}
		setAddDialogOpen(false);
		setSelectedTemplateId('');
	}, [selectedTemplateId, nextSequence, onAddPhase]);

	// Handle edit phase - open dialog
	const handleEditClick = useCallback((phase: WorkflowPhase) => {
		setEditingPhase(phase);
		setEditOverrides({
			modelOverride: phase.modelOverride || undefined,
			thinkingOverride: phase.thinkingOverride || undefined,
			gateTypeOverride: phase.gateTypeOverride,
			maxIterationsOverride: phase.maxIterationsOverride,
		});

		// Parse existing claude_config_override
		const override = parseClaudeConfig(phase.claudeConfigOverride as string | undefined);
		setOverrideHooks(override.hooks);
		setOverrideSkills(override.skillRefs);
		setOverrideMcpServers(override.mcpServers);
		setOverrideAllowedTools(override.allowedTools);
		setOverrideDisallowedTools(override.disallowedTools);
		setOverrideEnv(override.env);
		setJsonOverride('');
	}, []);

	// Serialize override config state to JSON
	const buildClaudeConfigOverride = useCallback((): string | undefined => {
		const state: ClaudeConfigState = {
			hooks: overrideHooks,
			skillRefs: overrideSkills,
			mcpServers: overrideMcpServers,
			allowedTools: overrideAllowedTools,
			disallowedTools: overrideDisallowedTools,
			env: overrideEnv,
			extra: {},
		};
		const serialized = serializeClaudeConfig(state);
		if (serialized === '{}') return undefined;
		return serialized;
	}, [overrideHooks, overrideSkills, overrideMcpServers, overrideAllowedTools, overrideDisallowedTools, overrideEnv]);

	// Handle save phase overrides
	const handleSavePhase = useCallback(async () => {
		if (!editingPhase) return;
		const claudeConfigOverride = buildClaudeConfigOverride();
		try {
			await onUpdatePhase(editingPhase.id, {
				...editOverrides,
				claudeConfigOverride,
			});
		} catch {
			return;
		}
		setEditingPhase(null);
		setEditOverrides({});
	}, [editingPhase, editOverrides, buildClaudeConfigOverride, onUpdatePhase]);

	// Handle remove phase
	const handleRemovePhase = useCallback(
		async (phase: WorkflowPhase) => {
			const template = getTemplate(phase.phaseTemplateId);
			const confirmed = window.confirm(
				`Remove phase "${template?.name || phase.phaseTemplateId}"?`
			);
			if (!confirmed) return;
			try {
				await onRemovePhase(phase.id);
			} catch {
				// Error already handled by parent (toast shown)
			}
		},
		[getTemplate, onRemovePhase]
	);

	// Handle move phase
	const handleMovePhase = useCallback(
		async (phase: WorkflowPhase, direction: 'up' | 'down') => {
			try {
				await onReorderPhase(phase.id, direction);
			} catch {
				// Error already handled by parent (toast shown)
			}
		},
		[onReorderPhase]
	);

	// Clear override for a specific section
	const handleClearOverride = useCallback((section: string) => {
		switch (section) {
			case 'hooks': setOverrideHooks([]); break;
			case 'skills': setOverrideSkills([]); break;
			case 'mcpServers': setOverrideMcpServers([]); break;
			case 'allowedTools': setOverrideAllowedTools([]); break;
			case 'disallowedTools': setOverrideDisallowedTools([]); break;
			case 'env': setOverrideEnv({}); break;
		}
	}, []);

	// Empty state
	if (phases.length === 0 && !loading && !addDialogOpen) {
		return (
			<div className="phase-list-editor">
				<div className="phase-list-empty">
					<Icon name="layers" size={24} />
					<p>No phases configured. Add your first phase to get started.</p>
					<Button
						variant="primary"
						size="sm"
						leftIcon={<Icon name="plus" size={14} />}
						onClick={() => setAddDialogOpen(true)}
						disabled={loading}
					>
						Add Phase
					</Button>
				</div>
			</div>
		);
	}

	return (
		<div className="phase-list-editor">
			{/* Loading indicator */}
			{loading && (
				<div className="phase-list-loading">
					<Icon name="loader" size={14} className="spinning" />
					<span>Loading...</span>
				</div>
			)}

			{/* Phase list */}
			<div className="phase-list">
				{sortedPhases.map((phase, index) => {
					const template = getTemplate(phase.phaseTemplateId);
					const isFirst = index === 0;
					const isLast = index === sortedPhases.length - 1;
					const hasOverrides =
						phase.modelOverride ||
						phase.thinkingOverride ||
						phase.gateTypeOverride !== undefined ||
						phase.maxIterationsOverride !== undefined;

					return (
						<div
							key={phase.id}
							data-testid={`phase-item-${phase.id}`}
							className="phase-item"
						>
							{/* Sequence number */}
							<div className="phase-item-sequence">{index + 1}</div>

							{/* Phase info */}
							<div className="phase-item-info">
								<span className="phase-item-name">
									{template?.name || phase.phaseTemplateId}
								</span>
								{hasOverrides && (
									<div className="phase-item-badges">
										{phase.modelOverride && (
											<span className="phase-badge phase-badge--model">
												{phase.modelOverride}
											</span>
										)}
										{phase.thinkingOverride && (
											<span className="phase-badge phase-badge--thinking">
												<Icon name="brain" size={10} />
											</span>
										)}
										{phase.gateTypeOverride !== undefined &&
											phase.gateTypeOverride !== GateType.UNSPECIFIED && (
												<span className="phase-badge phase-badge--gate">
													{GateType[phase.gateTypeOverride]}
												</span>
											)}
										{phase.maxIterationsOverride !== undefined && (
											<span className="phase-badge phase-badge--iterations">
												max {phase.maxIterationsOverride}
											</span>
										)}
									</div>
								)}
							</div>

							{/* Actions */}
							<div className="phase-item-actions">
								<Button
									variant="ghost"
									size="sm"
									title="Move up"
									aria-label="Move up"
									onClick={() => handleMovePhase(phase, 'up')}
									disabled={loading || isFirst}
								>
									<Icon name="chevron-up" size={14} />
								</Button>
								<Button
									variant="ghost"
									size="sm"
									title="Move down"
									aria-label="Move down"
									onClick={() => handleMovePhase(phase, 'down')}
									disabled={loading || isLast}
								>
									<Icon name="chevron-down" size={14} />
								</Button>
								<Button
									variant="ghost"
									size="sm"
									title="Edit"
									aria-label="Edit"
									onClick={() => handleEditClick(phase)}
									disabled={loading}
								>
									<Icon name="edit" size={14} />
								</Button>
								<Button
									variant="ghost"
									size="sm"
									title="Delete"
									aria-label="Delete"
									onClick={() => handleRemovePhase(phase)}
									disabled={loading}
								>
									<Icon name="trash" size={14} />
								</Button>
							</div>
						</div>
					);
				})}
			</div>

			{/* Add Phase button */}
			{!addDialogOpen && (
				<Button
					variant="secondary"
					size="sm"
					leftIcon={<Icon name="plus" size={14} />}
					onClick={() => setAddDialogOpen(true)}
					disabled={loading}
				>
					Add Phase
				</Button>
			)}

			{/* Add Phase Dialog */}
			{addDialogOpen && (
				<div className="phase-add-dialog">
					<div className="form-group">
						<label id="phase-template-label" className="form-label">
							Phase Template
						</label>
						{phaseTemplates.length === 0 ? (
							<div className="phase-add-empty">
								<Icon name="alert-circle" size={14} />
								<span>No templates available</span>
							</div>
						) : (
							<RadixSelect.Root
								value={selectedTemplateId}
								onValueChange={setSelectedTemplateId}
							>
								<RadixSelect.Trigger
									className="phase-template-trigger"
									aria-label="Phase template"
									aria-labelledby="phase-template-label"
								>
									<RadixSelect.Value placeholder="Select a template..." />
									<RadixSelect.Icon className="phase-template-trigger-icon">
										<Icon name="chevron-down" size={12} />
									</RadixSelect.Icon>
								</RadixSelect.Trigger>

								<RadixSelect.Portal>
									<RadixSelect.Content
										className="phase-template-content"
										position="popper"
										sideOffset={4}
									>
										<RadixSelect.Viewport className="phase-template-viewport">
											{phaseTemplates.map((template) => (
												<RadixSelect.Item
													key={template.id}
													value={template.id}
													className="phase-template-item"
												>
													<RadixSelect.ItemText>{template.name}</RadixSelect.ItemText>
													<div className="phase-template-item-desc">
														{template.description}
													</div>
												</RadixSelect.Item>
											))}
										</RadixSelect.Viewport>
									</RadixSelect.Content>
								</RadixSelect.Portal>
							</RadixSelect.Root>
						)}
					</div>
					<div className="phase-add-actions">
						<Button
							variant="ghost"
							size="sm"
							onClick={() => {
								setAddDialogOpen(false);
								setSelectedTemplateId('');
							}}
						>
							Cancel
						</Button>
						<Button
							variant="primary"
							size="sm"
							onClick={handleAddPhase}
							disabled={!selectedTemplateId}
						>
							Add
						</Button>
					</div>
				</div>
			)}

			{/* Edit Phase Dialog */}
			{editingPhase && (
				<div className="phase-edit-dialog">
					<h4 className="phase-edit-title">
						Edit Phase: {getTemplate(editingPhase.phaseTemplateId)?.name || editingPhase.phaseTemplateId}
					</h4>

					{/* Model override */}
					<div className="form-group">
						<label id="phase-model-label" className="form-label">
							Model
						</label>
						<RadixSelect.Root
							value={editOverrides.modelOverride || INHERIT_VALUE}
							onValueChange={(value) =>
								setEditOverrides((prev) => ({
									...prev,
									modelOverride: value === INHERIT_VALUE ? undefined : value,
								}))
							}
						>
							<RadixSelect.Trigger
								className="phase-template-trigger"
								aria-label="Model"
								aria-labelledby="phase-model-label"
							>
								<RadixSelect.Value placeholder="Inherit (default)">
									{MODEL_OPTIONS.find((opt) => opt.value === (editOverrides.modelOverride || INHERIT_VALUE))?.label}
								</RadixSelect.Value>
								<RadixSelect.Icon className="phase-template-trigger-icon">
									<Icon name="chevron-down" size={12} />
								</RadixSelect.Icon>
							</RadixSelect.Trigger>

							<RadixSelect.Portal>
								<RadixSelect.Content
									className="phase-template-content"
									position="popper"
									sideOffset={4}
								>
									<RadixSelect.Viewport className="phase-template-viewport">
										{MODEL_OPTIONS.map((opt) => (
											<RadixSelect.Item
												key={opt.value}
												value={opt.value}
												className="phase-template-item"
											>
												<RadixSelect.ItemText>{opt.label}</RadixSelect.ItemText>
											</RadixSelect.Item>
										))}
									</RadixSelect.Viewport>
								</RadixSelect.Content>
							</RadixSelect.Portal>
						</RadixSelect.Root>
					</div>

					{/* Thinking override */}
					<div className="form-group">
						<label className="form-checkbox">
							<input
								type="checkbox"
								checked={editOverrides.thinkingOverride || false}
								onChange={(e) =>
									setEditOverrides((prev) => ({
										...prev,
										thinkingOverride: e.target.checked || undefined,
									}))
								}
								aria-label="Thinking"
							/>
							<span className="form-checkbox-label">Enable thinking mode</span>
						</label>
					</div>

					{/* Gate type override */}
					<div className="form-group">
						<label id="phase-gate-label" className="form-label">
							Gate Type
						</label>
						<RadixSelect.Root
							value={String(editOverrides.gateTypeOverride ?? GateType.UNSPECIFIED)}
							onValueChange={(value) =>
								setEditOverrides((prev) => ({
									...prev,
									gateTypeOverride:
										Number(value) === GateType.UNSPECIFIED
											? undefined
											: (Number(value) as GateType),
								}))
							}
						>
							<RadixSelect.Trigger
								className="phase-template-trigger"
								aria-label="Gate"
								aria-labelledby="phase-gate-label"
							>
								<RadixSelect.Value placeholder="Inherit (default)">
									{GATE_TYPE_OPTIONS.find((opt) => opt.value === (editOverrides.gateTypeOverride ?? GateType.UNSPECIFIED))?.label}
								</RadixSelect.Value>
								<RadixSelect.Icon className="phase-template-trigger-icon">
									<Icon name="chevron-down" size={12} />
								</RadixSelect.Icon>
							</RadixSelect.Trigger>

							<RadixSelect.Portal>
								<RadixSelect.Content
									className="phase-template-content"
									position="popper"
									sideOffset={4}
								>
									<RadixSelect.Viewport className="phase-template-viewport">
										{GATE_TYPE_OPTIONS.map((opt) => (
											<RadixSelect.Item
												key={opt.value}
												value={String(opt.value)}
												className="phase-template-item"
											>
												<RadixSelect.ItemText>{opt.label}</RadixSelect.ItemText>
											</RadixSelect.Item>
										))}
									</RadixSelect.Viewport>
								</RadixSelect.Content>
							</RadixSelect.Portal>
						</RadixSelect.Root>
					</div>

					{/* Max iterations override */}
					<div className="form-group">
						<label htmlFor="phase-iterations-input" className="form-label">
							Max Iterations
						</label>
						<input
							id="phase-iterations-input"
							type="number"
							className="form-input"
							min={1}
							max={20}
							value={editOverrides.maxIterationsOverride ?? ''}
							onChange={(e) =>
								setEditOverrides((prev) => ({
									...prev,
									maxIterationsOverride: e.target.value
										? parseInt(e.target.value, 10)
										: undefined,
								}))
							}
							aria-label="Max iterations"
							placeholder="Inherit from template"
						/>
					</div>

					{/* ─── Claude Config Override Sections ─────────────────── */}

					<ClaudeConfigSections
						templateConfig={templateConfig}
						overrideHooks={overrideHooks}
						overrideSkills={overrideSkills}
						overrideMcpServers={overrideMcpServers}
						overrideAllowedTools={overrideAllowedTools}
						overrideDisallowedTools={overrideDisallowedTools}
						overrideEnv={overrideEnv}
						jsonOverride={jsonOverride}
						onOverrideHooksChange={setOverrideHooks}
						onOverrideSkillsChange={setOverrideSkills}
						onOverrideMcpServersChange={setOverrideMcpServers}
						onOverrideAllowedToolsChange={setOverrideAllowedTools}
						onOverrideDisallowedToolsChange={setOverrideDisallowedTools}
						onOverrideEnvChange={setOverrideEnv}
						onJsonOverrideChange={setJsonOverride}
						onClearOverride={handleClearOverride}
					/>

					<div className="phase-edit-actions">
						<Button
							variant="ghost"
							size="sm"
							onClick={() => {
								setEditingPhase(null);
								setEditOverrides({});
							}}
						>
							Cancel
						</Button>
						<Button variant="primary" size="sm" onClick={handleSavePhase}>
							Save Phase
						</Button>
					</div>
				</div>
			)}
		</div>
	);
}

// ─── Claude Config Sections Component ─────────────────────────────────────────

interface ClaudeConfigSectionsProps {
	templateConfig: ClaudeConfigState;
	overrideHooks: string[];
	overrideSkills: string[];
	overrideMcpServers: string[];
	overrideAllowedTools: string[];
	overrideDisallowedTools: string[];
	overrideEnv: Record<string, string>;
	jsonOverride: string;
	onOverrideHooksChange: (hooks: string[]) => void;
	onOverrideSkillsChange: (skills: string[]) => void;
	onOverrideMcpServersChange: (servers: string[]) => void;
	onOverrideAllowedToolsChange: (tools: string[]) => void;
	onOverrideDisallowedToolsChange: (tools: string[]) => void;
	onOverrideEnvChange: (env: Record<string, string>) => void;
	onJsonOverrideChange: (json: string) => void;
	onClearOverride: (section: string) => void;
}

function ClaudeConfigSections({
	templateConfig,
	overrideHooks,
	overrideSkills,
	overrideMcpServers,
	overrideAllowedTools,
	overrideDisallowedTools,
	overrideEnv,
	jsonOverride,
	onOverrideHooksChange,
	onOverrideSkillsChange,
	onOverrideMcpServersChange,
	onOverrideAllowedToolsChange,
	onOverrideDisallowedToolsChange,
	onOverrideEnvChange,
	onJsonOverrideChange,
	onClearOverride,
}: ClaudeConfigSectionsProps) {
	return (
		<div className="claude-config-sections">
			{/* Hooks */}
			<ListOverrideSection
				title="Hooks"
				testId="hooks-picker"
				inherited={templateConfig.hooks}
				overrides={overrideHooks}
				onChange={onOverrideHooksChange}
				onClear={() => onClearOverride('hooks')}
			/>

			{/* MCP Servers */}
			<ListOverrideSection
				title="MCP Servers"
				testId="mcp-servers-picker"
				inherited={templateConfig.mcpServers}
				overrides={overrideMcpServers}
				onChange={onOverrideMcpServersChange}
				onClear={() => onClearOverride('mcpServers')}
			/>

			{/* Skills */}
			<ListOverrideSection
				title="Skills"
				testId="skills-picker"
				inherited={templateConfig.skillRefs}
				overrides={overrideSkills}
				onChange={onOverrideSkillsChange}
				onClear={() => onClearOverride('skills')}
			/>

			{/* Allowed Tools */}
			<ToolsOverrideSection
				title="Allowed Tools"
				testId="allowed-tools-input"
				inherited={templateConfig.allowedTools}
				overrides={overrideAllowedTools}
				onChange={onOverrideAllowedToolsChange}
				onClear={() => onClearOverride('allowedTools')}
			/>

			{/* Disallowed Tools */}
			<ToolsOverrideSection
				title="Disallowed Tools"
				testId="disallowed-tools-input"
				inherited={templateConfig.disallowedTools}
				overrides={overrideDisallowedTools}
				onChange={onOverrideDisallowedToolsChange}
				onClear={() => onClearOverride('disallowedTools')}
			/>

			{/* Env Vars */}
			<EnvVarsOverrideSection
				templateEnv={templateConfig.env}
				overrideEnv={overrideEnv}
				onChange={onOverrideEnvChange}
				onClear={() => onClearOverride('env')}
			/>

			{/* JSON Override */}
			<CollapsibleSettingsSection
				title="JSON Override"
				badgeCount={0}
				badgeText={jsonOverride ? '1' : '0'}
			>
				<textarea
					className="claude-config-json-textarea"
					value={jsonOverride}
					onChange={(e) => onJsonOverrideChange(e.target.value)}
					aria-label="JSON Override"
					placeholder='{"hooks": ["my-hook"], ...}'
					rows={4}
				/>
			</CollapsibleSettingsSection>
		</div>
	);
}

// ─── List Override Section (hooks, skills, MCP servers) ─────────────────────

interface ListOverrideSectionProps {
	title: string;
	testId: string;
	inherited: string[];
	overrides: string[];
	onChange: (items: string[]) => void;
	onClear: () => void;
}

function ListOverrideSection({
	title,
	testId,
	inherited,
	overrides,
	onChange,
	onClear,
}: ListOverrideSectionProps) {
	const uniqueOverrideItems = uniqueOverrides(inherited, overrides);
	const inheritedCount = inherited.length;
	const overrideCount = uniqueOverrideItems.length;

	const handleAdd = useCallback(() => {
		const baseName = `new-${title.toLowerCase().replace(/\s+/g, '-')}`;
		let name = baseName;
		let counter = 1;
		while (overrides.includes(name) || inherited.includes(name)) {
			name = `${baseName}-${counter++}`;
		}
		onChange([...overrides, name]);
	}, [title, overrides, inherited, onChange]);

	return (
		<CollapsibleSettingsSection
			title={title}
			badgeCount={inheritedCount + overrideCount}
			badgeText={sectionBadgeText(inheritedCount, overrideCount)}
		>
			<div data-testid={testId}>
				{/* Inherited items */}
				{inherited.map((item) => (
					<div key={`inherited-${item}`} className="settings-item settings-item--inherited">
						<span className="settings-item__name">{item}</span>
					</div>
				))}

				{/* Override items (only those not in inherited) */}
				{uniqueOverrideItems.map((item) => (
					<div key={`override-${item}`} className="settings-item settings-item--override">
						<span className="settings-item__name">{item}</span>
					</div>
				))}

				{/* Actions */}
				<div className="settings-item__actions">
					<button
						type="button"
						className="settings-item__add-btn"
						onClick={handleAdd}
						aria-label="Add"
						role="button"
					>
						Add
					</button>
					{overrides.length > 0 && (
						<button
							type="button"
							className="settings-item__clear-btn"
							onClick={onClear}
							aria-label="Clear Override"
						>
							Clear Override
						</button>
					)}
					{overrides.length === 0 && inherited.length > 0 && (
						<button
							type="button"
							className="settings-item__clear-btn"
							onClick={onClear}
							aria-label="Clear Override"
							disabled
						>
							Clear Override
						</button>
					)}
				</div>
			</div>
		</CollapsibleSettingsSection>
	);
}

// ─── Tools Override Section (allowed/disallowed tools via TagInput) ─────────

interface ToolsOverrideSectionProps {
	title: string;
	testId: string;
	inherited: string[];
	overrides: string[];
	onChange: (items: string[]) => void;
	onClear: () => void;
}

function ToolsOverrideSection({
	title,
	testId,
	inherited,
	overrides,
	onChange,
	onClear,
}: ToolsOverrideSectionProps) {
	const uniqueOverrideItems = uniqueOverrides(inherited, overrides);
	const inheritedCount = inherited.length;
	const overrideCount = uniqueOverrideItems.length;

	return (
		<CollapsibleSettingsSection
			title={title}
			badgeCount={inheritedCount + overrideCount}
			badgeText={sectionBadgeText(inheritedCount, overrideCount)}
		>
			<div data-testid={testId}>
				{/* Inherited items */}
				{inherited.map((item) => (
					<div key={`inherited-${item}`} className="settings-item settings-item--inherited">
						<span className="settings-item__name">{item}</span>
					</div>
				))}

				{/* Override items via TagInput */}
				<TagInput
					tags={overrides}
					onChange={onChange}
					placeholder={`Add ${title.toLowerCase()}...`}
				/>

				{overrides.length > 0 && (
					<button
						type="button"
						className="settings-item__clear-btn"
						onClick={onClear}
						aria-label="Clear Override"
					>
						Clear Override
					</button>
				)}
			</div>
		</CollapsibleSettingsSection>
	);
}

// ─── Env Vars Override Section ───────────────────────────────────────────────

interface EnvVarsOverrideSectionProps {
	templateEnv: Record<string, string>;
	overrideEnv: Record<string, string>;
	onChange: (env: Record<string, string>) => void;
	onClear: () => void;
}

function EnvVarsOverrideSection({
	templateEnv,
	overrideEnv,
	onChange,
	onClear,
}: EnvVarsOverrideSectionProps) {
	const inheritedKeys = Object.keys(templateEnv);
	const overrideKeys = Object.keys(overrideEnv);
	// Override keys not in template
	const uniqueOverrideKeys = overrideKeys.filter((k) => !(k in templateEnv));
	const inheritedCount = inheritedKeys.length;
	const overrideCount = uniqueOverrideKeys.length;

	return (
		<CollapsibleSettingsSection
			title="Env Vars"
			badgeCount={inheritedCount + overrideCount}
			badgeText={sectionBadgeText(inheritedCount, overrideCount)}
		>
			<div data-testid="env-editor">
				{/* Inherited env vars */}
				{inheritedKeys.map((key) => (
					<div key={`inherited-${key}`} className="settings-item settings-item--inherited">
						<span className="settings-item__name">{key}</span>
						<span className="settings-item__value">= {templateEnv[key]}</span>
					</div>
				))}

				{/* Override env vars (shown as text items) */}
				{overrideKeys.map((key) => (
					<div key={`override-${key}`} className="settings-item settings-item--override">
						<span className="settings-item__name">{key}</span>
						<span className="settings-item__value">= {overrideEnv[key]}</span>
					</div>
				))}

				{/* Editor for modifying override env vars */}
				<KeyValueEditor
					entries={overrideEnv}
					onChange={onChange}
				/>

				{overrideKeys.length > 0 && (
					<button
						type="button"
						className="settings-item__clear-btn"
						onClick={onClear}
						aria-label="Clear Override"
					>
						Clear Override
					</button>
				)}
			</div>
		</CollapsibleSettingsSection>
	);
}
