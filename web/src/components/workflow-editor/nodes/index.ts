import type { GateType } from '@/gen/orc/v1/workflow_pb';
import { PhaseNode } from './PhaseNode';
import { StartEndNode } from './StartEndNode';

/** Status of a phase during execution */
export type PhaseStatus =
	| 'pending'
	| 'running'
	| 'completed'
	| 'failed'
	| 'skipped'
	| 'blocked'
	| 'unspecified';

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
	modelOverride?: string;
	thinkingEnabled?: boolean;
	status?: PhaseStatus;
	iterations?: number;
	costUsd?: number;
}

/** Data passed to StartEndNode via node.data */
export interface StartEndNodeData {
	[key: string]: unknown;
	variant: 'start' | 'end';
	label: string;
}

/**
 * Custom node types for React Flow.
 * Defined at module scope to avoid re-renders (React Flow requirement).
 */
export const nodeTypes = {
	phase: PhaseNode,
	startEnd: StartEndNode,
} as const;
