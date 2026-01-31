/**
 * TDD Tests for PhaseInspector component
 *
 * Tests for TASK-638: Phase Inspector panel with tabs
 *
 * Success Criteria Coverage:
 * - SC-1: Selecting phase node opens inspector with header (template name, ID, badge)
 * - SC-2: Inspector has 4 tabs (Phase Input, Prompt, Completion, Settings) using Radix Tabs
 * - SC-3: Deselecting closes inspector panel
 * - SC-4: Inspector replaces inline inspector in WorkflowEditorPage
 * - SC-7: Phase Input tab shows input variables with satisfaction status
 * - SC-8: Available Variables collapsible section shows workflow variables
 * - SC-9: Settings tab shows phase override controls
 * - SC-10: Settings changes call updatePhase API and refresh data
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PhaseInspector } from './PhaseInspector';
import {
	createMockWorkflowWithDetails,
	createMockWorkflow,
	createMockWorkflowPhase,
	createMockWorkflowVariable,
	createMockPhaseTemplate,
} from '@/test/factories';
import {
	GateType,
	PromptSource,
	VariableSourceType,
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

/** Create a phase with an embedded template for inspector testing */
function createPhaseWithTemplate(
	overrides: {
		phaseId?: number;
		templateId?: string;
		templateName?: string;
		description?: string;
		isBuiltin?: boolean;
		promptSource?: PromptSource;
		promptContent?: string;
		inputVariables?: string[];
		gateType?: GateType;
		maxIterations?: number;
		modelOverride?: string;
		thinkingOverride?: boolean;
		gateTypeOverride?: GateType;
		maxIterationsOverride?: number;
	} = {},
): WorkflowPhase {
	return createMockWorkflowPhase({
		id: overrides.phaseId ?? 1,
		phaseTemplateId: overrides.templateId ?? 'implement',
		sequence: 1,
		modelOverride: overrides.modelOverride,
		thinkingOverride: overrides.thinkingOverride,
		gateTypeOverride: overrides.gateTypeOverride,
		maxIterationsOverride: overrides.maxIterationsOverride,
		template: createMockPhaseTemplate({
			id: overrides.templateId ?? 'implement',
			name: overrides.templateName ?? 'Implement',
			description: overrides.description ?? 'Implement the feature',
			isBuiltin: overrides.isBuiltin ?? true,
			promptSource: overrides.promptSource ?? PromptSource.FILE,
			promptContent: overrides.promptContent,
			inputVariables: overrides.inputVariables ?? [],
			gateType: overrides.gateType ?? GateType.AUTO,
			maxIterations: overrides.maxIterations ?? 3,
		}),
	});
}

/** Create a workflow with details suitable for inspector testing */
function createTestWorkflowDetails(
	overrides: {
		isBuiltin?: boolean;
		phases?: WorkflowPhase[];
		variableNames?: string[];
	} = {},
): WorkflowWithDetails {
	const variables = (overrides.variableNames ?? []).map((name, i) =>
		createMockWorkflowVariable({
			id: i + 1,
			name,
			sourceType: VariableSourceType.STATIC,
			required: true,
		}),
	);

	return createMockWorkflowWithDetails({
		workflow: createMockWorkflow({
			id: 'test-wf',
			name: 'Test Workflow',
			isBuiltin: overrides.isBuiltin ?? true,
		}),
		phases: overrides.phases ?? [createPhaseWithTemplate()],
		variables,
	});
}

// ─── Tests ──────────────────────────────────────────────────────────────────

