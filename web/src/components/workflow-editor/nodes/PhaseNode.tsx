import { Handle, Position, type NodeProps } from '@xyflow/react';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import type { PhaseNodeData, PhaseStatus, PhaseCategory } from './index';
import './PhaseNode.css';

const STATUS_CLASSES: Record<string, string> = {
	running: 'phase-node--running',
	completed: 'phase-node--completed',
	failed: 'phase-node--failed',
	skipped: 'phase-node--skipped',
	pending: 'phase-node--pending',
	blocked: 'phase-node--blocked',
};

const CATEGORY_CLASSES: Record<PhaseCategory, string> = {
	specification: 'phase-node--category-spec',
	implementation: 'phase-node--category-impl',
	quality: 'phase-node--category-quality',
	documentation: 'phase-node--category-docs',
	other: 'phase-node--category-other',
};

function getStatusClass(status?: PhaseStatus): string {
	if (!status) return '';
	return STATUS_CLASSES[status] ?? '';
}

function getCategoryClass(category?: PhaseCategory): string {
	if (!category) return CATEGORY_CLASSES.other;
	return CATEGORY_CLASSES[category] ?? CATEGORY_CLASSES.other;
}

/**
 * Get the type badge label based on gate type
 * - AUTO = AI (automated)
 * - HUMAN = Human (requires approval)
 * - SKIP = Skip (will be skipped)
 */
function getTypeBadge(gateType: GateType): { label: string; variant: 'ai' | 'human' | 'skip' } | null {
	switch (gateType) {
		case GateType.HUMAN:
			return { label: 'Human', variant: 'human' };
		case GateType.SKIP:
			return { label: 'Skip', variant: 'skip' };
		case GateType.AUTO:
		default:
			return { label: 'AI', variant: 'ai' };
	}
}

export function PhaseNode({ data, selected, isConnectable }: NodeProps) {
	const d = data as unknown as PhaseNodeData;
	const displayName = d.templateName || d.phaseTemplateId;
	const statusClass = getStatusClass(d.status);
	const categoryClass = getCategoryClass(d.category);
	const typeBadge = getTypeBadge(d.gateType);

	// Truncate description to ~50 chars
	const description = d.description
		? d.description.length > 50
			? d.description.slice(0, 47) + '...'
			: d.description
		: null;

	const classes = ['phase-node', categoryClass];
	if (statusClass) classes.push(statusClass);
	if (selected) classes.push('phase-node--selected');

	return (
		<div className={classes.join(' ')}>
			<Handle
				type="target"
				position={Position.Left}
				isConnectable={isConnectable}
				data-handletype="target"
			/>

			<div className="phase-node__content">
				{/* Header: name + badge */}
				<div className="phase-node__header">
					<span className="phase-node__name">{displayName}</span>
					{typeBadge && (
						<span className={`phase-node__badge phase-node__badge--${typeBadge.variant}`}>
							{typeBadge.label}
						</span>
					)}
				</div>

				{/* Template ID */}
				<span className="phase-node__id">{d.phaseTemplateId}</span>

				{/* Description (if available) */}
				{description && (
					<p className="phase-node__description">{description}</p>
				)}

				{/* Execution footer (only during/after execution) */}
				{(d.iterations !== undefined || (d.costUsd !== undefined && d.costUsd > 0)) && (
					<div className="phase-node__footer">
						{d.iterations !== undefined && (
							<span className="phase-node__stat">
								{d.iterations} iter
							</span>
						)}
						{d.costUsd !== undefined && d.costUsd > 0 && (
							<span className="phase-node__stat">
								${d.costUsd.toFixed(2)}
							</span>
						)}
					</div>
				)}
			</div>

			<Handle
				type="source"
				position={Position.Right}
				isConnectable={isConnectable}
				data-handletype="source"
			/>
		</div>
	);
}
