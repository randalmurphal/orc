import type { NodeProps } from '@xyflow/react';
import './VirtualNode.css';

/**
 * VirtualNode - Invisible node for entry/exit points.
 *
 * These nodes serve as anchors for entry and exit gate edges
 * but are visually minimal (small circle) or invisible.
 */
export function VirtualNode(_props: NodeProps) {
	return (
		<div className="virtual-node">
			<div className="virtual-node__dot" />
		</div>
	);
}
