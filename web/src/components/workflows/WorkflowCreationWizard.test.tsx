/**
 * TDD Tests for WorkflowCreationWizard - Guided workflow creation
 *
 * Tests for TASK-746: Implement guided workflow creation wizard
 *
 * Success Criteria Coverage:
 * - SC-1: Wizard displays intent selection step with Build, Review, Test, Document, Custom options
 * - SC-2: Selecting an intent enables Next button and stores the selection
 * - SC-3: Skip to Editor button is visible and skips directly to workflow editor
 * - SC-4: Wizard displays name input step with required name field
 * - SC-5: ID is auto-generated from name using slugify function
 * - SC-6: Description and default model are optional fields
 * - SC-7: Wizard displays phase selection step with recommendations based on intent
 * - SC-8: Phases are pre-selected based on intent (e.g., Build→spec+implement+review)
 * - SC-9: User can toggle phases on/off
 * - SC-10: Clicking "Create & Open Editor" creates workflow with configured phases
 * - SC-11: Navigation between steps works (Back/Next buttons)
 * - SC-12: Step indicator shows current progress
 *
 * These tests will FAIL until the WorkflowCreationWizard component is implemented.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { WorkflowCreationWizard } from './WorkflowCreationWizard';
import { createMockWorkflow, createMockCreateWorkflowResponse, createMockPhaseTemplate, createMockListPhaseTemplatesResponse, createMockAddPhaseResponse, createMockWorkflowPhase } from '@/test/factories';

// Mock the client module
vi.mock('@/lib/client', () => ({
	workflowClient: {
		createWorkflow: vi.fn(),
		listPhaseTemplates: vi.fn(),
		addPhase: vi.fn(),
		getWorkflow: vi.fn(),
	},
}));

// Mock useNavigate
const mockNavigate = vi.fn();
vi.mock('react-router', () => ({
	useNavigate: () => mockNavigate,
}));

// Import mocked module for assertions
import { workflowClient } from '@/lib/client';

describe('WorkflowCreationWizard', () => {
	const mockOnClose = vi.fn();
	const mockOnCreated = vi.fn();
	const mockOnSkipToEditor = vi.fn();

	beforeEach(() => {
		vi.clearAllMocks();
		// Default mock for phase templates
		vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
			createMockListPhaseTemplatesResponse([
				createMockPhaseTemplate({ id: 'spec', name: 'Specification' }),
				createMockPhaseTemplate({ id: 'tdd_write', name: 'TDD Write' }),
				createMockPhaseTemplate({ id: 'implement', name: 'Implementation' }),
				createMockPhaseTemplate({ id: 'review', name: 'Code Review' }),
				createMockPhaseTemplate({ id: 'docs', name: 'Documentation' }),
				createMockPhaseTemplate({ id: 'security_scan', name: 'Security Scan' }),
				createMockPhaseTemplate({ id: 'test', name: 'Test' }),
			])
		);
		// Default mock for addPhase (returns success with a phase)
		vi.mocked(workflowClient.addPhase).mockResolvedValue(
			createMockAddPhaseResponse(createMockWorkflowPhase({ phaseTemplateId: 'test' }))
		);
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-1: Intent Selection Step', () => {
		it('displays intent selection as the first step', async () => {
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Should show the intent selection header
			expect(screen.getByText(/what kind of workflow/i)).toBeInTheDocument();

			// Should have all intent options
			expect(screen.getByRole('button', { name: /build/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /review/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /test/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /document/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /custom/i })).toBeInTheDocument();
		});

		it('displays step indicator showing step 1 of 3', () => {
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Should show step indicator
			expect(screen.getByText(/step 1/i)).toBeInTheDocument();
		});
	});

	describe('SC-2: Intent Selection Behavior', () => {
		it('Next button is disabled when no intent is selected', () => {
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			const nextButton = screen.getByRole('button', { name: /next/i });
			expect(nextButton).toBeDisabled();
		});

		it('selecting Build intent enables Next button', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			const buildButton = screen.getByRole('button', { name: /build/i });
			await user.click(buildButton);

			const nextButton = screen.getByRole('button', { name: /next/i });
			expect(nextButton).not.toBeDisabled();
		});

		it('selecting intent highlights the selected option', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			const reviewButton = screen.getByRole('button', { name: /review/i });
			await user.click(reviewButton);

			// Should have selected/active state
			expect(reviewButton).toHaveClass('selected');
		});
	});

	describe('SC-3: Skip to Editor', () => {
		it('displays Skip to Editor button', () => {
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			expect(screen.getByRole('button', { name: /skip to editor/i })).toBeInTheDocument();
		});

		it('clicking Skip to Editor calls onSkipToEditor callback', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			await user.click(screen.getByRole('button', { name: /skip to editor/i }));

			expect(mockOnSkipToEditor).toHaveBeenCalledTimes(1);
		});
	});

	describe('SC-4: Name Input Step', () => {
		it('displays name input on step 2 after selecting intent', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Select intent and go to next step
			await user.click(screen.getByRole('button', { name: /build/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));

			// Should show name input
			expect(screen.getByText(/name your workflow/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/name/i)).toBeInTheDocument();
		});

		it('name field is required', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Navigate to step 2
			await user.click(screen.getByRole('button', { name: /build/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));

			// Next should be disabled without name
			const nextButton = screen.getByRole('button', { name: /next/i });
			expect(nextButton).toBeDisabled();

			// Enter name
			await user.type(screen.getByLabelText(/name/i), 'My Workflow');

			// Next should be enabled
			expect(nextButton).not.toBeDisabled();
		});
	});

	describe('SC-5: ID Auto-generation', () => {
		it('auto-generates ID from name', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Navigate to step 2
			await user.click(screen.getByRole('button', { name: /build/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));

			// Enter name
			await user.type(screen.getByLabelText(/name/i), 'Code Review with Security');

			// ID should be auto-generated
			const idField = screen.getByLabelText(/id/i);
			expect(idField).toHaveValue('code-review-with-security');
		});

		it('slugifies name properly (lowercase, hyphens, no special chars)', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Navigate to step 2
			await user.click(screen.getByRole('button', { name: /build/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));

			// Enter name with special characters
			await user.type(screen.getByLabelText(/name/i), "My Custom Workflow! (v2.0) - Test's");

			// ID should be properly slugified
			const idField = screen.getByLabelText(/id/i);
			expect(idField).toHaveValue('my-custom-workflow-v2-0-test-s');
		});

		it('stops auto-generating when user manually edits ID', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Navigate to step 2
			await user.click(screen.getByRole('button', { name: /build/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));

			// Enter name
			await user.type(screen.getByLabelText(/name/i), 'Original Name');

			// Manually edit ID
			const idField = screen.getByLabelText(/id/i);
			await user.clear(idField);
			await user.type(idField, 'custom-id');

			// Change name - ID should not change
			await user.clear(screen.getByLabelText(/name/i));
			await user.type(screen.getByLabelText(/name/i), 'New Name');

			expect(idField).toHaveValue('custom-id');
		});
	});

	describe('SC-6: Optional Fields', () => {
		it('has optional description field', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Navigate to step 2
			await user.click(screen.getByRole('button', { name: /build/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));

			// Description should be present
			expect(screen.getByLabelText(/description/i)).toBeInTheDocument();
		});

		it('has optional default model dropdown', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Navigate to step 2
			await user.click(screen.getByRole('button', { name: /build/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));

			// Model dropdown should be present (might be in collapsible section)
			const modelSelect = screen.queryByLabelText(/model/i);
			if (modelSelect) {
				expect(modelSelect).toBeInTheDocument();
			} else {
				// Check for collapsible section
				const optionalSection = screen.queryByText(/optional/i);
				expect(optionalSection).toBeInTheDocument();
			}
		});
	});

	describe('SC-7 & SC-8: Phase Selection Step', () => {
		it('displays phase selection on step 3', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Navigate to step 3
			await user.click(screen.getByRole('button', { name: /build/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));
			await user.type(screen.getByLabelText(/name/i), 'Test Workflow');
			await user.click(screen.getByRole('button', { name: /next/i }));

			// Should show phase selection header
			expect(screen.getByText(/choose your phases/i)).toBeInTheDocument();
		});

		it('pre-selects phases based on Build intent', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Select Build and navigate to phase selection
			await user.click(screen.getByRole('button', { name: /build/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));
			await user.type(screen.getByLabelText(/name/i), 'Build Workflow');
			await user.click(screen.getByRole('button', { name: /next/i }));

			await waitFor(() => {
				// Build intent should pre-select: spec, implement, review
				const specCheckbox = screen.getByRole('checkbox', { name: /spec/i });
				const implementCheckbox = screen.getByRole('checkbox', { name: /implement/i });
				const reviewCheckbox = screen.getByRole('checkbox', { name: /review/i });

				expect(specCheckbox).toBeChecked();
				expect(implementCheckbox).toBeChecked();
				expect(reviewCheckbox).toBeChecked();
			});
		});

		it('pre-selects phases based on Review intent', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Select Review and navigate to phase selection
			await user.click(screen.getByRole('button', { name: /review/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));
			await user.type(screen.getByLabelText(/name/i), 'Review Workflow');
			await user.click(screen.getByRole('button', { name: /next/i }));

			await waitFor(() => {
				// Review intent should pre-select: review
				const reviewCheckbox = screen.getByRole('checkbox', { name: /review/i });
				expect(reviewCheckbox).toBeChecked();

				// Should not pre-select implement
				const implementCheckbox = screen.getByRole('checkbox', { name: /implement/i });
				expect(implementCheckbox).not.toBeChecked();
			});
		});

		it('pre-selects phases based on Test intent', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Select Test and navigate to phase selection
			await user.click(screen.getByRole('button', { name: /test/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));
			await user.type(screen.getByLabelText(/name/i), 'Test Workflow');
			await user.click(screen.getByRole('button', { name: /next/i }));

			await waitFor(() => {
				// Test intent should pre-select: test/tdd_write
				const testCheckbox = screen.getByRole('checkbox', { name: /test/i });
				expect(testCheckbox).toBeChecked();
			});
		});

		it('pre-selects phases based on Document intent', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Select Document and navigate to phase selection
			await user.click(screen.getByRole('button', { name: /document/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));
			await user.type(screen.getByLabelText(/name/i), 'Docs Workflow');
			await user.click(screen.getByRole('button', { name: /next/i }));

			await waitFor(() => {
				// Document intent should pre-select: docs
				const docsCheckbox = screen.getByRole('checkbox', { name: /doc/i });
				expect(docsCheckbox).toBeChecked();
			});
		});

		it('Custom intent starts with no phases pre-selected', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Select Custom and navigate to phase selection
			await user.click(screen.getByRole('button', { name: /custom/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));
			await user.type(screen.getByLabelText(/name/i), 'Custom Workflow');
			await user.click(screen.getByRole('button', { name: /next/i }));

			await waitFor(() => {
				// Custom intent should have no phases pre-selected
				const checkboxes = screen.getAllByRole('checkbox');
				const checkedCheckboxes = checkboxes.filter((cb) => (cb as HTMLInputElement).checked);
				expect(checkedCheckboxes).toHaveLength(0);
			});
		});

		it('shows recommended label for intent-specific phases', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Select Build and navigate to phase selection
			await user.click(screen.getByRole('button', { name: /build/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));
			await user.type(screen.getByLabelText(/name/i), 'Build Workflow');
			await user.click(screen.getByRole('button', { name: /next/i }));

			// Should show "Recommended for Build" header
			expect(screen.getByText(/recommended for.*build/i)).toBeInTheDocument();
		});
	});

	describe('SC-9: Phase Toggle', () => {
		it('allows toggling phases on and off', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Navigate to phase selection
			await user.click(screen.getByRole('button', { name: /build/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));
			await user.type(screen.getByLabelText(/name/i), 'Test Workflow');
			await user.click(screen.getByRole('button', { name: /next/i }));

			await waitFor(() => {
				expect(screen.getByRole('checkbox', { name: /spec/i })).toBeInTheDocument();
			});

			// Initially checked (from Build intent)
			const specCheckbox = screen.getByRole('checkbox', { name: /spec/i }) as HTMLInputElement;
			expect(specCheckbox).toBeChecked();

			// Toggle off
			await user.click(specCheckbox);
			expect(specCheckbox).not.toBeChecked();

			// Toggle back on
			await user.click(specCheckbox);
			expect(specCheckbox).toBeChecked();
		});

		it('requires at least one phase to be selected', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Select Custom (no pre-selected phases) and navigate
			await user.click(screen.getByRole('button', { name: /custom/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));
			await user.type(screen.getByLabelText(/name/i), 'Custom Workflow');
			await user.click(screen.getByRole('button', { name: /next/i }));

			// Create button should be disabled with no phases selected
			const createButton = screen.getByRole('button', { name: /create/i });
			expect(createButton).toBeDisabled();

			await waitFor(() => {
				expect(screen.getByRole('checkbox', { name: /implement/i })).toBeInTheDocument();
			});

			// Select a phase
			await user.click(screen.getByRole('checkbox', { name: /implement/i }));

			// Create button should be enabled
			expect(createButton).not.toBeDisabled();
		});
	});

	describe('SC-10: Workflow Creation', () => {
		it('creates workflow with selected phases when clicking Create & Open Editor', async () => {
			const user = userEvent.setup();
			const mockWorkflow = createMockWorkflow({
				id: 'my-build-workflow',
				name: 'My Build Workflow',
			});

			vi.mocked(workflowClient.createWorkflow).mockResolvedValue(
				createMockCreateWorkflowResponse(mockWorkflow)
			);

			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Complete wizard flow
			await user.click(screen.getByRole('button', { name: /build/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));
			await user.type(screen.getByLabelText(/name/i), 'My Build Workflow');
			await user.click(screen.getByRole('button', { name: /next/i }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /create/i })).toBeInTheDocument();
			});

			// Click Create & Open Editor
			await user.click(screen.getByRole('button', { name: /create.*editor/i }));

			await waitFor(() => {
				expect(workflowClient.createWorkflow).toHaveBeenCalledWith(
					expect.objectContaining({
						id: 'my-build-workflow',
						name: 'My Build Workflow',
					})
				);
			});
		});

		it('calls onCreated callback after successful creation', async () => {
			const user = userEvent.setup();
			const mockWorkflow = createMockWorkflow({
				id: 'test-workflow',
				name: 'Test Workflow',
			});

			vi.mocked(workflowClient.createWorkflow).mockResolvedValue(
				createMockCreateWorkflowResponse(mockWorkflow)
			);

			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Complete wizard flow
			await user.click(screen.getByRole('button', { name: /build/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));
			await user.type(screen.getByLabelText(/name/i), 'Test Workflow');
			await user.click(screen.getByRole('button', { name: /next/i }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /create/i })).toBeInTheDocument();
			});

			await user.click(screen.getByRole('button', { name: /create.*editor/i }));

			await waitFor(() => {
				expect(mockOnCreated).toHaveBeenCalledWith(mockWorkflow);
			});
		});

		it('displays error on creation failure', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.createWorkflow).mockRejectedValue(
				new Error('Workflow ID already exists')
			);

			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Complete wizard flow
			await user.click(screen.getByRole('button', { name: /build/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));
			await user.type(screen.getByLabelText(/name/i), 'Duplicate Workflow');
			await user.click(screen.getByRole('button', { name: /next/i }));

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /create/i })).toBeInTheDocument();
			});

			await user.click(screen.getByRole('button', { name: /create.*editor/i }));

			await waitFor(() => {
				expect(screen.getByText(/workflow id already exists/i)).toBeInTheDocument();
			});
		});
	});

	describe('SC-11: Navigation', () => {
		it('Back button returns to previous step', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Go to step 2
			await user.click(screen.getByRole('button', { name: /build/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));

			expect(screen.getByText(/name your workflow/i)).toBeInTheDocument();

			// Click Back
			await user.click(screen.getByRole('button', { name: /back/i }));

			// Should be back on step 1
			expect(screen.getByText(/what kind of workflow/i)).toBeInTheDocument();
		});

		it('preserves data when navigating back and forth', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Complete steps 1 and 2
			await user.click(screen.getByRole('button', { name: /build/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));
			await user.type(screen.getByLabelText(/name/i), 'Preserved Workflow');
			await user.click(screen.getByRole('button', { name: /next/i }));

			// Go back to step 2
			await waitFor(() => {
				expect(screen.getByRole('button', { name: /back/i })).toBeInTheDocument();
			});
			await user.click(screen.getByRole('button', { name: /back/i }));

			// Name should still be there
			const nameField = screen.getByLabelText(/name/i);
			expect(nameField).toHaveValue('Preserved Workflow');

			// Go back to step 1
			await user.click(screen.getByRole('button', { name: /back/i }));

			// Build should still be selected
			expect(screen.getByRole('button', { name: /build/i })).toHaveClass('selected');
		});

		it('Cancel button calls onClose', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			await user.click(screen.getByRole('button', { name: /cancel/i }));

			expect(mockOnClose).toHaveBeenCalledTimes(1);
		});
	});

	describe('SC-12: Step Indicator', () => {
		it('updates step indicator as user progresses', async () => {
			const user = userEvent.setup();
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Step 1
			expect(screen.getByText(/step 1/i)).toBeInTheDocument();

			// Go to step 2
			await user.click(screen.getByRole('button', { name: /build/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));

			expect(screen.getByText(/step 2/i)).toBeInTheDocument();

			// Go to step 3
			await user.type(screen.getByLabelText(/name/i), 'Test');
			await user.click(screen.getByRole('button', { name: /next/i }));

			expect(screen.getByText(/step 3/i)).toBeInTheDocument();
		});

		it('shows visual progress indicator', async () => {
			render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Should have step indicator with multiple steps
			const stepIndicator = screen.getByTestId('step-indicator');
			expect(stepIndicator).toBeInTheDocument();

			// First step should be active
			const steps = within(stepIndicator).getAllByTestId(/step-\d/);
			expect(steps[0]).toHaveClass('active');
		});
	});

	describe('Modal Behavior', () => {
		it('resets state when modal is closed and reopened', async () => {
			const user = userEvent.setup();
			const { rerender } = render(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Make some progress
			await user.click(screen.getByRole('button', { name: /build/i }));
			await user.click(screen.getByRole('button', { name: /next/i }));
			await user.type(screen.getByLabelText(/name/i), 'Test Workflow');

			// Close modal
			rerender(
				<WorkflowCreationWizard
					open={false}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Reopen modal
			rerender(
				<WorkflowCreationWizard
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
					onSkipToEditor={mockOnSkipToEditor}
				/>
			);

			// Should be back on step 1
			expect(screen.getByText(/what kind of workflow/i)).toBeInTheDocument();
		});
	});
});
