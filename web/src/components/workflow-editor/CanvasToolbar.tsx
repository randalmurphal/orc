/**
 * CanvasToolbar - Canvas controls for zoom, fit view, and layout reset (SC-12)
 *
 * Features:
 * - Fit View: Centers all nodes in viewport
 * - Reset Layout: Clears stored positions, re-runs dagre
 * - Zoom In/Out: Adjusts zoom level
 * - Read-only mode: Reset Layout disabled for built-in workflows
 */

import { useCallback, useState } from 'react';
import { useReactFlow } from '@xyflow/react';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import { useWorkflowEditorStore } from '@/stores/workflowEditorStore';
import { workflowClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';
import './CanvasToolbar.css';

interface CanvasToolbarProps {
	onWorkflowRefresh?: () => void;
}

export function CanvasToolbar({ onWorkflowRefresh }: CanvasToolbarProps) {
	const { fitView, zoomIn, zoomOut } = useReactFlow();
	const readOnly = useWorkflowEditorStore((s) => s.readOnly);
	const workflowDetails = useWorkflowEditorStore((s) => s.workflowDetails);
	const [resetting, setResetting] = useState(false);

	const handleFitView = useCallback(() => {
		fitView({ padding: 0.1 });
	}, [fitView]);

	const handleZoomIn = useCallback(() => {
		zoomIn();
	}, [zoomIn]);

	const handleZoomOut = useCallback(() => {
		zoomOut();
	}, [zoomOut]);

	const handleResetLayout = useCallback(async () => {
		if (!workflowDetails?.workflow?.id) return;

		setResetting(true);
		try {
			// Clear all stored positions by saving empty array
			await workflowClient.saveWorkflowLayout({
				workflowId: workflowDetails.workflow.id,
				positions: [],
			});

			// Refresh workflow to reload with dagre-computed positions
			if (onWorkflowRefresh) {
				onWorkflowRefresh();
			} else {
				// If no refresh callback, at least reload the workflow
				await workflowClient.getWorkflow({ id: workflowDetails.workflow.id });
			}
		} catch (error) {
			const message = error instanceof Error ? error.message : 'Failed to reset layout';
			toast.error(message);
		} finally {
			setResetting(false);
		}
	}, [workflowDetails, onWorkflowRefresh]);

	return (
		<div className="canvas-toolbar" role="toolbar" aria-label="Canvas controls">
			<Button
				variant="ghost"
				size="sm"
				onClick={handleFitView}
				aria-label="Fit View"
				title="Fit all nodes in view"
			>
				<Icon name="maximize" size={16} />
				<span className="canvas-toolbar__label">Fit View</span>
			</Button>

			<Button
				variant="ghost"
				size="sm"
				onClick={handleResetLayout}
				disabled={readOnly || resetting}
				aria-label="Reset Layout"
				title={readOnly ? 'Clone workflow to reset layout' : 'Reset to automatic layout'}
				loading={resetting}
			>
				<Icon name="refresh" size={16} />
				<span className="canvas-toolbar__label">Reset Layout</span>
			</Button>

			<div className="canvas-toolbar__divider" />

			<Button
				variant="ghost"
				size="sm"
				onClick={handleZoomIn}
				aria-label="Zoom In"
				title="Zoom in"
				iconOnly
			>
				<Icon name="plus" size={16} />
			</Button>

			<Button
				variant="ghost"
				size="sm"
				onClick={handleZoomOut}
				aria-label="Zoom Out"
				title="Zoom out"
				iconOnly
			>
				<Icon name="minus" size={16} />
			</Button>
		</div>
	);
}
