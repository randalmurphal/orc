import { useEffect, useState, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import { workflowClient } from '@/lib/client';
import { useWorkflowEditorStore } from '@/stores/workflowEditorStore';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import type { PhaseNodeData } from './nodes';
import { WorkflowCanvas } from './WorkflowCanvas';
import { PhaseTemplatePalette } from './panels/PhaseTemplatePalette';
import './WorkflowEditorPage.css';

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

export function WorkflowEditorPage() {
	const { id } = useParams<{ id: string }>();
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const workflowDetails = useWorkflowEditorStore((s) => s.workflowDetails);
	const selectedNodeId = useWorkflowEditorStore((s) => s.selectedNodeId);
	const nodes = useWorkflowEditorStore((s) => s.nodes);
	const loadFromWorkflow = useWorkflowEditorStore((s) => s.loadFromWorkflow);
	const reset = useWorkflowEditorStore((s) => s.reset);

	const fetchWorkflow = useCallback(async () => {
		if (!id) return;
		setLoading(true);
		setError(null);
		try {
			const response = await workflowClient.getWorkflow({ id });
			if (!response.workflow) {
				setError('Workflow not found');
				return;
			}
			loadFromWorkflow(response.workflow);
		} catch (err) {
			const message =
				err instanceof Error ? err.message : 'Failed to load workflow';
			if (message.includes('not found') || message.includes('404')) {
				setError('Workflow not found');
			} else {
				setError(message);
			}
		} finally {
			setLoading(false);
		}
	}, [id, loadFromWorkflow]);

	useEffect(() => {
		fetchWorkflow();
		return () => reset();
	}, [fetchWorkflow, reset]);

	if (loading) {
		return (
			<div className="workflow-editor-page">
				<div className="workflow-editor-loading">Loading workflow...</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className="workflow-editor-page">
				<div className="workflow-editor-error">
					<h2>{error}</h2>
					<p>
						{error === 'Workflow not found'
							? 'The requested workflow does not exist.'
							: 'Something went wrong loading this workflow.'}
					</p>
					<Link to="/workflows">Back to Workflows</Link>
					{error !== 'Workflow not found' && (
						<button onClick={fetchWorkflow}>Retry</button>
					)}
				</div>
			</div>
		);
	}

	const workflow = workflowDetails?.workflow;
	const workflowName = workflow?.name || id || 'Workflow';
	const isBuiltin = workflow?.isBuiltin ?? false;
	const inspectorOpen = selectedNodeId !== null;

	// Find selected node data for the inspector panel
	const selectedNode = selectedNodeId
		? nodes.find((n) => n.id === selectedNodeId)
		: null;
	const selectedPhaseData = selectedNode
		? (selectedNode.data as unknown as PhaseNodeData)
		: null;

	const handleClone = () => {
		if (workflow) {
			window.dispatchEvent(
				new CustomEvent('orc:clone-workflow', { detail: { workflow } })
			);
		}
	};

	const bodyClasses = ['workflow-editor-body'];
	if (inspectorOpen) bodyClasses.push('workflow-editor-body--inspector-open');

	return (
		<div className="workflow-editor-page">
			<div className="workflow-editor-header">
				<nav className="workflow-editor-breadcrumb">
					<Link to="/workflows">Workflows</Link>
					<span className="workflow-editor-breadcrumb-separator">/</span>
					<span className="workflow-editor-breadcrumb-current">
						{workflowName}
					</span>
				</nav>
				<div className="workflow-editor-header-actions">
					{isBuiltin && (
						<span className="workflow-editor-badge">Built-in</span>
					)}
					{isBuiltin && (
						<button
							className="workflow-editor-clone-btn"
							onClick={handleClone}
						>
							Clone
						</button>
					)}
				</div>
			</div>
			<div className={bodyClasses.join(' ')}>
				<aside className="workflow-editor-palette">
					<PhaseTemplatePalette readOnly={isBuiltin} workflowId={id || ''} />
				</aside>
				<div className="workflow-editor-canvas">
					<WorkflowCanvas />
				</div>
				{inspectorOpen && selectedPhaseData && (
					<aside className="workflow-editor-inspector">
						<div className="workflow-editor-inspector-header">
							<h3 className="workflow-editor-inspector-title">
								{selectedPhaseData.templateName || selectedPhaseData.phaseTemplateId}
							</h3>
						</div>
						{selectedPhaseData.description && (
							<p className="workflow-editor-inspector-description">
								{selectedPhaseData.description}
							</p>
						)}
						<div className="workflow-editor-inspector-details">
							<div className="workflow-editor-inspector-field">
								<span className="workflow-editor-inspector-label">Gate Type</span>
								<span className="workflow-editor-inspector-value">
									{formatGateType(selectedPhaseData.gateType)}
								</span>
							</div>
							<div className="workflow-editor-inspector-field">
								<span className="workflow-editor-inspector-label">Max Iterations</span>
								<span className="workflow-editor-inspector-value">
									{selectedPhaseData.maxIterations}
								</span>
							</div>
							{selectedPhaseData.modelOverride && (
								<div className="workflow-editor-inspector-field">
									<span className="workflow-editor-inspector-label">Model Override</span>
									<span className="workflow-editor-inspector-value">
										{selectedPhaseData.modelOverride}
									</span>
								</div>
							)}
						</div>
					</aside>
				)}
			</div>
		</div>
	);
}
