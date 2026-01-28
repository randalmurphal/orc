/**
 * WorkflowsPage wrapper component for the /workflows route.
 *
 * Handles custom events from WorkflowsView:
 * - orc:select-workflow → Opens detail panel
 * - orc:clone-workflow → Opens clone modal
 * - orc:add-workflow → Opens create modal
 * - orc:edit-workflow → Opens edit modal
 * - orc:select-phase-template → Opens phase template detail panel
 * - orc:clone-phase-template → Opens clone phase template modal
 * - orc:edit-phase-template → Opens edit phase template modal
 *
 * Manages state for modals and detail panels.
 */

import { useState, useEffect, useCallback } from 'react';
import {
	WorkflowsView,
	WorkflowDetailPanel,
	CloneWorkflowModal,
	CreateWorkflowModal,
	EditWorkflowModal,
	PhaseTemplateDetailPanel,
	ClonePhaseTemplateModal,
	EditPhaseTemplateModal,
} from '@/components/workflows';
import { useWorkflowStore } from '@/stores/workflowStore';
import { useDocumentTitle } from '@/hooks';
import type { Workflow, PhaseTemplate, DefinitionSource } from '@/gen/orc/v1/workflow_pb';
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

	// Phase template state
	const [selectedTemplate, setSelectedTemplate] = useState<PhaseTemplate | null>(null);
	const [selectedTemplateSource, setSelectedTemplateSource] = useState<DefinitionSource | undefined>(undefined);
	const [templateDetailOpen, setTemplateDetailOpen] = useState(false);
	const [cloneTemplate, setCloneTemplate] = useState<PhaseTemplate | null>(null);
	const [cloneTemplateModalOpen, setCloneTemplateModalOpen] = useState(false);
	const [editTemplate, setEditTemplate] = useState<PhaseTemplate | null>(null);
	const [editTemplateSource, setEditTemplateSource] = useState<DefinitionSource | undefined>(undefined);
	const [editTemplateModalOpen, setEditTemplateModalOpen] = useState(false);

	// Store for refreshing data
	const { addWorkflow, removeWorkflow, updateWorkflow, refreshPhaseTemplates } = useWorkflowStore();

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

	// Handle orc:select-phase-template event
	const handleSelectPhaseTemplate = useCallback((event: CustomEvent<{ template: PhaseTemplate; source?: DefinitionSource }>) => {
		setSelectedTemplate(event.detail.template);
		setSelectedTemplateSource(event.detail.source);
		setTemplateDetailOpen(true);
	}, []);

	// Handle orc:clone-phase-template event
	const handleClonePhaseTemplate = useCallback((event: CustomEvent<{ template: PhaseTemplate }>) => {
		setCloneTemplate(event.detail.template);
		setCloneTemplateModalOpen(true);
	}, []);

	// Handle orc:edit-phase-template event
	const handleEditPhaseTemplate = useCallback((event: CustomEvent<{ template: PhaseTemplate; source?: DefinitionSource }>) => {
		setEditTemplate(event.detail.template);
		setEditTemplateSource(event.detail.source);
		setEditTemplateModalOpen(true);
	}, []);

	// Register event listeners
	useEffect(() => {
		const selectHandler = handleSelectWorkflow as EventListener;
		const cloneHandler = handleCloneWorkflow as EventListener;
		const addHandler = handleAddWorkflow;
		const editHandler = handleEditWorkflow as EventListener;
		const selectTemplateHandler = handleSelectPhaseTemplate as EventListener;
		const cloneTemplateHandler = handleClonePhaseTemplate as EventListener;
		const editTemplateHandler = handleEditPhaseTemplate as EventListener;

		window.addEventListener('orc:select-workflow', selectHandler);
		window.addEventListener('orc:clone-workflow', cloneHandler);
		window.addEventListener('orc:add-workflow', addHandler);
		window.addEventListener('orc:edit-workflow', editHandler);
		window.addEventListener('orc:select-phase-template', selectTemplateHandler);
		window.addEventListener('orc:clone-phase-template', cloneTemplateHandler);
		window.addEventListener('orc:edit-phase-template', editTemplateHandler);

		return () => {
			window.removeEventListener('orc:select-workflow', selectHandler);
			window.removeEventListener('orc:clone-workflow', cloneHandler);
			window.removeEventListener('orc:add-workflow', addHandler);
			window.removeEventListener('orc:edit-workflow', editHandler);
			window.removeEventListener('orc:select-phase-template', selectTemplateHandler);
			window.removeEventListener('orc:clone-phase-template', cloneTemplateHandler);
			window.removeEventListener('orc:edit-phase-template', editTemplateHandler);
		};
	}, [handleSelectWorkflow, handleCloneWorkflow, handleAddWorkflow, handleEditWorkflow, handleSelectPhaseTemplate, handleClonePhaseTemplate, handleEditPhaseTemplate]);

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

	// Phase template handlers
	const handleTemplateDetailClose = useCallback(() => {
		setTemplateDetailOpen(false);
	}, []);

	const handleCloneTemplateFromPanel = useCallback((template: PhaseTemplate) => {
		setCloneTemplate(template);
		setCloneTemplateModalOpen(true);
	}, []);

	const handleEditTemplateFromPanel = useCallback((template: PhaseTemplate) => {
		setEditTemplate(template);
		setEditTemplateSource(selectedTemplateSource);
		setEditTemplateModalOpen(true);
	}, [selectedTemplateSource]);

	const handleTemplateDeleted = useCallback(
		(_id: string) => {
			refreshPhaseTemplates();
			setTemplateDetailOpen(false);
			setSelectedTemplate(null);
		},
		[refreshPhaseTemplates]
	);

	const handleCloneTemplateModalClose = useCallback(() => {
		setCloneTemplateModalOpen(false);
		setCloneTemplate(null);
	}, []);

	const handleTemplateCloned = useCallback(
		(template: PhaseTemplate) => {
			refreshPhaseTemplates();
			setCloneTemplateModalOpen(false);
			setCloneTemplate(null);
			setSelectedTemplate(template);
			setSelectedTemplateSource(undefined); // Will be refreshed when the detail panel loads
			setTemplateDetailOpen(true);
		},
		[refreshPhaseTemplates]
	);

	const handleEditTemplateModalClose = useCallback(() => {
		setEditTemplateModalOpen(false);
		setEditTemplate(null);
	}, []);

	const handleTemplateUpdated = useCallback(
		(template: PhaseTemplate) => {
			refreshPhaseTemplates();
			setEditTemplateModalOpen(false);
			setEditTemplate(null);
			setSelectedTemplate(template);
		},
		[refreshPhaseTemplates]
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

			{/* Phase Template Detail Panel */}
			<PhaseTemplateDetailPanel
				template={selectedTemplate}
				source={selectedTemplateSource}
				isOpen={templateDetailOpen}
				onClose={handleTemplateDetailClose}
				onClone={handleCloneTemplateFromPanel}
				onEdit={handleEditTemplateFromPanel}
				onDeleted={handleTemplateDeleted}
			/>

			{/* Clone Phase Template Modal */}
			<ClonePhaseTemplateModal
				open={cloneTemplateModalOpen}
				template={cloneTemplate}
				onClose={handleCloneTemplateModalClose}
				onCloned={handleTemplateCloned}
			/>

			{/* Edit Phase Template Modal */}
			{editTemplate && (
				<EditPhaseTemplateModal
					open={editTemplateModalOpen}
					template={editTemplate}
					isBuiltin={editTemplateSource === undefined}
					onClose={handleEditTemplateModalClose}
					onUpdated={handleTemplateUpdated}
				/>
			)}
		</div>
	);
}
