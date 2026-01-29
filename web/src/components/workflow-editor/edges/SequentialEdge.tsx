import { BaseEdge, getBezierPath, type EdgeProps } from '@xyflow/react';
import './edges.css';

export function SequentialEdge({
	id,
	sourceX,
	sourceY,
	targetX,
	targetY,
	sourcePosition,
	targetPosition,
	data,
}: EdgeProps) {
	const [edgePath] = getBezierPath({
		sourceX,
		sourceY,
		targetX,
		targetY,
		sourcePosition,
		targetPosition,
	});

	const isAnimated = (data as Record<string, unknown>)?.animated === true;

	return (
		<>
			<BaseEdge
				id={id}
				path={edgePath}
				className={`edge-sequential${isAnimated ? ' edge-animated' : ''}`}
			/>
			{isAnimated && (
				<circle r="3" className="edge-dot">
					<animateMotion dur="1.5s" repeatCount="indefinite" path={edgePath} />
				</circle>
			)}
		</>
	);
}
