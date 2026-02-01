import { getBezierPath, type EdgeProps, EdgeLabelRenderer } from '@xyflow/react';
import './edges.css';

export function ConditionalEdge({
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
	});

	const edgeData = data as Record<string, unknown> | undefined;
	const condition = edgeData?.condition as string | undefined;

	return (
		<>
			<g className="edge-conditional">
				<path
					id={id}
					d={edgePath}
					fill="none"
					className="react-flow__edge-path"
				/>
				<path
					d={edgePath}
					fill="none"
					strokeOpacity={0}
					strokeWidth={20}
					className="react-flow__edge-interaction"
				/>
			</g>
			{condition && (
				<EdgeLabelRenderer>
					<div
						className="edge-label edge-label-conditional"
						style={{
							position: 'absolute',
							transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
							pointerEvents: 'none',
						}}
					>
						{condition}
					</div>
				</EdgeLabelRenderer>
			)}
		</>
	);
}
