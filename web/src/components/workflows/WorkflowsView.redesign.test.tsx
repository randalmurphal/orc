/**
 * TDD Tests for TASK-750: Redesign Workflows page with Your Workflows / Built-in sections
 *
 * Tests the redesigned WorkflowsView with improved UX and section organization.
 *
 * Success Criteria Coverage:
 * - SC-1: "Your Workflows" section replaces "Custom Workflows" label
 * - SC-2: "Your Workflows" section appears first for workflow-first UX
 * - SC-3: "Built-in Workflows" section maintains clear system workflow organization
 * - SC-4: Section headers provide clear distinction between user and system workflows
 * - SC-5: All existing functionality (navigation, clone, create) continues to work
 * - SC-6: Visual hierarchy emphasizes workflow-first approach per initiative goals
 * - SC-7: Empty state messaging aligns with new "Your Workflows" terminology
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { WorkflowsView } from './WorkflowsView';
import {
	createMockWorkflow,
	createMockListWorkflowsResponse,
	createMockListPhaseTemplatesResponse,
} from '@/test/factories';
import { DefinitionSource } from '@/gen/orc/v1/workflow_pb';

// Mock the client module
vi.mock('@/lib/client', () => ({
	workflowClient: {
		listWorkflows: vi.fn(),
		listPhaseTemplates: vi.fn(),
	},
}));

// Mock the workflow store
vi.mock('@/stores/workflowStore', () => ({
	useWorkflowStore: () => ({
		workflows: [],
		phaseTemplates: [],
		setWorkflows: vi.fn(),
		setPhaseTemplates: vi.fn(),
		refreshKey: 0,
	}),
}));

import { workflowClient } from '@/lib/client';

/** Render WorkflowsView with router context */
function renderWorkflowsView() {
	return render(
		<MemoryRouter>
			<WorkflowsView />
		</MemoryRouter>
	);
}

