/**
 * TDD Tests for PhaseInspector AI gate type and agent picker.
 *
 * Tests for TASK-655: CLI and UI for gate management
 *
 * Success Criteria Coverage:
 * - SC-11: Gate Type dropdown includes AI option
 * - SC-12: AI gate type shows conditional agent picker
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

// GateType_AI (value 4) does not exist yet - it must be added to the proto.
// These tests will fail until GATE_TYPE_AI = 4 is added to orc.v1.GateType.
const GateType_AI = 4 as GateType;

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
	},
}));

import { workflowClient, configClient } from '@/lib/client';

// ─── Test Helpers ───────────────────────────────────────────────────────────

/** Create a phase with template for gate testing */
function createGateTestPhase(
	overrides: {
		phaseId?: number;
		templateId?: string;
		gateType?: GateType;
		gateTypeOverride?: GateType;
	} = {},
): WorkflowPhase {
	return createMockWorkflowPhase({
		id: overrides.phaseId ?? 1,
		phaseTemplateId: overrides.templateId ?? 'review',
		sequence: 1,
		gateTypeOverride: overrides.gateTypeOverride,
		template: createMockPhaseTemplate({
			id: overrides.templateId ?? 'review',
			name: 'Review',
			description: 'Code review phase',
			isBuiltin: false,
			promptSource: PromptSource.FILE,
			gateType: overrides.gateType ?? GateType.AUTO,
			maxIterations: 3,
		}),
	});
}

/** Create workflow details for gate testing */
function createGateTestWorkflow(
	overrides: {
		isBuiltin?: boolean;
		phases?: WorkflowPhase[];
	} = {},
): WorkflowWithDetails {
	const phases = overrides.phases ?? [createGateTestPhase()];
	return createMockWorkflowWithDetails({
		workflow: createMockWorkflow({
			isBuiltin: overrides.isBuiltin ?? false,
		}),
		phases,
		variables: [],
	});
}

describe('PhaseInspector - AI Gate Type (TASK-655)', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	// ─── SC-11: Gate Type dropdown includes AI option ────────────────────────

	describe('SC-11: AI gate type option in dropdown', () => {
		it('renders AI as a gate type option in the dropdown', async () => {
			const user = userEvent.setup();
			const phase = createGateTestPhase();
			const details = createGateTestWorkflow({ phases: [phase] });

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			// Navigate to Settings tab
			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Gate type dropdown should have AI option
			const gateTypeSelect = screen.getByLabelText(/gate type/i);
			expect(gateTypeSelect).toBeInTheDocument();

			// Check that AI is one of the options
			const options = gateTypeSelect.querySelectorAll('option');
			const optionValues = Array.from(options).map((opt) => opt.textContent);

			expect(optionValues).toContain('AI');
		});

		it('includes all expected gate type options: Inherit, Auto, Human, AI, Skip', async () => {
			const user = userEvent.setup();
			const phase = createGateTestPhase();
			const details = createGateTestWorkflow({ phases: [phase] });

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			const gateTypeSelect = screen.getByLabelText(/gate type/i);
			const options = gateTypeSelect.querySelectorAll('option');
			const optionTexts = Array.from(options).map((opt) => opt.textContent?.toLowerCase());

			expect(optionTexts).toContain('inherit from template');
			expect(optionTexts).toContain('auto');
			expect(optionTexts).toContain('human');
			expect(optionTexts).toContain('ai');
			expect(optionTexts).toContain('skip');
		});

		it('shows hint text when AI is selected but no agents configured', async () => {
			const user = userEvent.setup();
			const phase = createGateTestPhase({
				gateTypeOverride: GateType_AI,
			});
			const details = createGateTestWorkflow({ phases: [phase] });

			// Mock empty agents list
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [] } as any);

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Should show hint about configuring agents
			await waitFor(() => {
				expect(screen.getByText(/no agents available|configure agents/i)).toBeInTheDocument();
			});
		});
	});

	// ─── SC-12: AI gate agent picker ────────────────────────────────────────

	describe('SC-12: AI gate agent picker', () => {
		it('shows agent dropdown when AI gate type is selected', async () => {
			const user = userEvent.setup();
			const phase = createGateTestPhase({
				gateTypeOverride: GateType_AI,
			});
			const details = createGateTestWorkflow({ phases: [phase] });

			// Mock agents list with available agents
			vi.mocked(configClient.listAgents).mockResolvedValue({
				agents: [
					{ id: 'security-reviewer', name: 'Security Reviewer', model: 'claude-sonnet-4' },
					{ id: 'code-quality', name: 'Code Quality', model: 'claude-sonnet-4' },
				],
			} as any);

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Agent dropdown should appear when AI is the gate type
			await waitFor(() => {
				expect(screen.getByLabelText(/ai gate agent|agent/i)).toBeInTheDocument();
			});
		});

		it('does NOT show agent dropdown when gate type is not AI', async () => {
			const user = userEvent.setup();
			const phase = createGateTestPhase({
				gateTypeOverride: GateType.HUMAN,
			});
			const details = createGateTestWorkflow({ phases: [phase] });

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Agent dropdown should NOT be visible for non-AI gate types
			expect(screen.queryByLabelText(/ai gate agent/i)).not.toBeInTheDocument();
		});

		it('disables agent dropdown when no agents are available', async () => {
			const user = userEvent.setup();
			const phase = createGateTestPhase({
				gateTypeOverride: GateType_AI,
			});
			const details = createGateTestWorkflow({ phases: [phase] });

			// Mock empty agents
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [] } as any);

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			await waitFor(() => {
				const agentSelect = screen.queryByLabelText(/ai gate agent|agent/i);
				if (agentSelect) {
					expect(agentSelect).toBeDisabled();
				}
				// Or alternatively, a "No agents available" text
				expect(screen.getByText(/no agents available/i)).toBeInTheDocument();
			});
		});

		it('saves gate_type_override when AI is selected via dropdown', async () => {
			const user = userEvent.setup();
			const phase = createGateTestPhase({ phaseId: 42 });
			const details = createGateTestWorkflow({
				isBuiltin: false,
				phases: [phase],
			});

			// Mock successful API calls
			vi.mocked(workflowClient.updatePhase).mockResolvedValue({
				phase: createMockWorkflowPhase({ id: 42 }),
			} as any);
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue({
				workflow: details,
			} as any);

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Change gate type to AI
			const gateTypeSelect = screen.getByLabelText(/gate type/i);
			await user.selectOptions(gateTypeSelect, String(GateType_AI));

			// Should call updatePhase API with gate_type_override set to AI
			await waitFor(() => {
				expect(workflowClient.updatePhase).toHaveBeenCalledWith(
					expect.objectContaining({
						phaseId: 42,
						gateTypeOverride: GateType_AI,
					}),
				);
			});
		});
	});
});