describe('PhaseInspector', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	// ─── SC-1: Inspector header with template name, ID, and badge ───────────

	describe('SC-1: inspector header', () => {
		it('displays template name in header when phase is selected', () => {
			const phase = createPhaseWithTemplate({ templateName: 'Full Spec' });
			const details = createTestWorkflowDetails({ phases: [phase] });

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={true}
				/>,
			);

			// Header renders "{name} Phase"
			expect(screen.getByText(/Full Spec/)).toBeInTheDocument();
		});

		it('displays phase template ID in header', () => {
			const phase = createPhaseWithTemplate({
				templateId: 'spec',
				templateName: 'Full Spec',
			});
			const details = createTestWorkflowDetails({ phases: [phase] });

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={true}
				/>,
			);

			expect(screen.getByText('spec')).toBeInTheDocument();
		});

		it('shows "Built-in" badge for built-in phase templates', () => {
			const phase = createPhaseWithTemplate({ isBuiltin: true });
			const details = createTestWorkflowDetails({
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

			expect(screen.getByText('Built-in')).toBeInTheDocument();
		});

		it('does not show "Built-in" badge for custom phase templates', () => {
			const phase = createPhaseWithTemplate({ isBuiltin: false });
			const details = createTestWorkflowDetails({
				isBuiltin: false,
				phases: [phase],
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			// Custom templates do not get a "Built-in" badge
			expect(screen.queryByText('Built-in')).not.toBeInTheDocument();
		});

		it('does not render when phase is null', () => {
			const details = createTestWorkflowDetails();

			const { container } = render(
				<PhaseInspector
					phase={null}
					workflowDetails={details}
					readOnly={true}
				/>,
			);

			// Inspector should render nothing when no phase is selected
			expect(container.children).toHaveLength(0);
		});
	});

	// ─── SC-2: Four tabs (Phase Input, Prompt, Completion, Settings) ─────────

	describe('SC-2: tab structure', () => {
		it('renders four tab triggers: Phase Input, Prompt, Completion, Settings', () => {
			const phase = createPhaseWithTemplate();
			const details = createTestWorkflowDetails({ phases: [phase] });

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={true}
				/>,
			);

			expect(screen.getByRole('tab', { name: /phase input/i })).toBeInTheDocument();
			expect(screen.getByRole('tab', { name: /prompt/i })).toBeInTheDocument();
			expect(screen.getByRole('tab', { name: /completion/i })).toBeInTheDocument();
			expect(screen.getByRole('tab', { name: /settings/i })).toBeInTheDocument();
		});

		it('defaults to Prompt tab on first open', () => {
			const phase = createPhaseWithTemplate();
			const details = createTestWorkflowDetails({ phases: [phase] });

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={true}
				/>,
			);

			const promptTab = screen.getByRole('tab', { name: /^prompt$/i });
			expect(promptTab).toHaveAttribute('data-state', 'active');
		});

		it('switches to Phase Input tab when clicked', async () => {
			const user = userEvent.setup();
			const phase = createPhaseWithTemplate();
			const details = createTestWorkflowDetails({ phases: [phase] });

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={true}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /phase input/i }));

			expect(
				screen.getByRole('tab', { name: /phase input/i }),
			).toHaveAttribute('data-state', 'active');
		});

		it('switches to Settings tab when clicked', async () => {
			const user = userEvent.setup();
			const phase = createPhaseWithTemplate();
			const details = createTestWorkflowDetails({ phases: [phase] });

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={true}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			expect(
				screen.getByRole('tab', { name: /settings/i }),
			).toHaveAttribute('data-state', 'active');
		});
	});

	// ─── SC-7: Phase Input tab - variable satisfaction status ────────────────

	describe('SC-7: variable satisfaction status', () => {
		it('shows satisfied indicator for input variables matched by workflow variables', async () => {
			const user = userEvent.setup();
			const phase = createPhaseWithTemplate({
				inputVariables: ['TASK_DESCRIPTION'],
			});
			const details = createTestWorkflowDetails({
				phases: [phase],
				variableNames: ['TASK_DESCRIPTION'],
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={true}
				/>,
			);

			// Switch to Phase Input tab
			await user.click(screen.getByRole('tab', { name: /phase input/i }));

			// The matched variable should show as provided
			expect(screen.getAllByText(/TASK_DESCRIPTION/).length).toBeGreaterThan(0);
			expect(screen.getByText(/Provided/)).toBeInTheDocument();
		});

		it('shows warning for unmatched input variables', async () => {
			const user = userEvent.setup();
			const phase = createPhaseWithTemplate({
				inputVariables: ['TASK_DESCRIPTION', 'MISSING_VAR'],
			});
			const details = createTestWorkflowDetails({
				phases: [phase],
				variableNames: ['TASK_DESCRIPTION'],
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={true}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /phase input/i }));

			// MISSING_VAR has no matching workflow variable
			expect(screen.getByText(/Missing/)).toBeInTheDocument();
		});

		it('shows "No input variables" message when phase has no inputVariables', async () => {
			const user = userEvent.setup();
			const phase = createPhaseWithTemplate({ inputVariables: [] });
			const details = createTestWorkflowDetails({ phases: [phase] });

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={true}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /phase input/i }));

			expect(screen.getByText(/no input variables/i)).toBeInTheDocument();
		});
	});

	// ─── SC-8: Available workflow variables (collapsible section) ────────────

	describe('SC-8: available workflow variables', () => {
		it('lists workflow variables with source type badges in collapsible section', () => {
			const phase = createPhaseWithTemplate();
			const details = createMockWorkflowWithDetails({
				workflow: createMockWorkflow({ id: 'test-wf', isBuiltin: false }),
				phases: [phase],
				variables: [
					createMockWorkflowVariable({
						id: 1,
						name: 'TASK_DESCRIPTION',
						sourceType: VariableSourceType.STATIC,
						required: true,
					}),
					createMockWorkflowVariable({
						id: 2,
						name: 'API_KEY',
						sourceType: VariableSourceType.ENV,
						required: false,
					}),
				],
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			// Variables are in the collapsible section (always visible, not in a tab)
			expect(screen.getByText('Available Variables')).toBeInTheDocument();

			// Variable names visible
			expect(screen.getByText('TASK_DESCRIPTION')).toBeInTheDocument();
			expect(screen.getByText('API_KEY')).toBeInTheDocument();

			// Source type badges
			expect(screen.getByText(/static/i)).toBeInTheDocument();
			expect(screen.getByText(/env/i)).toBeInTheDocument();
		});

		it('shows "+ Add Variable" button for custom workflows', () => {
			const phase = createPhaseWithTemplate();
			const details = createTestWorkflowDetails({
				isBuiltin: false,
				phases: [phase],
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			// Add Variable button is in the collapsible section
			expect(screen.getByRole('button', { name: /add variable/i })).toBeInTheDocument();
		});

		it('hides "+ Add Variable" button for built-in workflows', () => {
			const phase = createPhaseWithTemplate();
			const details = createTestWorkflowDetails({
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

			expect(screen.queryByRole('button', { name: /add variable/i })).not.toBeInTheDocument();
		});

		it('shows empty state when workflow has no variables', () => {
			const phase = createPhaseWithTemplate();
			const details = createTestWorkflowDetails({
				phases: [phase],
				variableNames: [],
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={true}
				/>,
			);

			// Empty state text in the collapsible section
			expect(screen.getByText(/no variables defined/i)).toBeInTheDocument();
		});
	});

	// ─── SC-9: Settings tab override controls ───────────────────────────────

	describe('SC-9: settings tab controls', () => {
		it('renders model dropdown, thinking toggle, gate type dropdown, max iterations input', async () => {
			const user = userEvent.setup();
			const phase = createPhaseWithTemplate();
			const details = createTestWorkflowDetails({
				isBuiltin: false,
				phases: [phase],
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// All 4 controls should be present
			expect(screen.getByLabelText(/model/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/thinking/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/gate type/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/max iterations/i)).toBeInTheDocument();
		});

		it('shows current override values when overrides are set', async () => {
			const user = userEvent.setup();
			const phase = createPhaseWithTemplate({
				modelOverride: 'claude-opus-4',
				maxIterationsOverride: 5,
				gateTypeOverride: GateType.HUMAN,
			});
			const details = createTestWorkflowDetails({
				isBuiltin: false,
				phases: [phase],
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Max iterations should show the override value
			const maxIterationsInput = screen.getByLabelText(/max iterations/i);
			expect(maxIterationsInput).toHaveValue(5);
		});

		it('disables all controls for built-in workflows with "Clone to customize" message', async () => {
			const user = userEvent.setup();
			const phase = createPhaseWithTemplate({ isBuiltin: true });
			const details = createTestWorkflowDetails({
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

			expect(screen.getByText(/clone to customize/i)).toBeInTheDocument();

			// Controls should be disabled
			const maxIterationsInput = screen.getByLabelText(/max iterations/i);
			expect(maxIterationsInput).toBeDisabled();
		});
	});

	// ─── SC-10: Settings changes call updatePhase API ───────────────────────

	describe('SC-10: settings API calls', () => {
		it('calls updatePhase API when max iterations is changed', async () => {
			const user = userEvent.setup();
			const phase = createPhaseWithTemplate({ phaseId: 42 });
			const details = createTestWorkflowDetails({
				isBuiltin: false,
				phases: [phase],
			});

			// Mock successful updatePhase + getWorkflow refresh
			vi.mocked(workflowClient.updatePhase).mockResolvedValue({
				phase: createMockWorkflowPhase({ id: 42 }),
			} as any);
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue({
				workflow: details,
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

			// Change max iterations
			const maxIterationsInput = screen.getByLabelText(/max iterations/i);
			await user.clear(maxIterationsInput);
			await user.type(maxIterationsInput, '5');

			// Trigger the change (blur)
			await user.tab();

			await waitFor(() => {
				expect(workflowClient.updatePhase).toHaveBeenCalledWith(
					expect.objectContaining({
						workflowId: 'test-wf',
						phaseId: 42,
					}),
				);
			});
		});

		it('shows error and reverts value on API failure', async () => {
			const user = userEvent.setup();
			const phase = createPhaseWithTemplate({
				phaseId: 42,
				maxIterationsOverride: 3,
			});
			const details = createTestWorkflowDetails({
				isBuiltin: false,
				phases: [phase],
			});

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

			const maxIterationsInput = screen.getByLabelText(/max iterations/i);
			await user.clear(maxIterationsInput);
			await user.type(maxIterationsInput, '99');
			await user.tab();

			await waitFor(() => {
				// Error message should be visible
				expect(screen.getByText(/failed|error/i)).toBeInTheDocument();
			});
		});
	});

	// ─── Edge Cases ─────────────────────────────────────────────────────────

	describe('edge cases', () => {
		it('handles phase with no template gracefully', () => {
			const phase = createMockWorkflowPhase({
				id: 1,
				phaseTemplateId: 'missing',
				// No template field
			});
			const details = createTestWorkflowDetails({ phases: [phase] });

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={true}
				/>,
			);

			// Should show phase template ID as fallback and "Template not found" state
			expect(screen.getByText('missing')).toBeInTheDocument();
			expect(screen.getByText('Template not found')).toBeInTheDocument();
		});

		it('resets to Prompt tab when selected phase changes', async () => {
			const user = userEvent.setup();
			const phase1 = createPhaseWithTemplate({
				phaseId: 1,
				templateId: 'spec',
				templateName: 'Spec',
			});
			const phase2 = createPhaseWithTemplate({
				phaseId: 2,
				templateId: 'implement',
				templateName: 'Implement',
			});
			const details = createTestWorkflowDetails({
				phases: [phase1, phase2],
			});

			const { rerender } = render(
				<PhaseInspector
					phase={phase1}
					workflowDetails={details}
					readOnly={true}
				/>,
			);

			// Switch to Settings tab
			await user.click(screen.getByRole('tab', { name: /settings/i }));
			expect(
				screen.getByRole('tab', { name: /settings/i }),
			).toHaveAttribute('data-state', 'active');

			// Change selected phase
			rerender(
				<PhaseInspector
					phase={phase2}
					workflowDetails={details}
					readOnly={true}
				/>,
			);

			// Should reset to Prompt tab
			expect(
				screen.getByRole('tab', { name: /^prompt$/i }),
			).toHaveAttribute('data-state', 'active');
		});

		it('handles loading state when workflowDetails is null', () => {
			const phase = createPhaseWithTemplate();

			const { container } = render(
				<PhaseInspector
					phase={phase}
					workflowDetails={null}
					readOnly={true}
				/>,
			);

			// Should show loading skeleton or spinner
			expect(
				container.querySelector('.phase-inspector--loading') ||
				screen.queryByText(/loading/i),
			).toBeTruthy();
		});
	});

	// ─── TASK-670: Claude config editor in Settings tab ─────────────────────

	describe('SC-9: Claude config editor sections', () => {
		it('shows collapsible section headers in Settings tab', async () => {
			const user = userEvent.setup();
			const phase = createPhaseWithTemplate({
				templateId: 'implement',
				templateName: 'Implement',
			});
			// Set claude_config on the template
			Object.assign(phase.template!, {
				claudeConfig: '{"hooks": ["lint-hook"], "env": {"NODE_ENV": "test"}}',
			});
			const details = createTestWorkflowDetails({
				isBuiltin: false,
				phases: [phase],
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Claude Config heading
			expect(screen.getByText('Claude Config')).toBeInTheDocument();
			// Section headers rendered as collapsible sections (7 total: hooks, mcp, skills, allowed, disallowed, env, json)
			const sections = screen.getAllByTestId('collapsible-section');
			expect(sections.length).toBeGreaterThanOrEqual(6);
		});

		it('shows inherited template config hint when template has config', async () => {
			const user = userEvent.setup();
			const phase = createMockWorkflowPhase({
				id: 1,
				phaseTemplateId: 'implement',
				sequence: 1,
				claudeConfigOverride: '{"hooks": ["my-hook"], "env": {"DEBUG": "1"}}',
				template: createMockPhaseTemplate({
					id: 'implement',
					name: 'Implement',
					claudeConfig: '{"hooks": ["lint-hook"], "env": {"NODE_ENV": "test"}}',
				}),
			});
			const details = createTestWorkflowDetails({
				isBuiltin: false,
				phases: [phase],
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Should show inherited hint with template item counts
			expect(screen.getByText(/inherited from template/i)).toBeInTheDocument();
		});

		it('shows collapsible sections for each claude_config category', async () => {
			const user = userEvent.setup();
			const phase = createMockWorkflowPhase({
				id: 1,
				phaseTemplateId: 'implement',
				sequence: 1,
				template: createMockPhaseTemplate({
					id: 'implement',
					name: 'Implement',
					claudeConfig: JSON.stringify({
						hooks: ['h1'],
						skill_refs: ['s1'],
						mcp_servers: { 'mcp-1': {} },
						allowed_tools: ['Bash'],
						disallowed_tools: ['Write'],
						env: { K: 'V' },
					}),
				}),
			});
			const details = createTestWorkflowDetails({
				isBuiltin: false,
				phases: [phase],
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// All section header buttons should render (collapsed by default)
			const sectionButtons = screen.getAllByRole('button', { expanded: false });
			const sectionTexts = sectionButtons.map((b) => b.textContent);
			expect(sectionTexts.some((t) => t?.match(/hooks/i))).toBe(true);
			expect(sectionTexts.some((t) => t?.match(/skills/i))).toBe(true);
			expect(sectionTexts.some((t) => t?.match(/mcp servers/i))).toBe(true);
			expect(sectionTexts.some((t) => t?.match(/allowed tools/i))).toBe(true);
			expect(sectionTexts.some((t) => t?.match(/disallowed tools/i))).toBe(true);
			expect(sectionTexts.some((t) => t?.match(/env vars/i))).toBe(true);
		});

		it('is read-only (no edit controls for claude_config)', async () => {
			const user = userEvent.setup();
			const phase = createMockWorkflowPhase({
				id: 1,
				phaseTemplateId: 'implement',
				sequence: 1,
				template: createMockPhaseTemplate({
					id: 'implement',
					name: 'Implement',
					claudeConfig: '{"hooks": ["lint-hook"]}',
				}),
			});
			const details = createTestWorkflowDetails({
				isBuiltin: false,
				phases: [phase],
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Claude config sections should not have add/clear buttons
			expect(screen.queryByRole('button', { name: /add hook/i })).not.toBeInTheDocument();
			expect(screen.queryByRole('button', { name: /clear override/i })).not.toBeInTheDocument();
		});

		it('shows section headers even when phase has no claude_config or override', async () => {
			const user = userEvent.setup();
			const phase = createPhaseWithTemplate({
				templateId: 'implement',
				templateName: 'Implement',
			});
			const details = createTestWorkflowDetails({
				isBuiltin: false,
				phases: [phase],
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Section headers still render (collapsed, with 0 badge)
			const sections = screen.getAllByTestId('collapsible-section');
			expect(sections.length).toBeGreaterThanOrEqual(6);
			// No hook items should be shown
			expect(screen.queryByText('lint-hook')).not.toBeInTheDocument();
		});
	});

	describe('SC-10: Merge logic (override wins on env collision, BDD-4)', () => {
		it('shows override env vars in editor when key collides with template', async () => {
			const user = userEvent.setup();
			const phase = createMockWorkflowPhase({
				id: 1,
				phaseTemplateId: 'implement',
				sequence: 1,
				claudeConfigOverride: '{"env": {"A": "2", "B": "3"}}',
				template: createMockPhaseTemplate({
					id: 'implement',
					name: 'Implement',
					claudeConfig: '{"env": {"A": "1"}}',
				}),
			});
			const details = createTestWorkflowDetails({
				isBuiltin: false,
				phases: [phase],
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Expand the Env Vars section
			const envSection = screen.getAllByRole('button', { expanded: false })
				.find((b) => b.textContent?.match(/env vars/i));
			expect(envSection).toBeDefined();
			await user.click(envSection!);

			// Override env vars should be editable (A=2, B=3)
			const inputs = screen.getAllByRole('textbox');
			const inputValues = inputs.map((i) => (i as HTMLInputElement).value);
			expect(inputValues).toContain('A');
			expect(inputValues).toContain('2');
			expect(inputValues).toContain('B');
			expect(inputValues).toContain('3');
		});

		it('shows hooks badge count reflecting merged hooks', async () => {
			const user = userEvent.setup();
			const phase = createMockWorkflowPhase({
				id: 1,
				phaseTemplateId: 'implement',
				sequence: 1,
				claudeConfigOverride: '{"hooks": ["override-hook"]}',
				template: createMockPhaseTemplate({
					id: 'implement',
					name: 'Implement',
					claudeConfig: '{"hooks": ["template-hook"]}',
				}),
			});
			const details = createTestWorkflowDetails({
				isBuiltin: false,
				phases: [phase],
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Hooks section should show a badge with the merged count (2)
			const hooksButton = screen.getAllByRole('button', { expanded: false })
				.find((b) => b.textContent?.match(/hooks/i));
			expect(hooksButton).toBeDefined();
			expect(hooksButton!.textContent).toMatch(/2/);
		});

		it('uses template config badge when override is empty', async () => {
			const user = userEvent.setup();
			const phase = createMockWorkflowPhase({
				id: 1,
				phaseTemplateId: 'implement',
				sequence: 1,
				template: createMockPhaseTemplate({
					id: 'implement',
					name: 'Implement',
					claudeConfig: '{"hooks": ["only-template-hook"]}',
				}),
			});
			const details = createTestWorkflowDetails({
				isBuiltin: false,
				phases: [phase],
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Hooks section badge should show 1 (from template)
			const hooksButton = screen.getAllByRole('button', { expanded: false })
				.find((b) => b.textContent?.match(/hooks/i));
			expect(hooksButton).toBeDefined();
			expect(hooksButton!.textContent).toMatch(/1/);
		});

		it('uses override config badge when template is empty', async () => {
			const user = userEvent.setup();
			const phase = createMockWorkflowPhase({
				id: 1,
				phaseTemplateId: 'implement',
				sequence: 1,
				claudeConfigOverride: '{"hooks": ["only-override-hook"]}',
				template: createMockPhaseTemplate({
					id: 'implement',
					name: 'Implement',
				}),
			});
			const details = createTestWorkflowDetails({
				isBuiltin: false,
				phases: [phase],
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={details}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Hooks section badge should show 1 (from override)
			const hooksButton = screen.getAllByRole('button', { expanded: false })
				.find((b) => b.textContent?.match(/hooks/i));
			expect(hooksButton).toBeDefined();
			expect(hooksButton!.textContent).toMatch(/1/);
		});
	});
});
