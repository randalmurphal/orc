/**
 * WorkflowDetailPanel - Shows detailed information about a selected workflow.
 *
 * Features:
 * - Displays workflow name, description, and configuration
 * - Lists phases in sequence order with template details
 * - Shows workflow variables with source types
 * - Edit/Delete actions for custom workflows (built-ins are read-only)
 */

import { useState, useEffect, useCallback } from 'react';
import { RightPanel } from '@/components/layout/RightPanel';
import { Button, Icon } from '@/components/ui';
import { workflowClient } from '@/lib/client';
import type {
	Workflow,
	WorkflowWithDetails,
	WorkflowPhase,
	WorkflowVariable,
} from '@/gen/orc/v1/workflow_pb';
import './WorkflowDetailPanel.css';

export interface WorkflowDetailPanelProps {
	/** The workflow to display (basic info for initial render) */
	workflow: Workflow | null;
	/** Whether the panel is open */
	isOpen: boolean;
	/** Callback when panel should close */
	onClose: () => void;
	/** Callback when clone action is triggered */
	onClone: (workflow: Workflow) => void;
	/** Callback when workflow is deleted */
	onDeleted: (id: string) => void;
}

/**
 * Renders a single phase in the workflow sequence.
 */
function PhaseItem({
	phase,
	index,
}: {
	phase: WorkflowPhase;
	index: number;
}) {
	return (
		<div className="workflow-detail-phase">
			<div className="workflow-detail-phase-number">{index + 1}</div>
			<div className="workflow-detail-phase-info">
				<span className="workflow-detail-phase-id">{phase.phaseTemplateId}</span>
				{phase.gateTypeOverride !== undefined && (
					<span className="workflow-detail-phase-gate">
						<Icon name="shield" size={10} />
						{phase.gateTypeOverride}
					</span>
				)}
				{phase.modelOverride && (
					<span className="workflow-detail-phase-model">
						<Icon name="robot" size={10} />
						{phase.modelOverride}
					</span>
				)}
			</div>
			{phase.maxIterationsOverride !== undefined && (
				<span className="workflow-detail-phase-iterations">
					max {phase.maxIterationsOverride}
				</span>
			)}
		</div>
	);
}

/**
 * Renders a workflow variable definition.
 */
function VariableItem({ variable }: { variable: WorkflowVariable }) {
	const sourceTypeStr = String(variable.sourceType);
	const sourceIcon = {
		'0': 'settings', // UNSPECIFIED
		'1': 'code', // STATIC
		'2': 'terminal', // ENV
		'3': 'file-code', // SCRIPT
		'4': 'globe', // API
		'5': 'git-branch', // PHASE_OUTPUT
		'6': 'file-text', // PROMPT_FRAGMENT
	}[sourceTypeStr] || 'settings';

	return (
		<div className="workflow-detail-variable">
			<div className="workflow-detail-variable-header">
				<code className="workflow-detail-variable-name">{variable.name}</code>
				{variable.required && (
					<span className="workflow-detail-variable-required">required</span>
				)}
			</div>
			<div className="workflow-detail-variable-meta">
				<span className="workflow-detail-variable-source">
					<Icon name={sourceIcon as 'code'} size={10} />
					{variable.sourceType}
				</span>
				{variable.description && (
					<span className="workflow-detail-variable-desc">{variable.description}</span>
				)}
			</div>
		</div>
	);
}

/**
 * WorkflowDetailPanel displays detailed information about a workflow.
 */
