/**
 * PhaseListEditor - Sub-component for managing workflow phases.
 *
 * Features:
 * - Display phases in sequence order with visual indicators
 * - Add phases from template selector
 * - Edit phase overrides (model, thinking, gate, iterations)
 * - Edit runtime_config overrides (hooks, skills, MCP, tools, env) with inherited/override distinction
 * - Remove phases with confirmation
 * - Reorder phases with up/down buttons
 */

import { useState, useCallback, useMemo } from 'react';
import { Button, Icon } from '@/components/ui';
import type { WorkflowPhase, PhaseTemplate } from '@/gen/orc/v1/workflow_pb';
import { PhaseAddDialog } from './phase-list-editor/PhaseAddDialog';
import { PhaseEditDialog } from './phase-list-editor/PhaseEditDialog';
import { PhaseList } from './phase-list-editor/PhaseList';
import { type AddPhaseRequest, type PhaseOverrides } from './phase-list-editor/shared';
import './PhaseListEditor.css';
export type { AddPhaseRequest, PhaseOverrides } from './phase-list-editor/shared';

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
			return;
		}
		setAddDialogOpen(false);
		setSelectedTemplateId('');
	}, [selectedTemplateId, nextSequence, onAddPhase]);

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
			<PhaseList
				phases={sortedPhases}
				getTemplate={getTemplate}
				loading={loading}
				onEdit={setEditingPhase}
				onMove={handleMovePhase}
				onRemove={handleRemovePhase}
			/>

			{/* Add Phase button */}
			{!addDialogOpen && (
				<Button
					variant="secondary"
					size="sm"
					leftIcon={<Icon name="plus" size={14} />}
					onClick={(e) => {
						e.stopPropagation();
						setAddDialogOpen(true);
					}}
					disabled={loading}
				>
					Add Phase
				</Button>
			)}

			{/* Add Phase Dialog */}
			<PhaseAddDialog
				open={addDialogOpen}
				selectedTemplateId={selectedTemplateId}
				phaseTemplates={phaseTemplates}
				onSelectedTemplateIdChange={setSelectedTemplateId}
				onAdd={handleAddPhase}
				onCancel={() => {
					setAddDialogOpen(false);
					setSelectedTemplateId('');
				}}
			/>

			{/* Edit Phase Dialog */}
			<PhaseEditDialog
				phase={editingPhase}
				getTemplate={getTemplate}
				onSave={onUpdatePhase}
				onClose={() => setEditingPhase(null)}
			/>
		</div>
	);
}
