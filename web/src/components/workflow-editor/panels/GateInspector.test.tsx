/**
 * TDD Tests for GateInspector panel
 *
 * Tests for TASK-727: Implement gates as edges visual model
 *
 * Success Criteria Coverage:
 * - SC-5: GateInspector panel appears when edge is selected
 * - SC-6: GateInspector shows read-only mode for built-in workflows
 *
 * Failure Modes:
 * - GateInspector receives null edge → Render nothing
 *
 * These tests will FAIL until GateInspector is implemented.
 */

import { describe, it, expect, vi, beforeAll } from 'vitest';
import { render, screen } from '@testing-library/react';
import {
	createMockWorkflowWithDetails,
	createMockWorkflow,
	createMockWorkflowPhase,
	createMockPhaseTemplate,
} from '@/test/factories';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import type { Edge } from '@xyflow/react';

// Import the component that doesn't exist yet
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-expect-error - GateInspector doesn't exist yet, this is TDD
import { GateInspector } from './GateInspector';

// Mock IntersectionObserver
beforeAll(() => {
	class MockIntersectionObserver {
		observe() {}
		unobserve() {}
		disconnect() {}
	}
	Object.defineProperty(window, 'IntersectionObserver', {
		value: MockIntersectionObserver,
		writable: true,
	});
});

/** Gate edge data structure */
interface GateEdgeData {
	gateType: GateType;
	gateStatus?: 'pending' | 'passed' | 'blocked' | 'failed';
	phaseId?: number;
	position: 'entry' | 'exit' | 'between';
	maxRetries?: number;
	failureAction?: string;
}

/** Create a mock gate edge for testing */
function createMockGateEdge(data: Partial<GateEdgeData> = {}): Edge<GateEdgeData> {
	return {
		id: 'gate-edge-1',
		source: 'phase-1',
		target: 'phase-2',
		type: 'gate',
		data: {
			gateType: GateType.AUTO,
			position: 'between',
			...data,
		},
	};
}

