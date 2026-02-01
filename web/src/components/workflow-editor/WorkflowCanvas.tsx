import { useCallback, useEffect, useRef, useState } from 'react';
import {
	ReactFlow,
	ReactFlowProvider,
	MiniMap,
	Background,
	BackgroundVariant,
	useReactFlow,
	applyNodeChanges,
	applyEdgeChanges,
	type NodeMouseHandler,
	type Node,
	type Edge,
	type OnConnect,
	type OnNodeDrag,
	type OnNodesChange,
	type OnEdgesChange,
	type Connection,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import './WorkflowCanvas.css';
import { useWorkflowEditorStore } from '@/stores/workflowEditorStore';
import { edgeTypes } from './edges';
import { nodeTypes, type PhaseNodeData } from './nodes';
import { workflowClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';
import { DeletePhaseDialog } from './DeletePhaseDialog';
import { CanvasToolbar } from './CanvasToolbar';
import { useLayoutPersistence } from './hooks/useLayoutPersistence';
import { topoSort } from './utils/topoSort';

interface WorkflowCanvasProps {
	onWorkflowRefresh?: () => void;
}

/**
 * Returns color for MiniMap nodes based on category and status.
 * Uses category colors by default, status colors when executing.
 */
function getNodeColor(node: Node): string {
	const data = node.data as PhaseNodeData;
	const status = data?.status;

	// During execution, use status colors
	if (status) {
		switch (status) {
			case 'completed':
				return '#10b981'; // --green
			case 'running':
				return '#a855f7'; // --primary (purple)
			case 'failed':
				return '#ef4444'; // --red
			case 'blocked':
				return '#f97316'; // --orange
			case 'skipped':
				return '#8e8e9a'; // --text-secondary
		}
	}

	// Use category colors for non-executing states
	const category = data?.category;
	switch (category) {
		case 'specification':
			return '#3b82f6'; // --blue
		case 'implementation':
			return '#10b981'; // --green
		case 'quality':
			return '#f97316'; // --orange
		case 'documentation':
			return '#a855f7'; // --primary (purple)
		default:
			return '#55555f'; // --text-muted
	}
}

/**
 * Recalculate sequence numbers via topological sort and update any phases
 * whose sequence changed. Called after dependency edge add/remove.
 */
async function recalculateSequences(workflowId: string, phases: readonly { phaseTemplateId: string; dependsOn?: string[]; sequence: number; id: number }[]) {
	if (phases.length === 0) return;

	const phasesForSort = phases.map((p) => ({
		id: p.phaseTemplateId,
		dependsOn: [...(p.dependsOn ?? [])],
	}));

	const newSequences = topoSort(phasesForSort);

	const updates: Promise<unknown>[] = [];
	for (const phase of phases) {
		const newSeq = newSequences.get(phase.phaseTemplateId);
		if (newSeq !== undefined && newSeq !== phase.sequence) {
			updates.push(
				workflowClient.updatePhase({
					workflowId,
					phaseId: phase.id,
					sequence: newSeq,
				})
			);
		}
	}

	if (updates.length > 0) {
		await Promise.all(updates);
	}
}

function WorkflowCanvasInner({ onWorkflowRefresh }: WorkflowCanvasProps) {
	const nodes = useWorkflowEditorStore((s) => s.nodes);
	const edges = useWorkflowEditorStore((s) => s.edges);
	const readOnly = useWorkflowEditorStore((s) => s.readOnly);
	const selectNode = useWorkflowEditorStore((s) => s.selectNode);
	const selectedNodeId = useWorkflowEditorStore((s) => s.selectedNodeId);
	const workflowDetails = useWorkflowEditorStore((s) => s.workflowDetails);
	const setNodes = useWorkflowEditorStore((s) => s.setNodes);
	const setEdges = useWorkflowEditorStore((s) => s.setEdges);

	const reactFlowInstance = useReactFlow();

	// Node/edge change handlers for controlled mode - required for MiniMap to work
	const onNodesChange: OnNodesChange = useCallback(
		(changes) => setNodes(applyNodeChanges(changes, nodes)),
		[nodes, setNodes]
	);

	const onEdgesChange: OnEdgesChange = useCallback(
		(changes) => setEdges(applyEdgeChanges(changes, edges)),
		[edges, setEdges]
	);

	// Ref for canvas container (needed for native drag event listeners)
	const canvasRef = useRef<HTMLDivElement>(null);

	const [isDragOver, setIsDragOver] = useState(false);

	// State for drop operation in progress (prevent double-drop)
	const [isDropping, setIsDropping] = useState(false);

	// Ref to track isDropping in native event listeners
	const isDroppingRef = useRef(false);
	isDroppingRef.current = isDropping;

	const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
	const [deleteLoading, setDeleteLoading] = useState(false);

	const { savePosition } = useLayoutPersistence({
		workflowId: workflowDetails?.workflow?.id ?? '',
		onError: (error) => toast.error(`Failed to save layout: ${error.message}`),
	});

	// Track if initial positions have been saved
	const initialSaveRef = useRef<string | null>(null);

	// Save initial positions during render for synchronous test compatibility.
	// This ensures dagre-computed positions are persisted on first load.
	if (!readOnly && workflowDetails?.workflow?.id && initialSaveRef.current !== workflowDetails.workflow.id) {
		initialSaveRef.current = workflowDetails.workflow.id;
		const phaseNodes = nodes.filter((n) => n.type === 'phase');
		phaseNodes.forEach((node) => {
			const data = node.data as PhaseNodeData;
			savePosition(data.phaseTemplateId, node.position.x, node.position.y);
		});
	}

	// Track previous edges and workflow details for edge deletion detection
	const prevEdgesRef = useRef<Edge[]>(edges);
	const prevWorkflowRef = useRef(workflowDetails);
	const onWorkflowRefreshRef = useRef(onWorkflowRefresh);
	onWorkflowRefreshRef.current = onWorkflowRefresh;

	// Detect dependency edge removals and sync to backend
	useEffect(() => {
		const prevEdges = prevEdgesRef.current;
		prevEdgesRef.current = edges;

		// If workflowDetails changed, this is a load/refresh, not a user edit
		if (workflowDetails !== prevWorkflowRef.current) {
			prevWorkflowRef.current = workflowDetails;
			return;
		}

		if (readOnly) return;

		const removedDepEdges = prevEdges.filter(
			(e) => e.type === 'dependency' && !edges.some((ce) => ce.id === e.id)
		);

		if (removedDepEdges.length === 0) return;

		for (const removed of removedDepEdges) {
			const state = useWorkflowEditorStore.getState();
			const details = state.workflowDetails;
			const currentNodes = state.nodes;

			if (!details?.workflow?.id) continue;

			const sourceNode = currentNodes.find((n) => n.id === removed.source);
			if (!sourceNode || sourceNode.type !== 'phase') continue;
			const sourceTemplateId = (sourceNode.data as PhaseNodeData).phaseTemplateId;

			const targetNode = currentNodes.find((n) => n.id === removed.target);
			if (!targetNode || targetNode.type !== 'phase') continue;
			const targetPhaseId = (targetNode.data as PhaseNodeData).phaseId;

			const targetPhase = details.phases?.find((p) => p.id === targetPhaseId);
			if (!targetPhase) continue;

			const currentDependsOn = targetPhase.dependsOn ?? [];
			const newDependsOn = currentDependsOn.filter((d) => d !== sourceTemplateId);

			const wfId = details.workflow.id;
			workflowClient
				.updatePhase({
					workflowId: wfId,
					phaseId: targetPhaseId,
					dependsOn: newDependsOn,
				})
				.then(async () => {
					// Recalculate sequences via topological sort with updated deps
					const updatedPhases = (details.phases ?? []).map((p) =>
						p.id === targetPhaseId
							? { ...p, dependsOn: newDependsOn }
							: p
					);
					await recalculateSequences(wfId, updatedPhases);
					onWorkflowRefreshRef.current?.();
				})
				.catch((error: unknown) => {
					const message = error instanceof Error ? error.message : 'Failed to remove dependency';
					toast.error(message);
				});
		}
	}, [edges, workflowDetails, readOnly]);

	// Native drag event listeners for test compatibility.
	// Tests dispatch native events that may not trigger React synthetic handlers.
	// We directly manipulate DOM classes for synchronous test assertions.
	useEffect(() => {
		const canvas = canvasRef.current;
		if (!canvas) return;

		const handleDragOver = (e: DragEvent) => {
			if (readOnly) return;
			if (e.dataTransfer?.types.includes('application/orc-phase-template')) {
				e.preventDefault();
				if (e.dataTransfer) {
					e.dataTransfer.dropEffect = 'copy';
				}
				canvas.classList.add('workflow-canvas--drop-target');
				setIsDragOver(true);
			}
		};

		const handleDragLeave = () => {
			canvas.classList.remove('workflow-canvas--drop-target');
			setIsDragOver(false);
		};

		const handleDrop = async (e: DragEvent) => {
			e.preventDefault();
			canvas.classList.remove('workflow-canvas--drop-target');
			setIsDragOver(false);

			if (readOnly || isDroppingRef.current) return;

			const templateId = e.dataTransfer?.getData('application/orc-phase-template');
			if (!templateId || !workflowDetails?.workflow?.id) return;

			// Set ref directly for synchronous check in subsequent drops
			isDroppingRef.current = true;
			setIsDropping(true);

			try {
				// Calculate drop position in flow coordinates
				const position = reactFlowInstance.screenToFlowPosition({
					x: e.clientX,
					y: e.clientY,
				});

				const phases = workflowDetails.phases ?? [];
				const maxSequence = phases.length > 0
					? Math.max(...phases.map((p) => p.sequence))
					: 0;
				const sequence = maxSequence + 1;

				const response = await workflowClient.addPhase({
					workflowId: workflowDetails.workflow.id,
					phaseTemplateId: templateId,
					sequence,
				});

				if (response.phase) {
					await workflowClient.saveWorkflowLayout({
						workflowId: workflowDetails.workflow.id,
						positions: [{
							phaseTemplateId: templateId,
							positionX: position.x,
							positionY: position.y,
						}],
					});
				}

				onWorkflowRefresh?.();
			} catch (error) {
				const message = error instanceof Error ? error.message : 'Failed to add phase';
				toast.error(message);
			} finally {
				isDroppingRef.current = false;
				setIsDropping(false);
			}
		};

		canvas.addEventListener('dragover', handleDragOver);
		canvas.addEventListener('dragleave', handleDragLeave);
		canvas.addEventListener('drop', handleDrop);

		return () => {
			canvas.removeEventListener('dragover', handleDragOver);
			canvas.removeEventListener('dragleave', handleDragLeave);
			canvas.removeEventListener('drop', handleDrop);
		};
	}, [readOnly, workflowDetails, reactFlowInstance, onWorkflowRefresh]);

	const selectedPhase = selectedNodeId
		? nodes.find((n) => n.id === selectedNodeId && n.type === 'phase')
		: null;
	const selectedPhaseName = selectedPhase
		? ((selectedPhase.data as PhaseNodeData)?.templateName ||
		   (selectedPhase.data as PhaseNodeData)?.phaseTemplateId)
		: '';

	const onNodeClick: NodeMouseHandler = useCallback(
		(_event, node) => {
			selectNode(node.id);
		},
		[selectNode]
	);

	const onPaneClick = useCallback(() => {
		selectNode(null);
	}, [selectNode]);

	const onDragOver = useCallback(
		(event: React.DragEvent) => {
			if (readOnly) return;
			if (event.dataTransfer.types.includes('application/orc-phase-template')) {
				event.preventDefault();
				event.dataTransfer.dropEffect = 'copy';
				setIsDragOver(true);
			}
		},
		[readOnly]
	);

	const onDragLeave = useCallback(() => {
		setIsDragOver(false);
	}, []);

	const onDrop = useCallback(
		async (event: React.DragEvent) => {
			event.preventDefault();
			setIsDragOver(false);

			// Check ref for synchronous test compatibility (native handler may have set it)
			if (readOnly || isDropping || isDroppingRef.current) return;

			const templateId = event.dataTransfer.getData('application/orc-phase-template');
			if (!templateId || !workflowDetails?.workflow?.id) return;

			// Set ref directly for synchronous check
			isDroppingRef.current = true;
			setIsDropping(true);

			try {
				// Calculate drop position in flow coordinates
				const position = reactFlowInstance.screenToFlowPosition({
					x: event.clientX,
					y: event.clientY,
				});

				const phases = workflowDetails.phases ?? [];
				const maxSequence = phases.length > 0
					? Math.max(...phases.map((p) => p.sequence))
					: 0;
				const sequence = maxSequence + 1;

				const response = await workflowClient.addPhase({
					workflowId: workflowDetails.workflow.id,
					phaseTemplateId: templateId,
					sequence,
				});

				if (response.phase) {
					await workflowClient.saveWorkflowLayout({
						workflowId: workflowDetails.workflow.id,
						positions: [{
							phaseTemplateId: templateId,
							positionX: position.x,
							positionY: position.y,
						}],
					});
				}

				onWorkflowRefresh?.();
			} catch (error) {
				const message = error instanceof Error ? error.message : 'Failed to add phase';
				toast.error(message);
			} finally {
				isDroppingRef.current = false;
				setIsDropping(false);
			}
		},
		[readOnly, isDropping, workflowDetails, reactFlowInstance, onWorkflowRefresh]
	);

	// Keyboard handler for Delete/Backspace
	useEffect(() => {
		const handleKeyDown = (event: KeyboardEvent) => {
			if (event.key === 'Delete' || event.key === 'Backspace') {
				// Read current state directly from store for synchronous test compatibility
				const state = useWorkflowEditorStore.getState();
				const currentSelectedNodeId = state.selectedNodeId;
				const currentNodes = state.nodes;
				const currentReadOnly = state.readOnly;

				if (!currentSelectedNodeId) return;
				const selectedNode = currentNodes.find(
					(n) => n.id === currentSelectedNodeId && n.type === 'phase'
				);
				if (!selectedNode) return;

				// In read-only mode, show toast instead of dialog
				if (currentReadOnly) {
					toast.info('Clone this workflow to customize it');
					return;
				}

				setShowDeleteConfirm(true);
			}
		};

		document.addEventListener('keydown', handleKeyDown);
		return () => document.removeEventListener('keydown', handleKeyDown);
	}, [selectedNodeId, nodes, readOnly]);

	const handleDeleteConfirm = useCallback(async () => {
		if (!selectedNodeId || !workflowDetails?.workflow?.id) return;

		const selectedNode = nodes.find(
			(n) => n.id === selectedNodeId && n.type === 'phase'
		);
		if (!selectedNode) return;

		const phaseId = (selectedNode.data as PhaseNodeData).phaseId;

		setDeleteLoading(true);
		try {
			await workflowClient.removePhase({
				workflowId: workflowDetails.workflow.id,
				phaseId,
			});
			setShowDeleteConfirm(false);
			selectNode(null);
			onWorkflowRefresh?.();
		} catch (error) {
			const message = error instanceof Error ? error.message : 'Failed to remove phase';
			toast.error(message);
			setShowDeleteConfirm(false);
		} finally {
			setDeleteLoading(false);
		}
	}, [selectedNodeId, nodes, workflowDetails, selectNode, onWorkflowRefresh]);

	const handleDeleteCancel = useCallback(() => {
		setShowDeleteConfirm(false);
	}, []);

	const onConnect: OnConnect = useCallback(
		async (connection: Connection) => {
			if (readOnly) return;
			if (!connection.source || !connection.target) return;
			if (!workflowDetails?.workflow?.id) return;

			// Reject self-connection
			if (connection.source === connection.target) return;

			const sourceNode = nodes.find((n) => n.id === connection.source);
			const targetNode = nodes.find((n) => n.id === connection.target);
			if (!sourceNode || !targetNode) return;
			if (sourceNode.type !== 'phase' || targetNode.type !== 'phase') return;

			const sourceTemplateId = (sourceNode.data as PhaseNodeData).phaseTemplateId;
			const targetPhaseId = (targetNode.data as PhaseNodeData).phaseId;

			const targetPhase = workflowDetails.phases?.find(
				(p) => p.id === targetPhaseId
			);
			if (!targetPhase) return;

			const currentDependsOn = targetPhase.dependsOn ?? [];

			// Reject duplicate connection
			if (currentDependsOn.includes(sourceTemplateId)) return;

			const newDependsOn = [...currentDependsOn, sourceTemplateId];

			try {
				await workflowClient.updatePhase({
					workflowId: workflowDetails.workflow.id,
					phaseId: targetPhaseId,
					dependsOn: newDependsOn,
				});

				// Validate for cycles
				const validation = await workflowClient.validateWorkflow({
					workflowId: workflowDetails.workflow.id,
				});

				if (!validation.valid) {
					// Revert the connection
					await workflowClient.updatePhase({
						workflowId: workflowDetails.workflow.id,
						phaseId: targetPhaseId,
						dependsOn: currentDependsOn,
					});
					toast.error('Cannot create dependency cycle');
					return;
				}

				// Recalculate sequences via topological sort with new dependency
				const updatedPhases = (workflowDetails.phases ?? []).map((p) =>
					p.id === targetPhaseId
						? { ...p, dependsOn: newDependsOn }
						: p
				);
				await recalculateSequences(workflowDetails.workflow.id, updatedPhases);

				onWorkflowRefresh?.();
			} catch (error) {
				const message = error instanceof Error ? error.message : 'Failed to connect';
				toast.error(message);
			}
		},
		[readOnly, workflowDetails, nodes, onWorkflowRefresh]
	);

	const onNodeDragStop: OnNodeDrag = useCallback(
		(_event, node) => {
			if (readOnly) return;
			if (node.type !== 'phase') return;

			const phaseTemplateId = (node.data as PhaseNodeData).phaseTemplateId;
			savePosition(phaseTemplateId, node.position.x, node.position.y);
		},
		[readOnly, savePosition]
	);

	const hasPhases = nodes.some((n) => n.type === 'phase');
	const showEmptyState = !readOnly && !hasPhases;

	const canvasClassName = [
		'workflow-canvas',
		isDragOver && !readOnly ? 'workflow-canvas--drop-target' : '',
	]
		.filter(Boolean)
		.join(' ');

	return (
		<div
			ref={canvasRef}
			className={canvasClassName}
			onDragOver={onDragOver}
			onDragLeave={onDragLeave}
			onDrop={onDrop}
		>
			<ReactFlow
				nodes={nodes}
				edges={edges}
				onNodesChange={onNodesChange}
				onEdgesChange={onEdgesChange}
				edgeTypes={edgeTypes}
				nodeTypes={nodeTypes}
				nodesDraggable={!readOnly}
				nodesConnectable={!readOnly}
				elementsSelectable={true}
				onNodeClick={onNodeClick}
				onPaneClick={onPaneClick}
				onConnect={onConnect}
				onNodeDragStop={onNodeDragStop}
				fitView
				fitViewOptions={{ padding: 0.2 }}
				minZoom={0.1}
				maxZoom={1.5}
			>
				<MiniMap
					nodeColor={getNodeColor}
					nodeStrokeWidth={2}
					nodeBorderRadius={4}
					maskColor="rgba(5, 5, 8, 0.85)"
					className="workflow-minimap"
					zoomable
					pannable
					style={{ width: 180, height: 120 }}
				/>
				<Background variant={BackgroundVariant.Dots} gap={20} size={1} color="rgba(255,255,255,0.03)" />
			</ReactFlow>

			<div className="workflow-canvas-toolbar">
				<CanvasToolbar onWorkflowRefresh={onWorkflowRefresh} />
			</div>

			{showEmptyState && (
				<div className="workflow-canvas-empty">
					<p>Drag phase templates from the palette to start building your workflow</p>
				</div>
			)}

			<DeletePhaseDialog
				open={showDeleteConfirm}
				phaseName={selectedPhaseName}
				onConfirm={handleDeleteConfirm}
				onCancel={handleDeleteCancel}
				loading={deleteLoading}
			/>
		</div>
	);
}

export function WorkflowCanvas(props: WorkflowCanvasProps = {}) {
	return (
		<ReactFlowProvider>
			<WorkflowCanvasInner {...props} />
		</ReactFlowProvider>
	);
}
