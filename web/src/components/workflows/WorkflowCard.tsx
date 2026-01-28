/**
 * WorkflowCard displays a single workflow with its phases and metadata.
 */

import type { Workflow, DefinitionSource } from '@/gen/orc/v1/workflow_pb';
import { DefinitionSource as DS } from '@/gen/orc/v1/workflow_pb';
import { Badge } from '@/components/core/Badge';
import { Icon } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';

/** Get display name for a definition source */
function getSourceLabel(source?: DefinitionSource): string {
	switch (source) {
		case DS.EMBEDDED:
			return 'Built-in';
		case DS.PROJECT:
			return 'Project';
		case DS.SHARED:
			return 'Shared';
		case DS.LOCAL:
			return 'Local';
		case DS.PERSONAL:
			return 'Personal';
		default:
			return 'Unknown';
	}
}

/** Get badge variant for source */
function getSourceVariant(source?: DefinitionSource): 'active' | 'idle' | 'completed' | 'paused' {
	switch (source) {
		case DS.EMBEDDED:
			return 'active';
		case DS.PROJECT:
			return 'completed';
		case DS.SHARED:
			return 'paused';
		case DS.LOCAL:
		case DS.PERSONAL:
			return 'idle';
		default:
			return 'idle';
	}
}

export interface WorkflowCardProps {
	workflow: Workflow;
	phaseCount?: number;
	source?: DefinitionSource;
	onSelect?: (workflow: Workflow) => void;
	onClone?: (workflow: Workflow) => void;
}

/**
 * WorkflowCard displays a workflow with its name, phase count, and actions.
 */
export function WorkflowCard({ workflow, phaseCount, source, onSelect, onClone }: WorkflowCardProps) {
	const handleClick = () => {
		onSelect?.(workflow);
	};

	const handleClone = (e: React.MouseEvent) => {
		e.stopPropagation();
		onClone?.(workflow);
	};

	const handleEdit = (e: React.MouseEvent) => {
		e.stopPropagation();
		window.dispatchEvent(
			new CustomEvent('orc:edit-workflow', { detail: { workflow } })
		);
	};

	return (
		<article
			className="workflow-card"
			onClick={handleClick}
			role="button"
			tabIndex={0}
			onKeyDown={(e) => e.key === 'Enter' && handleClick()}
		>
			<header className="workflow-card-header">
				<div className="workflow-card-icon">
					<Icon name="git-branch" size={20} />
				</div>
				<div className="workflow-card-info">
					<h3 className="workflow-card-name">{workflow.name}</h3>
					<span className="workflow-card-id">{workflow.id}</span>
				</div>
				<Badge variant="status" status={getSourceVariant(source)}>
					{getSourceLabel(source)}
				</Badge>
			</header>

			{workflow.description && (
				<p className="workflow-card-description">{workflow.description}</p>
			)}

			<div className="workflow-card-stats">
				<div className="workflow-card-stat">
					<Icon name="layers" size={14} />
					<span>{phaseCount ?? 0} phases</span>
				</div>
				<div className="workflow-card-stat">
					<span className="workflow-card-type">{workflow.workflowType}</span>
				</div>
				{workflow.defaultModel && (
					<div className="workflow-card-stat">
						<Icon name="cpu" size={14} />
						<span>{workflow.defaultModel}</span>
					</div>
				)}
			</div>

			<div className="workflow-card-actions">
				{workflow.isBuiltin ? (
					<Button
						variant="ghost"
						size="sm"
						className="workflow-card-action"
						onClick={handleClone}
						title="Clone to customize"
						leftIcon={<Icon name="clipboard" size={14} />}
					>
						Clone
					</Button>
				) : (
					<Button
						variant="ghost"
						size="sm"
						className="workflow-card-action"
						onClick={handleEdit}
						title="Edit workflow"
						leftIcon={<Icon name="edit" size={14} />}
					>
						Edit
					</Button>
				)}
			</div>
		</article>
	);
}
