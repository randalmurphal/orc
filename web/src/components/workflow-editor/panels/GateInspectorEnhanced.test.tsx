/**
 * TDD Tests for Enhanced GateInspector panel
 *
 * Tests for TASK-728: Add gate inspector panel
 *
 * Success Criteria Coverage:
 * - SC-1: Gate type selector (Auto/Human/AI/None) functional in edit mode
 * - SC-2: Auto gate configuration with criteria checkboxes and custom pattern
 * - SC-3: Human gate configuration with review prompt textarea
 * - SC-4: AI gate configuration with agent dropdown and context sources
 * - SC-5: Failure handling section with On Fail dropdown and Retry From
 * - SC-6: Max Retries number input functional and editable
 * - SC-7: Advanced collapsible section with scripts and result variable
 * - SC-8: Read-only mode for built-in workflows
 * - SC-9: API integration to save configuration changes
 * - SC-10: Graceful null edge handling (already covered in existing tests)
 *
 * These tests will FAIL until enhanced GateInspector is implemented.
 */

import { describe, it, expect, beforeAll, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {
	createMockWorkflowWithDetails,
	createMockWorkflow,
	createMockWorkflowPhase,
	createMockPhaseTemplate,
} from '@/test/factories';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import type { Edge } from '@xyflow/react';

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

// Mock workflow API client
const mockWorkflowClient = {
	updatePhaseTemplate: vi.fn(),
	updateWorkflow: vi.fn(),
};

vi.mock('@/lib/api/workflowClient', () => ({
	workflowClient: mockWorkflowClient,
}));

// Enhanced gate edge data structure for configuration
interface GateConfigData extends Record<string, unknown> {
	// Basic gate info
	gateType: GateType;
	gateStatus?: 'pending' | 'passed' | 'blocked' | 'failed';
	phaseId?: number;
	position: 'entry' | 'exit' | 'between';
	maxRetries?: number;
	failureAction?: 'retry' | 'retry_from' | 'fail' | 'pause';

	// Auto gate configuration
	autoCriteria?: {
		hasOutput?: boolean;
		noErrors?: boolean;
		completionMarker?: boolean;
		customPattern?: string;
	};

	// Human gate configuration
	humanConfig?: {
		reviewPrompt?: string;
	};

	// AI gate configuration
	aiConfig?: {
		reviewerAgentId?: string;
		contextSources?: ('phase_outputs' | 'task_details' | 'vars')[];
	};

	// Failure handling
	retryFromPhaseId?: number;

	// Advanced configuration
	advancedConfig?: {
		beforeScript?: string;
		afterScript?: string;
		storeResultAs?: string;
	};
}

/** Create a mock gate edge with enhanced configuration data */
function createMockGateEdgeWithConfig(data: Partial<GateConfigData> = {}): Edge<GateConfigData> {
	return {
		id: 'gate-edge-1',
		source: 'phase-1',
		target: 'phase-2',
		type: 'gate',
		data: {
			gateType: GateType.AUTO,
			position: 'between',
			maxRetries: 3,
			failureAction: 'retry',
			...data,
		},
	};
}

describe('Enhanced GateInspector', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	describe('SC-1: Gate type selector functional in edit mode', () => {
		it('shows gate type dropdown in edit mode', () => {
			const edge = createMockGateEdgeWithConfig({ gateType: GateType.AUTO });
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

			const gateTypeSelect = screen.getByRole('combobox', { name: /gate type/i });
			expect(gateTypeSelect).toBeTruthy();
			expect(gateTypeSelect.disabled).toBe(false);
		});

		it('allows changing gate type from Auto to Human', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdgeWithConfig({ gateType: GateType.AUTO });
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

			const gateTypeSelect = screen.getByRole('combobox', { name: /gate type/i });
			await user.selectOptions(gateTypeSelect, GateType.HUMAN.toString());

			expect(gateTypeSelect.value).toBe(GateType.HUMAN.toString());
		});

		it('calls API to save gate type changes', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdgeWithConfig({ gateType: GateType.AUTO, phaseId: 2 });
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'workflow-1', isBuiltin: false }),
				phases: [
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement' }),
				],
			});

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			const gateTypeSelect = screen.getByRole('combobox', { name: /gate type/i });
			await user.selectOptions(gateTypeSelect, GateType.HUMAN.toString());

			await waitFor(() => {
				expect(mockWorkflowClient.updatePhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						phaseId: 2,
						gateType: GateType.HUMAN,
					})
				);
			});
		});
	});

	describe('SC-2: Auto gate configuration with criteria checkboxes', () => {
		it('shows auto criteria section when gate type is Auto', () => {
			const edge = createMockGateEdgeWithConfig({
				gateType: GateType.AUTO,
				autoCriteria: {
					hasOutput: true,
					noErrors: true,
					completionMarker: false,
				}
			});
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

			expect(screen.getByText(/Auto Configuration/i)).toBeTruthy();
			expect(screen.getByRole('checkbox', { name: /has output/i })).toBeTruthy();
			expect(screen.getByRole('checkbox', { name: /no errors/i })).toBeTruthy();
			expect(screen.getByRole('checkbox', { name: /completion marker/i })).toBeTruthy();
		});

		it('shows custom pattern input for auto gates', () => {
			const edge = createMockGateEdgeWithConfig({
				gateType: GateType.AUTO,
				autoCriteria: {
					customPattern: 'SUCCESS: .+',
				}
			});
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

			const customPatternInput = screen.getByRole('textbox', { name: /custom pattern/i });
			expect(customPatternInput).toBeTruthy();
			expect(customPatternInput.value).toBe('SUCCESS: .+');
		});

		it('hides auto configuration when gate type is not Auto', () => {
			const edge = createMockGateEdgeWithConfig({ gateType: GateType.HUMAN });
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

			expect(screen.queryByText(/Auto Configuration/i)).toBeNull();
			expect(screen.queryByRole('checkbox', { name: /has output/i })).toBeNull();
		});

		it('allows toggling criteria checkboxes', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdgeWithConfig({
				gateType: GateType.AUTO,
				autoCriteria: { hasOutput: false }
			});
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

			const hasOutputCheckbox = screen.getByRole('checkbox', { name: /has output/i });
			expect(hasOutputCheckbox.checked).toBe(false);

			await user.click(hasOutputCheckbox);
			expect(hasOutputCheckbox.checked).toBe(true);
		});
	});

	describe('SC-3: Human gate configuration with review prompt', () => {
		it('shows review prompt textarea when gate type is Human', () => {
			const edge = createMockGateEdgeWithConfig({
				gateType: GateType.HUMAN,
				humanConfig: {
					reviewPrompt: 'Please review the implementation for correctness.',
				}
			});
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

			expect(screen.getByText(/Human Configuration/i)).toBeTruthy();
			const reviewPromptTextarea = screen.getByRole('textbox', { name: /review prompt/i });
			expect(reviewPromptTextarea).toBeTruthy();
			expect(reviewPromptTextarea.value).toBe('Please review the implementation for correctness.');
		});

		it('hides human configuration when gate type is not Human', () => {
			const edge = createMockGateEdgeWithConfig({ gateType: GateType.AUTO });
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

			expect(screen.queryByText(/Human Configuration/i)).toBeNull();
			expect(screen.queryByRole('textbox', { name: /review prompt/i })).toBeNull();
		});

		it('allows editing review prompt', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdgeWithConfig({
				gateType: GateType.HUMAN,
				humanConfig: { reviewPrompt: '' }
			});
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

			const reviewPromptTextarea = screen.getByRole('textbox', { name: /review prompt/i });
			await user.type(reviewPromptTextarea, 'Check code quality and security.');

			expect(reviewPromptTextarea.value).toBe('Check code quality and security.');
		});
	});

	describe('SC-4: AI gate configuration with agent dropdown and context sources', () => {
		it('shows AI configuration section when gate type is AI', () => {
			const edge = createMockGateEdgeWithConfig({
				gateType: GateType.AI,
				aiConfig: {
					reviewerAgentId: 'security-reviewer',
					contextSources: ['phase_outputs', 'task_details'],
				}
			});
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

			expect(screen.getByText(/AI Configuration/i)).toBeTruthy();
			expect(screen.getByRole('combobox', { name: /reviewer agent/i })).toBeTruthy();
			expect(screen.getByText(/Context Sources/i)).toBeTruthy();
		});

		it('shows context source checkboxes for AI gates', () => {
			const edge = createMockGateEdgeWithConfig({
				gateType: GateType.AI,
				aiConfig: {
					contextSources: ['phase_outputs', 'vars'],
				}
			});
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

			expect(screen.getByRole('checkbox', { name: /phase outputs/i })).toBeTruthy();
			expect(screen.getByRole('checkbox', { name: /task details/i })).toBeTruthy();
			expect(screen.getByRole('checkbox', { name: /variables/i })).toBeTruthy();

			// Check that phase_outputs and vars are checked
			expect(screen.getByRole('checkbox', { name: /phase outputs/i }).checked).toBe(true);
			expect(screen.getByRole('checkbox', { name: /variables/i }).checked).toBe(true);
			expect(screen.getByRole('checkbox', { name: /task details/i }).checked).toBe(false);
		});

		it('hides AI configuration when gate type is not AI', () => {
			const edge = createMockGateEdgeWithConfig({ gateType: GateType.AUTO });
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

			expect(screen.queryByText(/AI Configuration/i)).toBeNull();
			expect(screen.queryByRole('combobox', { name: /reviewer agent/i })).toBeNull();
		});
	});

	describe('SC-5: Failure handling section with On Fail dropdown and Retry From', () => {
		it('shows failure handling section', () => {
			const edge = createMockGateEdgeWithConfig({
				gateType: GateType.AUTO,
				failureAction: 'retry_from',
				retryFromPhaseId: 1,
			});
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ isBuiltin: false }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec' }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement' }),
				],
			});

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			expect(screen.getByText(/Failure Handling/i)).toBeTruthy();
			expect(screen.getByRole('combobox', { name: /on fail/i })).toBeTruthy();
		});

		it('shows retry from dropdown when failure action is retry_from', () => {
			const edge = createMockGateEdgeWithConfig({
				failureAction: 'retry_from',
				retryFromPhaseId: 1,
			});
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ isBuiltin: false }),
				phases: [
					createMockWorkflowPhase({
						id: 1,
						phaseTemplateId: 'spec',
						template: createMockPhaseTemplate({ name: 'Spec' })
					}),
					createMockWorkflowPhase({
						id: 2,
						phaseTemplateId: 'implement',
						template: createMockPhaseTemplate({ name: 'Implement' })
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

			expect(screen.getByRole('combobox', { name: /retry from/i })).toBeTruthy();
			expect(screen.getByText(/Spec/i)).toBeTruthy();
		});

		it('hides retry from dropdown when failure action is not retry_from', () => {
			const edge = createMockGateEdgeWithConfig({ failureAction: 'retry' });
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

			expect(screen.queryByRole('combobox', { name: /retry from/i })).toBeNull();
		});

		it('shows all failure action options in dropdown', () => {
			const edge = createMockGateEdgeWithConfig({ failureAction: 'fail' });
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

			const onFailSelect = screen.getByRole('combobox', { name: /on fail/i });
			const options = Array.from(onFailSelect.querySelectorAll('option'));
			const optionValues = options.map(option => option.value);

			expect(optionValues).toContain('retry');
			expect(optionValues).toContain('retry_from');
			expect(optionValues).toContain('fail');
			expect(optionValues).toContain('pause');
		});
	});

	describe('SC-6: Max Retries number input functional and editable', () => {
		it('shows editable max retries input in edit mode', () => {
			const edge = createMockGateEdgeWithConfig({ maxRetries: 5 });
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

			const maxRetriesInput = screen.getByRole('spinbutton', { name: /max retries/i });
			expect(maxRetriesInput).toBeTruthy();
			expect(maxRetriesInput.disabled).toBe(false);
			expect(maxRetriesInput.value).toBe('5');
		});

		it('allows editing max retries value', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdgeWithConfig({ maxRetries: 3 });
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

			const maxRetriesInput = screen.getByRole('spinbutton', { name: /max retries/i });
			await user.clear(maxRetriesInput);
			await user.type(maxRetriesInput, '7');

			expect(maxRetriesInput.value).toBe('7');
		});

		it('shows max retries as disabled in read-only mode', () => {
			const edge = createMockGateEdgeWithConfig({ maxRetries: 5 });
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

			const maxRetriesInput = screen.getByRole('spinbutton', { name: /max retries/i });
			expect(maxRetriesInput.disabled).toBe(true);
		});
	});

	describe('SC-7: Advanced collapsible section with scripts and result variable', () => {
		it('shows collapsed Advanced section by default', () => {
			const edge = createMockGateEdgeWithConfig({ gateType: GateType.AUTO });
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

			const advancedSection = screen.getByText(/Advanced/i);
			expect(advancedSection).toBeTruthy();

			// Should be collapsed initially (no inputs visible)
			expect(screen.queryByRole('textbox', { name: /before script/i })).toBeNull();
		});

		it('expands Advanced section when clicked', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdgeWithConfig({
				gateType: GateType.AUTO,
				advancedConfig: {
					beforeScript: '/scripts/before.sh',
					afterScript: '/scripts/after.sh',
					storeResultAs: 'gate_result',
				}
			});
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

			const advancedToggle = screen.getByRole('button', { name: /advanced/i });
			await user.click(advancedToggle);

			// Should now show all advanced inputs
			expect(screen.getByRole('textbox', { name: /before script/i })).toBeTruthy();
			expect(screen.getByRole('textbox', { name: /after script/i })).toBeTruthy();
			expect(screen.getByRole('textbox', { name: /store result as/i })).toBeTruthy();
		});

		it('shows advanced configuration values when expanded', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdgeWithConfig({
				advancedConfig: {
					beforeScript: '/scripts/validate.sh',
					afterScript: '/scripts/notify.sh',
					storeResultAs: 'validation_result',
				}
			});
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

			const advancedToggle = screen.getByRole('button', { name: /advanced/i });
			await user.click(advancedToggle);

			const beforeScriptInput = screen.getByRole('textbox', { name: /before script/i });
			const afterScriptInput = screen.getByRole('textbox', { name: /after script/i });
			const storeResultInput = screen.getByRole('textbox', { name: /store result as/i });

			expect(beforeScriptInput.value).toBe('/scripts/validate.sh');
			expect(afterScriptInput.value).toBe('/scripts/notify.sh');
			expect(storeResultInput.value).toBe('validation_result');
		});
	});

	describe('SC-8: Read-only mode for built-in workflows', () => {
		it('disables all form controls in read-only mode', () => {
			const edge = createMockGateEdgeWithConfig({
				gateType: GateType.AUTO,
				maxRetries: 3,
				autoCriteria: { hasOutput: true }
			});
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

			// All form controls should be disabled
			const gateTypeSelect = screen.getByRole('combobox', { name: /gate type/i });
			const maxRetriesInput = screen.getByRole('spinbutton', { name: /max retries/i });

			expect(gateTypeSelect.disabled).toBe(true);
			expect(maxRetriesInput.disabled).toBe(true);
		});

		it('shows clone to customize notice in read-only mode', () => {
			const edge = createMockGateEdgeWithConfig({ gateType: GateType.AUTO });
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
	});

	describe('SC-9: API integration to save configuration changes', () => {
		it('calls updatePhaseTemplate API when gate configuration changes', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdgeWithConfig({
				gateType: GateType.AUTO,
				phaseId: 2,
				maxRetries: 3,
			});
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'workflow-1', isBuiltin: false }),
				phases: [
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement' }),
				],
			});

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			const maxRetriesInput = screen.getByRole('spinbutton', { name: /max retries/i });
			await user.clear(maxRetriesInput);
			await user.type(maxRetriesInput, '5');

			// Should trigger API call on blur or change
			fireEvent.blur(maxRetriesInput);

			await waitFor(() => {
				expect(mockWorkflowClient.updatePhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						phaseId: 2,
						maxRetries: 5,
					})
				);
			});
		});

		it('handles API errors gracefully', async () => {
			const user = userEvent.setup();
			const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

			mockWorkflowClient.updatePhaseTemplate.mockRejectedValue(new Error('API Error'));

			const edge = createMockGateEdgeWithConfig({ phaseId: 2 });
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ isBuiltin: false }),
				phases: [
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement' }),
				],
			});

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			const gateTypeSelect = screen.getByRole('combobox', { name: /gate type/i });
			await user.selectOptions(gateTypeSelect, GateType.HUMAN.toString());

			await waitFor(() => {
				expect(consoleErrorSpy).toHaveBeenCalledWith(
					'Failed to save gate configuration:',
					expect.any(Error)
				);
			});

			consoleErrorSpy.mockRestore();
		});

		it('shows loading state during API calls', async () => {
			const user = userEvent.setup();
			let resolvePromise: (value: any) => void;
			const promise = new Promise(resolve => { resolvePromise = resolve; });

			mockWorkflowClient.updatePhaseTemplate.mockReturnValue(promise);

			const edge = createMockGateEdgeWithConfig({ phaseId: 2 });
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ isBuiltin: false }),
				phases: [
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement' }),
				],
			});

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			const gateTypeSelect = screen.getByRole('combobox', { name: /gate type/i });
			await user.selectOptions(gateTypeSelect, GateType.HUMAN.toString());

			// Should show loading state
			expect(gateTypeSelect.disabled).toBe(true);

			// Resolve the API call
			resolvePromise!({});

			await waitFor(() => {
				expect(gateTypeSelect.disabled).toBe(false);
			});
		});
	});

	describe('Integration: Complete gate configuration workflows', () => {
		it('configures a complete auto gate with all settings', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdgeWithConfig({
				gateType: GateType.AUTO,
				phaseId: 2,
			});
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ isBuiltin: false }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec' }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement' }),
				],
			});

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			// Configure auto criteria
			const hasOutputCheckbox = screen.getByRole('checkbox', { name: /has output/i });
			await user.click(hasOutputCheckbox);

			// Configure custom pattern
			const customPatternInput = screen.getByRole('textbox', { name: /custom pattern/i });
			await user.type(customPatternInput, 'COMPLETED: \\d+');

			// Configure failure handling
			const onFailSelect = screen.getByRole('combobox', { name: /on fail/i });
			await user.selectOptions(onFailSelect, 'retry_from');

			const retryFromSelect = screen.getByRole('combobox', { name: /retry from/i });
			await user.selectOptions(retryFromSelect, '1');

			// Configure max retries
			const maxRetriesInput = screen.getByRole('spinbutton', { name: /max retries/i });
			await user.clear(maxRetriesInput);
			await user.type(maxRetriesInput, '5');

			// All configuration should be applied
			expect(hasOutputCheckbox.checked).toBe(true);
			expect(customPatternInput.value).toBe('COMPLETED: \\d+');
			expect(onFailSelect.value).toBe('retry_from');
			expect(retryFromSelect.value).toBe('1');
			expect(maxRetriesInput.value).toBe('5');
		});

		it('switches between gate types and shows appropriate sections', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdgeWithConfig({ gateType: GateType.AUTO });
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

			// Initially shows Auto configuration
			expect(screen.getByText(/Auto Configuration/i)).toBeTruthy();
			expect(screen.queryByText(/Human Configuration/i)).toBeNull();
			expect(screen.queryByText(/AI Configuration/i)).toBeNull();

			// Switch to Human
			const gateTypeSelect = screen.getByRole('combobox', { name: /gate type/i });
			await user.selectOptions(gateTypeSelect, GateType.HUMAN.toString());

			expect(screen.queryByText(/Auto Configuration/i)).toBeNull();
			expect(screen.getByText(/Human Configuration/i)).toBeTruthy();
			expect(screen.queryByText(/AI Configuration/i)).toBeNull();

			// Switch to AI
			await user.selectOptions(gateTypeSelect, GateType.AI.toString());

			expect(screen.queryByText(/Auto Configuration/i)).toBeNull();
			expect(screen.queryByText(/Human Configuration/i)).toBeNull();
			expect(screen.getByText(/AI Configuration/i)).toBeTruthy();
		});
	});

	describe('Edge Cases and Error Handling', () => {
		it('handles edge with missing data gracefully', () => {
			const edge = {
				id: 'gate-edge-1',
				source: 'phase-1',
				target: 'phase-2',
				type: 'gate',
				data: null, // Missing data
			};
			const workflowDetails = createMockWorkflowWithDetails();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			// Should render nothing or gracefully handle missing data
			const inspector = document.querySelector('.gate-inspector');
			expect(inspector).toBeNull();
		});

		it('handles edge with partial data gracefully', () => {
			const edge = {
				id: 'gate-edge-1',
				source: 'phase-1',
				target: 'phase-2',
				type: 'gate',
				data: {
					// Missing gateType and position
					maxRetries: 3,
				},
			};
			const workflowDetails = createMockWorkflowWithDetails();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			// Should render with defaults and not crash
			const inspector = document.querySelector('.gate-inspector');
			expect(inspector).toBeTruthy();
		});

		it('handles missing workflow details gracefully', () => {
			const edge = createMockGateEdgeWithConfig({ phaseId: 2 });

			render(
				<GateInspector
					edge={edge}
					workflowDetails={null}
					readOnly={false}
				/>
			);

			// Should render basic inspector without crashing
			const inspector = document.querySelector('.gate-inspector');
			expect(inspector).toBeTruthy();
		});

		it('handles phase not found in workflow gracefully', () => {
			const edge = createMockGateEdgeWithConfig({
				position: 'between',
				phaseId: 999, // Non-existent phase
			});
			const workflowDetails = createMockWorkflowWithDetails({
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec' }),
					createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement' }),
				],
			});

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			// Should show fallback text instead of crashing
			const inspector = document.querySelector('.gate-inspector');
			expect(inspector).toBeTruthy();
		});

		it('handles invalid gate type values', () => {
			const edge = createMockGateEdgeWithConfig({
				gateType: 999 as GateType, // Invalid gate type
			});
			const workflowDetails = createMockWorkflowWithDetails();

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			// Should render with fallback behavior
			const inspector = document.querySelector('.gate-inspector');
			expect(inspector).toBeTruthy();
		});

		it('validates max retries input accepts only positive numbers', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdgeWithConfig({ maxRetries: 3 });
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

			const maxRetriesInput = screen.getByRole('spinbutton', { name: /max retries/i });

			// Try to enter invalid values
			await user.clear(maxRetriesInput);
			await user.type(maxRetriesInput, '-5');

			// Input should either prevent negative values or validate on blur
			expect(maxRetriesInput.validity.valid || maxRetriesInput.value !== '-5').toBe(true);
		});

		it('handles deeply nested configuration objects without crashing', () => {
			const edge = createMockGateEdgeWithConfig({
				gateType: GateType.AUTO,
				autoCriteria: {
					hasOutput: true,
					noErrors: true,
					completionMarker: false,
					customPattern: 'RESULT: .+',
				},
				aiConfig: {
					reviewerAgentId: 'security-reviewer',
					contextSources: ['phase_outputs', 'task_details', 'vars'],
				},
				advancedConfig: {
					beforeScript: '/scripts/setup.sh',
					afterScript: '/scripts/cleanup.sh',
					storeResultAs: 'gate_validation_result',
				},
			});
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

			const inspector = document.querySelector('.gate-inspector');
			expect(inspector).toBeTruthy();

			// Should show auto configuration since gateType is AUTO
			expect(screen.getByText(/Auto Configuration/i)).toBeTruthy();
		});
	});

	describe('Accessibility and Usability', () => {
		it('provides proper labels for all form controls', () => {
			const edge = createMockGateEdgeWithConfig({
				gateType: GateType.AUTO,
				maxRetries: 3,
				failureAction: 'retry_from',
				retryFromPhaseId: 1,
			});
			const workflowDetails = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ isBuiltin: false }),
				phases: [
					createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec' }),
				],
			});

			render(
				<GateInspector
					edge={edge}
					workflowDetails={workflowDetails}
					readOnly={false}
				/>
			);

			// All form controls should have accessible labels
			expect(screen.getByRole('combobox', { name: /gate type/i })).toBeTruthy();
			expect(screen.getByRole('spinbutton', { name: /max retries/i })).toBeTruthy();
			expect(screen.getByRole('combobox', { name: /on fail/i })).toBeTruthy();
			expect(screen.getByRole('combobox', { name: /retry from/i })).toBeTruthy();
		});

		it('maintains focus management when switching gate types', async () => {
			const user = userEvent.setup();
			const edge = createMockGateEdgeWithConfig({ gateType: GateType.AUTO });
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

			const gateTypeSelect = screen.getByRole('combobox', { name: /gate type/i });
			gateTypeSelect.focus();

			await user.selectOptions(gateTypeSelect, GateType.HUMAN.toString());

			// Focus should remain on the gate type select after change
			expect(document.activeElement).toBe(gateTypeSelect);
		});

		it('provides clear visual hierarchy with sections', () => {
			const edge = createMockGateEdgeWithConfig({
				gateType: GateType.AUTO,
				failureAction: 'retry',
			});
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

			// Should have clear section headers
			expect(screen.getByText(/Auto Configuration/i)).toBeTruthy();
			expect(screen.getByText(/Failure Handling/i)).toBeTruthy();
			expect(screen.getByText(/Advanced/i)).toBeTruthy();
		});

		it('shows appropriate help text for complex configuration options', () => {
			const edge = createMockGateEdgeWithConfig({
				gateType: GateType.AUTO,
				autoCriteria: { customPattern: '' }
			});
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

			// Should show help text for custom pattern (assuming this gets implemented)
			const customPatternInput = screen.getByRole('textbox', { name: /custom pattern/i });
			expect(customPatternInput).toBeTruthy();

			// Could check for help text, tooltip, or placeholder
			expect(customPatternInput.placeholder || customPatternInput.title).toBeTruthy();
		});
	});
});