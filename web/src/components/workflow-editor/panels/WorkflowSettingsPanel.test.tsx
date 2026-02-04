/**
 * TDD Tests for WorkflowSettingsPanel
 *
 * Tests for TASK-751: Implement complete workflow settings dialog
 *
 * Success Criteria Coverage:
 * - SC-1: WorkflowSettingsPanel displays Identity section with name and description fields
 * - SC-2: WorkflowSettingsPanel displays Defaults section with model dropdown and thinking toggle
 * - SC-3: WorkflowSettingsPanel displays Completion section with action dropdown and target branch
 * - SC-4: Changes to any setting trigger API call on blur/change and update parent workflow state
 * - SC-5: Built-in workflows display all sections as read-only with "Clone to customize" message
 * - SC-6: Completion section conditionally shows target_branch only when completion_action is "pr" or "commit"
 * - SC-7: Name field shows validation error when empty on blur
 */

import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import { WorkflowSettingsPanel } from './WorkflowSettingsPanel';
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

const mockWorkflowWithNoneCompletion = {
	...mockWorkflow,
	completionAction: 'none',
} as unknown as Workflow;

const mockWorkflowWithCommitCompletion = {
	...mockWorkflow,
	completionAction: 'commit',
} as unknown as Workflow;

const mockWorkflowWithEmptyCompletion = {
	...mockWorkflow,
	completionAction: '',
} as unknown as Workflow;

