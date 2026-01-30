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
			agentId: phase.agentOverride || template?.agentId,
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
			type: 'sequential',
		});
	} else {
		// start -> first phase
		edges.push({
			id: `edge-${startId}-phase-${phases[0].id}`,
			source: startId,
			target: `phase-${phases[0].id}`,
			type: 'sequential',
		});

		// phase-to-phase sequential
		for (let i = 0; i < phases.length - 1; i++) {
			edges.push({
				id: `edge-phase-${phases[i].id}-phase-${phases[i + 1].id}`,
				source: `phase-${phases[i].id}`,
				target: `phase-${phases[i + 1].id}`,
				type: 'sequential',
			});
		}

		// last phase -> end
		edges.push({
			id: `edge-phase-${phases[phases.length - 1].id}-${endId}`,
			source: `phase-${phases[phases.length - 1].id}`,
			target: endId,
			type: 'sequential',
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

	// Loop edges from loopConfig (JSON string on WorkflowPhase)
	for (const phase of phases) {
		if (phase.loopConfig) {
			try {
				const config = JSON.parse(phase.loopConfig) as {
					condition: string;
					loop_to_phase: string;
					max_iterations: number;
				};
				if (config.loop_to_phase) {
					const targetNodeId = templateToNodeId.get(config.loop_to_phase);
					if (targetNodeId) {
						edges.push({
							id: `loop-phase-${phase.id}-${targetNodeId}`,
							source: `phase-${phase.id}`,
							target: targetNodeId,
							type: 'loop',
							data: {
								condition: config.condition,
								maxIterations: config.max_iterations,
								label: `${config.condition} ×${config.max_iterations}`,
							},
						});
					}
				}
			} catch {
				// Invalid JSON in loopConfig — skip
			}
		}
	}

	// Retry edges from retryFromPhase (lives on PhaseTemplate)
	for (const phase of phases) {
		const retryFrom = phase.template?.retryFromPhase;
		if (typeof retryFrom === 'string' && retryFrom) {
			const targetNodeId = templateToNodeId.get(retryFrom);
			if (targetNodeId) {
				edges.push({
					id: `retry-phase-${phase.id}-${targetNodeId}`,
					source: `phase-${phase.id}`,
					target: targetNodeId,
					type: 'retry',
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

	// Only use sequential + dependency edges for layout (not loop/retry back-edges)
	for (const edge of edges) {
		if (edge.type !== 'loop' && edge.type !== 'retry') {
			g.setEdge(edge.source, edge.target);
		}
	}

	dagre.layout(g);

	// Apply positions: use stored positions for phase nodes when available,
	// fall back to dagre-computed positions otherwise (SC-11)
	for (const node of nodes) {
		const dagreNode = g.node(node.id);

		// For phase nodes, check if stored positions are available
		if (node.type === 'phase') {
			const nodeData = node.data as PhaseNodeData;
			const phase = phases.find((p) => p.id === nodeData.phaseId);

			// Use stored position if BOTH x and y are set (not null/undefined)
			if (
				phase &&
				phase.positionX !== undefined &&
				phase.positionX !== null &&
				phase.positionY !== undefined &&
				phase.positionY !== null
			) {
				node.position = {
					x: phase.positionX,
					y: phase.positionY,
				};
				continue;
			}
		}

		// Fall back to dagre for start/end nodes or phases without stored positions
		if (dagreNode) {
			node.position = {
				x: dagreNode.x - dagreNode.width / 2,
				y: dagreNode.y - dagreNode.height / 2,
			};
		}
	}

	return { nodes, edges };
}
