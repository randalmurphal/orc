import {
	ReactFlow,
	ReactFlowProvider,
	Controls,
	MiniMap,
	Background,
	BackgroundVariant,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import './WorkflowCanvas.css';
import { useWorkflowEditorStore } from '@/stores/workflowEditorStore';
import { nodeTypes } from './nodes';

export function WorkflowCanvas() {
	const nodes = useWorkflowEditorStore((s) => s.nodes);
	const edges = useWorkflowEditorStore((s) => s.edges);
	const readOnly = useWorkflowEditorStore((s) => s.readOnly);

	return (
		<ReactFlowProvider>
			<div className="workflow-canvas">
				<ReactFlow
					nodes={nodes}
					edges={edges}
					nodeTypes={nodeTypes}
					nodesDraggable={!readOnly}
					nodesConnectable={!readOnly}
					elementsSelectable={!readOnly}
					fitView
				>
					<Controls />
					<MiniMap />
					<Background variant={BackgroundVariant.Dots} gap={16} size={1} />
				</ReactFlow>
			</div>
		</ReactFlowProvider>
	);
}
