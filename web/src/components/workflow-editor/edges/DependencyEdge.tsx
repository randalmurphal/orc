import { BaseEdge, getBezierPath, type EdgeProps } from '@xyflow/react';
import './edges.css';

/**
 * DependencyEdge - Renders explicit phase dependencies (from dependsOn field)
 *
 * Visually distinct from sequential edges:
 * - Uses dashed line style (vs solid for sequential)
 * - Uses accent color to indicate user-defined dependency
 */
export function DependencyEdge({
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
	});

	return (
		<BaseEdge
			id={id}
			path={edgePath}
			className="edge-dependency"
		/>
	);
}
