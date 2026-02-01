/**
 * TDD Tests for PhaseInspector condition integration
 *
 * Tests for TASK-694: Condition editor UI in phase inspector
 *
 * Integration tests verifying condition support is wired into SettingsTab:
 * - SC-6: Condition JSON included in UpdatePhase API call when user saves
 * - SC-8: Condition changes tracked by dirty detection, reset by discard
 *
 * Failure Modes:
 * - API error on save shows error banner, condition state preserved (stays dirty)
 *
 * These tests will FAIL until the condition editor is wired into PhaseInspector.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PhaseInspector } from './PhaseInspector';
import {
	createMockWorkflowWithDetails,
	createMockWorkflow,
	createMockWorkflowPhase,
	createMockPhaseTemplate,
} from '@/test/factories';
import {
	GateType,
	PromptSource,
} from '@/gen/orc/v1/workflow_pb';
import type { WorkflowWithDetails, WorkflowPhase } from '@/gen/orc/v1/workflow_pb';

// Mock the API client
vi.mock('@/lib/client', () => ({
	workflowClient: {
		getPromptContent: vi.fn(),
		updatePhase: vi.fn(),
		updatePhaseTemplate: vi.fn(),
		getWorkflow: vi.fn(),
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

// ─── Test Helpers ───────────────────────────────────────────────────────────

/** Create a phase with template for condition testing */
function createConditionTestPhase(
	overrides: {
		phaseId?: number;
		templateId?: string;
		condition?: string;
		isBuiltin?: boolean;
	} = {},
): WorkflowPhase {
	return createMockWorkflowPhase({
		id: overrides.phaseId ?? 1,
		phaseTemplateId: overrides.templateId ?? 'implement',
		sequence: 1,
		condition: overrides.condition,
		template: createMockPhaseTemplate({
			id: overrides.templateId ?? 'implement',
			name: 'Implement',
			description: 'Implement the feature',
			isBuiltin: overrides.isBuiltin ?? false,
			promptSource: PromptSource.FILE,
			gateType: GateType.AUTO,
			maxIterations: 3,
		}),
	});
}

/** Create workflow details for condition testing */
function createConditionTestWorkflow(
	overrides: {
		isBuiltin?: boolean;
		phases?: WorkflowPhase[];
	} = {},
): WorkflowWithDetails {
	const phases = overrides.phases ?? [createConditionTestPhase()];
	return createMockWorkflowWithDetails({
		workflow: createMockWorkflow({
			id: 'test-wf',
			name: 'Test Workflow',
			isBuiltin: overrides.isBuiltin ?? false,
		}),
		phases,
		variables: [],
	});
}

// ─── Tests ──────────────────────────────────────────────────────────────────