export function WorkflowDetailPanel({
	workflow,
	isOpen,
	onClose,
	onClone,
	onDeleted,
}: WorkflowDetailPanelProps) {
	const [details, setDetails] = useState<WorkflowWithDetails | null>(null);
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [deleting, setDeleting] = useState(false);

	// Load full workflow details when a workflow is selected
	useEffect(() => {
		if (!workflow?.id || !isOpen) {
			setDetails(null);
			setError(null);
			return;
		}

		let cancelled = false;
		setLoading(true);
		setError(null);

		workflowClient
			.getWorkflow({ id: workflow.id })
			.then((response) => {
				if (!cancelled && response.workflow) {
					setDetails(response.workflow);
				}
			})
			.catch((e) => {
				if (!cancelled) {
					setError(e instanceof Error ? e.message : 'Failed to load workflow');
				}
			})
			.finally(() => {
				if (!cancelled) {
					setLoading(false);
				}
			});

		return () => {
			cancelled = true;
		};
	}, [workflow?.id, isOpen]);

	const handleDelete = useCallback(async () => {
		if (!workflow || workflow.isBuiltin) return;

		const confirmed = window.confirm(
			`Delete workflow "${workflow.name}"? This cannot be undone.`
		);
		if (!confirmed) return;

		setDeleting(true);
		try {
			await workflowClient.deleteWorkflow({ id: workflow.id });
			onDeleted(workflow.id);
			onClose();
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to delete workflow');
		} finally {
			setDeleting(false);
		}
	}, [workflow, onDeleted, onClose]);

	const handleClone = useCallback(() => {
		if (workflow) {
			onClone(workflow);
		}
	}, [workflow, onClone]);

	const handleEdit = useCallback(() => {
		if (workflow && !workflow.isBuiltin) {
			window.dispatchEvent(
				new CustomEvent('orc:edit-workflow', { detail: { workflow } })
			);
		}
	}, [workflow]);

	if (!workflow) {
		return null;
	}

	const displayWorkflow = details?.workflow || workflow;
	const phases = details?.phases || [];
	const variables = details?.variables || [];

	return (
		<RightPanel isOpen={isOpen} onClose={onClose}>
			{/* Header Section */}
			<RightPanel.Section id="workflow-header">
				<div className="workflow-detail-header">
					<div className="workflow-detail-header-icon">
						<Icon name="workflow" size={20} />
					</div>
					<div className="workflow-detail-header-info">
						<h2 className="workflow-detail-title">{displayWorkflow.name}</h2>
						<code className="workflow-detail-id">{displayWorkflow.id}</code>
					</div>
					{displayWorkflow.isBuiltin && (
						<span className="workflow-detail-badge builtin">Built-in</span>
					)}
				</div>

				{displayWorkflow.description && (
					<p className="workflow-detail-description">{displayWorkflow.description}</p>
				)}

				<div className="workflow-detail-meta">
					<span className="workflow-detail-meta-item">
						<Icon name="layers" size={12} />
						{displayWorkflow.workflowType}
					</span>
					{displayWorkflow.defaultModel && (
						<span className="workflow-detail-meta-item">
							<Icon name="robot" size={12} />
							{displayWorkflow.defaultModel}
						</span>
					)}
					{displayWorkflow.defaultThinking && (
						<span className="workflow-detail-meta-item">
							<Icon name="brain" size={12} />
							Thinking
						</span>
					)}
					{displayWorkflow.basedOn && (
						<span className="workflow-detail-meta-item">
							<Icon name="git-branch" size={12} />
							from {displayWorkflow.basedOn}
						</span>
					)}
				</div>

				{/* Actions */}
				<div className="workflow-detail-actions">
					{!displayWorkflow.isBuiltin && (
						<Button
							variant="primary"
							size="sm"
							leftIcon={<Icon name="edit" size={12} />}
							onClick={handleEdit}
						>
							Edit
						</Button>
					)}
					<Button
						variant="secondary"
						size="sm"
						leftIcon={<Icon name="copy" size={12} />}
						onClick={handleClone}
					>
						Clone
					</Button>
					{!displayWorkflow.isBuiltin && (
						<Button
							variant="danger"
							size="sm"
							leftIcon={<Icon name="trash" size={12} />}
							onClick={handleDelete}
							disabled={deleting}
						>
							{deleting ? 'Deleting...' : 'Delete'}
						</Button>
					)}
				</div>
			</RightPanel.Section>

			{/* Loading State */}
			{loading && (
				<div className="workflow-detail-loading">
					<Icon name="loader" size={16} className="spinning" />
					<span>Loading details...</span>
				</div>
			)}

			{/* Error State */}
			{error && (
				<div className="workflow-detail-error">
					<Icon name="alert-circle" size={14} />
					<span>{error}</span>
				</div>
			)}

			{/* Phases Section */}
			{!loading && !error && phases.length > 0 && (
				<RightPanel.Section id="workflow-phases" defaultCollapsed={false}>
					<RightPanel.Header
						title="Phases"
						icon="layers"
						iconColor="cyan"
						count={phases.length}
						badgeColor="cyan"
					/>
					<RightPanel.Body>
						<div className="workflow-detail-phases">
							{phases
								.sort((a, b) => a.sequence - b.sequence)
								.map((phase, index) => (
									<PhaseItem key={phase.id} phase={phase} index={index} />
								))}
						</div>
					</RightPanel.Body>
				</RightPanel.Section>
			)}

			{/* Variables Section */}
			{!loading && !error && variables.length > 0 && (
				<RightPanel.Section id="workflow-variables" defaultCollapsed={true}>
					<RightPanel.Header
						title="Variables"
						icon="code"
						iconColor="purple"
						count={variables.length}
						badgeColor="purple"
					/>
					<RightPanel.Body>
						<div className="workflow-detail-variables">
							{variables.map((variable) => (
								<VariableItem key={variable.id} variable={variable} />
							))}
						</div>
					</RightPanel.Body>
				</RightPanel.Section>
			)}
		</RightPanel>
	);
}
