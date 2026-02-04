import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { vi } from 'vitest';
import { WorkflowSettingsPanel } from './WorkflowSettingsPanel';
import { workflowClient } from '@/lib/client';
import type { Workflow } from '@/gen/orc/v1/workflow_pb';

// Mock the workflow client
vi.mock('@/lib/client', () => ({
	workflowClient: {
		updateWorkflow: vi.fn(),
	},
}));

const mockWorkflow = {
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
} as unknown as Workflow;

const mockBuiltinWorkflow = {
	...mockWorkflow,
	id: 'builtin-workflow',
	name: 'Built-in Workflow',
	isBuiltin: true,
};

describe('WorkflowSettingsPanel', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	// SC-1: Workflow Settings Panel Visibility
	describe('Panel Visibility', () => {
		it('renders workflow settings section for custom workflows', () => {
			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			expect(screen.getByText('Workflow Settings')).toBeInTheDocument();
			expect(screen.getByLabelText('Name')).toBeInTheDocument();
			expect(screen.getByLabelText('Description')).toBeInTheDocument();
		});

		it('shows read-only message for builtin workflows', () => {
			render(<WorkflowSettingsPanel workflow={mockBuiltinWorkflow} onWorkflowUpdate={vi.fn()} />);

			expect(screen.getByText('Clone to customize')).toBeInTheDocument();
			expect(screen.getByLabelText('Name')).toBeDisabled();
			expect(screen.getByLabelText('Description')).toBeDisabled();
		});

		it('displays builtin badge for builtin workflows', () => {
			render(<WorkflowSettingsPanel workflow={mockBuiltinWorkflow} onWorkflowUpdate={vi.fn()} />);

			expect(screen.getByText('Built-in')).toBeInTheDocument();
		});
	});

	// SC-2: Basic Information Editing
	describe('Basic Information Editing', () => {
		it('allows editing workflow name', async () => {
			const onUpdate = vi.fn();
			(workflowClient.updateWorkflow as any).mockResolvedValue({ workflow: { ...mockWorkflow, name: 'Updated Name' } });

			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />);

			const nameInput = screen.getByLabelText('Name');
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

		it('allows editing workflow description', async () => {
			const onUpdate = vi.fn();
			(workflowClient.updateWorkflow as any).mockResolvedValue({ workflow: { ...mockWorkflow, description: 'Updated description' } });

			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />);

			const descInput = screen.getByLabelText('Description');
			fireEvent.change(descInput, { target: { value: 'Updated description' } });
			fireEvent.blur(descInput);

			await waitFor(() => {
				expect(workflowClient.updateWorkflow).toHaveBeenCalledWith({
					id: 'test-workflow',
					description: 'Updated description',
				});
			});
		});

		it('prevents editing builtin workflow basic info', () => {
			render(<WorkflowSettingsPanel workflow={mockBuiltinWorkflow} onWorkflowUpdate={vi.fn()} />);

			const nameInput = screen.getByLabelText('Name');
			const descInput = screen.getByLabelText('Description');

			expect(nameInput).toBeDisabled();
			expect(descInput).toBeDisabled();
		});
	});

	// SC-3: Execution Defaults Configuration
	describe('Execution Defaults Configuration', () => {
		it('renders default model dropdown with current value', () => {
			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			expect(screen.getByLabelText('Default Model')).toBeInTheDocument();
			expect(screen.getByDisplayValue('claude-sonnet-3-5')).toBeInTheDocument();
		});

		it('allows changing default model', async () => {
			const onUpdate = vi.fn();
			(workflowClient.updateWorkflow as any).mockResolvedValue({ workflow: { ...mockWorkflow, defaultModel: 'claude-opus-3' } });

			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />);

			const modelSelect = screen.getByLabelText('Default Model');
			fireEvent.change(modelSelect, { target: { value: 'claude-opus-3' } });

			await waitFor(() => {
				expect(workflowClient.updateWorkflow).toHaveBeenCalledWith({
					id: 'test-workflow',
					defaultModel: 'claude-opus-3',
				});
			});
		});

		it('renders default thinking toggle with current value', () => {
			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			const thinkingToggle = screen.getByLabelText('Enable Thinking by Default');
			expect(thinkingToggle).toBeChecked();
		});

		it('allows toggling default thinking', async () => {
			const onUpdate = vi.fn();
			(workflowClient.updateWorkflow as any).mockResolvedValue({ workflow: { ...mockWorkflow, defaultThinking: false } });

			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />);

			const thinkingToggle = screen.getByLabelText('Enable Thinking by Default');
			fireEvent.click(thinkingToggle);

			await waitFor(() => {
				expect(workflowClient.updateWorkflow).toHaveBeenCalledWith({
					id: 'test-workflow',
					defaultThinking: false,
				});
			});
		});

		it('renders default max iterations input', () => {
			const workflowWithIterations = { ...mockWorkflow, defaultMaxIterations: 20 };
			render(<WorkflowSettingsPanel workflow={workflowWithIterations} onWorkflowUpdate={vi.fn()} />);

			expect(screen.getByLabelText('Default Max Iterations')).toBeInTheDocument();
			expect(screen.getByDisplayValue('20')).toBeInTheDocument();
		});

		it('allows changing default max iterations', async () => {
			const onUpdate = vi.fn();
			(workflowClient.updateWorkflow as any).mockResolvedValue({ workflow: { ...mockWorkflow, defaultMaxIterations: 30 } });

			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />);

			const iterationsInput = screen.getByLabelText('Default Max Iterations');
			fireEvent.change(iterationsInput, { target: { value: '30' } });
			fireEvent.blur(iterationsInput);

			await waitFor(() => {
				expect(workflowClient.updateWorkflow).toHaveBeenCalledWith({
					id: 'test-workflow',
					defaultMaxIterations: 30,
				});
			});
		});
	});

	// SC-4: Completion Settings Configuration
	describe('Completion Settings Configuration', () => {
		it('renders completion action dropdown with current value', () => {
			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			expect(screen.getByLabelText('On Complete')).toBeInTheDocument();
			expect(screen.getByDisplayValue('Create PR')).toBeInTheDocument();
		});

		it('allows changing completion action', async () => {
			const onUpdate = vi.fn();
			(workflowClient.updateWorkflow as any).mockResolvedValue({ workflow: { ...mockWorkflow, completionAction: 'commit' } });

			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />);

			const actionSelect = screen.getByLabelText('On Complete');
			fireEvent.change(actionSelect, { target: { value: 'commit' } });

			await waitFor(() => {
				expect(workflowClient.updateWorkflow).toHaveBeenCalledWith({
					id: 'test-workflow',
					completionAction: 'commit',
				});
			});
		});

		it('renders target branch input with current value', () => {
			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			expect(screen.getByLabelText('Target Branch')).toBeInTheDocument();
			expect(screen.getByDisplayValue('main')).toBeInTheDocument();
		});

		it('allows changing target branch', async () => {
			const onUpdate = vi.fn();
			(workflowClient.updateWorkflow as any).mockResolvedValue({ workflow: { ...mockWorkflow, targetBranch: 'develop' } });

			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />);

			const branchInput = screen.getByLabelText('Target Branch');
			fireEvent.change(branchInput, { target: { value: 'develop' } });
			fireEvent.blur(branchInput);

			await waitFor(() => {
				expect(workflowClient.updateWorkflow).toHaveBeenCalledWith({
					id: 'test-workflow',
					targetBranch: 'develop',
				});
			});
		});
	});

	// SC-5: Settings Persistence - Error Handling
	describe('Settings Persistence', () => {
		it('handles API errors gracefully', async () => {
			const onUpdate = vi.fn();
			const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
			(workflowClient.updateWorkflow as any).mockRejectedValue(new Error('API Error'));

			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />);

			const nameInput = screen.getByLabelText('Name');
			fireEvent.change(nameInput, { target: { value: 'New Name' } });
			fireEvent.blur(nameInput);

			await waitFor(() => {
				expect(screen.getByText(/Failed to update workflow/)).toBeInTheDocument();
			});

			expect(onUpdate).not.toHaveBeenCalled();
			consoleError.mockRestore();
		});

		it('shows loading state during API calls', async () => {
			const onUpdate = vi.fn();
			// Create a promise that we can control
			let resolveUpdate: (value: any) => void;
			const updatePromise = new Promise(resolve => { resolveUpdate = resolve; });
			(workflowClient.updateWorkflow as any).mockReturnValue(updatePromise);

			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />);

			const nameInput = screen.getByLabelText('Name');
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

	// SC-6: Integration with existing components
	describe('Integration', () => {
		it('calls onWorkflowUpdate callback when workflow is updated', async () => {
			const onUpdate = vi.fn();
			const updatedWorkflow = { ...mockWorkflow, name: 'Updated Name' };
			(workflowClient.updateWorkflow as any).mockResolvedValue({ workflow: updatedWorkflow });

			render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={onUpdate} />);

			const nameInput = screen.getByLabelText('Name');
			fireEvent.change(nameInput, { target: { value: 'Updated Name' } });
			fireEvent.blur(nameInput);

			await waitFor(() => {
				expect(onUpdate).toHaveBeenCalledWith(updatedWorkflow);
			});
		});

		it('applies consistent styling classes', () => {
			const { container } = render(<WorkflowSettingsPanel workflow={mockWorkflow} onWorkflowUpdate={vi.fn()} />);

			// Check for expected CSS classes that would integrate with existing palette styling
			expect(container.querySelector('.workflow-settings-panel')).toBeInTheDocument();
			expect(container.querySelector('.workflow-settings-section')).toBeInTheDocument();
		});
	});
});