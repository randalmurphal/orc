import { BaseEdge, getBezierPath, type EdgeProps } from '@xyflow/react';
import './edges.css';

export function RetryEdge({
	id,
	sourceX,
	sourceY,
	targetX,
	targetY,
	sourcePosition,
	targetPosition,
}: EdgeProps) {
	const [edgePath] = getBezierPath({
		sourceX,
		sourceY,
		targetX,
		targetY,
		sourcePosition,
		targetPosition,
		curvature: 0.5,
	});

	return <BaseEdge id={id} path={edgePath} className="edge-retry" />;
}
