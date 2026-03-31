/**
 * WorkflowsPage wrapper component for the /workflows route.
 *
 * Handles workflow and phase-template selection, modal state, and detail panels.
 *
 * Manages state for modals and detail panels.
 */

import { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
	WorkflowsView,
	WorkflowDetailPanel,
	CloneWorkflowModal,
	EditWorkflowModal,
	PhaseTemplateDetailPanel,
	ClonePhaseTemplateModal,
	EditPhaseTemplateModal,
	CreatePhaseTemplateModal,
	WorkflowCreationWizard,
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
	const navigate = useNavigate();

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
	const [createTemplateModalOpen, setCreateTemplateModalOpen] = useState(false);

	// Store for refreshing data
	const { addWorkflow, removeWorkflow, updateWorkflow, refreshPhaseTemplates } = useWorkflowStore();

	const handleSelectWorkflow = useCallback((workflow: Workflow) => {
		setSelectedWorkflow(workflow);
		setDetailPanelOpen(true);
	}, []);

	const handleCloneWorkflow = useCallback((workflow: Workflow) => {
		setCloneWorkflow(workflow);
		setCloneModalOpen(true);
	}, []);

	const handleAddWorkflow = useCallback(() => {
		setCreateModalOpen(true);
	}, []);

	const handleEditWorkflow = useCallback((workflow: Workflow) => {
		setEditWorkflow(workflow);
		setEditModalOpen(true);
	}, []);

	const handleSelectPhaseTemplate = useCallback((template: PhaseTemplate, source?: DefinitionSource) => {
		setSelectedTemplate(template);
		setSelectedTemplateSource(source);
		setTemplateDetailOpen(true);
	}, []);

	const handleClonePhaseTemplate = useCallback((template: PhaseTemplate) => {
		setCloneTemplate(template);
		setCloneTemplateModalOpen(true);
	}, []);

	const handleEditPhaseTemplate = useCallback((template: PhaseTemplate) => {
		setEditTemplate(template);
		setEditTemplateSource(selectedTemplateSource);
		setEditTemplateModalOpen(true);
	}, [selectedTemplateSource]);

	const handleCreatePhaseTemplate = useCallback(() => {
		setCreateTemplateModalOpen(true);
	}, []);

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

	// Handle workflow created - navigate to editor
	const handleWorkflowCreated = useCallback(
		(workflow: Workflow) => {
			addWorkflow(workflow);
			setCreateModalOpen(false);
			// Navigate to the workflow editor
			navigate(`/workflows/${workflow.id}`);
		},
		[addWorkflow, navigate]
	);

	// Handle skip to editor - close wizard and navigate to create a blank workflow
	const handleSkipToEditor = useCallback(() => {
		setCreateModalOpen(false);
		// For skip to editor, we just close the wizard
		// The user can use the quick create in the editor or come back to wizard
	}, []);

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

	const handleCreateTemplateModalClose = useCallback(() => {
		setCreateTemplateModalOpen(false);
	}, []);

	const handleTemplateCreated = useCallback(
		(template: PhaseTemplate) => {
			refreshPhaseTemplates();
			setCreateTemplateModalOpen(false);
			setSelectedTemplate(template);
			setSelectedTemplateSource(undefined);
			setTemplateDetailOpen(true);
		},
		[refreshPhaseTemplates]
	);

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
				<WorkflowsView
					onSelectWorkflow={handleSelectWorkflow}
					onCloneWorkflow={handleCloneWorkflow}
					onEditWorkflow={handleEditWorkflow}
					onSelectPhaseTemplate={handleSelectPhaseTemplate}
					onCreateWorkflow={handleAddWorkflow}
					onCreatePhaseTemplate={handleCreatePhaseTemplate}
				/>
			</div>

			{/* Detail Panel */}
			<WorkflowDetailPanel
				workflow={selectedWorkflow}
				isOpen={detailPanelOpen}
				onClose={handleDetailPanelClose}
				onClone={handleCloneFromPanel}
				onEdit={handleEditWorkflow}
				onDeleted={handleWorkflowDeleted}
			/>

			{/* Clone Modal */}
			<CloneWorkflowModal
				open={cloneModalOpen}
				workflow={cloneWorkflow}
				onClose={handleCloneModalClose}
				onCloned={handleWorkflowCloned}
			/>

			{/* Create Wizard */}
			<WorkflowCreationWizard
				open={createModalOpen}
				onClose={handleCreateModalClose}
				onCreated={handleWorkflowCreated}
				onSkipToEditor={handleSkipToEditor}
			/>

			{/* Edit Modal */}
			{editWorkflow && (
				<EditWorkflowModal
					open={editModalOpen}
					workflowId={editWorkflow.id}
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

			{/* Create Phase Template Modal */}
			<CreatePhaseTemplateModal
				open={createTemplateModalOpen}
				onClose={handleCreateTemplateModalClose}
				onCreated={handleTemplateCreated}
			/>
		</div>
	);
}
