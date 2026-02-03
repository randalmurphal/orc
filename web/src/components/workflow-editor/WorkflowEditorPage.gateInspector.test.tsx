/**
 * TDD Tests for WorkflowEditorPage - Gate Inspector Panel Switching
 *
 * Tests for TASK-727: Implement gates as edges visual model
 *
 * Success Criteria Coverage:
 * - SC-5: GateInspector panel appears when edge is selected
 *
 * Integration tests to verify that the WorkflowEditorPage correctly switches
 * between PhaseInspector and GateInspector based on selection state.
 *
 * These tests will FAIL until WorkflowEditorPage is updated to render GateInspector.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, act } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { WorkflowEditorPage } from './WorkflowEditorPage';
import { useWorkflowEditorStore } from '@/stores/workflowEditorStore';
import { useWorkflowStore } from '@/stores/workflowStore';
import {
	createMockWorkflow,
	createMockWorkflowWithDetails,
	createMockWorkflowPhase,
	createMockPhaseTemplate,
	createMockGetWorkflowResponse,
} from '@/test/factories';
import { GateType } from '@/gen/orc/v1/workflow_pb';

// Mock the client module
vi.mock('@/lib/client', () => ({
	workflowClient: {
		getWorkflow: vi.fn(),
		listPhaseTemplates: vi.fn().mockResolvedValue({ templates: [], sources: {} }),
		listWorkflowRuns: vi.fn().mockResolvedValue({ runs: [] }),
		saveWorkflowLayout: vi.fn().mockResolvedValue({}),
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

import { workflowClient } from '@/lib/client';

/** Render WorkflowEditorPage at /workflows/:id */
function renderEditorPage(workflowId: string = 'test-wf') {
	return render(
		<MemoryRouter initialEntries={[`/workflows/${workflowId}`]}>
			<Routes>
				<Route path="/workflows/:id" element={<WorkflowEditorPage />} />
				<Route path="/workflows" element={<div data-testid="workflows-list">Workflows List</div>} />
			</Routes>
		</MemoryRouter>
	);
}

/** Create a workflow with multiple phases and gates */
function createWorkflowWithGates(overrides: { isBuiltin?: boolean } = {}) {
	return createMockWorkflowWithDetails({
		workflow: createMockWorkflow({
			id: 'test-wf',
			name: 'Test Workflow',
			isBuiltin: overrides.isBuiltin ?? false,
		}),
		phases: [
			createMockWorkflowPhase({
				id: 1,
				phaseTemplateId: 'spec',
				sequence: 1,
				template: createMockPhaseTemplate({
					id: 'spec',
					name: 'Specification',
					gateType: GateType.AUTO,
				}),
			}),
			createMockWorkflowPhase({
				id: 2,
				phaseTemplateId: 'implement',
				sequence: 2,
				gateTypeOverride: GateType.HUMAN,
				template: createMockPhaseTemplate({
					id: 'implement',
					name: 'Implementation',
					gateType: GateType.AUTO,
				}),
			}),
			createMockWorkflowPhase({
				id: 3,
				phaseTemplateId: 'review',
				sequence: 3,
				template: createMockPhaseTemplate({
					id: 'review',
					name: 'Review',
					gateType: GateType.AI,
				}),
			}),
		],
	});
}

