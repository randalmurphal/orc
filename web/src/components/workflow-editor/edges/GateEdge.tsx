import { useState } from 'react';
import { getSmoothStepPath, type EdgeProps, EdgeLabelRenderer } from '@xyflow/react';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import type { GateEdgeData } from '../utils/layoutWorkflow';
import './GateEdge.css';

/**
 * Map GateType enum to human-readable label
 */
function gateTypeLabel(gateType: GateType): string {
	switch (gateType) {
		case GateType.AUTO:
			return 'Auto';
		case GateType.HUMAN:
			return 'Human';
		case GateType.AI:
			return 'AI';
		case GateType.SKIP:
			return 'Skip';
		case GateType.UNSPECIFIED:
		default:
			return 'Passthrough';
	}
}

/**
 * Get CSS class modifier for gate type
 */
function gateTypeClass(gateType: GateType | undefined): string {
	switch (gateType) {
		case GateType.AUTO:
			return 'gate-edge__symbol--auto';
		case GateType.HUMAN:
			return 'gate-edge__symbol--human';
		case GateType.AI:
			return 'gate-edge__symbol--ai';
		case GateType.SKIP:
			return 'gate-edge__symbol--skip';
		case GateType.UNSPECIFIED:
		default:
			return 'gate-edge__symbol--passthrough';
	}
}

/**
 * Get CSS class modifier for gate status
 */
function gateStatusClass(status: GateEdgeData['gateStatus'] | undefined): string {
	switch (status) {
		case 'passed':
			return 'gate-edge__symbol--passed';
		case 'blocked':
			return 'gate-edge__symbol--blocked';
		case 'failed':
			return 'gate-edge__symbol--failed';
		default:
			return '';
	}
}

/**
 * GateEdge - Custom edge component that displays gate symbols on workflow edges.
 *
 * Visual design:
 * - Diamond symbol (◆) sits on the edge midpoint
 * - Color indicates gate type: gray=passthrough, blue=auto, yellow=human, purple=AI
 * - Status colors override type colors: green=passed, red=blocked/failed
 * - Tooltip on hover shows gate configuration
 */
export function GateEdge({
	id,
	sourceX,
	sourceY,
	targetX,
	targetY,
	sourcePosition,
	targetPosition,
	data,
}: EdgeProps) {
	const [showTooltip, setShowTooltip] = useState(false);

	// Use smooth step path for consistent edge rendering
	const [edgePath, labelX, labelY] = getSmoothStepPath({
		sourceX,
		sourceY,
		targetX,
		targetY,
		sourcePosition,
		targetPosition,
		borderRadius: 8,
	});

	const edgeData = data as GateEdgeData | undefined;
	const gateType = edgeData?.gateType ?? GateType.UNSPECIFIED;
	const gateStatus = edgeData?.gateStatus;
	const position = edgeData?.position ?? 'between';

	// Build CSS classes
	const typeClass = gateTypeClass(gateType);
	const statusClass = gateStatusClass(gateStatus);

	// Status class takes precedence over type class for visual styling
	const symbolClasses = [
		'gate-edge__symbol',
		typeClass,
		statusClass,
	].filter(Boolean).join(' ');

	const edgeClasses = [
		'gate-edge',
		position === 'entry' ? 'gate-edge--entry' : '',
		position === 'exit' ? 'gate-edge--exit' : '',
		position === 'between' ? 'gate-edge--between' : '',
	].filter(Boolean).join(' ');

	// Determine if we show the diamond symbol (skip/passthrough shows simple line)
	const showDiamond = gateType !== GateType.SKIP && gateType !== GateType.UNSPECIFIED;

	return (
		<>
			<g className={edgeClasses}>
				<path
					id={id}
					d={edgePath}
					fill="none"
					className="react-flow__edge-path"
				/>
				<path
					d={edgePath}
					fill="none"
					strokeOpacity={0}
					strokeWidth={20}
					className="react-flow__edge-interaction"
				/>
			</g>
			<EdgeLabelRenderer>
				<div
					className={symbolClasses}
					style={{
						position: 'absolute',
						transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
						pointerEvents: 'all',
					}}
					onMouseEnter={() => setShowTooltip(true)}
					onMouseLeave={() => setShowTooltip(false)}
				>
					{showDiamond ? '◆' : null}

					{showTooltip && (
						<div className="gate-edge__tooltip">
							<div className="gate-edge__tooltip-row">
								<span className="gate-edge__tooltip-label">Type:</span>
								<span className="gate-edge__tooltip-value">{gateTypeLabel(gateType)}</span>
							</div>
							{edgeData?.maxRetries !== undefined && (
								<div className="gate-edge__tooltip-row">
									<span className="gate-edge__tooltip-label">Max retries:</span>
									<span className="gate-edge__tooltip-value">{edgeData.maxRetries}</span>
								</div>
							)}
							{edgeData?.failureAction && (
								<div className="gate-edge__tooltip-row">
									<span className="gate-edge__tooltip-label">On failure:</span>
									<span className="gate-edge__tooltip-value">{edgeData.failureAction}</span>
								</div>
							)}
						</div>
					)}
				</div>
			</EdgeLabelRenderer>
		</>
	);
}
