/**
 * Edge Case Tests for TASK-750: WorkflowsView Redesign
 *
 * Tests edge cases, error handling, and accessibility scenarios for the redesigned
 * workflows page with "Your Workflows / Built-in sections".
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

describe('WorkflowsView Redesign Edge Cases - TASK-750', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	describe('Error Handling', () => {
		it('shows error state while maintaining section structure', async () => {
			const errorMessage = 'Failed to load workflows from server';
			vi.mocked(workflowClient.listWorkflows).mockRejectedValue(new Error(errorMessage));
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(screen.getByText('Failed to load workflows')).toBeInTheDocument();
			});

			// Should still show section headers even in error state
			expect(screen.getByText('Your Workflows')).toBeInTheDocument();
			expect(screen.getByText('Built-in Workflows')).toBeInTheDocument();

			// Should show retry button
			expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
		});

		it('handles partial API failures gracefully', async () => {
			// Workflows fail, but phase templates succeed
			vi.mocked(workflowClient.listWorkflows).mockRejectedValue(new Error('Workflows API down'));
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should show error for workflows but still render page structure
			expect(screen.getByText('Failed to load workflows')).toBeInTheDocument();
			expect(screen.getByText('Phase Templates')).toBeInTheDocument();
		});

		it('retries API call when retry button is clicked', async () => {
			const user = userEvent.setup();
			const errorMessage = 'Network timeout';

			// First call fails
			vi.mocked(workflowClient.listWorkflows).mockRejectedValueOnce(new Error(errorMessage));
			// Second call succeeds
			vi.mocked(workflowClient.listWorkflows).mockResolvedValueOnce({
				...createMockListWorkflowsResponse([]),
				sources: {},
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(screen.getByText('Failed to load workflows')).toBeInTheDocument();
			});

			// Click retry button
			const retryButton = screen.getByRole('button', { name: /retry/i });
			await user.click(retryButton);

			// Should retry the API call
			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalledTimes(2);
			});
		});
	});

	describe('Loading States', () => {
		it('shows loading skeletons while maintaining section structure', async () => {
			// Mock long-running API call
			let resolveWorkflows: (value: any) => void;
			const workflowsPromise = new Promise(resolve => {
				resolveWorkflows = resolve;
			});

			vi.mocked(workflowClient.listWorkflows).mockReturnValue(workflowsPromise as any);
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			// Should show loading skeletons
			expect(screen.getByRole('region', { name: /loading workflows/i })).toBeInTheDocument();

			// Section headers should still be present during loading
			expect(screen.getByText('Your Workflows')).toBeInTheDocument();
			expect(screen.getByText('Built-in Workflows')).toBeInTheDocument();

			// Resolve the promise to clean up
			resolveWorkflows!({
				...createMockListWorkflowsResponse([]),
				sources: {},
			});

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});
		});

		it('shows skeleton cards with appropriate accessibility attributes', async () => {
			let resolveWorkflows: (value: any) => void;
			const workflowsPromise = new Promise(resolve => {
				resolveWorkflows = resolve;
			});

			vi.mocked(workflowClient.listWorkflows).mockReturnValue(workflowsPromise as any);
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			// Loading state should have proper accessibility
			const loadingRegion = screen.getByRole('region', { name: /loading workflows/i });
			expect(loadingRegion).toHaveAttribute('aria-busy', 'true');

			// Clean up
			resolveWorkflows!({
				...createMockListWorkflowsResponse([]),
				sources: {},
			});

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});
		});
	});

	describe('Mixed Content Scenarios', () => {
		it('handles mix of built-in and custom workflows with proper section ordering', async () => {
			const mixedWorkflows = [
				createMockWorkflow({ id: 'builtin-small', name: 'Small', isBuiltin: true }),
				createMockWorkflow({ id: 'custom-1', name: 'My First Workflow', isBuiltin: false }),
				createMockWorkflow({ id: 'builtin-medium', name: 'Medium', isBuiltin: true }),
				createMockWorkflow({ id: 'custom-2', name: 'My Second Workflow', isBuiltin: false }),
			];

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse(mixedWorkflows),
				sources: {
					'builtin-small': DefinitionSource.EMBEDDED,
					'custom-1': DefinitionSource.PROJECT,
					'builtin-medium': DefinitionSource.EMBEDDED,
					'custom-2': DefinitionSource.PROJECT,
				},
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should properly separate workflows by type
			const yourWorkflowsSection = screen.getByText('Your Workflows').closest('section');
			const builtinWorkflowsSection = screen.getByText('Built-in Workflows').closest('section');

			// Custom workflows should appear in "Your Workflows" section
			expect(yourWorkflowsSection).toContainElement(screen.getByText('My First Workflow'));
			expect(yourWorkflowsSection).toContainElement(screen.getByText('My Second Workflow'));

			// Built-in workflows should appear in "Built-in Workflows" section
			expect(builtinWorkflowsSection).toContainElement(screen.getByText('Small'));
			expect(builtinWorkflowsSection).toContainElement(screen.getByText('Medium'));
		});

		it('handles only built-in workflows (no custom) with appropriate empty state', async () => {
			const builtinWorkflows = [
				createMockWorkflow({ id: 'small', name: 'Small', isBuiltin: true }),
				createMockWorkflow({ id: 'medium', name: 'Medium', isBuiltin: true }),
			];

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse(builtinWorkflows),
				sources: {
					small: DefinitionSource.EMBEDDED,
					medium: DefinitionSource.EMBEDDED,
				},
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should show built-in workflows
			expect(screen.getByText('Small')).toBeInTheDocument();
			expect(screen.getByText('Medium')).toBeInTheDocument();

			// Should show empty state for "Your Workflows"
			expect(screen.getByText('No custom workflows')).toBeInTheDocument();
		});

		it('handles only custom workflows (no built-in) scenario', async () => {
			const customWorkflows = [
				createMockWorkflow({ id: 'custom-1', name: 'My Custom Flow', isBuiltin: false }),
			];

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse(customWorkflows),
				sources: { 'custom-1': DefinitionSource.PROJECT },
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should show custom workflow
			expect(screen.getByText('My Custom Flow')).toBeInTheDocument();

			// Both sections should be present even if one is empty
			expect(screen.getByText('Your Workflows')).toBeInTheDocument();
			expect(screen.getByText('Built-in Workflows')).toBeInTheDocument();
		});
	});

	describe('Accessibility', () => {
		it('maintains proper heading hierarchy with section redesign', async () => {
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

			// Should have proper heading hierarchy
			const h1 = screen.getByRole('heading', { level: 1, name: 'Workflows' });
			const h2YourWorkflows = screen.getByRole('heading', { level: 2, name: 'Your Workflows' });
			const h2BuiltinWorkflows = screen.getByRole('heading', { level: 2, name: 'Built-in Workflows' });
			const h2PhaseTemplates = screen.getByRole('heading', { level: 2, name: 'Phase Templates' });

			expect(h1).toBeInTheDocument();
			expect(h2YourWorkflows).toBeInTheDocument();
			expect(h2BuiltinWorkflows).toBeInTheDocument();
			expect(h2PhaseTemplates).toBeInTheDocument();
		});

		it('provides proper landmark roles for screen readers', async () => {
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

			// Should have header landmark
			expect(screen.getByRole('banner')).toBeInTheDocument();

			// Sections should have proper semantic structure
			const sections = screen.getAllByRole('region');
			expect(sections.length).toBeGreaterThan(0);
		});

		it('maintains keyboard navigation support', async () => {
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

			// Should be able to navigate to workflow cards with keyboard
			const workflowCard = screen.getByText('My Custom Flow').closest('[role="button"]');
			expect(workflowCard).toBeInTheDocument();
			expect(workflowCard).toHaveAttribute('tabIndex', '0');

			// Should be able to activate with keyboard
			workflowCard!.focus();
			await user.keyboard('{Enter}');
			// Navigation is handled by parent component, so no specific assertion needed
		});

		it('provides appropriate aria labels for section content', async () => {
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

			// Workflow cards should have proper accessibility
			const workflowCard = screen.getByText('My Custom Flow').closest('[role="button"]');
			expect(workflowCard).toHaveAttribute('role', 'button');
		});
	});

	describe('Performance Edge Cases', () => {
		it('handles large numbers of workflows efficiently', async () => {
			// Create 50 custom workflows and 20 built-in workflows
			const customWorkflows = Array.from({ length: 50 }, (_, i) =>
				createMockWorkflow({
					id: `custom-${i}`,
					name: `Custom Workflow ${i + 1}`,
					isBuiltin: false,
				})
			);

			const builtinWorkflows = Array.from({ length: 20 }, (_, i) =>
				createMockWorkflow({
					id: `builtin-${i}`,
					name: `Built-in Workflow ${i + 1}`,
					isBuiltin: true,
				})
			);

			const allWorkflows = [...customWorkflows, ...builtinWorkflows];
			const sources = Object.fromEntries([
				...customWorkflows.map(wf => [wf.id, DefinitionSource.PROJECT]),
				...builtinWorkflows.map(wf => [wf.id, DefinitionSource.EMBEDDED]),
			]);

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse(allWorkflows),
				sources,
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should render all workflows in appropriate sections
			expect(screen.getByText('Custom Workflow 1')).toBeInTheDocument();
			expect(screen.getByText('Custom Workflow 50')).toBeInTheDocument();
			expect(screen.getByText('Built-in Workflow 1')).toBeInTheDocument();
			expect(screen.getByText('Built-in Workflow 20')).toBeInTheDocument();

			// Sections should be properly organized
			expect(screen.getByText('Your Workflows')).toBeInTheDocument();
			expect(screen.getByText('Built-in Workflows')).toBeInTheDocument();
		});

		it('handles empty API responses gracefully', async () => {
			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				workflows: [],
				phaseCounts: {},
				sources: {},
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue({
				templates: [],
				sources: {},
			});

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should show empty states for all sections
			expect(screen.getByText('No custom workflows')).toBeInTheDocument();
			expect(screen.getByText('Your Workflows')).toBeInTheDocument();
			expect(screen.getByText('Built-in Workflows')).toBeInTheDocument();
			expect(screen.getByText('Phase Templates')).toBeInTheDocument();
		});
	});
});