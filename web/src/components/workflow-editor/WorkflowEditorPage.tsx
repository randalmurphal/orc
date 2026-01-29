import { useEffect, useState, useCallback, useRef } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import { workflowClient } from '@/lib/client';
import { useWorkflowEditorStore } from '@/stores/workflowEditorStore';
import { RunStatus, type WorkflowRunWithDetails, type Workflow } from '@/gen/orc/v1/workflow_pb';
import { PhaseStatus } from '@/gen/orc/v1/task_pb';
import type { PhaseNodeData, PhaseStatus as UIPhaseStatus } from './nodes';
import { WorkflowCanvas } from './WorkflowCanvas';
import { PhaseTemplatePalette } from './panels/PhaseTemplatePalette';
import { PhaseInspector } from './panels/PhaseInspector';
import { ExecutionHeader } from './ExecutionHeader';
import { CloneWorkflowModal } from '@/components/workflows/CloneWorkflowModal';
import { formatDuration } from '@/stores/sessionStore';
import './WorkflowEditorPage.css';

/**
 * Map proto PhaseStatus to UI PhaseStatus
 *
 * AMENDMENT AMEND-001: Proto PhaseStatus only has UNSPECIFIED(0), PENDING(1), COMPLETED(3), SKIPPED(7)
 * Values RUNNING, FAILED, BLOCKED were removed - these are now derived from context:
 * - 'running': derived when this phase is the current running phase
 * - 'failed': derived when the phase has an error (future: would need error info from run)
 * - 'blocked': derived from gate blocking conditions (future)
 */
function mapPhaseStatusToUI(
	protoStatus: PhaseStatus,
	isCurrentPhase: boolean = false
): UIPhaseStatus {
	switch (protoStatus) {
		case PhaseStatus.COMPLETED:
			return 'completed';
		case PhaseStatus.SKIPPED:
			return 'skipped';
		case PhaseStatus.PENDING:
		case PhaseStatus.UNSPECIFIED:
		default:
			// If this is the current phase in a running run, it's "running"
			return isCurrentPhase ? 'running' : 'pending';
	}
}

