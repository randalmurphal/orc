import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { WorkflowEditorPage } from './WorkflowEditorPage';
import { workflowClient } from '@/lib/client';
import type { Workflow, WorkflowWithDetails } from '@/gen/orc/v1/workflow_pb';

// Mock the client
vi.mock('@/lib/client', () => ({
	workflowClient: {
		getWorkflow: vi.fn(),
		updateWorkflow: vi.fn(),
		listPhaseTemplates: vi.fn(),
		listWorkflowRuns: vi.fn(),
	},
}));

// Mock other dependencies
vi.mock('@/stores/workflowEditorStore');
vi.mock('@/stores/workflowStore');

const mockWorkflowDetails: WorkflowWithDetails = {
	workflow: {
		id: 'test-workflow',
		name: 'Test Workflow',
		description: 'A test workflow',
		defaultModel: 'claude-sonnet-3-5',
		defaultThinking: true,
		completionAction: 'pr',
		targetBranch: 'main',
		isBuiltin: false,
		basedOn: '',
		createdAt: undefined,
		updatedAt: undefined,
	},
	phases: [],
	variables: [],
};

const mockBuiltinWorkflowDetails: WorkflowWithDetails = {
	...mockWorkflowDetails,
	workflow: {
		...mockWorkflowDetails.workflow!,
		id: 'builtin-workflow',
		name: 'Built-in Workflow',
		isBuiltin: true,
	},
};

