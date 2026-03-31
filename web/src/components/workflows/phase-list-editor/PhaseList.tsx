import { Button, Icon } from '@/components/ui';
import { GateType, type WorkflowPhase, type PhaseTemplate } from '@/gen/orc/v1/workflow_pb';

interface PhaseListProps {
	phases: WorkflowPhase[];
	getTemplate: (templateId: string) => PhaseTemplate | undefined;
	loading: boolean;
	onEdit: (phase: WorkflowPhase) => void;
	onMove: (phase: WorkflowPhase, direction: 'up' | 'down') => void;
	onRemove: (phase: WorkflowPhase) => void;
}

export function PhaseList({
	phases,
	getTemplate,
	loading,
	onEdit,
	onMove,
	onRemove,
}: PhaseListProps) {
	return (
		<div className="phase-list">
			{phases.map((phase, index) => {
				const template = getTemplate(phase.phaseTemplateId);
				const isFirst = index === 0;
				const isLast = index === phases.length - 1;
				const hasOverrides =
					phase.modelOverride ||
					phase.thinkingOverride ||
					phase.gateTypeOverride !== undefined;

				return (
					<div key={phase.id} data-testid={`phase-item-${phase.id}`} className="phase-item">
						<div className="phase-item-sequence">{index + 1}</div>

						<div className="phase-item-info">
							<span className="phase-item-name">{template?.name || phase.phaseTemplateId}</span>
							{hasOverrides && (
								<div className="phase-item-badges">
									{phase.modelOverride && (
										<span className="phase-badge phase-badge--model">{phase.modelOverride}</span>
									)}
									{phase.thinkingOverride && (
										<span className="phase-badge phase-badge--thinking">
											<Icon name="brain" size={10} />
										</span>
									)}
									{phase.gateTypeOverride !== undefined && phase.gateTypeOverride !== GateType.UNSPECIFIED && (
										<span className="phase-badge phase-badge--gate">
											{GateType[phase.gateTypeOverride]}
										</span>
									)}
								</div>
							)}
						</div>

						<div className="phase-item-actions">
							<Button variant="ghost" size="sm" title="Move up" aria-label="Move up" onClick={(e) => {
								e.stopPropagation();
								onMove(phase, 'up');
							}} disabled={loading || isFirst}>
								<Icon name="chevron-up" size={14} />
							</Button>
							<Button variant="ghost" size="sm" title="Move down" aria-label="Move down" onClick={(e) => {
								e.stopPropagation();
								onMove(phase, 'down');
							}} disabled={loading || isLast}>
								<Icon name="chevron-down" size={14} />
							</Button>
							<Button variant="ghost" size="sm" title="Edit" aria-label="Edit" onClick={(e) => {
								e.stopPropagation();
								onEdit(phase);
							}} disabled={loading}>
								<Icon name="edit" size={14} />
							</Button>
							<Button variant="ghost" size="sm" title="Delete" aria-label="Delete" onClick={(e) => {
								e.stopPropagation();
								onRemove(phase);
							}} disabled={loading}>
								<Icon name="trash" size={14} />
							</Button>
						</div>
					</div>
				);
			})}
		</div>
	);
}
