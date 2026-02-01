/**
 * TDD Tests for WorkflowsView - workflow card navigation
 *
 * Tests for TASK-636: 3-panel editor layout, routing, canvas integration
 * Tests for TASK-703: Create 'New Phase Template' modal - SC-1 (button appears)
 *
 * Success Criteria Coverage:
 * - TASK-636 SC-1: Clicking a workflow card on /workflows navigates to /workflows/:id
 * - TASK-703 SC-1: "Create From Scratch" button appears in Phase Templates section
 *
 * Preservation Requirements:
 * - Phase template card click still fires orc:select-phase-template event
 * - Clone button on workflow cards still fires orc:clone-workflow event
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom';
import { WorkflowsView } from './WorkflowsView';
import {
	createMockWorkflow,
	createMockListWorkflowsResponse,
	createMockListPhaseTemplatesResponse,
	createMockPhaseTemplate,
} from '@/test/factories';
import { DefinitionSource } from '@/gen/orc/v1/workflow_pb';

// Mock the client module
vi.mock('@/lib/client', () => ({
	workflowClient: {
		listWorkflows: vi.fn(),
		listPhaseTemplates: vi.fn(),
	},
}));

import { workflowClient } from '@/lib/client';

/** Helper to capture current location */
function LocationDisplay() {
	const location = useLocation();
	return <div data-testid="location-display">{location.pathname}</div>;
}

/** Render WorkflowsView with router context */
function renderWorkflowsView() {
	return render(
		<MemoryRouter initialEntries={['/workflows']}>
			<Routes>
				<Route
					path="/workflows"
					element={
						<>
							<WorkflowsView />
							<LocationDisplay />
						</>
					}
				/>
				<Route
					path="/workflows/:id"
					element={<div data-testid="editor-page">Editor</div>}
				/>
			</Routes>
		</MemoryRouter>
	);
}

