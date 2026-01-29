import { Handle, Position, type NodeProps } from '@xyflow/react';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import type { PhaseNodeData, PhaseStatus } from './index';
import './PhaseNode.css';

const STATUS_CLASSES: Record<string, string> = {
	running: 'phase-node--running',
	completed: 'phase-node--completed',
	failed: 'phase-node--failed',
	skipped: 'phase-node--skipped',
	pending: 'phase-node--pending',
	blocked: 'phase-node--blocked',
};

function getStatusClass(status?: PhaseStatus): string {
	if (!status) return '';
	return STATUS_CLASSES[status] ?? '';
}

function formatGateType(gt: GateType): string {
	switch (gt) {
		case GateType.HUMAN:
			return 'human';
		case GateType.SKIP:
			return 'skip';
		default:
			return '';
	}
}

// Show iterations badge only for notably high values (> typical template default of 3)
const ITERATIONS_BADGE_THRESHOLD = 3;

export function PhaseNode({ data, selected }: NodeProps) {
	const d = data as unknown as PhaseNodeData;
	const displayName = d.templateName || d.phaseTemplateId;
	const statusClass = getStatusClass(d.status);
	const gateLabel = formatGateType(d.gateType);
	const showIterBadge = d.maxIterations > ITERATIONS_BADGE_THRESHOLD;
	const showBadges = gateLabel || showIterBadge || d.modelOverride;
	const hasExecutionData =
		d.iterations !== undefined || d.costUsd !== undefined;

	const classes = ['phase-node'];
	if (statusClass) classes.push(statusClass);
	if (selected) classes.push('selected');

	return (
		<div className={classes.join(' ')}>
			<Handle type="target" position={Position.Left} />
			<div className="phase-node-header">
				<span className="phase-node-sequence">{d.sequence}</span>
				<div className="phase-node-title">
					<span className="phase-node-name">{displayName}</span>
					<span className="phase-node-template-id">
						{d.phaseTemplateId}
					</span>
				</div>
			</div>
			{showBadges && (
				<div className="phase-node-badges">
					{gateLabel && (
						<span className="phase-node-badge phase-node-badge--gate">
							{gateLabel}
						</span>
					)}
					{showIterBadge && (
						<span className="phase-node-badge phase-node-badge--iterations">
							×{d.maxIterations}
						</span>
					)}
					{d.modelOverride && (
						<span className="phase-node-badge phase-node-badge--model">
							{d.modelOverride}
						</span>
					)}
				</div>
			)}
			{hasExecutionData && (
				<div className="phase-node-footer">
					{d.iterations !== undefined && (
						<span className="phase-node-iterations">
							{d.iterations} iter
						</span>
					)}
					{d.costUsd !== undefined && (
						<span className="phase-node-cost">
							${d.costUsd.toFixed(2)}
						</span>
					)}
				</div>
			)}
			<Handle type="source" position={Position.Right} />
		</div>
	);
}
