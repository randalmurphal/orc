import type { GateType } from '@/gen/orc/v1/workflow_pb';
import { PhaseNode } from './PhaseNode';

/** Status of a phase during execution */
export type PhaseStatus =
	| 'pending'
	| 'running'
	| 'completed'
	| 'failed'
	| 'skipped'
	| 'blocked'
	| 'unspecified';

/** Phase category for color coding */
export type PhaseCategory =
	| 'specification'
	| 'implementation'
	| 'quality'
	| 'documentation'
	| 'other';

/** Data passed to PhaseNode via node.data */
export interface PhaseNodeData {
	[key: string]: unknown;
	phaseTemplateId: string;
	templateName: string;
	description?: string;
	sequence: number;
	phaseId: number;
	gateType: GateType;
	maxIterations: number;
	agentId?: string;
	thinkingEnabled?: boolean;
	status?: PhaseStatus;
	iterations?: number;
	costUsd?: number;
	/** Category for visual color coding */
	category?: PhaseCategory;
}

/**
 * Custom node types for React Flow.
 * Defined at module scope to avoid re-renders (React Flow requirement).
 *
 * NOTE: Start/End nodes removed per design spec - workflow flows directly
 * from first phase to last phase without explicit terminals.
 */
export const nodeTypes = {
	phase: PhaseNode,
} as const;
