import type { EdgeTypes } from '@xyflow/react';
import { SequentialEdge } from './SequentialEdge';
import { LoopEdge } from './LoopEdge';
import { RetryEdge } from './RetryEdge';

export const edgeTypes: EdgeTypes = {
	sequential: SequentialEdge,
	loop: LoopEdge,
	retry: RetryEdge,
};

export { SequentialEdge, LoopEdge, RetryEdge };
