import type { EdgeTypes } from '@xyflow/react';
import { SequentialEdge } from './SequentialEdge';
import { LoopEdge } from './LoopEdge';
import { RetryEdge } from './RetryEdge';
import { DependencyEdge } from './DependencyEdge';

export const edgeTypes: EdgeTypes = {
	sequential: SequentialEdge,
	loop: LoopEdge,
	retry: RetryEdge,
	dependency: DependencyEdge,
};

export { SequentialEdge, LoopEdge, RetryEdge, DependencyEdge };