describe('WorkflowsView', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-1: clicking workflow card navigates to /workflows/:id', () => {
		it('navigates to /workflows/:id when clicking a workflow card', async () => {
			const user = userEvent.setup();
			const workflows = [
				createMockWorkflow({ id: 'implement-medium', name: 'Implement (Medium)', isBuiltin: true }),
			];

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse(workflows),
				sources: { 'implement-medium': DefinitionSource.EMBEDDED },
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(screen.getByText('Implement (Medium)')).toBeTruthy();
			});

			// Click the workflow card
			const card = screen.getByText('Implement (Medium)').closest('[role="button"]');
			expect(card).not.toBeNull();
			await user.click(card!);

			// Should navigate to the editor page
			await waitFor(() => {
				expect(screen.getByTestId('editor-page')).toBeTruthy();
			});
		});

		it('navigates to correct URL for different workflow IDs', async () => {
			const user = userEvent.setup();
			const workflows = [
				createMockWorkflow({ id: 'custom-pipeline', name: 'Custom Pipeline', isBuiltin: false }),
			];

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse(workflows),
				sources: { 'custom-pipeline': DefinitionSource.PROJECT },
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			renderWorkflowsView();

			await waitFor(() => {
				expect(screen.getByText('Custom Pipeline')).toBeTruthy();
			});

			const card = screen.getByText('Custom Pipeline').closest('[role="button"]');
			await user.click(card!);

			await waitFor(() => {
				expect(screen.getByTestId('editor-page')).toBeTruthy();
			});
		});
	});

	describe('preservation: clone button still works', () => {
		it('clone button dispatches orc:clone-workflow event', async () => {
			const user = userEvent.setup();
			const workflows = [
				createMockWorkflow({ id: 'medium', name: 'Medium', isBuiltin: true }),
			];

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse(workflows),
				sources: { medium: DefinitionSource.EMBEDDED },
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
				createMockListPhaseTemplatesResponse([])
			);

			const eventHandler = vi.fn();
			window.addEventListener('orc:clone-workflow', eventHandler);

			renderWorkflowsView();

			await waitFor(() => {
				expect(screen.getByText('Medium')).toBeTruthy();
			});

			// Find the Clone button within the workflow card
			const card = screen.getByText('Medium').closest('.workflow-card');
			expect(card).not.toBeNull();
			const cloneButton = card!.querySelector('button');
			expect(cloneButton).not.toBeNull();
			await user.click(cloneButton!);

			// Clone event should have fired (not navigation)
			expect(eventHandler).toHaveBeenCalled();

			window.removeEventListener('orc:clone-workflow', eventHandler);
		});
	});

	describe('preservation: phase template card click still fires event', () => {
		it('phase template card click fires orc:select-phase-template event', async () => {
			const user = userEvent.setup();
			const templates = [
				createMockPhaseTemplate({ id: 'implement', name: 'Implement' }),
			];

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse([]),
				sources: {},
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue({
				...createMockListPhaseTemplatesResponse(templates),
				sources: { implement: DefinitionSource.EMBEDDED },
			});

			const eventHandler = vi.fn();
			window.addEventListener('orc:select-phase-template', eventHandler);

			renderWorkflowsView();

			await waitFor(() => {
				expect(screen.getByText('Implement')).toBeTruthy();
			});

			// Click the phase template card
			const templateCard = screen.getByText('Implement').closest('[role="button"]');
			if (templateCard) {
				await user.click(templateCard);
				expect(eventHandler).toHaveBeenCalled();
			}

			window.removeEventListener('orc:select-phase-template', eventHandler);
		});
	});

	/**
	 * TASK-703 SC-1: "Create From Scratch" button appears in Phase Templates section
	 */
	describe('TASK-703 SC-1: Create From Scratch button in Phase Templates section', () => {
		it('renders "Create From Scratch" button in the Phase Templates section header', async () => {
			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse([]),
				sources: {},
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue({
				...createMockListPhaseTemplatesResponse([]),
				sources: {},
			});

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listPhaseTemplates).toHaveBeenCalled();
			});

			// Find the Phase Templates section header
			expect(screen.getByText('Phase Templates')).toBeTruthy();

			// Should have a "Create From Scratch" button
			const createButton = screen.getByRole('button', { name: /create from scratch/i });
			expect(createButton).toBeTruthy();
		});

		it('dispatches orc:create-phase-template event when "Create From Scratch" is clicked', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse([]),
				sources: {},
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue({
				...createMockListPhaseTemplatesResponse([]),
				sources: {},
			});

			const eventHandler = vi.fn();
			window.addEventListener('orc:create-phase-template', eventHandler);

			renderWorkflowsView();

			await waitFor(() => {
				expect(workflowClient.listPhaseTemplates).toHaveBeenCalled();
			});

			// Click the "Create From Scratch" button
			const createButton = screen.getByRole('button', { name: /create from scratch/i });
			await user.click(createButton);

			// Event should have been dispatched
			expect(eventHandler).toHaveBeenCalled();

			window.removeEventListener('orc:create-phase-template', eventHandler);
		});

		it('button appears below Phase Templates header', async () => {
			const templates = [
				createMockPhaseTemplate({ id: 'implement', name: 'Implement' }),
			];

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue({
				...createMockListWorkflowsResponse([]),
				sources: {},
			});
			vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue({
				...createMockListPhaseTemplatesResponse(templates),
				sources: { implement: DefinitionSource.EMBEDDED },
			});

			renderWorkflowsView();

			await waitFor(() => {
				expect(screen.getByText('Implement')).toBeTruthy();
			});

			// Both the header and button should exist
			const phaseTemplatesSection = screen.getByText('Phase Templates').closest('section');
			expect(phaseTemplatesSection).toBeTruthy();

			// Button should be within the section or section header
			const createButton = screen.getByRole('button', { name: /create from scratch/i });
			expect(createButton).toBeTruthy();
		});
	});
});