export function WorkflowEditorPage() {
	const { id } = useParams<{ id: string }>();
	const navigate = useNavigate();
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const workflowDetails = useWorkflowEditorStore((s) => s.workflowDetails);
	const selectedNodeId = useWorkflowEditorStore((s) => s.selectedNodeId);
	const nodes = useWorkflowEditorStore((s) => s.nodes);
	const loadFromWorkflow = useWorkflowEditorStore((s) => s.loadFromWorkflow);
	const reset = useWorkflowEditorStore((s) => s.reset);

	// Clone modal state (QA-002 fix)
	const [cloneModalOpen, setCloneModalOpen] = useState(false);

	// Execution tracking state (TASK-639)
	const activeRun = useWorkflowEditorStore((s) => s.activeRun);
	const setActiveRun = useWorkflowEditorStore((s) => s.setActiveRun);
	const updateNodeStatus = useWorkflowEditorStore((s) => s.updateNodeStatus);
	const updateEdgesForActivePhase = useWorkflowEditorStore((s) => s.updateEdgesForActivePhase);
	const clearExecution = useWorkflowEditorStore((s) => s.clearExecution);

	// Duration ticker state
	const [durationTick, setDurationTick] = useState(0);
	const durationIntervalRef = useRef<number | null>(null);

	// Compute duration from run start time
	const runStartTime = activeRun?.run?.startedAt
		? new Date(Number(activeRun.run.startedAt.seconds) * 1000)
		: null;
	const duration = runStartTime ? formatDuration(runStartTime) : '0s';

	// Compute metrics from active run
	const totalTokens = (activeRun?.run?.totalInputTokens ?? 0) + (activeRun?.run?.totalOutputTokens ?? 0);
	const totalCost = activeRun?.run?.totalCostUsd ?? 0;
	const runStatus = activeRun?.run?.status ?? RunStatus.PENDING;

	// Start/stop duration ticker
	useEffect(() => {
		if (activeRun?.run?.status === RunStatus.RUNNING) {
			durationIntervalRef.current = window.setInterval(() => {
				setDurationTick((t) => t + 1);
			}, 1000);
		}
		return () => {
			if (durationIntervalRef.current) {
				clearInterval(durationIntervalRef.current);
				durationIntervalRef.current = null;
			}
		};
	}, [activeRun?.run?.status]);

	// Apply run phase statuses to nodes when run changes
	const applyRunPhasesToNodes = useCallback(
		(run: WorkflowRunWithDetails) => {
			if (!run.phases) return;

			// The current phase is derived from run.run.currentPhase
			const currentPhaseName = run.run?.currentPhase ?? '';
			let activePhaseTemplateId: string | null = null;

			for (const phase of run.phases) {
				// AMEND-001: Derive 'running' status from whether this is the current phase
				const isCurrentPhase = phase.phaseTemplateId === currentPhaseName;
				const uiStatus = mapPhaseStatusToUI(phase.status, isCurrentPhase);
				updateNodeStatus(phase.phaseTemplateId, uiStatus, {
					costUsd: phase.costUsd,
					iterations: phase.iterations,
				});

				// Track active phase for edge animations
				if (isCurrentPhase && phase.status === PhaseStatus.PENDING) {
					activePhaseTemplateId = phase.phaseTemplateId;
				}
			}

			// Update edge animations for active phase
			updateEdgesForActivePhase(activePhaseTemplateId);
		},
		[updateNodeStatus, updateEdgesForActivePhase]
	);

	const fetchActiveRun = useCallback(async () => {
		if (!id) return;
		try {
			// List runs for this workflow, looking for a running one
			const response = await workflowClient.listWorkflowRuns({
				workflowId: id,
				status: RunStatus.RUNNING,
				page: { page: 1, limit: 1 }, // First page, limit to 1 result
			});

			if (response.runs && response.runs.length > 0) {
				// Found a running run - fetch its details
				const runId = response.runs[0].id;
				const detailsResponse = await workflowClient.getWorkflowRun({ id: runId });
				if (detailsResponse.run) {
					setActiveRun(detailsResponse.run);
					applyRunPhasesToNodes(detailsResponse.run);
				}
			}
		} catch (err) {
			// Failed to fetch run - non-fatal, just means no active run display
			console.warn('Failed to fetch active run:', err);
		}
	}, [id, setActiveRun, applyRunPhasesToNodes]);

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

	// Fetch active run after workflow loads
	useEffect(() => {
		if (!loading && workflowDetails) {
			fetchActiveRun();
		}
	}, [loading, workflowDetails, fetchActiveRun]);

	// Cancel handler
	const handleCancel = useCallback(async () => {
		if (!activeRun?.run?.id) {
			throw new Error('No active run to cancel');
		}
		const runId = activeRun.run.id;
		await workflowClient.cancelWorkflowRun({ id: runId });
		// Refresh run state
		const response = await workflowClient.getWorkflowRun({ id: runId });
		if (response.run) {
			setActiveRun(response.run);
		} else {
			clearExecution();
		}
	}, [activeRun, setActiveRun, clearExecution]);

	// Clone modal handlers (QA-002 fix) - must be before early returns
	const handleClone = useCallback(() => {
		setCloneModalOpen(true);
	}, []);

	const handleCloneModalClose = useCallback(() => {
		setCloneModalOpen(false);
	}, []);

	const handleWorkflowCloned = useCallback((clonedWorkflow: Workflow) => {
		setCloneModalOpen(false);
		// Navigate to the cloned workflow
		navigate(`/workflows/${clonedWorkflow.id}`);
	}, [navigate]);

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

	// Find selected phase for the inspector panel
	const selectedNode = selectedNodeId
		? nodes.find((n) => n.id === selectedNodeId)
		: null;
	const selectedPhaseData = selectedNode
		? (selectedNode.data as unknown as PhaseNodeData)
		: null;
	// Find the actual WorkflowPhase from workflowDetails
	const selectedPhase = selectedPhaseData
		? workflowDetails?.phases.find((p) => p.id === selectedPhaseData.phaseId) ?? null
		: null;

	const bodyClasses = ['workflow-editor-body'];
	if (inspectorOpen) bodyClasses.push('workflow-editor-body--inspector-open');

	// Suppress unused variable warning - durationTick is used to trigger re-renders
	void durationTick;

	// Show execution header when there's an active run
	const showExecutionHeader = activeRun !== null;

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
					{isBuiltin && !showExecutionHeader && (
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
			{showExecutionHeader && (
				<ExecutionHeader
					runStatus={runStatus}
					duration={duration}
					totalTokens={totalTokens}
					totalCost={totalCost}
					onCancel={handleCancel}
				/>
			)}
			<div className={bodyClasses.join(' ')}>
				<aside className="workflow-editor-palette">
					<PhaseTemplatePalette readOnly={isBuiltin} workflowId={id || ''} />
				</aside>
				<div className="workflow-editor-canvas">
					<WorkflowCanvas />
				</div>
				{inspectorOpen && (
					<aside className="workflow-editor-inspector">
						<PhaseInspector
							phase={selectedPhase}
							workflowDetails={workflowDetails}
							readOnly={isBuiltin}
							onWorkflowRefresh={fetchWorkflow}
						/>
					</aside>
				)}
			</div>

			{/* Clone modal (QA-002 fix) */}
			<CloneWorkflowModal
				open={cloneModalOpen}
				workflow={workflow ?? null}
				onClose={handleCloneModalClose}
				onCloned={handleWorkflowCloned}
			/>
		</div>
	);
}
