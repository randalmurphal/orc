/**
 * TDD Tests for GateInspector Component
 *
 * Tests for TASK-774: Restore test coverage for components with deleted tests
 *
 * Success Criteria Coverage:
 * - SC-1: Displays gate type selector with all options (Auto, Human, AI, Skip)
 * - SC-2: Shows type-specific configuration sections based on selected gate type
 * - SC-3: Saves configuration changes via API
 * - SC-4: Shows read-only notice for built-in workflows
 * - SC-5: Displays gate status during execution
 * - SC-6: Handles failure handling configuration (retry, retry_from, fail, pause)
 * - SC-7: Supports advanced configuration (collapsible)
 * - SC-8: Updates local state optimistically before API save
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { Edge } from '@xyflow/react';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import {
	createMockWorkflowWithDetails,
	createMockWorkflow,
	createMockWorkflowPhase,
	createMockPhaseTemplate,
} from '@/test/factories';
import type { WorkflowWithDetails } from '@/gen/orc/v1/workflow_pb';
import type { GateEdgeData } from '../utils/layoutWorkflow';

// Mock the workflow client
const mockUpdatePhaseTemplate = vi.fn().mockResolvedValue({ template: {} });

vi.mock('@/lib/client', () => ({
	workflowClient: {
		updatePhaseTemplate: (...args: unknown[]) => mockUpdatePhaseTemplate(...args),
	},
}));

// Import after mocks are set up
import { GateInspector } from './GateInspector';

/** Create a mock Gate Edge */
function createMockGateEdge(overrides: Partial<GateEdgeData> = {}): Edge<GateEdgeData> {
	return {
		id: 'gate-1',
		source: 'phase-1',
		target: 'phase-2',
		type: 'gate',
		data: {
			gateType: GateType.AUTO,
			position: 'between',
			phaseId: 1,
			maxRetries: 3,
			failureAction: 'retry',
			...overrides,
		},
	};
}

