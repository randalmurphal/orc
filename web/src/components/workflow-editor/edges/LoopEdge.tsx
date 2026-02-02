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
	const edgeData = data as Record<string, unknown> | undefined;

	// Extract sequence information for backward detection
	const sourceSequence = edgeData?.sourceSequence as number | undefined;
	const targetSequence = edgeData?.targetSequence as number | undefined;

	// Detect backward flow: source sequence > target sequence
	const isBackward = typeof sourceSequence === 'number' && typeof targetSequence === 'number'
		? sourceSequence > targetSequence
		: false;

	// Calculate curvature based on direction
	// Backward edges get higher curvature (more pronounced curve)
	const curvature = isBackward ? 0.8 : 0.5;

	const [edgePath, labelX, labelY] = getBezierPath({
		sourceX,
		sourceY,
		targetX,
		targetY,
		sourcePosition,
		targetPosition,
		curvature,
	});

	const label = edgeData?.label as string | undefined;

	// Build CSS classes based on direction
	const edgeClassName = isBackward
		? 'edge-loop edge-loop-backward'
		: 'edge-loop edge-loop-forward';

	// Add direction indicator for backward edges
	const displayLabel = isBackward && label
		? `↩ ${label}`
		: label;

	return (
		<>
			<BaseEdge id={id} path={edgePath} className={edgeClassName} />
			{displayLabel && (
				<EdgeLabelRenderer>
					<div
						className="edge-label edge-label-loop"
						style={{
							position: 'absolute',
							transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
							pointerEvents: 'none',
						}}
					>
						{displayLabel}
					</div>
				</EdgeLabelRenderer>
			)}
		</>
	);
}
