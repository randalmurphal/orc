import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import type { Node, Edge } from '@xyflow/react';
import type { WorkflowWithDetails, WorkflowRunWithDetails } from '@/gen/orc/v1/workflow_pb';
import { layoutWorkflow } from '@/components/workflow-editor/utils/layoutWorkflow';
import type { PhaseNodeData, PhaseStatus } from '@/components/workflow-editor/nodes';

/** Additional data to update on a node (cost, iterations) */
interface NodeUpdateData {
	costUsd?: number;
	iterations?: number;
}

interface WorkflowEditorStore {
	// State
	nodes: Node[];
	edges: Edge[];
	readOnly: boolean;
	selectedNodeId: string | null;
	selectedEdgeId: string | null;
	workflowDetails: WorkflowWithDetails | null;

	// Execution tracking state (TASK-639)
	activeRun: WorkflowRunWithDetails | null;

	// Actions
	loadFromWorkflow: (details: WorkflowWithDetails) => void;
	setReadOnly: (readOnly: boolean) => void;
	selectNode: (nodeId: string | null) => void;
	selectEdge: (edgeId: string | null) => void;
	setNodes: (nodes: Node[]) => void;
	setEdges: (edges: Edge[]) => void;
	reset: () => void;
	getSelectedEdge: () => Edge | null;

	// Execution tracking actions (TASK-639)
	setActiveRun: (run: WorkflowRunWithDetails | null) => void;
	updateNodeStatus: (phaseTemplateId: string, status: PhaseStatus, data?: NodeUpdateData) => void;
	updateEdgesForActivePhase: (activePhaseTemplateId: string | null) => void;
	clearExecution: () => void;
}

const initialState = {
	nodes: [] as Node[],
	edges: [] as Edge[],
	readOnly: false,
	selectedNodeId: null as string | null,
	selectedEdgeId: null as string | null,
	workflowDetails: null as WorkflowWithDetails | null,
	activeRun: null as WorkflowRunWithDetails | null,
};

export const useWorkflowEditorStore = create<WorkflowEditorStore>()(
	subscribeWithSelector((set) => ({
		...initialState,

		loadFromWorkflow: (details: WorkflowWithDetails) => {
			const { nodes, edges } = layoutWorkflow(details);
			const isBuiltin = details.workflow?.isBuiltin ?? false;
			set((state) => {
				// Preserve selection if the node still exists (e.g. after a settings refresh)
				const preservedSelection =
					state.selectedNodeId && nodes.some((n) => n.id === state.selectedNodeId)
						? state.selectedNodeId
						: null;
				return {
					nodes,
					edges,
					workflowDetails: details,
					readOnly: isBuiltin,
					selectedNodeId: preservedSelection,
					selectedEdgeId: null, // Clear edge selection when loading new workflow
				};
			});
		},

		setReadOnly: (readOnly: boolean) => set({ readOnly }),

		selectNode: (nodeId: string | null) =>
			set({ selectedNodeId: nodeId, selectedEdgeId: null }), // Clear edge when selecting node

		selectEdge: (edgeId: string | null) =>
			set({ selectedEdgeId: edgeId, selectedNodeId: null }), // Clear node when selecting edge

		getSelectedEdge: () => {
			const state = useWorkflowEditorStore.getState();
			if (!state.selectedEdgeId) return null;
			return state.edges.find((e) => e.id === state.selectedEdgeId) ?? null;
		},

		setNodes: (nodes: Node[]) => set({ nodes }),

		setEdges: (edges: Edge[]) => set({ edges }),

		reset: () => set(initialState),

		// Execution tracking actions (TASK-639)
		setActiveRun: (run: WorkflowRunWithDetails | null) => {
			set({ activeRun: run });
		},

		updateNodeStatus: (phaseTemplateId: string, status: PhaseStatus, data?: NodeUpdateData) => {
			set((state) => {
				const nodeIndex = state.nodes.findIndex(
					(n) => n.type === 'phase' && (n.data as PhaseNodeData).phaseTemplateId === phaseTemplateId
				);

				if (nodeIndex === -1) {
					// Phase not found in current workflow - skip silently (template may have changed)
					return state;
				}

				const updatedNodes = [...state.nodes];
				const node = updatedNodes[nodeIndex];
				const currentData = node.data as PhaseNodeData;

				updatedNodes[nodeIndex] = {
					...node,
					data: {
						...currentData,
						status,
						...(data?.costUsd !== undefined ? { costUsd: data.costUsd } : {}),
						...(data?.iterations !== undefined ? { iterations: data.iterations } : {}),
					},
				};

				return { nodes: updatedNodes };
			});
		},

		updateEdgesForActivePhase: (activePhaseTemplateId: string | null) => {
			set((state) => {
				if (!activePhaseTemplateId) {
					// No active phase - disable all animations
					const updatedEdges = state.edges.map((edge) => ({
						...edge,
						animated: false,
					}));
					return { edges: updatedEdges };
				}

				// Find the node ID for the active phase
				const activeNode = state.nodes.find(
					(n) => n.type === 'phase' && (n.data as PhaseNodeData).phaseTemplateId === activePhaseTemplateId
				);

				if (!activeNode) {
					return state;
				}

				// Animate edges that target the active phase node
				const updatedEdges = state.edges.map((edge) => ({
					...edge,
					animated: edge.target === activeNode.id,
				}));

				return { edges: updatedEdges };
			});
		},

		clearExecution: () => {
			set((state) => {
				// Reset all nodes to no execution state
				const clearedNodes = state.nodes.map((node) => {
					if (node.type !== 'phase') return node;
					const data = node.data as PhaseNodeData;
					return {
						...node,
						data: {
							...data,
							status: undefined,
							costUsd: undefined,
							iterations: undefined,
						},
					};
				});

				// Remove all edge animations
				const clearedEdges = state.edges.map((edge) => ({
					...edge,
					animated: false,
				}));

				return {
					nodes: clearedNodes,
					edges: clearedEdges,
					activeRun: null,
				};
			});
		},
	}))
);

// Selector hooks
export const useEditorNodes = () =>
	useWorkflowEditorStore((state) => state.nodes);
export const useEditorEdges = () =>
	useWorkflowEditorStore((state) => state.edges);
export const useEditorReadOnly = () =>
	useWorkflowEditorStore((state) => state.readOnly);
export const useEditorSelectedNodeId = () =>
	useWorkflowEditorStore((state) => state.selectedNodeId);
export const useEditorSelectedEdgeId = () =>
	useWorkflowEditorStore((state) => state.selectedEdgeId);
export const useEditorWorkflowDetails = () =>
	useWorkflowEditorStore((state) => state.workflowDetails);
export const useEditorActiveRun = () =>
	useWorkflowEditorStore((state) => state.activeRun);