describe('GateInspector', () => {
	describe('SC-5: GateInspector panel appears when edge is selected', () => {
		it('renders when a gate edge is provided', () => {
			const edge = createMockGateEdge();
			const workflowDetails = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
				],
			});

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			// Should render the inspector panel
			const inspector = document.querySelector('.gate-inspector');
			expect(inspector).not.toBeNull();
		});

		it('shows gate type label', () => {
			const edge = createMockGateEdge({ gateType: GateType.HUMAN });
			const workflowDetails = createMockWorkflowWithDetails();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			// Should show the gate type
			expect(screen.getByText(/Human/i)).toBeTruthy();
		});

		it('shows gate type indicator for AUTO gate', () => {
			const edge = createMockGateEdge({ gateType: GateType.AUTO });
			const workflowDetails = createMockWorkflowWithDetails();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			expect(screen.getByText(/Auto/i)).toBeTruthy();
		});

		it('shows gate type indicator for AI gate', () => {
			const edge = createMockGateEdge({ gateType: GateType.AI });
			const workflowDetails = createMockWorkflowWithDetails();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			expect(screen.getByText(/AI/i)).toBeTruthy();
		});

		it('shows gate type indicator for SKIP gate', () => {
			const edge = createMockGateEdge({ gateType: GateType.SKIP });
			const workflowDetails = createMockWorkflowWithDetails();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			expect(screen.getByText(/Skip/i)).toBeTruthy();
		});

		it('shows max retries when configured', () => {
			const edge = createMockGateEdge({
				gateType: GateType.AUTO,
				maxRetries: 5,
			});
			const workflowDetails = createMockWorkflowWithDetails();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			expect(screen.getByText(/5/)).toBeTruthy();
		});

		it('shows failure action when configured', () => {
			const edge = createMockGateEdge({
				gateType: GateType.AUTO,
				failureAction: 'retry',
			});
			const workflowDetails = createMockWorkflowWithDetails();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			// Should show the failure action somewhere
			const content = document.querySelector('.gate-inspector')?.textContent;
			expect(content?.toLowerCase()).toContain('retry');
		});

		it('shows "Entry Gate" label for entry position', () => {
			const edge = createMockGateEdge({ position: 'entry' });
			const workflowDetails = createMockWorkflowWithDetails();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			expect(screen.getByText(/Entry Gate/i)).toBeTruthy();
		});

		it('shows "Exit Gate" label for exit position', () => {
			const edge = createMockGateEdge({ position: 'exit' });
			const workflowDetails = createMockWorkflowWithDetails();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			expect(screen.getByText(/Exit Gate/i)).toBeTruthy();
		});

		it('shows target phase name for between gates', () => {
			const edge = createMockGateEdge({
				position: 'between',
				phaseId: 2,
			});
			const workflowDetails = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 2,
						phaseTemplateId: 'implement',
						sequence: 2,
						template: createMockPhaseTemplate({
							id: 'implement',
							name: 'Implement',
						}),
					}),
				],
			});

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			// Should show the target phase name
			expect(screen.getByText(/Implement/i)).toBeTruthy();
		});
	});

	describe('SC-6: GateInspector shows read-only mode for built-in workflows', () => {
		it('shows "Clone to customize" notice in read-only mode', () => {
			const edge = createMockGateEdge();
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ isBuiltin: true }),
			});

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={true}
				/>
			);

			expect(screen.getByText(/Clone to customize/i)).toBeTruthy();
		});

		it('disables form controls in read-only mode', () => {
			const edge = createMockGateEdge();
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ isBuiltin: true }),
			});

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={true}
				/>
			);

			// All selects should be disabled
			const selects = document.querySelectorAll('select');
			selects.forEach((select) => {
				expect(select.disabled).toBe(true);
			});

			// All inputs should be disabled
			const inputs = document.querySelectorAll('input');
			inputs.forEach((input) => {
				expect(input.disabled).toBe(true);
			});
		});

		it('does not show "Clone to customize" in edit mode', () => {
			const edge = createMockGateEdge();
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ isBuiltin: false }),
			});

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			expect(screen.queryByText(/Clone to customize/i)).toBeNull();
		});

		it('enables form controls in edit mode', () => {
			const edge = createMockGateEdge();
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ isBuiltin: false }),
			});

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			// At least some controls should be enabled
			const selects = document.querySelectorAll('select:not([disabled])');
			expect(selects.length).toBeGreaterThan(0);
		});
	});

	describe('Failure mode: GateInspector receives null edge', () => {
		it('renders nothing when edge is null', () => {
			const workflowDetails = createMockWorkflowWithDetails();

			const { container } = render(
				<GateInspector
					edge={null}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			// Should render nothing (empty or null return)
			expect(container.querySelector('.gate-inspector')).toBeNull();
		});

		it('renders nothing when edge is undefined', () => {
			const workflowDetails = createMockWorkflowWithDetails();

			const { container } = render(
				<GateInspector
					edge={undefined}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			expect(container.querySelector('.gate-inspector')).toBeNull();
		});
	});

	describe('Gate configuration display', () => {
		it('shows gate settings section', () => {
			const edge = createMockGateEdge({ gateType: GateType.HUMAN });
			const workflowDetails = createMockWorkflowWithDetails();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			// Should have a settings section
			const settingsSection = document.querySelector('.gate-inspector__settings');
			expect(settingsSection).not.toBeNull();
		});

		it('shows gate status during execution', () => {
			const edge = createMockGateEdge({
				gateType: GateType.HUMAN,
				gateStatus: 'blocked',
			});
			const workflowDetails = createMockWorkflowWithDetails();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			// Should show the status
			expect(screen.getByText(/Blocked/i)).toBeTruthy();
		});

		it('shows passed status with green indicator', () => {
			const edge = createMockGateEdge({
				gateType: GateType.AUTO,
				gateStatus: 'passed',
			});
			const workflowDetails = createMockWorkflowWithDetails();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			const statusIndicator = document.querySelector('.gate-inspector__status--passed');
			expect(statusIndicator).not.toBeNull();
		});

		it('shows failed status with red indicator', () => {
			const edge = createMockGateEdge({
				gateType: GateType.AUTO,
				gateStatus: 'failed',
			});
			const workflowDetails = createMockWorkflowWithDetails();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			const statusIndicator = document.querySelector('.gate-inspector__status--failed');
			expect(statusIndicator).not.toBeNull();
		});
	});

	describe('Header displays edge context', () => {
		it('shows header with gate title', () => {
			const edge = createMockGateEdge();
			const workflowDetails = createMockWorkflowWithDetails();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			const header = document.querySelector('.gate-inspector__header');
			expect(header).not.toBeNull();
		});

		it('header shows source → target relationship for between gates', () => {
			const edge = createMockGateEdge({
				position: 'between',
				phaseId: 2,
			});
			const workflowDetails = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						sequence: 1,
						template: createMockPhaseTemplate({ name: 'Spec' }),
					}),
					createMockWorkflowPhase({
						id: 2,
						phaseTemplateId: 'implement',
						sequence: 2,
						template: createMockPhaseTemplate({ name: 'Implement' }),
					}),
				],
			});

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			// Should indicate this is a gate before the implement phase
			const content = document.querySelector('.gate-inspector__header')?.textContent;
			expect(content).toContain('Implement');
		});
	});
});
