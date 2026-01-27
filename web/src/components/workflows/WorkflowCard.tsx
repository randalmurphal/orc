/**
 * WorkflowCard displays a single workflow with its phases and metadata.
 */

import type { Workflow } from '@/gen/orc/v1/workflow_pb';
import { Badge } from '@/components/core/Badge';
import { Icon } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';

export interface WorkflowCardProps {
	workflow: Workflow;
	phaseCount?: number;
	onSelect?: (workflow: Workflow) => void;
	onClone?: (workflow: Workflow) => void;
}

/**
 * WorkflowCard displays a workflow with its name, phase count, and actions.
 */
export function WorkflowCard({ workflow, phaseCount, onSelect, onClone }: WorkflowCardProps) {
	const handleClick = () => {
		onSelect?.(workflow);
	};

	const handleClone = (e: React.MouseEvent) => {
		e.stopPropagation();
		onClone?.(workflow);
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
				{workflow.isBuiltin ? (
					<Badge variant="status" status="active">
						Built-in
					</Badge>
				) : (
					<Badge variant="status" status="idle">
						Custom
					</Badge>
				)}
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
						onClick={handleClick}
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
