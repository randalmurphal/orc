/**
 * Integration Tests for WorkflowsPage - WorkflowSettingsModal Integration
 *
 * Tests for TASK-751: Implement complete workflow settings dialog
 *
 * Success Criteria Coverage:
 * - SC-9: WorkflowSettingsModal integrates with WorkflowsPage via orc:workflow-settings custom event
 *
 * INTEGRATION TEST PATTERN:
 * This test verifies that WorkflowSettingsModal is properly wired into WorkflowsPage.
 * It dispatches the orc:workflow-settings custom event and asserts that:
 * 1. The modal opens with the correct workflow data
 * 2. The modal is rendered within WorkflowsPage (not in isolation)
 *
 * This catches wiring bugs where:
 * - The event handler is not registered
 * - The modal component is not imported
 * - The modal is not rendered in the JSX
 */

import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { WorkflowsPage } from './WorkflowsPage';
import { workflowClient } from '@/lib/client';
import type { Workflow } from '@/gen/orc/v1/workflow_pb';
import { createMockWorkflow, createMockListWorkflowsResponse } from '@/test/factories';

// Mock all client methods used by WorkflowsPage and child components
vi.mock('@/lib/client', async (importOriginal) => {
	const actual = await importOriginal<typeof import('@/lib/client')>();
	return {
		...actual,
		workflowClient: {
			listWorkflows: vi.fn(),
			updateWorkflow: vi.fn(),
			getWorkflow: vi.fn(),
			createWorkflow: vi.fn(),
			deleteWorkflow: vi.fn(),
			cloneWorkflow: vi.fn(),
			listPhaseTemplates: vi.fn(),
			createPhaseTemplate: vi.fn(),
			updatePhaseTemplate: vi.fn(),
			deletePhaseTemplate: vi.fn(),
			clonePhaseTemplate: vi.fn(),
			addPhase: vi.fn(),
			updatePhase: vi.fn(),
			removePhase: vi.fn(),
		},
		configClient: {
			listAgents: vi.fn().mockResolvedValue({ agents: [] }),
			listHooks: vi.fn().mockResolvedValue({ hooks: [] }),
			listSkills: vi.fn().mockResolvedValue({ skills: [] }),
		},
		mcpClient: {
			listMCPServers: vi.fn().mockResolvedValue({ servers: [] }),
		},
	};
});

// Mock the workflow store
vi.mock('@/stores/workflowStore', () => ({
	useWorkflowStore: vi.fn(() => ({
		workflows: [],
		phaseTemplates: [],
		addWorkflow: vi.fn(),
		removeWorkflow: vi.fn(),
		updateWorkflow: vi.fn(),
		refreshPhaseTemplates: vi.fn(),
	})),
}));

// NOTE: Browser API mocks are set up globally in test-setup.ts - do not duplicate here

const mockWorkflow = createMockWorkflow({
	id: 'custom-workflow-1',
	name: 'My Custom Workflow',
	description: 'A custom workflow for testing',
	defaultModel: 'sonnet',
	defaultThinking: true,
	completionAction: 'pr',
	targetBranch: 'main',
	isBuiltin: false,
});

