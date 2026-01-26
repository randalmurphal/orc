/**
 * PhaseTemplateCard displays a single phase template with its configuration.
 */

import type { PhaseTemplate } from '@/gen/orc/v1/workflow_pb';
import { Badge } from '@/components/core/Badge';
import { Icon } from '@/components/ui/Icon';

export interface PhaseTemplateCardProps {
	template: PhaseTemplate;
	onSelect?: (template: PhaseTemplate) => void;
}

/**
 * PhaseTemplateCard displays a phase template with its prompt configuration.
 */
export function PhaseTemplateCard({ template, onSelect }: PhaseTemplateCardProps) {
	const handleClick = () => {
		onSelect?.(template);
	};

	return (
		<article
			className="phase-template-card"
			onClick={handleClick}
			role="button"
			tabIndex={0}
			onKeyDown={(e) => e.key === 'Enter' && handleClick()}
		>
			<header className="phase-template-card-header">
				<div className="phase-template-card-icon">
					<Icon name="file-text" size={18} />
				</div>
				<div className="phase-template-card-info">
					<h3 className="phase-template-card-name">{template.name}</h3>
					<span className="phase-template-card-id">{template.id}</span>
				</div>
				{template.isBuiltin ? (
					<Badge variant="status" status="active">
						Built-in
					</Badge>
				) : (
					<Badge variant="status" status="idle">
						Custom
					</Badge>
				)}
			</header>

			{template.description && (
				<p className="phase-template-card-description">{template.description}</p>
			)}

			<div className="phase-template-card-config">
				<div className="phase-template-card-config-item">
					<span className="phase-template-card-config-label">Gate</span>
					<span className="phase-template-card-config-value">{template.gateType}</span>
				</div>
				<div className="phase-template-card-config-item">
					<span className="phase-template-card-config-label">Max iterations</span>
					<span className="phase-template-card-config-value">{template.maxIterations}</span>
				</div>
				{template.producesArtifact && (
					<div className="phase-template-card-config-item">
						<span className="phase-template-card-config-label">Produces</span>
						<Badge variant="status" status="completed">
							{template.artifactType || 'artifact'}
						</Badge>
					</div>
				)}
			</div>

			<div className="phase-template-card-footer">
				<span className="phase-template-card-prompt-source">
					<Icon name="file-text" size={12} />
					{template.promptSource}
				</span>
				{template.modelOverride && (
					<span className="phase-template-card-model">
						<Icon name="cpu" size={12} />
						{template.modelOverride}
					</span>
				)}
			</div>
		</article>
	);
}
