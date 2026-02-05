import { Handle, Position, type NodeProps } from '@xyflow/react';
import './VirtualNode.css';

/**
 * VirtualNode - Invisible node for entry/exit points.
 *
 * These nodes serve as anchors for entry and exit gate edges
 * but are visually minimal (small circle) or invisible.
 *
 * - virtual-entry: Source handle (right) for edge to first phase
 * - virtual-exit: Target handle (left) for edge from last phase
 *
 * Both handles are rendered but only the appropriate one is used
 * based on the edge connections.
 */
export function VirtualNode({ id }: NodeProps) {
	const isEntry = id === 'virtual-entry';
	const isExit = id === 'virtual-exit';

	return (
		<div className="virtual-node">
			{/* Target handle for virtual-exit (incoming from last phase) */}
			{isExit && (
				<Handle
					type="target"
					position={Position.Left}
					className="virtual-node__handle"
				/>
			)}

			<div className="virtual-node__dot" />

			{/* Source handle for virtual-entry (outgoing to first phase) */}
			{isEntry && (
				<Handle
					type="source"
					position={Position.Right}
					className="virtual-node__handle"
				/>
			)}
		</div>
	);
}
