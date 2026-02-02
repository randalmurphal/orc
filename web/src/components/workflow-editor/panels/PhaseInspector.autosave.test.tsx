import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PhaseInspector } from './PhaseInspector';
import { workflowClient } from '@/lib/client';

vi.mock('@/lib/client', () => ({
	workflowClient: {
		updatePhase: vi.fn(),
	},
	configClient: {
		listAgents: vi.fn().mockResolvedValue({ agents: [] }),
		listHooks: vi.fn().mockResolvedValue({ hooks: [] }),
		listSkills: vi.fn().mockResolvedValue({ skills: [] }),
	},
	mcpClient: {
		listMCPServers: vi.fn().mockResolvedValue({ servers: [] }),
	},
}));

describe('PhaseInspector - Auto-save Behavior (TDD)', () => {
	const mockUser = userEvent.setup();

	const mockPhase = {
		id: 1,
		sequence: 1,
		phaseTemplateId: 'spec',
		template: {
			id: 'spec',
			name: 'Specification',
			isBuiltin: false,
			agentId: 'default-agent',
			model: 'claude-sonnet-4',
			maxIterations: 3,
			inputVariables: [],
			promptSource: 'template',
			promptContent: 'Write a spec',
			gateType: 0,
		},
	};

	const mockWorkflowDetails = {
		workflow: { id: 'test-workflow', name: 'Test Workflow' },
		phases: [mockPhase],
		variables: [],
	};

	beforeEach(() => {
		vi.clearAllMocks();
		(workflowClient.updatePhase as any).mockResolvedValue({});
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	describe('Auto-save Debounce Behavior (SC-23)', () => {
		it('debounces rapid changes to prevent excessive API calls', async () => {
			vi.useFakeTimers();

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const nameInput = screen.getByDisplayValue('Specification');

			// Make rapid consecutive changes
			await mockUser.type(nameInput, '1');
			vi.advanceTimersByTime(100);
			await mockUser.type(nameInput, '2');
			vi.advanceTimersByTime(100);
			await mockUser.type(nameInput, '3');

			// Should not have called API yet
			expect(workflowClient.updatePhase).not.toHaveBeenCalled();

			// Complete debounce period (500ms per spec)
			vi.advanceTimersByTime(500);

			// Should call API only once with final value
			await waitFor(() => {
				expect(workflowClient.updatePhase).toHaveBeenCalledTimes(1);
			});

			vi.useRealTimers();
		});

		it('triggers auto-save immediately on blur regardless of debounce timer', async () => {
			vi.useFakeTimers();

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const nameInput = screen.getByDisplayValue('Specification');
			await mockUser.type(nameInput, ' Updated');

			// Don't wait for debounce - blur should trigger immediate save
			fireEvent.blur(nameInput);

			await waitFor(() => {
				expect(workflowClient.updatePhase).toHaveBeenCalled();
			});

			vi.useRealTimers();
		});

		it('cancels pending debounced save when field is blurred', async () => {
			vi.useFakeTimers();

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const nameInput = screen.getByDisplayValue('Specification');
			await mockUser.type(nameInput, ' Updated');

			// Start debounce timer but don't complete it
			vi.advanceTimersByTime(200);
			expect(workflowClient.updatePhase).not.toHaveBeenCalled();

			// Blur should cancel debounce and save immediately
			fireEvent.blur(nameInput);

			// Complete original debounce period
			vi.advanceTimersByTime(300);

			// Should only be called once (from blur, not from debounce)
			await waitFor(() => {
				expect(workflowClient.updatePhase).toHaveBeenCalledTimes(1);
			});

			vi.useRealTimers();
		});
	});

	describe('Save Error Handling (SC-23)', () => {
		it('reverts field value when save fails', async () => {
			(workflowClient.updatePhase as any).mockRejectedValue(new Error('Save failed'));

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const nameInput = screen.getByDisplayValue('Specification');
			const originalValue = nameInput.value;

			await mockUser.clear(nameInput);
			await mockUser.type(nameInput, 'Failed Update');
			fireEvent.blur(nameInput);

			await waitFor(() => {
				// Field should revert to original value
				expect(nameInput).toHaveValue(originalValue);
			});
		});

		it('shows error message below field when save fails', async () => {
			(workflowClient.updatePhase as any).mockRejectedValue(new Error('Network error'));

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const nameInput = screen.getByDisplayValue('Specification');
			await mockUser.type(nameInput, ' Updated');
			fireEvent.blur(nameInput);

			await waitFor(() => {
				const errorMessage = screen.getByText(/network error/i);
				expect(errorMessage).toBeInTheDocument();

				// Error should be visually associated with the field
				expect(errorMessage).toHaveClass('field-error');
			});
		});

		it('clears error message when field is successfully saved', async () => {
			// First attempt fails
			(workflowClient.updatePhase as any).mockRejectedValueOnce(new Error('Save failed'));

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const nameInput = screen.getByDisplayValue('Specification');
			await mockUser.type(nameInput, ' Failed');
			fireEvent.blur(nameInput);

			await waitFor(() => {
				expect(screen.getByText(/save failed/i)).toBeInTheDocument();
			});

			// Second attempt succeeds
			(workflowClient.updatePhase as any).mockResolvedValue({});
			await mockUser.clear(nameInput);
			await mockUser.type(nameInput, 'Specification Success');
			fireEvent.blur(nameInput);

			await waitFor(() => {
				expect(screen.queryByText(/save failed/i)).not.toBeInTheDocument();
			});
		});

		it('shows red border on field when save fails', async () => {
			(workflowClient.updatePhase as any).mockRejectedValue(new Error('Save failed'));

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const nameInput = screen.getByDisplayValue('Specification');
			await mockUser.type(nameInput, ' Failed');
			fireEvent.blur(nameInput);

			await waitFor(() => {
				expect(nameInput).toHaveClass('field-error');
			});
		});
	});

	describe('Save State Indicators', () => {
		it('shows saving indicator while save is in progress', async () => {
			// Mock slow save
			let resolvePromise;
			const savePromise = new Promise((resolve) => {
				resolvePromise = resolve;
			});
			(workflowClient.updatePhase as any).mockReturnValue(savePromise);

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const nameInput = screen.getByDisplayValue('Specification');
			await mockUser.type(nameInput, ' Saving');
			fireEvent.blur(nameInput);

			// Should show saving indicator
			await waitFor(() => {
				expect(screen.getByText(/saving/i)).toBeInTheDocument();
			});

			// Complete the save
			resolvePromise!({});

			await waitFor(() => {
				expect(screen.queryByText(/saving/i)).not.toBeInTheDocument();
			});
		});

		it('disables field while save is in progress to prevent conflicts', async () => {
			let resolvePromise;
			const savePromise = new Promise((resolve) => {
				resolvePromise = resolve;
			});
			(workflowClient.updatePhase as any).mockReturnValue(savePromise);

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const nameInput = screen.getByDisplayValue('Specification');
			await mockUser.type(nameInput, ' Saving');
			fireEvent.blur(nameInput);

			// Field should be disabled during save
			await waitFor(() => {
				expect(nameInput).toBeDisabled();
			});

			// Complete the save
			resolvePromise!({});

			await waitFor(() => {
				expect(nameInput).not.toBeDisabled();
			});
		});
	});

	describe('Multiple Field Auto-save', () => {
		it('handles concurrent saves on different fields independently', async () => {
			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const nameInput = screen.getByDisplayValue('Specification');
			const iterationsInput = screen.getByLabelText(/max iterations/i);

			// Edit both fields simultaneously
			await mockUser.type(nameInput, ' Updated');
			await mockUser.clear(iterationsInput);
			await mockUser.type(iterationsInput, '5');

			// Blur both fields
			fireEvent.blur(nameInput);
			fireEvent.blur(iterationsInput);

			// Should make separate API calls for each field
			await waitFor(() => {
				expect(workflowClient.updatePhase).toHaveBeenCalledTimes(2);
			});
		});

		it('batches changes made within the debounce window', async () => {
			vi.useFakeTimers();

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const nameInput = screen.getByDisplayValue('Specification');
			const iterationsInput = screen.getByLabelText(/max iterations/i);

			// Make changes to multiple fields within debounce window
			await mockUser.type(nameInput, ' Updated');
			await mockUser.clear(iterationsInput);
			await mockUser.type(iterationsInput, '5');

			// Complete debounce
			vi.advanceTimersByTime(500);

			// Should make single batched API call
			await waitFor(() => {
				expect(workflowClient.updatePhase).toHaveBeenCalledTimes(1);
				expect(workflowClient.updatePhase).toHaveBeenCalledWith(
					expect.objectContaining({
						// Should contain updates for both fields
						maxIterationsOverride: 5,
						// Phase name updates would be included
					})
				);
			});

			vi.useRealTimers();
		});
	});

	describe('Auto-save with Validation', () => {
		it('does not save when field validation fails', async () => {
			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const iterationsInput = screen.getByLabelText(/max iterations/i);

			// Enter invalid value
			await mockUser.clear(iterationsInput);
			await mockUser.type(iterationsInput, '0'); // Invalid: below minimum
			fireEvent.blur(iterationsInput);

			// Should not call API due to validation failure
			await waitFor(() => {
				expect(screen.getByText(/must be between 1 and 20/i)).toBeInTheDocument();
				expect(workflowClient.updatePhase).not.toHaveBeenCalled();
			});
		});

		it('saves when validation passes after previous failure', async () => {
			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const iterationsInput = screen.getByLabelText(/max iterations/i);

			// First, enter invalid value
			await mockUser.clear(iterationsInput);
			await mockUser.type(iterationsInput, '0');
			fireEvent.blur(iterationsInput);

			await waitFor(() => {
				expect(screen.getByText(/must be between 1 and 20/i)).toBeInTheDocument();
			});

			// Then, enter valid value
			await mockUser.clear(iterationsInput);
			await mockUser.type(iterationsInput, '5');
			fireEvent.blur(iterationsInput);

			// Should now save successfully
			await waitFor(() => {
				expect(workflowClient.updatePhase).toHaveBeenCalled();
				expect(screen.queryByText(/must be between 1 and 20/i)).not.toBeInTheDocument();
			});
		});
	});
});