describe('WorkflowEditorPage Integration', () => {
	beforeEach(() => {
		vi.clearAllMocks();

		// Mock successful API responses by default
		(workflowClient.getWorkflow as any).mockResolvedValue({ workflow: mockWorkflowDetails });
		(workflowClient.listPhaseTemplates as any).mockResolvedValue({ templates: [], sources: {} });
		(workflowClient.listWorkflowRuns as any).mockResolvedValue({ runs: [] });
	});

	// SC-6: Integration with Existing Editor
	describe('Left Palette Integration', () => {
		it('renders workflow settings in left palette alongside phase templates', async () => {
			(workflowClient.getWorkflow as any).mockResolvedValue({ workflow: mockWorkflowDetails });

			render(
				<MemoryRouter initialEntries={['/workflows/test-workflow']}>
					<WorkflowEditorPage />
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeInTheDocument();
			});

			// Should have the left palette with both sections
			expect(screen.getByTestId('left-palette')).toBeInTheDocument();
			expect(screen.getByTestId('workflow-settings-panel')).toBeInTheDocument();
			expect(screen.getByTestId('phase-template-palette')).toBeInTheDocument();
		});

		it('preserves existing phase template drag and drop functionality', async () => {
			render(
				<MemoryRouter initialEntries={['/workflows/test-workflow']}>
					<WorkflowEditorPage />
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeInTheDocument();
			});

			// Phase template palette should still be present and functional
			const phaseTemplatePalette = screen.getByTestId('phase-template-palette');
			expect(phaseTemplatePalette).toBeInTheDocument();
			expect(phaseTemplatePalette).toHaveAttribute('data-preserves-drag-drop', 'true');
		});

		it('maintains existing canvas and inspector layout', async () => {
			render(
				<MemoryRouter initialEntries={['/workflows/test-workflow']}>
					<WorkflowEditorPage />
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeInTheDocument();
			});

			// Should maintain the existing three-column layout
			expect(screen.getByTestId('workflow-editor-canvas')).toBeInTheDocument();
			expect(screen.getByTestId('left-palette')).toBeInTheDocument();

			// Inspector panels should still work (when nodes/edges selected)
			const editorBody = screen.getByTestId('workflow-editor-body');
			expect(editorBody).toBeInTheDocument();
		});
	});

	// SC-5: Settings Persistence through Editor
	describe('Settings Persistence Integration', () => {
		it('updates workflow settings and refreshes editor state', async () => {
			const updatedWorkflow = {
				...mockWorkflowDetails.workflow!,
				name: 'Updated Workflow Name',
			};

			// Mock the update call and subsequent refresh
			(workflowClient.updateWorkflow as any).mockResolvedValue({ workflow: updatedWorkflow });
			(workflowClient.getWorkflow as any)
				.mockResolvedValueOnce({ workflow: mockWorkflowDetails })
				.mockResolvedValueOnce({ workflow: { ...mockWorkflowDetails, workflow: updatedWorkflow } });

			render(
				<MemoryRouter initialEntries={['/workflows/test-workflow']}>
					<WorkflowEditorPage />
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeInTheDocument();
			});

			// Find and update the name field in workflow settings
			const nameInput = screen.getByLabelText('Name');
			fireEvent.change(nameInput, { target: { value: 'Updated Workflow Name' } });
			fireEvent.blur(nameInput);

			await waitFor(() => {
				expect(workflowClient.updateWorkflow).toHaveBeenCalledWith({
					id: 'test-workflow',
					name: 'Updated Workflow Name',
				});
			});

			// The header should eventually show the updated name
			await waitFor(() => {
				expect(screen.getByText('Updated Workflow Name')).toBeInTheDocument();
			});
		});

		it('handles workflow settings update errors gracefully', async () => {
			(workflowClient.updateWorkflow as any).mockRejectedValue(new Error('Update failed'));

			render(
				<MemoryRouter initialEntries={['/workflows/test-workflow']}>
					<WorkflowEditorPage />
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeInTheDocument();
			});

			const nameInput = screen.getByLabelText('Name');
			fireEvent.change(nameInput, { target: { value: 'New Name' } });
			fireEvent.blur(nameInput);

			// Should show error message and not break the editor
			await waitFor(() => {
				expect(screen.getByText(/Failed to update workflow/)).toBeInTheDocument();
			});

			// Editor should still be functional
			expect(screen.getByTestId('workflow-editor-canvas')).toBeInTheDocument();
		});
	});

	// SC-1: Read-only behavior for builtin workflows
	describe('Builtin Workflow Handling', () => {
		it('shows read-only workflow settings for builtin workflows', async () => {
			(workflowClient.getWorkflow as any).mockResolvedValue({ workflow: mockBuiltinWorkflowDetails });

			render(
				<MemoryRouter initialEntries={['/workflows/builtin-workflow']}>
					<WorkflowEditorPage />
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Built-in Workflow')).toBeInTheDocument();
			});

			// Settings should be read-only
			expect(screen.getByText('Clone to customize')).toBeInTheDocument();
			expect(screen.getByLabelText('Name')).toBeDisabled();

			// Built-in badge should be shown in header
			expect(screen.getByText('Built-in')).toBeInTheDocument();

			// Clone button should be present
			expect(screen.getByText('Clone')).toBeInTheDocument();
		});

		it('maintains existing clone workflow functionality', async () => {
			(workflowClient.getWorkflow as any).mockResolvedValue({ workflow: mockBuiltinWorkflowDetails });

			render(
				<MemoryRouter initialEntries={['/workflows/builtin-workflow']}>
					<WorkflowEditorPage />
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Built-in Workflow')).toBeInTheDocument();
			});

			// Clone button should still work (existing functionality)
			const cloneButton = screen.getByText('Clone');
			fireEvent.click(cloneButton);

			// Should open clone modal (existing functionality)
			expect(screen.getByTestId('clone-workflow-modal')).toBeInTheDocument();
		});
	});

	// Performance and layout tests
	describe('Performance and Layout', () => {
		it('does not affect editor loading performance', async () => {
			const startTime = Date.now();

			render(
				<MemoryRouter initialEntries={['/workflows/test-workflow']}>
					<WorkflowEditorPage />
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeInTheDocument();
			});

			const loadTime = Date.now() - startTime;
			// Should load within reasonable time (< 1 second in test environment)
			expect(loadTime).toBeLessThan(1000);
		});

		it('maintains responsive layout with settings panel', async () => {
			// Mock narrow viewport
			Object.defineProperty(window, 'innerWidth', {
				writable: true,
				configurable: true,
				value: 768,
			});

			render(
				<MemoryRouter initialEntries={['/workflows/test-workflow']}>
					<WorkflowEditorPage />
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeInTheDocument();
			});

			const leftPalette = screen.getByTestId('left-palette');
			expect(leftPalette).toHaveClass('left-palette');

			// Should stack vertically on narrow screens
			const computedStyle = window.getComputedStyle(leftPalette);
			expect(computedStyle.flexDirection).toBe('column');
		});
	});
});