describe('TASK-774: GateInspector Component', () => {
	const defaultWorkflowDetails: WorkflowWithDetails = createMockWorkflowWithDetails({
		workflow: createMockWorkflow({ id: 'test-workflow', name: 'Test', isBuiltin: false }),
		phases: [
			createMockWorkflowPhase({
				id: 1,
				phaseTemplateId: 'spec',
				template: createMockPhaseTemplate({ name: 'Spec' }),
			}),
			createMockWorkflowPhase({
				id: 2,
				phaseTemplateId: 'implement',
				template: createMockPhaseTemplate({ name: 'Implement' }),
			}),
		],
		variables: [],
	});

	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-1: Displays gate type selector with all options', () => {
		it('renders gate type dropdown with all options', async () => {
			const edge = createMockGateEdge();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			const select = screen.getByLabelText(/gate type/i);
			expect(select).toBeInTheDocument();

			// Check all options are present
			expect(screen.getByRole('option', { name: /auto/i })).toBeInTheDocument();
			expect(screen.getByRole('option', { name: /human/i })).toBeInTheDocument();
			expect(screen.getByRole('option', { name: /ai/i })).toBeInTheDocument();
			expect(screen.getByRole('option', { name: /skip/i })).toBeInTheDocument();
		});

		it('displays correct gate type header based on position', async () => {
			const entryGate = createMockGateEdge({ position: 'entry' });
			const exitGate = createMockGateEdge({ position: 'exit' });
			const betweenGate = createMockGateEdge({ position: 'between' });

			const { rerender } = render(
				<GateInspector
					edge={entryGate}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);
			expect(screen.getByText('Entry Gate')).toBeInTheDocument();

			rerender(
				<GateInspector
					edge={exitGate}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);
			expect(screen.getByText('Exit Gate')).toBeInTheDocument();

			rerender(
				<GateInspector
					edge={betweenGate}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);
			// Between gate shows target phase name
			expect(screen.getByText(/Gate → Spec/i)).toBeInTheDocument();
		});

		it('returns null when no edge is provided', () => {
			const { container } = render(
				<GateInspector
					edge={null}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			expect(container.firstChild).toBeNull();
		});

		it('returns null when edge has no data', () => {
			const edgeWithoutData = { id: 'gate-1', source: 'a', target: 'b' };

			const { container } = render(
				<GateInspector
					edge={edgeWithoutData as Edge<GateEdgeData>}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			expect(container.firstChild).toBeNull();
		});
	});

	describe('SC-2: Shows type-specific configuration sections', () => {
		it('shows Auto configuration when AUTO gate type is selected', async () => {
			const edge = createMockGateEdge({ gateType: GateType.AUTO });

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			expect(screen.getByText('Auto Configuration')).toBeInTheDocument();
			expect(screen.getByLabelText(/has output/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/no errors/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/completion marker/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/custom pattern/i)).toBeInTheDocument();
		});

		it('shows Human configuration when HUMAN gate type is selected', async () => {
			const edge = createMockGateEdge({ gateType: GateType.HUMAN });

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			expect(screen.getByText('Human Configuration')).toBeInTheDocument();
			expect(screen.getByLabelText(/review prompt/i)).toBeInTheDocument();
		});

		it('shows AI configuration when AI gate type is selected', async () => {
			const edge = createMockGateEdge({ gateType: GateType.AI });

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			expect(screen.getByText('AI Configuration')).toBeInTheDocument();
			expect(screen.getByLabelText(/reviewer agent/i)).toBeInTheDocument();
			expect(screen.getByText('Context Sources')).toBeInTheDocument();
		});

		it('does not show type-specific config for SKIP gate type', async () => {
			const edge = createMockGateEdge({ gateType: GateType.SKIP });

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			expect(screen.queryByText('Auto Configuration')).not.toBeInTheDocument();
			expect(screen.queryByText('Human Configuration')).not.toBeInTheDocument();
			expect(screen.queryByText('AI Configuration')).not.toBeInTheDocument();
		});
	});

	describe('SC-3: Saves configuration changes via API', () => {
		it('calls API when gate type is changed', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdge({ gateType: GateType.AUTO });

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			const select = screen.getByLabelText(/gate type/i);
			await user.selectOptions(select, GateType.HUMAN.toString());

			await waitFor(() => {
				expect(mockUpdatePhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						gateType: GateType.HUMAN,
					})
				);
			});
		});

		it('calls API when max retries is changed', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdge({ maxRetries: 3 });

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			const input = screen.getByLabelText(/max retries/i);
			await user.clear(input);
			await user.type(input, '5');

			await waitFor(() => {
				expect(mockUpdatePhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						maxIterations: 5,
					})
				);
			});
		});

		it('calls API when auto criteria checkbox is toggled', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdge({ gateType: GateType.AUTO });

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			const checkbox = screen.getByLabelText(/has output/i);
			await user.click(checkbox);

			await waitFor(() => {
				expect(mockUpdatePhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						autoCriteria: expect.objectContaining({
							hasOutput: true,
						}),
					})
				);
			});
		});
	});

	describe('SC-4: Shows read-only notice for built-in workflows', () => {
		it('displays read-only notice when readOnly is true', () => {
			const edge = createMockGateEdge();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={true}
				/>
			);

			expect(screen.getByText(/clone to customize/i)).toBeInTheDocument();
		});

		it('disables all inputs when readOnly is true', () => {
			const edge = createMockGateEdge();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={true}
				/>
			);

			expect(screen.getByLabelText(/gate type/i)).toBeDisabled();
			expect(screen.getByLabelText(/max retries/i)).toBeDisabled();
			expect(screen.getByLabelText(/on fail/i)).toBeDisabled();
		});
	});

	describe('SC-5: Displays gate status during execution', () => {
		it('shows gate status when provided', () => {
			const edge = createMockGateEdge({ gateStatus: 'passed' });

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			expect(screen.getByText('Passed')).toBeInTheDocument();
			expect(screen.getByText('Passed').closest('span')).toHaveClass('gate-inspector__status--passed');
		});

		it('shows blocked status with appropriate styling', () => {
			const edge = createMockGateEdge({ gateStatus: 'blocked' });

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			expect(screen.getByText('Blocked')).toBeInTheDocument();
			expect(screen.getByText('Blocked').closest('span')).toHaveClass('gate-inspector__status--blocked');
		});

		it('shows failed status with appropriate styling', () => {
			const edge = createMockGateEdge({ gateStatus: 'failed' });

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			expect(screen.getByText('Failed')).toBeInTheDocument();
			expect(screen.getByText('Failed').closest('span')).toHaveClass('gate-inspector__status--failed');
		});

		it('does not show status section when no status is set', () => {
			const edge = createMockGateEdge({ gateStatus: undefined });

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			expect(screen.queryByText(/Status/i)).not.toBeInTheDocument();
		});
	});

	describe('SC-6: Handles failure handling configuration', () => {
		it('displays failure action dropdown', () => {
			const edge = createMockGateEdge();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			const select = screen.getByLabelText(/on fail/i);
			expect(select).toBeInTheDocument();

			expect(screen.getByRole('option', { name: /retry$/i })).toBeInTheDocument();
			expect(screen.getByRole('option', { name: /retry from/i })).toBeInTheDocument();
			expect(screen.getByRole('option', { name: /fail$/i })).toBeInTheDocument();
			expect(screen.getByRole('option', { name: /pause/i })).toBeInTheDocument();
		});

		it('shows retry_from phase selector when retry_from is selected', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdge({ failureAction: 'retry' });

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			const failureSelect = screen.getByLabelText(/on fail/i);
			await user.selectOptions(failureSelect, 'retry_from');

			await waitFor(() => {
				expect(screen.getByLabelText(/retry from$/i)).toBeInTheDocument();
			});

			// Should show available phases in dropdown
			expect(screen.getByRole('option', { name: /Spec/i })).toBeInTheDocument();
			expect(screen.getByRole('option', { name: /Implement/i })).toBeInTheDocument();
		});

		it('saves failure action change via API', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdge();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			const select = screen.getByLabelText(/on fail/i);
			await user.selectOptions(select, 'fail');

			await waitFor(() => {
				expect(mockUpdatePhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						failureAction: 'fail',
					})
				);
			});
		});
	});

	describe('SC-7: Supports advanced configuration (collapsible)', () => {
		it('shows advanced section as collapsed by default', () => {
			const edge = createMockGateEdge();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			expect(screen.getByText(/advanced/i)).toBeInTheDocument();
			expect(screen.queryByLabelText(/before script/i)).not.toBeInTheDocument();
		});

		it('expands advanced section when clicked', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdge();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			await user.click(screen.getByText(/advanced/i));

			await waitFor(() => {
				expect(screen.getByLabelText(/before script/i)).toBeInTheDocument();
				expect(screen.getByLabelText(/after script/i)).toBeInTheDocument();
				expect(screen.getByLabelText(/store result as/i)).toBeInTheDocument();
			});
		});

		it('saves advanced config changes via API', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdge();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			// Expand advanced section
			await user.click(screen.getByText(/advanced/i));

			// Change before script
			const beforeScriptInput = await screen.findByLabelText(/before script/i);
			await user.type(beforeScriptInput, 'echo "before"');

			await waitFor(() => {
				expect(mockUpdatePhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						advancedConfig: expect.objectContaining({
							beforeScript: 'echo "before"',
						}),
					})
				);
			});
		});
	});

	describe('SC-8: Updates local state optimistically', () => {
		it('updates UI immediately when gate type changes', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdge({ gateType: GateType.AUTO });

			// Slow down the API to observe optimistic update
			mockUpdatePhaseTemplate.mockImplementation(
				() => new Promise((resolve) => setTimeout(() => resolve({ template: {} }), 500))
			);

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			// Initially shows Auto configuration
			expect(screen.getByText('Auto Configuration')).toBeInTheDocument();

			// Change to Human
			const select = screen.getByLabelText(/gate type/i);
			await user.selectOptions(select, GateType.HUMAN.toString());

			// Should immediately show Human configuration (before API completes)
			expect(screen.getByText('Human Configuration')).toBeInTheDocument();
			expect(screen.queryByText('Auto Configuration')).not.toBeInTheDocument();
		});

		it('reverts local state on API failure', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdge({ gateType: GateType.AUTO });

			mockUpdatePhaseTemplate.mockRejectedValue(new Error('API Error'));

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			// Change gate type
			const select = screen.getByLabelText(/gate type/i);
			await user.selectOptions(select, GateType.HUMAN.toString());

			// After API failure, should revert to original value
			await waitFor(() => {
				expect((select as HTMLSelectElement).value).toBe(GateType.AUTO.toString());
			});
		});

		it('disables input during loading', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdge();

			mockUpdatePhaseTemplate.mockImplementation(
				() => new Promise((resolve) => setTimeout(() => resolve({ template: {} }), 200))
			);

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			const select = screen.getByLabelText(/gate type/i);
			await user.selectOptions(select, GateType.HUMAN.toString());

			// Should be disabled while loading
			expect(select).toBeDisabled();

			// Should be enabled after save completes
			await waitFor(() => {
				expect(select).not.toBeDisabled();
			});
		});
	});

	describe('Edge Cases', () => {
		it('handles edge with missing phase ID gracefully', () => {
			const edge = createMockGateEdge({ phaseId: undefined });

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			// Should still render without crashing
			expect(screen.getByLabelText(/gate type/i)).toBeInTheDocument();
		});

		it('handles missing workflow phases gracefully', () => {
			const edge = createMockGateEdge();
			const emptyWorkflow = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'test', name: 'Test' }),
				phases: [],
			});

			render(
				<GateInspector
					edge={edge}
					workflowDetails={emptyWorkflow}
					readOnly={false}
				/>
			);

			// Should still render
			expect(screen.getByLabelText(/gate type/i)).toBeInTheDocument();
		});

		it('handles AI config context sources toggle', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdge({ gateType: GateType.AI });

			render(
				<GateInspector
					edge={edge}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
				/>
			);

			// Toggle phase_outputs checkbox
			const phaseOutputsCheckbox = screen.getByLabelText(/phase outputs/i);
			await user.click(phaseOutputsCheckbox);

			await waitFor(() => {
				expect(mockUpdatePhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						aiConfig: expect.objectContaining({
							contextSources: expect.arrayContaining(['phase_outputs']),
						}),
					})
				);
			});
		});
	});
});
