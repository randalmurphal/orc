/**
 * Integration Tests for WorkflowsPage - Workflow Creation Wizard Integration
 *
 * Tests for TASK-746: Implement guided workflow creation wizard
 *
 * SUCCESS CRITERIA COVERAGE (Integration):
 * - SC-13: WorkflowsPage integrates wizard and opens editor on completion
 *
 * CRITICAL: These are INTEGRATION tests that verify the WorkflowCreationWizard
 * is properly wired into the WorkflowsPage. Unit tests in WorkflowCreationWizard.test.tsx
 * test the wizard in isolation, but if the wiring is missing, the wizard is never
 * rendered.
 *
 * This test MUST:
 * 1. Import WorkflowsPage (the parent), NOT WorkflowCreationWizard directly
 * 2. Verify clicking "+ New Workflow" opens the wizard (not old CreateWorkflowModal)
 * 3. Verify completing the wizard navigates to the workflow editor
 *
 * These tests will FAIL until:
 * - WorkflowCreationWizard component is created
 * - WorkflowsPage uses WorkflowCreationWizard instead of CreateWorkflowModal
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { WorkflowsPage } from './WorkflowsPage';
import {
	createMockWorkflow,
	createMockCreateWorkflowResponse,
	createMockListWorkflowsResponse,
	createMockPhaseTemplate,
	createMockListPhaseTemplatesResponse,
	createMockAddPhaseResponse,
	createMockWorkflowPhase,
} from '@/test/factories';

// Mock the workflow store
vi.mock('@/stores/workflowStore', () => ({
	useWorkflowStore: vi.fn(() => ({
		workflows: [
			createMockWorkflow({ id: 'small', name: 'Small', isBuiltin: true }),
			createMockWorkflow({ id: 'medium', name: 'Medium', isBuiltin: true }),
		],
		phaseTemplates: [
			createMockPhaseTemplate({ id: 'spec', name: 'Specification' }),
			createMockPhaseTemplate({ id: 'implement', name: 'Implementation' }),
			createMockPhaseTemplate({ id: 'review', name: 'Review' }),
		],
		loading: false,
		error: null,
		addWorkflow: vi.fn(),
		removeWorkflow: vi.fn(),
		updateWorkflow: vi.fn(),
		setWorkflows: vi.fn(),
		refreshWorkflows: vi.fn(),
		refreshPhaseTemplates: vi.fn(),
	})),
}));

// Mock the client module - use importOriginal and only override what we need
vi.mock('@/lib/client', async (importOriginal) => {
	const actual = await importOriginal<typeof import('@/lib/client')>();
	return {
		...actual,
		workflowClient: {
			createWorkflow: vi.fn(),
			listWorkflows: vi.fn().mockResolvedValue({ workflows: [], phaseCounts: {} }),
			listPhaseTemplates: vi.fn().mockResolvedValue({ templates: [] }),
			getWorkflow: vi.fn(),
			deleteWorkflow: vi.fn(),
			updateWorkflow: vi.fn(),
			cloneWorkflow: vi.fn(),
			createPhaseTemplate: vi.fn(),
			updatePhaseTemplate: vi.fn(),
			deletePhaseTemplate: vi.fn(),
			clonePhaseTemplate: vi.fn(),
			saveWorkflowLayout: vi.fn(),
			validateWorkflow: vi.fn(),
			addPhase: vi.fn(),
			updatePhase: vi.fn(),
			removePhase: vi.fn(),
		},
		configClient: {
			listAgents: vi.fn().mockResolvedValue({ agents: [] }),
			listHooks: vi.fn().mockResolvedValue({ hooks: [] }),
			listSkills: vi.fn().mockResolvedValue({ skills: [] }),
			listMcpServers: vi.fn().mockResolvedValue({ servers: [] }),
			getConfig: vi.fn().mockResolvedValue({ config: {} }),
		},
		mcpClient: {
			listMCPServers: vi.fn().mockResolvedValue({ servers: [] }),
		},
		taskClient: {
			listTasks: vi.fn().mockResolvedValue({ tasks: [] }),
		},
		projectClient: {
			getProject: vi.fn().mockResolvedValue({ project: { id: 'test-project' } }),
		},
	};
});

// Mock useNavigate - must mock 'react-router-dom' since that's what WorkflowsPage imports
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
	const actual = await vi.importActual('react-router-dom');
	return {
		...actual,
		useNavigate: () => mockNavigate,
	};
});

import { workflowClient } from '@/lib/client';

function renderWithRouter(component: React.ReactElement) {
	return render(
		<MemoryRouter initialEntries={['/workflows']}>
			{component}
		</MemoryRouter>
	);
}

describe('WorkflowsPage - Wizard Integration', () => {
	beforeEach(() => {
		vi.clearAllMocks();

		// Setup default mock responses
		vi.mocked(workflowClient.listWorkflows).mockResolvedValue(
			createMockListWorkflowsResponse([
				createMockWorkflow({ id: 'small', name: 'Small', isBuiltin: true }),
				createMockWorkflow({ id: 'medium', name: 'Medium', isBuiltin: true }),
			])
		);

		vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
			createMockListPhaseTemplatesResponse([
				createMockPhaseTemplate({ id: 'spec', name: 'Specification' }),
				createMockPhaseTemplate({ id: 'implement', name: 'Implementation' }),
				createMockPhaseTemplate({ id: 'review', name: 'Review' }),
				createMockPhaseTemplate({ id: 'docs', name: 'Documentation' }),
			])
		);

		// Default mock for addPhase (returns success - just needs to resolve)
		vi.mocked(workflowClient.addPhase).mockResolvedValue(
			createMockAddPhaseResponse(createMockWorkflowPhase())
		);
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-13: WorkflowsPage integrates wizard', () => {
		it('clicking New Workflow button opens the creation wizard', async () => {
			const user = userEvent.setup();
			renderWithRouter(<WorkflowsPage />);

			// Find and click the New Workflow button
			const newWorkflowButton = await screen.findByRole('button', { name: /new workflow/i });
			await user.click(newWorkflowButton);

			// The WIZARD should open, showing intent selection (Step 1)
			// This FAILS if WorkflowsPage still uses CreateWorkflowModal
			await waitFor(() => {
				expect(screen.getByText(/what kind of workflow/i)).toBeInTheDocument();
			});

			// Verify wizard intent buttons are present
			expect(screen.getByRole('button', { name: /build/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /review/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /custom/i })).toBeInTheDocument();
		});

		it('dispatches orc:add-workflow event opens wizard (not old modal)', async () => {
			renderWithRouter(<WorkflowsPage />);

			// Wait for page to load
			await waitFor(() => {
				expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
			});

			// Dispatch the custom event (this is how WorkflowsView triggers modal open)
			window.dispatchEvent(new CustomEvent('orc:add-workflow'));

			// Should show the WIZARD, not the old CreateWorkflowModal
			await waitFor(() => {
				// The wizard shows "What kind of workflow?" as its first step
				expect(screen.getByText(/what kind of workflow/i)).toBeInTheDocument();
			});

			// Verify it's not the old modal (which would show "Workflow ID" as first field)
			expect(screen.queryByLabelText(/workflow id/i)).not.toBeInTheDocument();
		});

		it('completing wizard creates workflow and navigates to editor', async () => {
			const user = userEvent.setup();
			const mockWorkflow = createMockWorkflow({
				id: 'my-new-workflow',
				name: 'My New Workflow',
				isBuiltin: false,
			});

			vi.mocked(workflowClient.createWorkflow).mockResolvedValue(
				createMockCreateWorkflowResponse(mockWorkflow)
			);

			renderWithRouter(<WorkflowsPage />);

			// Open wizard
			window.dispatchEvent(new CustomEvent('orc:add-workflow'));

			await waitFor(() => {
				expect(screen.getByText(/what kind of workflow/i)).toBeInTheDocument();
			});

			// Step 1: Select intent
			await user.click(screen.getByRole('button', { name: /build/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));

			// Step 2: Enter name
			await waitFor(() => {
				expect(screen.getByLabelText(/name/i)).toBeInTheDocument();
			});
			await user.type(screen.getByLabelText(/name/i), 'My New Workflow');
			await user.click(screen.getByRole('button', { name: /next/i }));

			// Step 3: Confirm phases and create
			await waitFor(() => {
				expect(screen.getByRole('button', { name: /create.*editor/i })).toBeInTheDocument();
			});
			await user.click(screen.getByRole('button', { name: /create.*editor/i }));

			// Should create workflow via API
			await waitFor(() => {
				expect(workflowClient.createWorkflow).toHaveBeenCalledWith(
					expect.objectContaining({
						id: 'my-new-workflow',
						name: 'My New Workflow',
					})
				);
			});

			// addPhase should be called for each selected phase
			await waitFor(() => {
				expect(workflowClient.addPhase).toHaveBeenCalled();
			});

			// Should navigate to workflow editor
			await waitFor(() => {
				expect(mockNavigate).toHaveBeenCalledWith(
					'/workflows/my-new-workflow'
				);
			});
		});

		it('Skip to Editor button navigates directly to editor (blank workflow)', async () => {
			const user = userEvent.setup();
			const mockWorkflow = createMockWorkflow({
				id: 'blank-workflow',
				name: 'Blank Workflow',
				isBuiltin: false,
			});

			vi.mocked(workflowClient.createWorkflow).mockResolvedValue(
				createMockCreateWorkflowResponse(mockWorkflow)
			);

			renderWithRouter(<WorkflowsPage />);

			// Open wizard
			window.dispatchEvent(new CustomEvent('orc:add-workflow'));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /skip to editor/i })).toBeInTheDocument();
			});

			// Click Skip to Editor
			await user.click(screen.getByRole('button', { name: /skip to editor/i }));

			// Should either:
			// 1. Navigate to editor with a new blank workflow, OR
			// 2. Show a quick name prompt then navigate
			// The exact behavior depends on implementation, but it should close the wizard
			await waitFor(() => {
				// Modal should close
				expect(screen.queryByText(/what kind of workflow/i)).not.toBeInTheDocument();
			});
		});

		it('wizard closes on Cancel without creating workflow', async () => {
			const user = userEvent.setup();
			renderWithRouter(<WorkflowsPage />);

			// Open wizard
			window.dispatchEvent(new CustomEvent('orc:add-workflow'));

			await waitFor(() => {
				expect(screen.getByText(/what kind of workflow/i)).toBeInTheDocument();
			});

			// Click Cancel
			await user.click(screen.getByRole('button', { name: /cancel/i }));

			// Wizard should close
			await waitFor(() => {
				expect(screen.queryByText(/what kind of workflow/i)).not.toBeInTheDocument();
			});

			// No workflow should be created
			expect(workflowClient.createWorkflow).not.toHaveBeenCalled();
		});

		it('wizard closes via escape key', async () => {
			renderWithRouter(<WorkflowsPage />);

			// Open wizard
			window.dispatchEvent(new CustomEvent('orc:add-workflow'));

			await waitFor(() => {
				expect(screen.getByText(/what kind of workflow/i)).toBeInTheDocument();
			});

			// Press Escape
			fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' });

			// Wizard should close
			await waitFor(() => {
				expect(screen.queryByText(/what kind of workflow/i)).not.toBeInTheDocument();
			});
		});
	});

	describe('Wiring Verification - CreateWorkflowModal replacement', () => {
		/**
		 * CRITICAL: This test verifies that the OLD CreateWorkflowModal is no longer
		 * used for the main "New Workflow" action. The wizard should replace it.
		 *
		 * The old modal showed:
		 * - "Workflow ID" as the first field
		 * - No multi-step flow
		 * - No intent selection
		 *
		 * The new wizard shows:
		 * - "What kind of workflow?" as the first screen
		 * - Multi-step flow with Back/Next
		 * - Intent-based phase recommendations
		 */
		it('does NOT show old CreateWorkflowModal UI elements', async () => {
			renderWithRouter(<WorkflowsPage />);

			// Open the workflow creation flow
			window.dispatchEvent(new CustomEvent('orc:add-workflow'));

			await waitFor(() => {
				// Wait for modal to appear
				expect(screen.getByRole('dialog')).toBeInTheDocument();
			});

			// The OLD modal would show these elements immediately:
			// - "Workflow ID" label as first field
			// - No step indicator
			// - No intent buttons

			// These should NOT be present in the new wizard's first step
			expect(screen.queryByLabelText(/^workflow id$/i)).not.toBeInTheDocument();

			// The NEW wizard SHOULD show:
			// - Intent selection header
			// - Intent buttons (Build, Review, etc.)
			// - Skip to Editor option
			expect(screen.getByText(/what kind of workflow/i)).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /build/i })).toBeInTheDocument();
		});
	});
});
