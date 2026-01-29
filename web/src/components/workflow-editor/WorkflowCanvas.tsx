import { useCallback } from 'react';
import {
	ReactFlow,
	ReactFlowProvider,
	Controls,
	MiniMap,
	Background,
	BackgroundVariant,
	type NodeMouseHandler,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import './WorkflowCanvas.css';
import { useWorkflowEditorStore } from '@/stores/workflowEditorStore';
import { edgeTypes } from './edges';
import { nodeTypes } from './nodes';

export function WorkflowCanvas() {
	const nodes = useWorkflowEditorStore((s) => s.nodes);
	const edges = useWorkflowEditorStore((s) => s.edges);
	const readOnly = useWorkflowEditorStore((s) => s.readOnly);
	const selectNode = useWorkflowEditorStore((s) => s.selectNode);

	const onNodeClick: NodeMouseHandler = useCallback(
		(_event, node) => {
			if (node.type === 'startEnd') return;
			selectNode(node.id);
		},
		[selectNode]
	);

	const onPaneClick = useCallback(() => {
		selectNode(null);
	}, [selectNode]);

	return (
		<ReactFlowProvider>
			<div className="workflow-canvas">
				<ReactFlow
					nodes={nodes}
					edges={edges}
					edgeTypes={edgeTypes}
					nodeTypes={nodeTypes}
					nodesDraggable={!readOnly}
					nodesConnectable={!readOnly}
					elementsSelectable={true}
					onNodeClick={onNodeClick}
					onPaneClick={onPaneClick}
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
