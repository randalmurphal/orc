import type { Workflow } from '@/gen/orc/v1/workflow_pb';
import type { Agent } from '@/gen/orc/v1/config_pb';
import { WorkflowSettingsPanel } from './WorkflowSettingsPanel';
import { AgentsPalette } from './AgentsPalette';
import { PhaseTemplatePalette } from './PhaseTemplatePalette';
import './LeftPalette.css';

interface LeftPaletteProps {
	workflow: Workflow;
	onWorkflowUpdate: (workflow: Workflow) => void;
	onAgentClick: (agent: Agent) => void;
	onAgentAssign: (agent: Agent) => void;
	selectedNodeId: string | null;
}

export function LeftPalette({
	workflow,
	onWorkflowUpdate,
	onAgentClick,
	onAgentAssign,
	selectedNodeId,
}: LeftPaletteProps) {
	const readOnly = workflow.isBuiltin;

	return (
		<div className="left-palette" data-testid="left-palette">
			{/* Workflow Settings Section - First */}
			<div className="left-palette-section">
				<WorkflowSettingsPanel
					workflow={workflow}
					onWorkflowUpdate={onWorkflowUpdate}
				/>
			</div>

			{/* Agents Section - Second */}
			<div className="left-palette-section">
				<AgentsPalette
					readOnly={readOnly}
					onAgentClick={onAgentClick}
					onAgentAssign={onAgentAssign}
					selectedNodeId={selectedNodeId}
				/>
			</div>

			{/* Phase Template Section - Third */}
			<div className="left-palette-section">
				<PhaseTemplatePalette
					readOnly={readOnly}
					workflowId={workflow.id}
				/>
			</div>
		</div>
	);
}