describe('WorkflowsPage Integration: WorkflowSettingsModal', () => {
	beforeEach(() => {
		vi.clearAllMocks();

		// Setup default mock responses
		(workflowClient.listWorkflows as ReturnType<typeof vi.fn>).mockResolvedValue(
			createMockListWorkflowsResponse([mockWorkflow])
		);
		(workflowClient.listPhaseTemplates as ReturnType<typeof vi.fn>).mockResolvedValue({
			templates: [],
		});
	});

	afterEach(() => {
		vi.clearAllMocks();
	});

	describe('SC-9: Custom Event Integration', () => {
		it('opens WorkflowSettingsModal when orc:workflow-settings event is dispatched', async () => {
			render(
				<MemoryRouter>
					<WorkflowsPage />
				</MemoryRouter>
			);

			// Wait for initial load to complete
			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Dispatch the custom event
			await act(async () => {
				const event = new CustomEvent('orc:workflow-settings', {
					detail: { workflow: mockWorkflow },
				});
				window.dispatchEvent(event);
			});

			// The modal should open with the workflow data
			await waitFor(() => {
				expect(screen.getByRole('dialog')).toBeInTheDocument();
			});

			// Should display the workflow settings form
			expect(screen.getByLabelText(/name/i)).toHaveValue('My Custom Workflow');
		});

		it('passes correct workflow data to modal', async () => {
			render(
				<MemoryRouter>
					<WorkflowsPage />
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Dispatch event with workflow data
			await act(async () => {
				const event = new CustomEvent('orc:workflow-settings', {
					detail: { workflow: mockWorkflow },
				});
				window.dispatchEvent(event);
			});

			await waitFor(() => {
				expect(screen.getByRole('dialog')).toBeInTheDocument();
			});

			// Verify all fields are populated with workflow data
			expect(screen.getByLabelText(/name/i)).toHaveValue('My Custom Workflow');
			expect(screen.getByLabelText(/description/i)).toHaveValue('A custom workflow for testing');
		});

		it('closes modal when close is triggered', async () => {
			render(
				<MemoryRouter>
					<WorkflowsPage />
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Open the modal
			await act(async () => {
				const event = new CustomEvent('orc:workflow-settings', {
					detail: { workflow: mockWorkflow },
				});
				window.dispatchEvent(event);
			});

			await waitFor(() => {
				expect(screen.getByRole('dialog')).toBeInTheDocument();
			});

			// Close the modal
			const closeButton = screen.getByRole('button', { name: /close|cancel|×/i });
			await act(async () => {
				fireEvent.click(closeButton);
			});

			// Modal should be closed
			await waitFor(() => {
				expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
			});
		});

		it('cleans up event listener on unmount', async () => {
			const removeEventListenerSpy = vi.spyOn(window, 'removeEventListener');

			const { unmount } = render(
				<MemoryRouter>
					<WorkflowsPage />
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			unmount();

			// Should clean up the event listener
			expect(removeEventListenerSpy).toHaveBeenCalledWith(
				'orc:workflow-settings',
				expect.any(Function)
			);

			removeEventListenerSpy.mockRestore();
		});

		it('event listener is registered on mount', async () => {
			const addEventListenerSpy = vi.spyOn(window, 'addEventListener');

			render(
				<MemoryRouter>
					<WorkflowsPage />
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should have registered the event listener
			expect(addEventListenerSpy).toHaveBeenCalledWith(
				'orc:workflow-settings',
				expect.any(Function)
			);

			addEventListenerSpy.mockRestore();
		});

		it('updates workflow via modal and reflects changes', async () => {
			const updatedWorkflow = createMockWorkflow({
				...mockWorkflow,
				name: 'Updated Workflow Name',
			});

			(workflowClient.updateWorkflow as ReturnType<typeof vi.fn>).mockResolvedValue({
				workflow: updatedWorkflow,
			});

			render(
				<MemoryRouter>
					<WorkflowsPage />
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Open the modal
			await act(async () => {
				const event = new CustomEvent('orc:workflow-settings', {
					detail: { workflow: mockWorkflow },
				});
				window.dispatchEvent(event);
			});

			await waitFor(() => {
				expect(screen.getByRole('dialog')).toBeInTheDocument();
			});

			// Update the name field
			const nameInput = screen.getByLabelText(/name/i);
			await act(async () => {
				fireEvent.change(nameInput, { target: { value: 'Updated Workflow Name' } });
				fireEvent.blur(nameInput);
			});

			// API should be called with updated data
			await waitFor(() => {
				expect(workflowClient.updateWorkflow).toHaveBeenCalledWith(
					expect.objectContaining({
						id: 'custom-workflow-1',
						name: 'Updated Workflow Name',
					})
				);
			});
		});

		it('handles missing workflow in event detail gracefully', async () => {
			render(
				<MemoryRouter>
					<WorkflowsPage />
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Dispatch event with null workflow
			await act(async () => {
				const event = new CustomEvent('orc:workflow-settings', {
					detail: { workflow: null },
				});
				window.dispatchEvent(event);
			});

			// Modal should not open with null workflow
			expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
		});
	});
});

/**
 * WIRING VERIFICATION
 *
 * This test file verifies the critical wiring:
 *
 * | New Code | Imported By | Verified By |
 * |----------|-------------|-------------|
 * | WorkflowSettingsModal | WorkflowsPage | Event handler opens modal |
 * | orc:workflow-settings handler | WorkflowsPage useEffect | Event listener registered |
 *
 * If the implementation forgets to:
 * - Import WorkflowSettingsModal → Test fails: modal not found
 * - Register event listener → Test fails: modal doesn't open on event
 * - Render modal in JSX → Test fails: dialog role not found
 * - Pass workflow prop → Test fails: form fields empty
 */