describe('WorkflowsView Redesign - TASK-750', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-1: "Your Workflows" section replaces "Custom Workflows" label', () => {
		it('displays "Your Workflows" section header instead of "Custom Workflows"', async () => {
			const customWorkflows = [
				createMockWorkflow({ id: 'my-workflow', name: 'My Custom Flow', isBuiltin: false }),
			];

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse(customWorkflows),
				sources: { 'my-workflow': DefinitionSource.PROJECT },
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should show "Your Workflows" section header
			expect(screen.getByText('Your Workflows')).toBeInTheDocument();

			// Should NOT show old "Custom Workflows" label
			expect(screen.queryByText('Custom Workflows')).not.toBeInTheDocument();
		});

		it('displays appropriate subtitle for "Your Workflows" section', async () => {
			const customWorkflows = [
				createMockWorkflow({ id: 'my-workflow', name: 'My Custom Flow', isBuiltin: false }),
			];

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse(customWorkflows),
				sources: { 'my-workflow': DefinitionSource.PROJECT },
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should have clear subtitle explaining "Your Workflows"
			expect(screen.getByText('Your customized workflow configurations')).toBeInTheDocument();
		});
	});

	describe('SC-2: "Your Workflows" section appears first for workflow-first UX', () => {
		it('renders "Your Workflows" section before "Built-in Workflows" when user has custom workflows', async () => {
			const customWorkflows = [
				createMockWorkflow({ id: 'my-workflow', name: 'My Custom Flow', isBuiltin: false }),
			];
			const builtinWorkflows = [
				createMockWorkflow({ id: 'builtin-medium', name: 'Medium', isBuiltin: true }),
			];
			const allWorkflows = [...customWorkflows, ...builtinWorkflows];

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse(allWorkflows),
				sources: {
					'my-workflow': DefinitionSource.PROJECT,
					'builtin-medium': DefinitionSource.EMBEDDED
				},
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(screen.getByText('Your Workflows')).toBeInTheDocument();
				expect(screen.getByText('Built-in Workflows')).toBeInTheDocument();
			});

			// Get all section elements
			const sections = screen.getAllByRole('region').filter((section: HTMLElement) =>
				section.querySelector('.section-title')
			);

			// "Your Workflows" should appear before "Built-in Workflows"
			const yourWorkflowsSection = sections.find((section: HTMLElement) =>
				section.textContent?.includes('Your Workflows')
			);
			const builtinWorkflowsSection = sections.find((section: HTMLElement) =>
				section.textContent?.includes('Built-in Workflows')
			);

			expect(yourWorkflowsSection).toBeTruthy();
			expect(builtinWorkflowsSection).toBeTruthy();

			// Check DOM order - Your Workflows should come before Built-in
			const yourIndex = sections.indexOf(yourWorkflowsSection!);
			const builtinIndex = sections.indexOf(builtinWorkflowsSection!);
			expect(yourIndex).toBeLessThan(builtinIndex);
		});

		it('emphasizes workflow-first approach by showing custom workflows prominently', async () => {
			const customWorkflows = [
				createMockWorkflow({ id: 'my-workflow', name: 'My Custom Flow', isBuiltin: false }),
			];

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse(customWorkflows),
				sources: { 'my-workflow': DefinitionSource.PROJECT },
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(screen.getByText('My Custom Flow')).toBeInTheDocument();
			});

			// Your Workflows section should have proper semantic structure
			const yourWorkflowsSection = screen.getByText('Your Workflows').closest('section');
			expect(yourWorkflowsSection).toBeInTheDocument();
			expect(yourWorkflowsSection).toHaveClass('workflows-view-section');
		});
	});

	describe('SC-3: "Built-in Workflows" section maintains clear system workflow organization', () => {
		it('keeps "Built-in Workflows" section with clear system workflow labeling', async () => {
			const builtinWorkflows = [
				createMockWorkflow({ id: 'medium', name: 'Medium', isBuiltin: true }),
				createMockWorkflow({ id: 'large', name: 'Large', isBuiltin: true }),
			];

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse(builtinWorkflows),
				sources: {
					medium: DefinitionSource.EMBEDDED,
					large: DefinitionSource.EMBEDDED
				},
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should maintain "Built-in Workflows" section
			expect(screen.getByText('Built-in Workflows')).toBeInTheDocument();
			expect(screen.getByText('Default workflow templates (clone to customize)')).toBeInTheDocument();

			// Should show builtin workflow cards
			expect(screen.getByText('Medium')).toBeInTheDocument();
			expect(screen.getByText('Large')).toBeInTheDocument();
		});

		it('maintains clone functionality for built-in workflows', async () => {
			const user = userEvent.setup();
			const builtinWorkflows = [
				createMockWorkflow({ id: 'medium', name: 'Medium', isBuiltin: true }),
			];

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse(builtinWorkflows),
				sources: { medium: DefinitionSource.EMBEDDED },
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			const cloneEventHandler = vi.fn();
			window.addEventListener('orc:clone-workflow', cloneEventHandler);

			renderWorkflowsView();

			await waitFor(() => {
				expect(screen.getByText('Medium')).toBeInTheDocument();
			});

			// Should be able to clone built-in workflows
			const cloneButton = screen.getByRole('button', { name: /clone/i });
			await user.click(cloneButton);

			expect(cloneEventHandler).toHaveBeenCalled();

			window.removeEventListener('orc:clone-workflow', cloneEventHandler);
		});
	});

	describe('SC-4: Section headers provide clear distinction between user and system workflows', () => {
		it('provides clear visual distinction between user and system workflow sections', async () => {
			const customWorkflows = [
				createMockWorkflow({ id: 'my-workflow', name: 'My Custom Flow', isBuiltin: false }),
			];
			const builtinWorkflows = [
				createMockWorkflow({ id: 'medium', name: 'Medium', isBuiltin: true }),
			];
			const allWorkflows = [...customWorkflows, ...builtinWorkflows];

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse(allWorkflows),
				sources: {
					'my-workflow': DefinitionSource.PROJECT,
					medium: DefinitionSource.EMBEDDED
				},
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Both sections should have clear, distinct headers
			const yourWorkflowsHeader = screen.getByText('Your Workflows');
			const builtinWorkflowsHeader = screen.getByText('Built-in Workflows');

			expect(yourWorkflowsHeader).toBeInTheDocument();
			expect(builtinWorkflowsHeader).toBeInTheDocument();

			// Should have descriptive subtitles
			expect(screen.getByText('Your customized workflow configurations')).toBeInTheDocument();
			expect(screen.getByText('Default workflow templates (clone to customize)')).toBeInTheDocument();
		});

		it('maintains semantic section structure with proper headings hierarchy', async () => {
			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse([]),
				sources: {},
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Section headers should use proper heading levels
			const yourWorkflowsHeading = screen.getByRole('heading', { name: 'Your Workflows', level: 2 });
			const builtinWorkflowsHeading = screen.getByRole('heading', { name: 'Built-in Workflows', level: 2 });

			expect(yourWorkflowsHeading).toBeInTheDocument();
			expect(builtinWorkflowsHeading).toBeInTheDocument();
		});
	});

	describe('SC-5: All existing functionality continues to work', () => {
		it('preserves workflow card navigation functionality', async () => {
			const user = userEvent.setup();
			const customWorkflows = [
				createMockWorkflow({ id: 'my-workflow', name: 'My Custom Flow', isBuiltin: false }),
			];

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse(customWorkflows),
				sources: { 'my-workflow': DefinitionSource.PROJECT },
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(screen.getByText('My Custom Flow')).toBeInTheDocument();
			});

			// Should still be able to click workflow cards (navigation handled by parent)
			const workflowCard = screen.getByText('My Custom Flow').closest('[role="button"]');
			expect(workflowCard).toBeInTheDocument();

			// Clicking should work without errors
			await user.click(workflowCard!);
		});

		it('preserves "New Workflow" button functionality', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse([]),
				sources: {},
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			const addWorkflowEventHandler = vi.fn();
			window.addEventListener('orc:add-workflow', addWorkflowEventHandler);

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should preserve "New Workflow" button
			const newWorkflowButton = screen.getByRole('button', { name: /new workflow/i });
			expect(newWorkflowButton).toBeInTheDocument();

			await user.click(newWorkflowButton);
			expect(addWorkflowEventHandler).toHaveBeenCalled();

			window.removeEventListener('orc:add-workflow', addWorkflowEventHandler);
		});

		it('preserves phase templates section functionality', async () => {
			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse([]),
				sources: {},
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listPhaseTemplates).toHaveBeenCalled();
			});

			// Phase Templates section should still exist and work
			expect(screen.getByText('Phase Templates')).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /create from scratch/i })).toBeInTheDocument();
		});
	});

	describe('SC-6: Visual hierarchy emphasizes workflow-first approach', () => {
		it('emphasizes workflow sections over phase templates in layout hierarchy', async () => {
			const customWorkflows = [
				createMockWorkflow({ id: 'my-workflow', name: 'My Custom Flow', isBuiltin: false }),
			];

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse(customWorkflows),
				sources: { 'my-workflow': DefinitionSource.PROJECT },
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Main page title should emphasize workflows
			expect(screen.getByRole('heading', { name: 'Workflows', level: 1 })).toBeInTheDocument();

			// Workflow sections should appear before Phase Templates
			const allSections = screen.getAllByRole('region').filter((section: HTMLElement) =>
				section.querySelector('.section-title')
			);

			const workflowSections = allSections.filter((section: HTMLElement) =>
				section.textContent?.includes('Workflows')
			);
			const phaseSection = allSections.find((section: HTMLElement) =>
				section.textContent?.includes('Phase Templates')
			);

			if (phaseSection) {
				const phaseSectionIndex = allSections.indexOf(phaseSection);
				workflowSections.forEach((workflowSection: HTMLElement) => {
					const workflowIndex = allSections.indexOf(workflowSection);
					expect(workflowIndex).toBeLessThan(phaseSectionIndex);
				});
			}
		});

		it('maintains clear content hierarchy with proper section organization', async () => {
			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse([]),
				sources: {},
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should have clear content structure
			expect(screen.getByRole('banner')).toBeInTheDocument(); // header
			expect(screen.getByText('Workflows')).toBeInTheDocument(); // main title
			expect(screen.getByText('Composable task execution plans with configurable phases')).toBeInTheDocument(); // subtitle
		});
	});

	describe('SC-7: Empty state messaging aligns with new terminology', () => {
		it('shows updated empty state message for "Your Workflows" when no custom workflows exist', async () => {
			const builtinWorkflows = [
				createMockWorkflow({ id: 'medium', name: 'Medium', isBuiltin: true }),
			];

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse(builtinWorkflows),
				sources: { medium: DefinitionSource.EMBEDDED },
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should show appropriate empty state for "Your Workflows"
			expect(screen.getByText('No custom workflows')).toBeInTheDocument();
			expect(screen.getByText('Clone a built-in workflow or create a new one to customize your task execution.')).toBeInTheDocument();
		});

		it('displays empty state in proper section context', async () => {
			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse([]),
				sources: {},
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Empty state should appear within the correct section context
			const yourWorkflowsSection = screen.getByText('Your Workflows').closest('section');
			expect(yourWorkflowsSection).toBeInTheDocument();

			// Empty state should be in the right section
			const emptyState = screen.getByText('No custom workflows');
			expect(yourWorkflowsSection).toContainElement(emptyState);
		});
	});

	describe('Integration: Event system wiring', () => {
		it('maintains all event dispatching for workflow operations', async () => {
			const user = userEvent.setup();
			const customWorkflows = [
				createMockWorkflow({ id: 'my-workflow', name: 'My Custom Flow', isBuiltin: false }),
			];

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse(customWorkflows),
				sources: { 'my-workflow': DefinitionSource.PROJECT },
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			// Listen for all workflow-related events
			const selectHandler = vi.fn();
			const cloneHandler = vi.fn();
			const addHandler = vi.fn();

			window.addEventListener('orc:select-workflow', selectHandler);
			window.addEventListener('orc:clone-workflow', cloneHandler);
			window.addEventListener('orc:add-workflow', addHandler);

			renderWorkflowsView();

			await waitFor(() => {
				expect(screen.getByText('My Custom Flow')).toBeInTheDocument();
			});

			// Test "New Workflow" button event
			const newButton = screen.getByRole('button', { name: /new workflow/i });
			await user.click(newButton);
			expect(addHandler).toHaveBeenCalled();

			// Cleanup
			window.removeEventListener('orc:select-workflow', selectHandler);
			window.removeEventListener('orc:clone-workflow', cloneHandler);
			window.removeEventListener('orc:add-workflow', addHandler);
		});

		it('preserves workflow store integration', async () => {
			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse([]),
				sources: {},
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should call the API to load workflows
			expect(workflowClient.listWorkflows).toHaveBeenCalledWith({ includeBuiltin: true });
			expect(workflowClient.listPhaseTemplates).toHaveBeenCalledWith({ includeBuiltin: true });
		});
	});
});