// NOTE: Browser API mocks (ResizeObserver, IntersectionObserver) provided by global test-setup.ts
describe('WorkflowEditorPage - Gate Inspector Panel', () => {
	beforeEach(() => {
		useWorkflowEditorStore.getState().reset();
		useWorkflowStore.getState().reset();
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-5: GateInspector appears when edge is selected', () => {
		it('shows GateInspector panel when selectedEdgeId is set in store', async () => {
			const details = createWorkflowWithGates();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(details)
			);

			renderEditorPage();

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeTruthy();
			});

			// Simulate selecting an edge via the store
			const edges = useWorkflowEditorStore.getState().edges;
			const gateEdge = edges.find((e) => e.type === 'gate');
			expect(gateEdge).toBeDefined();

			await act(async () => {
				useWorkflowEditorStore.getState().selectEdge(gateEdge!.id);
			});

			await waitFor(() => {
				// GateInspector should be visible
				const gateInspector = document.querySelector('.gate-inspector');
				expect(gateInspector).not.toBeNull();
			});
		});

		it('shows PhaseInspector when node is selected (not GateInspector)', async () => {
			const details = createWorkflowWithGates();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(details)
			);

			renderEditorPage();

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeTruthy();
			});

			// Select a node
			const nodes = useWorkflowEditorStore.getState().nodes;
			const phaseNode = nodes.find((n) => n.type === 'phase');
			expect(phaseNode).toBeDefined();

			await act(async () => {
				useWorkflowEditorStore.getState().selectNode(phaseNode!.id);
			});

			await waitFor(() => {
				// PhaseInspector should be visible (not GateInspector)
				const phaseInspector = document.querySelector('.phase-inspector');
				expect(phaseInspector).not.toBeNull();

				const gateInspector = document.querySelector('.gate-inspector');
				expect(gateInspector).toBeNull();
			});
		});

		it('hides both inspectors when nothing is selected', async () => {
			const details = createWorkflowWithGates();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(details)
			);

			renderEditorPage();

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeTruthy();
			});

			// No selection - both inspectors should be hidden
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBeNull();
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBeNull();

			// Body should have 2-column layout (no inspector)
			const body = document.querySelector('.workflow-editor-body');
			expect(body).not.toBeNull();
			expect(body!.classList.contains('workflow-editor-body--inspector-open')).toBe(false);
		});

		it('switches from PhaseInspector to GateInspector when selection changes', async () => {
			const details = createWorkflowWithGates();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(details)
			);

			renderEditorPage();

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeTruthy();
			});

			// First select a node
			const nodes = useWorkflowEditorStore.getState().nodes;
			const phaseNode = nodes.find((n) => n.type === 'phase');
			await act(async () => {
				useWorkflowEditorStore.getState().selectNode(phaseNode!.id);
			});

			await waitFor(() => {
				expect(document.querySelector('.phase-inspector')).not.toBeNull();
			});

			// Now select an edge (this should clear node selection and show gate inspector)
			const edges = useWorkflowEditorStore.getState().edges;
			const gateEdge = edges.find((e) => e.type === 'gate');
			await act(async () => {
				useWorkflowEditorStore.getState().selectEdge(gateEdge!.id);
			});

			await waitFor(() => {
				// GateInspector should be visible
				expect(document.querySelector('.gate-inspector')).not.toBeNull();
				// PhaseInspector should be hidden
				expect(document.querySelector('.phase-inspector')).toBeNull();
			});
		});

		it('switches from GateInspector to PhaseInspector when selection changes', async () => {
			const details = createWorkflowWithGates();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(details)
			);

			renderEditorPage();

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeTruthy();
			});

			// First select an edge
			const edges = useWorkflowEditorStore.getState().edges;
			const gateEdge = edges.find((e) => e.type === 'gate');
			await act(async () => {
				useWorkflowEditorStore.getState().selectEdge(gateEdge!.id);
			});

			await waitFor(() => {
				expect(document.querySelector('.gate-inspector')).not.toBeNull();
			});

			// Now select a node
			const nodes = useWorkflowEditorStore.getState().nodes;
			const phaseNode = nodes.find((n) => n.type === 'phase');
			await act(async () => {
				useWorkflowEditorStore.getState().selectNode(phaseNode!.id);
			});

			await waitFor(() => {
				// PhaseInspector should be visible
				expect(document.querySelector('.phase-inspector')).not.toBeNull();
				// GateInspector should be hidden
				expect(document.querySelector('.gate-inspector')).toBeNull();
			});
		});
	});

	describe('Inspector panel layout', () => {
		it('body has 3-column layout when gate is selected', async () => {
			const details = createWorkflowWithGates();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(details)
			);

			renderEditorPage();

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeTruthy();
			});

			// Select an edge
			const edges = useWorkflowEditorStore.getState().edges;
			const gateEdge = edges.find((e) => e.type === 'gate');
			await act(async () => {
				useWorkflowEditorStore.getState().selectEdge(gateEdge!.id);
			});

			await waitFor(() => {
				const body = document.querySelector('.workflow-editor-body');
				expect(body).not.toBeNull();
				expect(body!.classList.contains('workflow-editor-body--inspector-open')).toBe(true);
			});
		});
	});

	describe('GateInspector receives correct props', () => {
		it('passes readOnly=true for built-in workflows', async () => {
			const details = createWorkflowWithGates({ isBuiltin: true });
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(details)
			);

			renderEditorPage();

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeTruthy();
			});

			// Select an edge
			const edges = useWorkflowEditorStore.getState().edges;
			const gateEdge = edges.find((e) => e.type === 'gate');
			await act(async () => {
				useWorkflowEditorStore.getState().selectEdge(gateEdge!.id);
			});

			await waitFor(() => {
				// Should show "Clone to customize" notice in GateInspector
				const gateInspector = document.querySelector('.gate-inspector');
				expect(gateInspector).not.toBeNull();
				const readonlyNotice = gateInspector!.querySelector('.gate-inspector__readonly-notice');
				expect(readonlyNotice).not.toBeNull();
				expect(readonlyNotice!.textContent).toMatch(/Clone to customize/i);
			});
		});

		it('passes readOnly=false for custom workflows', async () => {
			const details = createWorkflowWithGates({ isBuiltin: false });
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(details)
			);

			renderEditorPage();

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeTruthy();
			});

			// Select an edge
			const edges = useWorkflowEditorStore.getState().edges;
			const gateEdge = edges.find((e) => e.type === 'gate');
			await act(async () => {
				useWorkflowEditorStore.getState().selectEdge(gateEdge!.id);
			});

			await waitFor(() => {
				const gateInspector = document.querySelector('.gate-inspector');
				expect(gateInspector).not.toBeNull();

				// Should NOT show "Clone to customize" notice
				expect(screen.queryByText(/Clone to customize/i)).toBeNull();
			});
		});

		it('passes the selected edge to GateInspector', async () => {
			const details = createWorkflowWithGates();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(details)
			);

			renderEditorPage();

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeTruthy();
			});

			// Select an edge that has HUMAN gate type
			const edges = useWorkflowEditorStore.getState().edges;
			const humanGateEdge = edges.find(
				(e) => e.type === 'gate' && e.data?.gateType === GateType.HUMAN
			);

			if (humanGateEdge) {
				await act(async () => {
					useWorkflowEditorStore.getState().selectEdge(humanGateEdge.id);
				});

				await waitFor(() => {
					// GateInspector should show "Human Configuration" section for human gate type
					expect(screen.getByText(/Human Configuration/i)).toBeTruthy();
				});
			}
		});
	});

	describe('Store cleanup on unmount', () => {
		it('resets selectedEdgeId when navigating away', async () => {
			const details = createWorkflowWithGates();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(details)
			);

			const { unmount } = renderEditorPage();

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeTruthy();
			});

			// Select an edge
			const edges = useWorkflowEditorStore.getState().edges;
			const gateEdge = edges.find((e) => e.type === 'gate');
			await act(async () => {
				useWorkflowEditorStore.getState().selectEdge(gateEdge!.id);
			});
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBe(gateEdge!.id);

			unmount();

			// Store should be reset after unmount
			expect(useWorkflowEditorStore.getState().selectedEdgeId).toBeNull();
		});
	});
});
