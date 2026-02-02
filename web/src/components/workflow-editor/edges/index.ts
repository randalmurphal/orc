import type { EdgeTypes } from '@xyflow/react';
import { SequentialEdge } from './SequentialEdge';
import { LoopEdge } from './LoopEdge';
import { RetryEdge } from './RetryEdge';
import { DependencyEdge } from './DependencyEdge';
import { ConditionalEdge } from './ConditionalEdge';
import { GateEdge } from './GateEdge';

export const edgeTypes: EdgeTypes = {
	sequential: SequentialEdge,
	loop: LoopEdge,
	retry: RetryEdge,
	dependency: DependencyEdge,
	conditional: ConditionalEdge,
	gate: GateEdge,
};

export { SequentialEdge, LoopEdge, RetryEdge, DependencyEdge, ConditionalEdge, GateEdge };
