import { Handle, Position, type NodeProps } from '@xyflow/react';
import type { StartEndNodeData } from './index';
import './StartEndNode.css';

export function StartEndNode({ data }: NodeProps) {
	const d = data as unknown as StartEndNodeData;
	const isStart = d.variant === 'start';

	return (
		<div
			className={`start-end-node start-end-node--${d.variant}`}
		>
			{!isStart && (
				<Handle type="target" position={Position.Left} />
			)}
			<span className="start-end-node-label">{d.label}</span>
			{isStart && (
				<Handle type="source" position={Position.Right} />
			)}
		</div>
	);
}