describe('WorkflowSettingsPanel', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	// SC-1: Identity section with name and description fields
	describe('SC-1: Identity Section', () => {
		it('renders Identity section heading', () => {
			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			// Should have "Identity" section heading (not "Basic Information")
			expect(screen.getByRole('heading', { name: /identity/i })).toBeInTheDocument();
		});

		it('displays name input in Identity section', () => {
			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			const nameInput = screen.getByLabelText(/name/i);
			expect(nameInput).toBeInTheDocument();
			expect(nameInput).toHaveValue('Test Workflow');
		});

		it('displays description textarea in Identity section', () => {
			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			const descInput = screen.getByLabelText(/description/i);
			expect(descInput).toBeInTheDocument();
			expect(descInput.tagName.toLowerCase()).toBe('textarea');
			expect(descInput).toHaveValue('A test workflow');
		});
	});

	// SC-2: Defaults section with model dropdown and thinking toggle
	describe('SC-2: Defaults Section', () => {
		it('renders Defaults section heading', () => {
			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			// Should have "Defaults" section heading (not "Execution Defaults")
			expect(screen.getByRole('heading', { name: /^defaults$/i })).toBeInTheDocument();
		});

		it('displays model dropdown with correct options', () => {
			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			const modelSelect = screen.getByLabelText(/default model/i) as HTMLSelectElement;
			expect(modelSelect).toBeInTheDocument();
			expect(modelSelect.value).toBe('sonnet');

			// Check all options are present
			const options = modelSelect.querySelectorAll('option');
			const optionValues = Array.from(options).map(o => o.value);
			expect(optionValues).toContain('sonnet');
			expect(optionValues).toContain('opus');
			expect(optionValues).toContain('haiku');
		});

		it('displays thinking toggle checkbox', () => {
			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			const thinkingToggle = screen.getByLabelText(/thinking/i);
			expect(thinkingToggle).toBeInTheDocument();
			expect(thinkingToggle).toBeChecked();
		});
	});

	// SC-3: Completion section with action dropdown and target branch
	describe('SC-3: Completion Section', () => {
		it('renders Completion section heading', () => {
			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			expect(screen.getByRole('heading', { name: /completion/i })).toBeInTheDocument();
		});

		it('displays completion action dropdown', () => {
			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			const actionSelect = screen.getByLabelText(/on complete|completion action/i);
			expect(actionSelect).toBeInTheDocument();
		});

		it('displays target branch input when completion_action is pr', () => {
			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			const branchInput = screen.getByLabelText(/target branch/i);
			expect(branchInput).toBeInTheDocument();
			expect(branchInput).toHaveValue('main');
		});
	});

	// SC-4: Changes trigger API call on blur/change
	describe('SC-4: API Updates on Change', () => {
		it('calls updateWorkflow API when name is changed and blurred', async () => {
			const onUpdate = vi.fn();
			(workflowClient.updateWorkflow as ReturnType<typeof vi.fn>).mockResolvedValue({
				workflow: { ...mockWorkflow, name: 'Updated Name' },
			});

			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />);

			const nameInput = screen.getByLabelText(/name/i);
			fireEvent.change(nameInput, { target: { value: 'Updated Name' } });
			fireEvent.blur(nameInput);

			await waitFor(() => {
				expect(workflowClient.updateWorkflow).toHaveBeenCalledWith({
					id: 'test-workflow',
					name: 'Updated Name',
				});
			});

			expect(onUpdate).toHaveBeenCalled();
		});

		it('calls updateWorkflow API when model is changed', async () => {
			const onUpdate = vi.fn();
			(workflowClient.updateWorkflow as ReturnType<typeof vi.fn>).mockResolvedValue({
				workflow: { ...mockWorkflow, defaultModel: 'opus' },
			});

			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />);

			const modelSelect = screen.getByLabelText(/default model/i);
			fireEvent.change(modelSelect, { target: { value: 'opus' } });

			await waitFor(() => {
				expect(workflowClient.updateWorkflow).toHaveBeenCalledWith({
					id: 'test-workflow',
					defaultModel: 'opus',
				});
			});
		});

		it('calls updateWorkflow API when thinking toggle is changed', async () => {
			const onUpdate = vi.fn();
			(workflowClient.updateWorkflow as ReturnType<typeof vi.fn>).mockResolvedValue({
				workflow: { ...mockWorkflow, defaultThinking: false },
			});

			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />);

			const thinkingToggle = screen.getByLabelText(/thinking/i);
			fireEvent.click(thinkingToggle);

			await waitFor(() => {
				expect(workflowClient.updateWorkflow).toHaveBeenCalledWith({
					id: 'test-workflow',
					defaultThinking: false,
				});
			});
		});

		it('calls updateWorkflow API when completion action is changed', async () => {
			const onUpdate = vi.fn();
			(workflowClient.updateWorkflow as ReturnType<typeof vi.fn>).mockResolvedValue({
				workflow: { ...mockWorkflow, completionAction: 'commit' },
			});

			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />);

			const actionSelect = screen.getByLabelText(/on complete|completion action/i);
			fireEvent.change(actionSelect, { target: { value: 'commit' } });

			await waitFor(() => {
				expect(workflowClient.updateWorkflow).toHaveBeenCalledWith({
					id: 'test-workflow',
					completionAction: 'commit',
				});
			});
		});

		it('calls onWorkflowUpdate callback with updated workflow', async () => {
			const onUpdate = vi.fn();
			const updatedWorkflow = { ...mockWorkflow, name: 'Updated Name' };
			(workflowClient.updateWorkflow as ReturnType<typeof vi.fn>).mockResolvedValue({
				workflow: updatedWorkflow,
			});

			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />);

			const nameInput = screen.getByLabelText(/name/i);
			fireEvent.change(nameInput, { target: { value: 'Updated Name' } });
			fireEvent.blur(nameInput);

			await waitFor(() => {
				expect(onUpdate).toHaveBeenCalledWith(updatedWorkflow);
			});
		});

		it('displays error message when API call fails', async () => {
			const onUpdate = vi.fn();
			const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
			(workflowClient.updateWorkflow as ReturnType<typeof vi.fn>).mockRejectedValue(
				new Error('API Error')
			);

			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />);

			const nameInput = screen.getByLabelText(/name/i);
			fireEvent.change(nameInput, { target: { value: 'New Name' } });
			fireEvent.blur(nameInput);

			await waitFor(() => {
				expect(screen.getByText(/failed to update workflow/i)).toBeInTheDocument();
			});

			expect(onUpdate).not.toHaveBeenCalled();
			consoleError.mockRestore();
		});

		it('shows loading state during API calls (inputs disabled)', async () => {
			const onUpdate = vi.fn();
			let resolveUpdate: (value: unknown) => void;
			const updatePromise = new Promise(resolve => {
				resolveUpdate = resolve;
			});
			(workflowClient.updateWorkflow as ReturnType<typeof vi.fn>).mockReturnValue(updatePromise);

			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />);

			const nameInput = screen.getByLabelText(/name/i);
			fireEvent.change(nameInput, { target: { value: 'New Name' } });
			fireEvent.blur(nameInput);

			// Should show loading state
			expect(nameInput).toBeDisabled();

			// Resolve the promise
			resolveUpdate!({ workflow: { ...mockWorkflow, name: 'New Name' } });

			await waitFor(() => {
				expect(nameInput).not.toBeDisabled();
			});
		});
	});

	// SC-5: Built-in workflows display read-only with "Clone to customize"
	describe('SC-5: Built-in Workflow Read-Only', () => {
		it('shows "Clone to customize" message for builtin workflows', () => {
			render(<WorkflowSettingsPanel workflow={mockBuiltinWorkflow} onWorkflowUpdate={vi.fn()} />);

			expect(screen.getByText(/clone to customize/i)).toBeInTheDocument();
		});

		it('disables name input for builtin workflows', () => {
			render(<WorkflowSettingsPanel workflow={mockBuiltinWorkflow} onWorkflowUpdate={vi.fn()} />);

			expect(screen.getByLabelText(/name/i)).toBeDisabled();
		});

		it('disables description textarea for builtin workflows', () => {
			render(<WorkflowSettingsPanel workflow={mockBuiltinWorkflow} onWorkflowUpdate={vi.fn()} />);

			expect(screen.getByLabelText(/description/i)).toBeDisabled();
		});

		it('disables model select for builtin workflows', () => {
			render(<WorkflowSettingsPanel workflow={mockBuiltinWorkflow} onWorkflowUpdate={vi.fn()} />);

			expect(screen.getByLabelText(/default model/i)).toBeDisabled();
		});

		it('disables thinking toggle for builtin workflows', () => {
			render(<WorkflowSettingsPanel workflow={mockBuiltinWorkflow} onWorkflowUpdate={vi.fn()} />);

			expect(screen.getByLabelText(/thinking/i)).toBeDisabled();
		});

		it('disables completion action select for builtin workflows', () => {
			render(<WorkflowSettingsPanel workflow={mockBuiltinWorkflow} onWorkflowUpdate={vi.fn()} />);

			expect(screen.getByLabelText(/on complete|completion action/i)).toBeDisabled();
		});

		it('displays "Built-in" badge for builtin workflows', () => {
			render(<WorkflowSettingsPanel workflow={mockBuiltinWorkflow} onWorkflowUpdate={vi.fn()} />);

			expect(screen.getByText(/built-in/i)).toBeInTheDocument();
		});

		it('does not call API when builtin workflow fields are interacted with', async () => {
			render(<WorkflowSettingsPanel workflow={mockBuiltinWorkflow} onWorkflowUpdate={vi.fn()} />);

			// Even if user somehow triggers change (e.g., via devtools), API should not be called
			const nameInput = screen.getByLabelText(/name/i);
			fireEvent.change(nameInput, { target: { value: 'Hacked Name' } });
			fireEvent.blur(nameInput);

			// Wait a bit to ensure no API call was made
			await new Promise(resolve => setTimeout(resolve, 50));
			expect(workflowClient.updateWorkflow).not.toHaveBeenCalled();
		});
	});

	// SC-6: Conditional visibility of target_branch
	describe('SC-6: Conditional Target Branch Visibility', () => {
		it('shows target branch input when completion_action is "pr"', () => {
			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			expect(screen.getByLabelText(/target branch/i)).toBeVisible();
		});

		it('shows target branch input when completion_action is "commit"', () => {
			render(
				<WorkflowSettingsPanel workflow={mockWorkflowWithCommitCompletion} onWorkflowUpdate={vi.fn()} />
			);

			expect(screen.getByLabelText(/target branch/i)).toBeVisible();
		});

		it('hides target branch input when completion_action is "none"', () => {
			render(
				<WorkflowSettingsPanel workflow={mockWorkflowWithNoneCompletion} onWorkflowUpdate={vi.fn()} />
			);

			// Target branch field should not be in the document or should be hidden
			expect(screen.queryByLabelText(/target branch/i)).not.toBeInTheDocument();
		});

		it('shows target branch input when completion_action is empty (inherit)', () => {
			render(
				<WorkflowSettingsPanel workflow={mockWorkflowWithEmptyCompletion} onWorkflowUpdate={vi.fn()} />
			);

			// Empty means inherit from config, which could be "pr" - so show the field
			expect(screen.getByLabelText(/target branch/i)).toBeVisible();
		});

		it('hides target branch when user changes completion_action to "none"', async () => {
			const onUpdate = vi.fn();
			(workflowClient.updateWorkflow as ReturnType<typeof vi.fn>).mockResolvedValue({
				workflow: { ...mockWorkflow, completionAction: 'none' },
			});

			const { rerender } = render(
				<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />
			);

			// Initially visible
			expect(screen.getByLabelText(/target branch/i)).toBeVisible();

			// Change to "none"
			const actionSelect = screen.getByLabelText(/on complete|completion action/i);
			fireEvent.change(actionSelect, { target: { value: 'none' } });

			await waitFor(() => {
				expect(workflowClient.updateWorkflow).toHaveBeenCalled();
			});

			// Rerender with updated workflow
			rerender(
				<WorkflowSettingsPanel
					workflow={{ ...mockWorkflow, completionAction: 'none' } as unknown as Workflow}
					onWorkflowUpdate={onUpdate}
				/>
			);

			// Should be hidden now
			expect(screen.queryByLabelText(/target branch/i)).not.toBeInTheDocument();
		});
	});

	// SC-7: Name field validation
	describe('SC-7: Name Field Validation', () => {
		it('shows validation error when name is cleared and blurred', async () => {
			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			const nameInput = screen.getByLabelText(/name/i);
			fireEvent.change(nameInput, { target: { value: '' } });
			fireEvent.blur(nameInput);

			await waitFor(() => {
				expect(screen.getByText(/name is required/i)).toBeInTheDocument();
			});
		});

		it('does not call API when name validation fails', async () => {
			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			const nameInput = screen.getByLabelText(/name/i);
			fireEvent.change(nameInput, { target: { value: '' } });
			fireEvent.blur(nameInput);

			// Wait to ensure validation runs but API is not called
			await waitFor(() => {
				expect(screen.getByText(/name is required/i)).toBeInTheDocument();
			});

			expect(workflowClient.updateWorkflow).not.toHaveBeenCalled();
		});

		it('clears validation error when valid name is entered', async () => {
			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			const nameInput = screen.getByLabelText(/name/i);

			// First, trigger validation error
			fireEvent.change(nameInput, { target: { value: '' } });
			fireEvent.blur(nameInput);

			await waitFor(() => {
				expect(screen.getByText(/name is required/i)).toBeInTheDocument();
			});

			// Now enter valid name
			fireEvent.change(nameInput, { target: { value: 'Valid Name' } });

			// Error should be cleared
			expect(screen.queryByText(/name is required/i)).not.toBeInTheDocument();
		});

		it('shows inline error message near the name field', async () => {
			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			const nameInput = screen.getByLabelText(/name/i);
			fireEvent.change(nameInput, { target: { value: '' } });
			fireEvent.blur(nameInput);

			await waitFor(() => {
				// The error should be near the input field (within the same form-field container)
				const errorMessage = screen.getByText(/name is required/i);
				expect(errorMessage).toBeInTheDocument();
				// Optionally check it has error styling class
				expect(errorMessage.closest('.form-field') || errorMessage.className).toBeTruthy();
			});
		});
	});

	// Edge cases and error handling
	describe('Edge Cases', () => {
		it('handles workflow with empty description gracefully', () => {
			const workflowNoDesc = { ...mockWorkflow, description: '' } as unknown as Workflow;
			render(<WorkflowSettingsPanel workflow={workflowNoDesc} onWorkflowUpdate={vi.fn()} />);

			const descInput = screen.getByLabelText(/description/i);
			expect(descInput).toHaveValue('');
		});

		it('handles workflow with undefined defaultModel', () => {
			const workflowNoModel = { ...mockWorkflow, defaultModel: '' } as unknown as Workflow;
			render(<WorkflowSettingsPanel workflow={workflowNoModel} onWorkflowUpdate={vi.fn()} />);

			const modelSelect = screen.getByLabelText(/default model/i) as HTMLSelectElement;
			expect(modelSelect.value).toBe('');
		});

		it('handles workflow with undefined targetBranch', () => {
			const workflowNoBranch = { ...mockWorkflow, targetBranch: '' } as unknown as Workflow;
			render(<WorkflowSettingsPanel workflow={workflowNoBranch} onWorkflowUpdate={vi.fn()} />);

			const branchInput = screen.getByLabelText(/target branch/i);
			expect(branchInput).toHaveValue('');
		});

		it('applies consistent styling classes', () => {
			const { container } = render(
				<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />
			);

			expect(container.querySelector('.workflow-settings-panel')).toBeInTheDocument();
			expect(container.querySelector('.workflow-settings-section')).toBeInTheDocument();
		});
	});
});
