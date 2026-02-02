import type { Workflow } from '@/gen/orc/v1/workflow_pb';
import { WorkflowSettingsPanel } from './WorkflowSettingsPanel';
import { PhaseTemplatePalette } from './PhaseTemplatePalette';
import './LeftPalette.css';

interface LeftPaletteProps {
	workflow: Workflow;
	onWorkflowUpdate: (workflow: Workflow) => void;
}

export function LeftPalette({ workflow, onWorkflowUpdate }: LeftPaletteProps) {
	const readOnly = workflow.isBuiltin;

	return (
		<div className="left-palette">
			{/* Workflow Settings Section - First */}
			<div className="left-palette-section">
				<WorkflowSettingsPanel
					workflow={workflow}
					onWorkflowUpdate={onWorkflowUpdate}
				/>
			</div>

			{/* Phase Template Section - Second */}
			<div className="left-palette-section">
				<PhaseTemplatePalette
					readOnly={readOnly}
					workflowId={workflow.id}
				/>
			</div>
		</div>
	);
}