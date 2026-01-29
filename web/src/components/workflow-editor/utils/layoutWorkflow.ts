import dagre from 'dagre';
import type { Node, Edge } from '@xyflow/react';
import type { WorkflowWithDetails } from '@/gen/orc/v1/workflow_pb';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import type { PhaseNodeData, StartEndNodeData } from '../nodes/index';

const PHASE_NODE_WIDTH = 240;
const PHASE_NODE_HEIGHT = 100;
const START_END_NODE_SIZE = 40;

export interface LayoutResult {
	nodes: Node[];
	edges: Edge[];
}

/**
 * Pure function that converts WorkflowWithDetails into React Flow nodes and edges
 * using dagre for left-to-right auto-layout.
 */
export function layoutWorkflow(details: WorkflowWithDetails): LayoutResult {
	const phases = [...(details.phases ?? [])].sort(
		(a, b) => a.sequence - b.sequence
	);

	// Build a lookup from phaseTemplateId -> node ID for dependency/loop edges
	const templateToNodeId = new Map<string, string>();
	for (const phase of phases) {
		templateToNodeId.set(phase.phaseTemplateId, `phase-${phase.id}`);
	}

	// Create nodes
	const nodes: Node[] = [];
	const startId = '__start__';
	const endId = '__end__';

	const startData: StartEndNodeData = { variant: 'start', label: 'Start' };
	nodes.push({
		id: startId,
		type: 'startEnd',
		position: { x: 0, y: 0 },
		data: startData,
	});

	for (const phase of phases) {
		const template = phase.template;
		const phaseData: PhaseNodeData = {
			phaseTemplateId: phase.phaseTemplateId,
			templateName: template?.name || phase.phaseTemplateId,
			description: template?.description,
			sequence: phase.sequence,
			phaseId: phase.id,
			gateType:
				phase.gateTypeOverride ??
				template?.gateType ??
				GateType.AUTO,
			maxIterations:
				phase.maxIterationsOverride ??
				template?.maxIterations ??
				1,
			modelOverride:
				phase.modelOverride ?? template?.modelOverride,
			thinkingEnabled:
				phase.thinkingOverride ?? template?.thinkingEnabled,
		};
		nodes.push({
			id: `phase-${phase.id}`,
			type: 'phase',
			position: { x: 0, y: 0 },
			data: phaseData,
		});
	}

	const endData: StartEndNodeData = { variant: 'end', label: 'End' };
	nodes.push({
		id: endId,
		type: 'startEnd',
		position: { x: 0, y: 0 },
		data: endData,
	});

	// Create edges
	const edges: Edge[] = [];

	// Sequential edges: start -> phase1 -> phase2 -> ... -> end
	if (phases.length === 0) {
		edges.push({
			id: `edge-${startId}-${endId}`,
			source: startId,
			target: endId,
		});
	} else {
		// start -> first phase
		edges.push({
			id: `edge-${startId}-phase-${phases[0].id}`,
			source: startId,
			target: `phase-${phases[0].id}`,
		});

		// phase-to-phase sequential
		for (let i = 0; i < phases.length - 1; i++) {
			edges.push({
				id: `edge-phase-${phases[i].id}-phase-${phases[i + 1].id}`,
				source: `phase-${phases[i].id}`,
				target: `phase-${phases[i + 1].id}`,
			});
		}

		// last phase -> end
		edges.push({
			id: `edge-phase-${phases[phases.length - 1].id}-${endId}`,
			source: `phase-${phases[phases.length - 1].id}`,
			target: endId,
		});
	}

	// Dependency edges
	for (const phase of phases) {
		if (phase.dependsOn && phase.dependsOn.length > 0) {
			for (const dep of phase.dependsOn) {
				const sourceNodeId = templateToNodeId.get(dep);
				if (sourceNodeId) {
					edges.push({
						id: `dep-${sourceNodeId}-phase-${phase.id}`,
						source: sourceNodeId,
						target: `phase-${phase.id}`,
						type: 'dependency',
					});
				}
			}
		}
	}

	// Loop-back edges from retryFromPhase (lives on PhaseTemplate)
	for (const phase of phases) {
		const retryFrom = phase.template?.retryFromPhase;
		if (typeof retryFrom === 'string' && retryFrom) {
			const targetNodeId = templateToNodeId.get(retryFrom);
			if (targetNodeId) {
				edges.push({
					id: `loop-phase-${phase.id}-${targetNodeId}`,
					source: `phase-${phase.id}`,
					target: targetNodeId,
					type: 'loop',
				});
			}
		}
	}

	// Apply dagre layout
	const g = new dagre.graphlib.Graph();
	g.setDefaultEdgeLabel(() => ({}));
	g.setGraph({ rankdir: 'LR', nodesep: 60, ranksep: 120 });

	for (const node of nodes) {
		const isStartEnd = node.type === 'startEnd';
		const w = isStartEnd ? START_END_NODE_SIZE : PHASE_NODE_WIDTH;
		const h = isStartEnd ? START_END_NODE_SIZE : PHASE_NODE_HEIGHT;
		g.setNode(node.id, { width: w, height: h });
	}

	// Only use sequential + dependency edges for layout (not loop-back)
	for (const edge of edges) {
		if (edge.type !== 'loop') {
			g.setEdge(edge.source, edge.target);
		}
	}

	dagre.layout(g);

	// Apply computed positions (dagre gives center, React Flow uses top-left)
	for (const node of nodes) {
		const dagreNode = g.node(node.id);
		if (dagreNode) {
			node.position = {
				x: dagreNode.x - dagreNode.width / 2,
				y: dagreNode.y - dagreNode.height / 2,
			};
		}
	}

	return { nodes, edges };
}
