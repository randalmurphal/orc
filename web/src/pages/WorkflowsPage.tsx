/**
 * WorkflowsPage wrapper component for the /workflows route.
 *
 * Handles custom events from WorkflowsView:
 * - orc:select-workflow → Opens detail panel
 * - orc:clone-workflow → Opens clone modal
 * - orc:add-workflow → Opens create modal
 * - orc:edit-workflow → Opens edit modal
 *
 * Manages state for modals and detail panel.
 */

import { useState, useEffect, useCallback } from 'react';
import {
	WorkflowsView,
	WorkflowDetailPanel,
	CloneWorkflowModal,
	CreateWorkflowModal,
	EditWorkflowModal,
} from '@/components/workflows';
import { useWorkflowStore } from '@/stores/workflowStore';
import { useDocumentTitle } from '@/hooks';
import type { Workflow } from '@/gen/orc/v1/workflow_pb';
import './WorkflowsPage.css';

/**
 * WorkflowsPage displays the workflows and phase templates configuration.
 * This is the page-level wrapper used in the router.
 */
export function WorkflowsPage() {
	useDocumentTitle('Workflows');
	// Selected workflow for detail panel
	const [selectedWorkflow, setSelectedWorkflow] = useState<Workflow | null>(null);
	const [detailPanelOpen, setDetailPanelOpen] = useState(false);

	// Clone modal state
	const [cloneWorkflow, setCloneWorkflow] = useState<Workflow | null>(null);
	const [cloneModalOpen, setCloneModalOpen] = useState(false);

	// Create modal state
	const [createModalOpen, setCreateModalOpen] = useState(false);

	// Edit modal state
	const [editWorkflow, setEditWorkflow] = useState<Workflow | null>(null);
	const [editModalOpen, setEditModalOpen] = useState(false);

	// Store for refreshing data
	const { addWorkflow, removeWorkflow, updateWorkflow } = useWorkflowStore();

	// Handle orc:select-workflow event
	const handleSelectWorkflow = useCallback((event: CustomEvent<{ workflow: Workflow }>) => {
		setSelectedWorkflow(event.detail.workflow);
		setDetailPanelOpen(true);
	}, []);

	// Handle orc:clone-workflow event
	const handleCloneWorkflow = useCallback((event: CustomEvent<{ workflow: Workflow }>) => {
		setCloneWorkflow(event.detail.workflow);
		setCloneModalOpen(true);
	}, []);

	// Handle orc:add-workflow event
	const handleAddWorkflow = useCallback(() => {
		setCreateModalOpen(true);
	}, []);

	// Handle orc:edit-workflow event
	const handleEditWorkflow = useCallback((event: CustomEvent<{ workflow: Workflow }>) => {
		setEditWorkflow(event.detail.workflow);
		setEditModalOpen(true);
	}, []);

	// Register event listeners
	useEffect(() => {
		const selectHandler = handleSelectWorkflow as EventListener;
		const cloneHandler = handleCloneWorkflow as EventListener;
		const addHandler = handleAddWorkflow;
		const editHandler = handleEditWorkflow as EventListener;

		window.addEventListener('orc:select-workflow', selectHandler);
		window.addEventListener('orc:clone-workflow', cloneHandler);
		window.addEventListener('orc:add-workflow', addHandler);
		window.addEventListener('orc:edit-workflow', editHandler);

		return () => {
			window.removeEventListener('orc:select-workflow', selectHandler);
			window.removeEventListener('orc:clone-workflow', cloneHandler);
			window.removeEventListener('orc:add-workflow', addHandler);
			window.removeEventListener('orc:edit-workflow', editHandler);
		};
	}, [handleSelectWorkflow, handleCloneWorkflow, handleAddWorkflow, handleEditWorkflow]);

	// Handle workflow cloned
	const handleWorkflowCloned = useCallback(
		(workflow: Workflow) => {
			addWorkflow(workflow);
			// Close clone modal and optionally open detail panel for new workflow
			setCloneModalOpen(false);
			setCloneWorkflow(null);
			setSelectedWorkflow(workflow);
			setDetailPanelOpen(true);
		},
		[addWorkflow]
	);

	// Handle workflow created
	const handleWorkflowCreated = useCallback(
		(workflow: Workflow) => {
			addWorkflow(workflow);
			// Close create modal and open detail panel for new workflow
			setCreateModalOpen(false);
			setSelectedWorkflow(workflow);
			setDetailPanelOpen(true);
		},
		[addWorkflow]
	);

	// Handle workflow deleted
	const handleWorkflowDeleted = useCallback(
		(id: string) => {
			removeWorkflow(id);
			setDetailPanelOpen(false);
			setSelectedWorkflow(null);
		},
		[removeWorkflow]
	);

	// Handle detail panel close
	const handleDetailPanelClose = useCallback(() => {
		setDetailPanelOpen(false);
	}, []);

	// Handle clone modal close
	const handleCloneModalClose = useCallback(() => {
		setCloneModalOpen(false);
		setCloneWorkflow(null);
	}, []);

	// Handle create modal close
	const handleCreateModalClose = useCallback(() => {
		setCreateModalOpen(false);
	}, []);

	// Handle clone from detail panel
	const handleCloneFromPanel = useCallback((workflow: Workflow) => {
		setCloneWorkflow(workflow);
		setCloneModalOpen(true);
	}, []);

	// Handle edit modal close
	const handleEditModalClose = useCallback(() => {
		setEditModalOpen(false);
		setEditWorkflow(null);
	}, []);

	// Handle workflow updated
	const handleWorkflowUpdated = useCallback(
		(workflow: Workflow) => {
			updateWorkflow(workflow.id, workflow);
			// Close edit modal and update selection
			setEditModalOpen(false);
			setEditWorkflow(null);
			setSelectedWorkflow(workflow);
		},
		[updateWorkflow]
	);

	return (
		<div className="workflows-page">
			<div className="workflows-page-content">
				<WorkflowsView />
			</div>

			{/* Detail Panel */}
			<WorkflowDetailPanel
				workflow={selectedWorkflow}
				isOpen={detailPanelOpen}
				onClose={handleDetailPanelClose}
				onClone={handleCloneFromPanel}
				onDeleted={handleWorkflowDeleted}
			/>

			{/* Clone Modal */}
			<CloneWorkflowModal
				open={cloneModalOpen}
				workflow={cloneWorkflow}
				onClose={handleCloneModalClose}
				onCloned={handleWorkflowCloned}
			/>

			{/* Create Modal */}
			<CreateWorkflowModal
				open={createModalOpen}
				onClose={handleCreateModalClose}
				onCreated={handleWorkflowCreated}
			/>

			{/* Edit Modal */}
			{editWorkflow && (
				<EditWorkflowModal
					open={editModalOpen}
					workflow={editWorkflow}
					onClose={handleEditModalClose}
					onUpdated={handleWorkflowUpdated}
				/>
			)}
		</div>
	);
}
