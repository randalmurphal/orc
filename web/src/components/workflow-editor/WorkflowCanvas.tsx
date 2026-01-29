import { useCallback } from 'react';
import {
	ReactFlow,
	ReactFlowProvider,
	Controls,
	MiniMap,
	Background,
	BackgroundVariant,
	type NodeMouseHandler,
	type Node,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import './WorkflowCanvas.css';
import { useWorkflowEditorStore } from '@/stores/workflowEditorStore';
import { edgeTypes } from './edges';
import { nodeTypes, type PhaseNodeData } from './nodes';

/**
 * Returns status-based color for MiniMap nodes.
 * Maps phase execution status to semantic colors.
 */
function getNodeColor(node: Node): string {
	// Only phase nodes have status - startEnd nodes use default
	if (node.type !== 'phase') {
		return '#55555f'; // --text-muted
	}

	const status = (node.data as PhaseNodeData)?.status;
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
		default:
			return '#55555f'; // --text-muted (pending/unspecified)
	}
}

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

	// SC-3: Show empty state for custom workflows with no phases
	const hasPhases = nodes.some((n) => n.type === 'phase');
	const showEmptyState = !readOnly && !hasPhases;

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
					<MiniMap nodeColor={getNodeColor} />
					<Background variant={BackgroundVariant.Dots} gap={16} size={1} />
				</ReactFlow>
				{showEmptyState && (
					<div className="workflow-canvas-empty">
						<p>Drag phase templates from the palette to start building your workflow</p>
					</div>
				)}
			</div>
		</ReactFlowProvider>
	);
}
