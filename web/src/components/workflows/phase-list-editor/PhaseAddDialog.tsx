import * as RadixSelect from '@radix-ui/react-select';
import { Button, Icon } from '@/components/ui';
import type { PhaseTemplate } from '@/gen/orc/v1/workflow_pb';

interface PhaseAddDialogProps {
	open: boolean;
	selectedTemplateId: string;
	phaseTemplates: PhaseTemplate[];
	onSelectedTemplateIdChange: (value: string) => void;
	onAdd: () => void | Promise<void>;
	onCancel: () => void;
}

export function PhaseAddDialog({
	open,
	selectedTemplateId,
	phaseTemplates,
	onSelectedTemplateIdChange,
	onAdd,
	onCancel,
}: PhaseAddDialogProps) {
	if (!open) {
		return null;
	}

	return (
		<div className="phase-add-dialog">
			<div className="form-group">
				<label id="phase-template-label" className="form-label">
					Phase Template
				</label>
				{phaseTemplates.length === 0 ? (
					<div className="phase-add-empty">
						<Icon name="alert-circle" size={14} />
						<span>No templates available</span>
					</div>
				) : (
					<RadixSelect.Root value={selectedTemplateId} onValueChange={onSelectedTemplateIdChange}>
						<RadixSelect.Trigger
							className="phase-template-trigger"
							aria-label="Phase template"
							aria-labelledby="phase-template-label"
						>
							<RadixSelect.Value placeholder="Select a template..." />
							<RadixSelect.Icon className="phase-template-trigger-icon">
								<Icon name="chevron-down" size={12} />
							</RadixSelect.Icon>
						</RadixSelect.Trigger>

						<RadixSelect.Portal>
							<RadixSelect.Content className="phase-template-content" position="popper" sideOffset={4}>
								<RadixSelect.Viewport className="phase-template-viewport">
									{phaseTemplates.map((template) => (
										<RadixSelect.Item key={template.id} value={template.id} className="phase-template-item">
											<RadixSelect.ItemText>{template.name}</RadixSelect.ItemText>
											<div className="phase-template-item-desc">{template.description}</div>
										</RadixSelect.Item>
									))}
								</RadixSelect.Viewport>
							</RadixSelect.Content>
						</RadixSelect.Portal>
					</RadixSelect.Root>
				)}
			</div>
			<div className="phase-add-actions">
				<Button variant="ghost" size="sm" onClick={onCancel}>Cancel</Button>
				<Button variant="primary" size="sm" onClick={() => void onAdd()} disabled={!selectedTemplateId}>
					Add
				</Button>
			</div>
		</div>
	);
}
