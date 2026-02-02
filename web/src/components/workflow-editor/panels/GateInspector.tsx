import type { Edge } from '@xyflow/react';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import type { WorkflowWithDetails } from '@/gen/orc/v1/workflow_pb';
import type { GateEdgeData } from '../utils/layoutWorkflow';
import './GateInspector.css';

interface GateInspectorProps {
	edge: Edge<GateEdgeData> | null | undefined;
	workflowDetails: WorkflowWithDetails | null;
	readOnly: boolean;
}

/**
 * Get position label for gate edge
 */
function getPositionLabel(position: GateEdgeData['position']): string {
	switch (position) {
		case 'entry':
			return 'Entry Gate';
		case 'exit':
			return 'Exit Gate';
		case 'between':
			return 'Gate';
		default:
			return 'Gate';
	}
}

/**
 * GateInspector - Panel for inspecting and editing gate configurations.
 *
 * Shows:
 * - Gate type (Auto, Human, AI, Skip)
 * - Position label (Entry Gate, Exit Gate, or phase-to-phase)
 * - Max retries and failure action when configured
 * - Gate status during execution
 * - Read-only notice for built-in workflows
 */
export function GateInspector({
	edge,
	workflowDetails,
	readOnly,
}: GateInspectorProps) {
	// Return nothing if no edge is selected
	if (!edge || !edge.data) {
		return null;
	}

	const edgeData = edge.data as GateEdgeData;
	const gateType = edgeData.gateType ?? GateType.UNSPECIFIED;
	const position = edgeData.position ?? 'between';
	const gateStatus = edgeData.gateStatus;
	const maxRetries = edgeData.maxRetries;
	const failureAction = edgeData.failureAction;
	const phaseId = edgeData.phaseId;

	// Find the target phase name for between gates
	const targetPhase = phaseId
		? workflowDetails?.phases?.find((p) => p.id === phaseId)
		: null;
	const targetPhaseName = targetPhase?.template?.name ?? targetPhase?.phaseTemplateId;

	// Build header text based on position
	let headerText: string;
	if (position === 'entry') {
		headerText = 'Entry Gate';
	} else if (position === 'exit') {
		headerText = 'Exit Gate';
	} else if (targetPhaseName) {
		headerText = `Gate → ${targetPhaseName}`;
	} else {
		headerText = getPositionLabel(position);
	}

	// Get status CSS class
	const statusClass = gateStatus ? `gate-inspector__status--${gateStatus}` : '';

	return (
		<div className="gate-inspector">
			<div className="gate-inspector__header">
				<h3>{headerText}</h3>
			</div>

			{readOnly && (
				<div className="gate-inspector__readonly-notice">
					Clone to customize
				</div>
			)}

			<div className="gate-inspector__settings">
				{/* Gate Type */}
				<div className="gate-inspector__field">
					<label className="gate-inspector__label">Gate Type</label>
					<select
						className="gate-inspector__select"
						value={gateType}
						disabled={readOnly}
					>
						<option value={GateType.AUTO}>Auto</option>
						<option value={GateType.HUMAN}>Human</option>
						<option value={GateType.AI}>AI</option>
						<option value={GateType.SKIP}>Skip</option>
					</select>
				</div>

				{/* Max Retries */}
				{maxRetries !== undefined && (
					<div className="gate-inspector__field">
						<label className="gate-inspector__label">Max Retries</label>
						<input
							type="number"
							className="gate-inspector__input"
							value={maxRetries}
							disabled={readOnly}
							readOnly
						/>
					</div>
				)}

				{/* Failure Action */}
				{failureAction && (
					<div className="gate-inspector__field">
						<label className="gate-inspector__label">On Failure</label>
						<span className="gate-inspector__value">{failureAction}</span>
					</div>
				)}

				{/* Gate Status (during execution) */}
				{gateStatus && (
					<div className="gate-inspector__field">
						<label className="gate-inspector__label">Status</label>
						<span className={`gate-inspector__status ${statusClass}`}>
							{gateStatus.charAt(0).toUpperCase() + gateStatus.slice(1)}
						</span>
					</div>
				)}
			</div>
		</div>
	);
}
