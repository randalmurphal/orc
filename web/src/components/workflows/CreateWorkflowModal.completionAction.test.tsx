/**
 * TDD Tests for CreateWorkflowModal - completion_action field
 *
 * Tests for TASK-680: Add completion_action to workflow model
 *
 * Success Criteria Coverage:
 * - SC-7a: CreateWorkflowModal has completion_action select field
 * - SC-7b: Form submits completion_action value to API
 * - SC-7c: Default value is empty (inherit from config)
 * - SC-7d: Valid options are: "", "pr", "commit", "none"
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CreateWorkflowModal } from './CreateWorkflowModal';
import { createMockWorkflow, createMockCreateWorkflowResponse } from '@/test/factories';

// Mock the client module
vi.mock('@/lib/client', () => ({
	workflowClient: {
		createWorkflow: vi.fn(),
	},
}));

// Import mocked module for assertions
import { workflowClient } from '@/lib/client';

describe('CreateWorkflowModal - completion_action field', () => {
	// NOTE: Browser API mocks (ResizeObserver, IntersectionObserver, scrollIntoView) provided by global test-setup.ts
	const mockOnClose = vi.fn();
	const mockOnCreated = vi.fn();

	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	it('SC-7a: displays completion_action select field', async () => {
		render(
			<CreateWorkflowModal
				open={true}
				onClose={mockOnClose}
				onCreated={mockOnCreated}
			/>
		);

		// Should have a completion action field
		expect(screen.getByLabelText(/completion action/i)).toBeInTheDocument();
	});

	it('SC-7c: completion_action defaults to empty (inherit)', async () => {
		render(
			<CreateWorkflowModal
				open={true}
				onClose={mockOnClose}
				onCreated={mockOnCreated}
			/>
		);

		const select = screen.getByLabelText(/completion action/i);
		// Default should be empty/inherit
		expect(select).toHaveValue('');
	});

	it('SC-7d: completion_action options include pr, commit, none', async () => {
		render(
			<CreateWorkflowModal
				open={true}
				onClose={mockOnClose}
				onCreated={mockOnCreated}
			/>
		);

		const select = screen.getByLabelText(/completion action/i);

		// Check options exist
		const options = select.querySelectorAll('option');
		const optionValues = Array.from(options).map((opt) => opt.value);

		expect(optionValues).toContain('');
		expect(optionValues).toContain('pr');
		expect(optionValues).toContain('commit');
		expect(optionValues).toContain('none');
	});

	it('SC-7b: submits completion_action=pr to API', async () => {
		const user = userEvent.setup();
		const mockWorkflow = createMockWorkflow({
			id: 'test-workflow',
			name: 'Test Workflow',
		});

		vi.mocked(workflowClient.createWorkflow).mockResolvedValue(
			createMockCreateWorkflowResponse(mockWorkflow)
		);

		render(
			<CreateWorkflowModal
				open={true}
				onClose={mockOnClose}
				onCreated={mockOnCreated}
			/>
		);

		// Fill in required fields
		await user.type(screen.getByLabelText(/name/i), 'Test Workflow');

		// Select completion action
		const select = screen.getByLabelText(/completion action/i);
		await user.selectOptions(select, 'pr');

		// Submit form
		await user.click(screen.getByRole('button', { name: /create/i }));

		await waitFor(() => {
			expect(workflowClient.createWorkflow).toHaveBeenCalledWith(
				expect.objectContaining({
					completionAction: 'pr',
				})
			);
		});
	});

	it('SC-7b: submits completion_action=commit to API', async () => {
		const user = userEvent.setup();
		const mockWorkflow = createMockWorkflow({
			id: 'commit-workflow',
			name: 'Commit Workflow',
		});

		vi.mocked(workflowClient.createWorkflow).mockResolvedValue(
			createMockCreateWorkflowResponse(mockWorkflow)
		);

		render(
			<CreateWorkflowModal
				open={true}
				onClose={mockOnClose}
				onCreated={mockOnCreated}
			/>
		);

		await user.type(screen.getByLabelText(/name/i), 'Commit Workflow');

		const select = screen.getByLabelText(/completion action/i);
		await user.selectOptions(select, 'commit');

		await user.click(screen.getByRole('button', { name: /create/i }));

		await waitFor(() => {
			expect(workflowClient.createWorkflow).toHaveBeenCalledWith(
				expect.objectContaining({
					completionAction: 'commit',
				})
			);
		});
	});

	it('SC-7b: submits empty completion_action when not changed', async () => {
		const user = userEvent.setup();
		const mockWorkflow = createMockWorkflow({
			id: 'default-workflow',
			name: 'Default Workflow',
		});

		vi.mocked(workflowClient.createWorkflow).mockResolvedValue(
			createMockCreateWorkflowResponse(mockWorkflow)
		);

		render(
			<CreateWorkflowModal
				open={true}
				onClose={mockOnClose}
				onCreated={mockOnCreated}
			/>
		);

		await user.type(screen.getByLabelText(/name/i), 'Default Workflow');

		// Don't change completion action - keep default
		await user.click(screen.getByRole('button', { name: /create/i }));

		await waitFor(() => {
			expect(workflowClient.createWorkflow).toHaveBeenCalledWith(
				expect.objectContaining({
					completionAction: '',
				})
			);
		});
	});
});
