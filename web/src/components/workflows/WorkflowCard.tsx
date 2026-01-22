/**
 * WorkflowCard displays a single workflow with its phases and metadata.
 */

import type { Workflow } from '@/lib/types';
import { Badge } from '@/components/core/Badge';
import { Icon } from '@/components/ui/Icon';

export interface WorkflowCardProps {
	workflow: Workflow;
	onSelect?: (workflow: Workflow) => void;
	onClone?: (workflow: Workflow) => void;
}

/**
 * WorkflowCard displays a workflow with its name, phase count, and actions.
 */
export function WorkflowCard({ workflow, onSelect, onClone }: WorkflowCardProps) {
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
				{workflow.is_builtin ? (
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
					<span>{workflow.phase_count ?? 0} phases</span>
				</div>
				<div className="workflow-card-stat">
					<span className="workflow-card-type">{workflow.workflow_type}</span>
				</div>
				{workflow.default_model && (
					<div className="workflow-card-stat">
						<Icon name="cpu" size={14} />
						<span>{workflow.default_model}</span>
					</div>
				)}
			</div>

			<div className="workflow-card-actions">
				{workflow.is_builtin ? (
					<button
						className="workflow-card-action"
						onClick={handleClone}
						title="Clone to customize"
					>
						<Icon name="clipboard" size={14} />
						<span>Clone</span>
					</button>
				) : (
					<button className="workflow-card-action" onClick={handleClick} title="Edit workflow">
						<Icon name="edit" size={14} />
						<span>Edit</span>
					</button>
				)}
			</div>
		</article>
	);
}
