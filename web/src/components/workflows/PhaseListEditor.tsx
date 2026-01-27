/**
 * PhaseListEditor - Sub-component for managing workflow phases.
 *
 * Features:
 * - Display phases in sequence order with visual indicators
 * - Add phases from template selector
 * - Edit phase overrides (model, thinking, gate, iterations)
 * - Remove phases with confirmation
 * - Reorder phases with up/down buttons
 */

import { useState, useCallback, useMemo } from 'react';
import * as RadixSelect from '@radix-ui/react-select';
import { Button, Icon } from '@/components/ui';
import { GateType, type WorkflowPhase, type PhaseTemplate } from '@/gen/orc/v1/workflow_pb';
import './PhaseListEditor.css';

export interface PhaseOverrides {
	modelOverride?: string;
	thinkingOverride?: boolean;
	gateTypeOverride?: GateType;
	maxIterationsOverride?: number;
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

	// Handle add phase
	const handleAddPhase = useCallback(async () => {
		if (!selectedTemplateId) return;
		try {
			await onAddPhase({
				phaseTemplateId: selectedTemplateId,
				sequence: nextSequence,
			});
		} catch {
			// Error already handled by parent (toast shown), dialog stays open for retry
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
	}, []);

	// Handle save phase overrides
	const handleSavePhase = useCallback(async () => {
		if (!editingPhase) return;
		try {
			await onUpdatePhase(editingPhase.id, editOverrides);
		} catch {
			// Error already handled by parent (toast shown), dialog stays open
			return;
		}
		setEditingPhase(null);
		setEditOverrides({});
	}, [editingPhase, editOverrides, onUpdatePhase]);

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
