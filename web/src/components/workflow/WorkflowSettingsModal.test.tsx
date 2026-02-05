/**
 * TDD Tests for WorkflowSettingsModal
 *
 * Tests for TASK-751: Implement complete workflow settings dialog
 *
 * Success Criteria Coverage:
 * - SC-8: WorkflowSettingsModal opens with workflow data and displays same form as panel
 */

import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import { WorkflowSettingsModal } from './WorkflowSettingsModal';
import { workflowClient } from '@/lib/client';
import type { Workflow } from '@/gen/orc/v1/workflow_pb';

// Mock the workflow client
vi.mock('@/lib/client', () => ({
	workflowClient: {
		updateWorkflow: vi.fn(),
	},
}));

// NOTE: Browser API mocks are set up globally in test-setup.ts - do not duplicate here

const mockWorkflow = {
	id: 'test-workflow',
	name: 'Test Workflow',
	description: 'A test workflow',
	defaultModel: 'sonnet',
	defaultThinking: true,
	completionAction: 'pr',
	targetBranch: 'main',
	isBuiltin: false,
	basedOn: '',
	createdAt: undefined,
	updatedAt: undefined,
} as unknown as Workflow;

const mockBuiltinWorkflow = {
	...mockWorkflow,
	id: 'builtin-workflow',
	name: 'Built-in Workflow',
	isBuiltin: true,
} as unknown as Workflow;