describe('PhaseInspector - Condition Integration (TASK-694)', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	// ─── SC-6: Condition JSON included in UpdatePhase API call ────────────────

	describe('SC-6: condition included in updatePhase API call', () => {
		it('includes condition field in updatePhase call when condition is set and saved', async () => {
			const user = userEvent.setup();
			const phase = createConditionTestPhase({ phaseId: 42 });
			const details = createConditionTestWorkflow({ phases: [phase] });

			vi.mocked(workflowClient.updatePhase).mockResolvedValue({
				phase: createMockWorkflowPhase({ id: 42 }),
			} as any);

			const onWorkflowRefresh = vi.fn();

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
					onWorkflowRefresh={onWorkflowRefresh}
				/>,
			);

			// Navigate to Settings tab
			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// The condition section should be visible in settings
			// Find the condition section (CollapsibleSettingsSection with "Condition" title)
			const conditionSections = screen.getAllByText(/Condition/).filter(el => el.closest('.settings-section__header')); expect(conditionSections.length).toBeGreaterThan(0); const conditionSection = conditionSections[0];
			expect(conditionSection).toBeInTheDocument();

			// Add a condition via the ConditionEditor
			// The ConditionEditor should be rendered inside the section
			await user.click(screen.getByRole('button', { name: /add condition/i }));

			// Fill in a condition
			const fieldSelect = screen.getByLabelText(/field/i);
			await user.selectOptions(fieldSelect, 'task.weight');

			const opSelect = screen.getByLabelText(/operator/i);
			await user.selectOptions(opSelect, 'eq');

			const valueInput = screen.getByLabelText(/value/i);
			await user.clear(valueInput);
			await user.type(valueInput, 'medium');

			// Click Save Changes button
			const saveBtn = screen.getByRole('button', { name: /save changes/i });
			await user.click(saveBtn);

			// updatePhase should have been called with condition field
			await waitFor(() => {
				expect(workflowClient.updatePhase).toHaveBeenCalledWith(
					expect.objectContaining({
						workflowId: 'test-wf',
						phaseId: 42,
						condition: expect.stringContaining('task.weight'),
					}),
				);
			});
		});

		it('sends empty/undefined condition to clear existing condition', async () => {
			const user = userEvent.setup();
			const phase = createConditionTestPhase({
				phaseId: 42,
				condition: '{"field":"task.weight","op":"eq","value":"medium"}',
			});
			const details = createConditionTestWorkflow({ phases: [phase] });

			vi.mocked(workflowClient.updatePhase).mockResolvedValue({
				phase: createMockWorkflowPhase({ id: 42 }),
			} as any);

			const onWorkflowRefresh = vi.fn();

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
					onWorkflowRefresh={onWorkflowRefresh}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Remove the existing condition
			const removeButton = screen.getByRole('button', { name: /remove/i });
			await user.click(removeButton);

			// Save
			const saveBtn = screen.getByRole('button', { name: /save changes/i });
			await user.click(saveBtn);

			// updatePhase should be called with condition cleared
			await waitFor(() => {
				expect(workflowClient.updatePhase).toHaveBeenCalledWith(
					expect.objectContaining({
						workflowId: 'test-wf',
						phaseId: 42,
					}),
				);
				// The condition field should be empty/undefined to clear it
				const callArgs = vi.mocked(workflowClient.updatePhase).mock.calls[0][0] as Record<string, unknown>;
				expect(
					callArgs.condition === '' ||
					callArgs.condition === undefined,
				).toBe(true);
			});
		});

		it('shows error banner on API failure, condition state preserved for retry', async () => {
			const user = userEvent.setup();
			const phase = createConditionTestPhase({ phaseId: 42 });
			const details = createConditionTestWorkflow({ phases: [phase] });

			vi.mocked(workflowClient.updatePhase).mockRejectedValue(
				new Error('Network error'),
			);

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Add a condition
			await user.click(screen.getByRole('button', { name: /add condition/i }));

			const fieldSelect = screen.getByLabelText(/field/i);
			await user.selectOptions(fieldSelect, 'task.weight');

			const opSelect = screen.getByLabelText(/operator/i);
			await user.selectOptions(opSelect, 'eq');

			const valueInput = screen.getByLabelText(/value/i);
			await user.type(valueInput, 'medium');

			// Save (will fail)
			const saveBtn = screen.getByRole('button', { name: /save changes/i });
			await user.click(saveBtn);

			// Error should be displayed
			await waitFor(() => {
				expect(screen.getByText(/failed|error/i)).toBeInTheDocument();
			});

			// Condition state should be preserved (save/discard bar still visible)
			expect(screen.getByRole('button', { name: /save changes/i })).toBeInTheDocument();
		});
	});

	// ─── SC-8: Dirty detection and discard ────────────────────────────────────

	describe('SC-8: dirty detection and discard', () => {
		it('shows save/discard bar when condition is changed', async () => {
			const user = userEvent.setup();
			const phase = createConditionTestPhase({ phaseId: 42 });
			const details = createConditionTestWorkflow({ phases: [phase] });

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Initially no save bar (no changes)
			expect(screen.queryByRole('button', { name: /save changes/i })).not.toBeInTheDocument();

			// Add a condition
			await user.click(screen.getByRole('button', { name: /add condition/i }));

			const fieldSelect = screen.getByLabelText(/field/i);
			await user.selectOptions(fieldSelect, 'task.weight');

			const opSelect = screen.getByLabelText(/operator/i);
			await user.selectOptions(opSelect, 'eq');

			// Save/Discard bar should appear (dirty)
			expect(screen.getByRole('button', { name: /save changes/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /discard/i })).toBeInTheDocument();
		});

		it('reverts condition to original on discard', async () => {
			const user = userEvent.setup();
			const phase = createConditionTestPhase({
				phaseId: 42,
				condition: '{"field":"task.weight","op":"eq","value":"medium"}',
			});
			const details = createConditionTestWorkflow({ phases: [phase] });

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// The condition editor should show the existing condition
			// Remove the condition to create a dirty state
			const removeButton = screen.getByRole('button', { name: /remove/i });
			await user.click(removeButton);

			// Save bar should appear
			expect(screen.getByRole('button', { name: /discard/i })).toBeInTheDocument();

			// Click Discard
			await user.click(screen.getByRole('button', { name: /discard/i }));

			// Condition should revert - the condition row should be back
			// The original condition should be restored
			expect(screen.getByLabelText(/field/i)).toBeInTheDocument();
			const fieldSelect = screen.getByLabelText(/field/i) as HTMLSelectElement;
			expect(fieldSelect.value).toBe('task.weight');

			// Save bar should disappear (no longer dirty)
			expect(screen.queryByRole('button', { name: /save changes/i })).not.toBeInTheDocument();
		});

		it('resets condition when selected phase changes', async () => {
			const user = userEvent.setup();

			const phase1 = createConditionTestPhase({
				phaseId: 1,
				templateId: 'spec',
				condition: '{"field":"task.weight","op":"eq","value":"medium"}',
			});
			phase1.template = createMockPhaseTemplate({
				id: 'spec',
				name: 'Spec',
				isBuiltin: false,
				promptSource: PromptSource.FILE,
			});

			const phase2 = createConditionTestPhase({
				phaseId: 2,
				templateId: 'implement',
				// No condition
			});
			phase2.template = createMockPhaseTemplate({
				id: 'implement',
				name: 'Implement',
				isBuiltin: false,
				promptSource: PromptSource.FILE,
			});

			const details = createConditionTestWorkflow({
				phases: [phase1, phase2],
			});

			const { rerender } = render(
				<PhaseInspector
					phase={phase1}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Phase 1 has a condition
			expect(screen.getByLabelText(/field/i)).toBeInTheDocument();

			// Switch to phase 2 (no condition)
			rerender(
				<PhaseInspector
					phase={phase2}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			// Wait for tab to reset to Prompt (default on phase change)
			// Navigate back to Settings
			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Phase 2 has no condition — should show empty state
			expect(screen.getByRole('button', { name: /add condition/i })).toBeInTheDocument();
			expect(screen.queryByLabelText(/field/i)).not.toBeInTheDocument();
		});

		it('does not show save/discard bar when condition has not changed', async () => {
			const user = userEvent.setup();
			const phase = createConditionTestPhase({
				phaseId: 42,
				condition: '{"field":"task.weight","op":"eq","value":"medium"}',
			});
			const details = createConditionTestWorkflow({ phases: [phase] });

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// No changes made - save bar should NOT appear
			expect(screen.queryByRole('button', { name: /save changes/i })).not.toBeInTheDocument();
		});
	});

	// ─── SC-7 integration: existing condition loaded in inspector ─────────────

	describe('SC-7 integration: existing condition loaded', () => {
		it('displays existing phase.condition in the condition editor within settings tab', async () => {
			const user = userEvent.setup();
			const phase = createConditionTestPhase({
				phaseId: 42,
				condition: '{"field":"task.weight","op":"in","value":["medium","large"]}',
			});
			const details = createConditionTestWorkflow({ phases: [phase] });

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// The condition editor should load and display the existing condition
			const fieldSelect = screen.getByLabelText(/field/i) as HTMLSelectElement;
			expect(fieldSelect.value).toBe('task.weight');

			// Operator should show "in"
			const opSelect = screen.getByLabelText(/operator/i) as HTMLSelectElement;
			expect(opSelect.value).toBe('in');

			// Values should be displayed
			expect(screen.getByText('medium')).toBeInTheDocument();
			expect(screen.getByText('large')).toBeInTheDocument();
		});
	});

	// ─── SC-10 integration: read-only condition in inspector ──────────────────

	describe('SC-10 integration: read-only condition', () => {
		it('shows condition as read-only for built-in workflows', async () => {
			const user = userEvent.setup();
			const phase = createConditionTestPhase({
				phaseId: 42,
				condition: '{"field":"task.weight","op":"eq","value":"medium"}',
				isBuiltin: true,
			});
			const details = createConditionTestWorkflow({
				isBuiltin: true,
				phases: [phase],
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={true}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Controls should be present but disabled
			const fieldSelect = screen.getByLabelText(/field/i);
			expect(fieldSelect).toBeDisabled();

			// No add condition button
			expect(screen.queryByRole('button', { name: /add condition/i })).not.toBeInTheDocument();
		});
	});

	// ─── Wiring: ConditionEditor renders inside SettingsTab ───────────────────

	describe('wiring: ConditionEditor rendered in SettingsTab', () => {
		it('renders a Condition section in the Settings tab', async () => {
			const user = userEvent.setup();
			const phase = createConditionTestPhase();
			const details = createConditionTestWorkflow({ phases: [phase] });

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// A "Condition" section header should exist in the settings tab
			expect(screen.getByText(/^condition$/i)).toBeInTheDocument();
		});
	});
});
