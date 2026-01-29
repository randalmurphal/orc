import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import type { Node, Edge } from '@xyflow/react';
import type { WorkflowWithDetails } from '@/gen/orc/v1/workflow_pb';
import { layoutWorkflow } from '@/components/workflow-editor/utils/layoutWorkflow';

interface WorkflowEditorStore {
	// State
	nodes: Node[];
	edges: Edge[];
	readOnly: boolean;
	selectedNodeId: string | null;
	workflowDetails: WorkflowWithDetails | null;

	// Actions
	loadFromWorkflow: (details: WorkflowWithDetails) => void;
	setReadOnly: (readOnly: boolean) => void;
	selectNode: (nodeId: string | null) => void;
	reset: () => void;
}

const initialState = {
	nodes: [] as Node[],
	edges: [] as Edge[],
	readOnly: false,
	selectedNodeId: null as string | null,
	workflowDetails: null as WorkflowWithDetails | null,
};

export const useWorkflowEditorStore = create<WorkflowEditorStore>()(
	subscribeWithSelector((set) => ({
		...initialState,

		loadFromWorkflow: (details: WorkflowWithDetails) => {
			const { nodes, edges } = layoutWorkflow(details);
			const isBuiltin = details.workflow?.isBuiltin ?? false;
			set({
				nodes,
				edges,
				workflowDetails: details,
				readOnly: isBuiltin,
				selectedNodeId: null,
			});
		},

		setReadOnly: (readOnly: boolean) => set({ readOnly }),

		selectNode: (nodeId: string | null) => set({ selectedNodeId: nodeId }),

		reset: () => set(initialState),
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
export const useEditorWorkflowDetails = () =>
	useWorkflowEditorStore((state) => state.workflowDetails);