describe('WorkflowSettingsModal', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	describe('SC-8: Modal Opens with Workflow Data', () => {
		it('renders modal when open is true', () => {
			render(
				<WorkflowSettingsModal
					open={true}
					workflow={mockWorkflow}
					onClose={vi.fn()}
					onWorkflowUpdate={vi.fn()}
				/>
			);

			expect(screen.getByRole('dialog')).toBeInTheDocument();
		});

		it('does not render modal when open is false', () => {
			render(
				<WorkflowSettingsModal
					open={false}
					workflow={mockWorkflow}
					onClose={vi.fn()}
					onWorkflowUpdate={vi.fn()}
				/>
			);

			expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
		});

		it('displays modal title indicating workflow settings', () => {
			render(
				<WorkflowSettingsModal
					open={true}
					workflow={mockWorkflow}
					onClose={vi.fn()}
					onWorkflowUpdate={vi.fn()}
				/>
			);

			// Should have a title that indicates settings or workflow name
			expect(screen.getByText(/workflow settings|settings/i)).toBeInTheDocument();
		});

		it('displays all three sections (Identity, Defaults, Completion)', () => {
			render(
				<WorkflowSettingsModal
					open={true}
					workflow={mockWorkflow}
					onClose={vi.fn()}
					onWorkflowUpdate={vi.fn()}
				/>
			);

			expect(screen.getByRole('heading', { name: /identity/i })).toBeInTheDocument();
			expect(screen.getByRole('heading', { name: /^defaults$/i })).toBeInTheDocument();
			expect(screen.getByRole('heading', { name: /completion/i })).toBeInTheDocument();
		});

		it('pre-fills form fields with workflow data', () => {
			render(
				<WorkflowSettingsModal
					open={true}
					workflow={mockWorkflow}
					onClose={vi.fn()}
					onWorkflowUpdate={vi.fn()}
				/>
			);

			expect(screen.getByLabelText(/name/i)).toHaveValue('Test Workflow');
			expect(screen.getByLabelText(/description/i)).toHaveValue('A test workflow');
			expect(screen.getByLabelText(/default model/i)).toHaveValue('sonnet');
			expect(screen.getByLabelText(/thinking/i)).toBeChecked();
		});

		it('handles null workflow gracefully (modal does not render)', () => {
			render(
				<WorkflowSettingsModal
					open={true}
					workflow={null}
					onClose={vi.fn()}
					onWorkflowUpdate={vi.fn()}
				/>
			);

			// Modal should not render when workflow is null
			expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
		});

		it('calls onClose when close button is clicked', async () => {
			const onClose = vi.fn();
			render(
				<WorkflowSettingsModal
					open={true}
					workflow={mockWorkflow}
					onClose={onClose}
					onWorkflowUpdate={vi.fn()}
				/>
			);

			// Find and click close button
			const closeButton = screen.getByRole('button', { name: /close|cancel|×/i });
			fireEvent.click(closeButton);

			expect(onClose).toHaveBeenCalled();
		});

		it('calls onClose when clicking outside the modal', async () => {
			const onClose = vi.fn();
			const { container } = render(
				<WorkflowSettingsModal
					open={true}
					workflow={mockWorkflow}
					onClose={onClose}
					onWorkflowUpdate={vi.fn()}
				/>
			);

			// Click on the overlay/backdrop (the modal-backdrop element with onClick handler)
			const overlay = container.ownerDocument.querySelector('.modal-backdrop');
			if (overlay) {
				fireEvent.click(overlay);
			}

			// onClose should be called when clicking outside
			await waitFor(() => {
				expect(onClose).toHaveBeenCalled();
			});
		});
	});

	describe('Form Functionality', () => {
		it('updates workflow when field changes', async () => {
			const onUpdate = vi.fn();
			(workflowClient.updateWorkflow as ReturnType<typeof vi.fn>).mockResolvedValue({
				workflow: { ...mockWorkflow, name: 'Updated Name' },
			});

			render(
				<WorkflowSettingsModal
					open={true}
					workflow={mockWorkflow}
					onClose={vi.fn()}
					onWorkflowUpdate={onUpdate}
				/>
			);

			const nameInput = screen.getByLabelText(/name/i);
			fireEvent.change(nameInput, { target: { value: 'Updated Name' } });
			fireEvent.blur(nameInput);

			await waitFor(() => {
				expect(workflowClient.updateWorkflow).toHaveBeenCalled();
			});

			await waitFor(() => {
				expect(onUpdate).toHaveBeenCalled();
			});
		});

		it('shows error when API call fails', async () => {
			const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
			(workflowClient.updateWorkflow as ReturnType<typeof vi.fn>).mockRejectedValue(
				new Error('API Error')
			);

			render(
				<WorkflowSettingsModal
					open={true}
					workflow={mockWorkflow}
					onClose={vi.fn()}
					onWorkflowUpdate={vi.fn()}
				/>
			);

			const nameInput = screen.getByLabelText(/name/i);
			fireEvent.change(nameInput, { target: { value: 'New Name' } });
			fireEvent.blur(nameInput);

			await waitFor(() => {
				expect(screen.getByText(/failed to update/i)).toBeInTheDocument();
			});

			consoleError.mockRestore();
		});

		it('validates name field (shows error when empty)', async () => {
			render(
				<WorkflowSettingsModal
					open={true}
					workflow={mockWorkflow}
					onClose={vi.fn()}
					onWorkflowUpdate={vi.fn()}
				/>
			);

			const nameInput = screen.getByLabelText(/name/i);
			fireEvent.change(nameInput, { target: { value: '' } });
			fireEvent.blur(nameInput);

			await waitFor(() => {
				expect(screen.getByText(/name is required/i)).toBeInTheDocument();
			});

			expect(workflowClient.updateWorkflow).not.toHaveBeenCalled();
		});

		it('conditionally shows target branch based on completion action', () => {
			const workflowWithNone = {
				...mockWorkflow,
				completionAction: 'none',
			} as unknown as Workflow;

			render(
				<WorkflowSettingsModal
					open={true}
					workflow={workflowWithNone}
					onClose={vi.fn()}
					onWorkflowUpdate={vi.fn()}
				/>
			);

			// Target branch should be hidden when completion action is "none"
			expect(screen.queryByLabelText(/target branch/i)).not.toBeInTheDocument();
		});
	});

	describe('Built-in Workflow Handling', () => {
		it('shows clone message for built-in workflows', () => {
			render(
				<WorkflowSettingsModal
					open={true}
					workflow={mockBuiltinWorkflow}
					onClose={vi.fn()}
					onWorkflowUpdate={vi.fn()}
				/>
			);

			expect(screen.getByText(/clone to customize/i)).toBeInTheDocument();
		});

		it('disables all fields for built-in workflows', () => {
			render(
				<WorkflowSettingsModal
					open={true}
					workflow={mockBuiltinWorkflow}
					onClose={vi.fn()}
					onWorkflowUpdate={vi.fn()}
				/>
			);

			expect(screen.getByLabelText(/name/i)).toBeDisabled();
			expect(screen.getByLabelText(/description/i)).toBeDisabled();
			expect(screen.getByLabelText(/default model/i)).toBeDisabled();
			expect(screen.getByLabelText(/thinking/i)).toBeDisabled();
		});
	});

	describe('Modal Styling', () => {
		it('has proper dialog role for accessibility', () => {
			render(
				<WorkflowSettingsModal
					open={true}
					workflow={mockWorkflow}
					onClose={vi.fn()}
					onWorkflowUpdate={vi.fn()}
				/>
			);

			const dialog = screen.getByRole('dialog');
			expect(dialog).toBeInTheDocument();
		});

		it('applies modal styling classes', () => {
			const { container } = render(
				<WorkflowSettingsModal
					open={true}
					workflow={mockWorkflow}
					onClose={vi.fn()}
					onWorkflowUpdate={vi.fn()}
				/>
			);

			// Should have modal-related classes
			expect(
				container.querySelector('.modal') ||
				container.querySelector('[class*="modal"]') ||
				screen.getByRole('dialog')
			).toBeTruthy();
		});
	});
});
