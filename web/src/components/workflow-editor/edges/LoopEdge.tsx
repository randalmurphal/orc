import { BaseEdge, getBezierPath, type EdgeProps, EdgeLabelRenderer } from '@xyflow/react';
import './edges.css';

export function LoopEdge({
	id,
	sourceX,
	sourceY,
	targetX,
	targetY,
	sourcePosition,
	targetPosition,
	data,
}: EdgeProps) {
	const [edgePath, labelX, labelY] = getBezierPath({
		sourceX,
		sourceY,
		targetX,
		targetY,
		sourcePosition,
		targetPosition,
		curvature: 0.5,
	});

	const edgeData = data as Record<string, unknown> | undefined;
	const label = edgeData?.label as string | undefined;

	return (
		<>
			<BaseEdge id={id} path={edgePath} className="edge-loop" />
			{label && (
				<EdgeLabelRenderer>
					<div
						className="edge-label edge-label-loop"
						style={{
							position: 'absolute',
							transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
							pointerEvents: 'none',
						}}
					>
						{label}
					</div>
				</EdgeLabelRenderer>
			)}
		</>
	);
}
