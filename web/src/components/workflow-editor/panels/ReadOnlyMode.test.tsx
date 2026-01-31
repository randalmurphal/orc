/**
 * TDD Tests for Read-Only Mode in Built-in Workflows (TASK-641)
 *
 * These tests verify the read-only behavior for built-in workflows and templates.
 * The key distinction is:
 * - Workflow read-only: workflow.isBuiltin determines if workflow structure can be modified
 * - Template read-only: template.isBuiltin determines if prompt content can be edited
 * - Settings read-only: workflow.isBuiltin determines if settings overrides can be changed
 *
 * Success Criteria Coverage:
 * - SC-1: Palette disabled interaction feedback (cursor-not-allowed + toast)
 * - SC-2: Prompt read-only determined by template.isBuiltin (not workflow)
 * - SC-3: Settings always editable in custom workflows (even with built-in templates)
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PhaseTemplatePalette } from './PhaseTemplatePalette';
import { PhaseInspector } from './PhaseInspector';
import {
	createMockWorkflowWithDetails,
	createMockWorkflow,
	createMockWorkflowPhase,
	createMockPhaseTemplate,
} from '@/test/factories';
import { usePhaseTemplates } from '@/stores/workflowStore';
import { GateType, PromptSource } from '@/gen/orc/v1/workflow_pb';
import type { WorkflowWithDetails, WorkflowPhase } from '@/gen/orc/v1/workflow_pb';

// Mock the workflowStore
vi.mock('@/stores/workflowStore', () => ({
	useWorkflowStore: vi.fn(),
	usePhaseTemplates: vi.fn(),
}));

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

// Mock toast notifications
const mockToast = vi.fn();
vi.mock('@/hooks/useToast', () => ({
	useToast: () => ({ toast: mockToast }),
}));

// ─── Test Helpers ───────────────────────────────────────────────────────────

/** Create a phase with an embedded template for testing */
function createPhaseWithTemplate(
	overrides: {
		phaseId?: number;
		templateId?: string;
		templateName?: string;
		isBuiltin?: boolean;
		promptSource?: PromptSource;
		promptContent?: string;
	} = {},
): WorkflowPhase {
	return createMockWorkflowPhase({
		id: overrides.phaseId ?? 1,
		phaseTemplateId: overrides.templateId ?? 'implement',
		sequence: 1,
		template: createMockPhaseTemplate({
			id: overrides.templateId ?? 'implement',
			name: overrides.templateName ?? 'Implement',
			isBuiltin: overrides.isBuiltin ?? true,
			promptSource: overrides.promptSource ?? PromptSource.EMBEDDED,
			promptContent: overrides.promptContent ?? 'Test prompt content',
		}),
	});
}

/** Create a workflow with details for testing */
function createTestWorkflowDetails(
	overrides: {
		workflowId?: string;
		isBuiltin?: boolean;
		phases?: WorkflowPhase[];
	} = {},
): WorkflowWithDetails {
	return createMockWorkflowWithDetails({
		workflow: createMockWorkflow({
			id: overrides.workflowId ?? 'test-wf',
			name: 'Test Workflow',
			isBuiltin: overrides.isBuiltin ?? false,
		}),
		phases: overrides.phases ?? [createPhaseWithTemplate()],
		variables: [],
	});
}

/** Create standard templates for palette tests */
function createStandardTemplates() {
	return [
		createMockPhaseTemplate({
			id: 'spec',
			name: 'Full Spec',
			description: 'Generate specification',
			isBuiltin: true,
		}),
		createMockPhaseTemplate({
			id: 'implement',
			name: 'Implement',
			description: 'Implement the feature',
			isBuiltin: true,
		}),
	];
}

// ─── Tests ──────────────────────────────────────────────────────────────────

describe('Read-Only Mode for Built-in Workflows (TASK-641)', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	// ─── SC-1: Palette disabled interaction feedback ────────────────────────

	describe('SC-1: Palette disabled interaction feedback', () => {
		it('renders template cards with cursor-not-allowed class when readOnly=true', () => {
			vi.mocked(usePhaseTemplates).mockReturnValue(createStandardTemplates());

			render(<PhaseTemplatePalette readOnly={true} workflowId="test-wf" />);

			// Template cards should have cursor-not-allowed styling when readOnly
			const cards = document.querySelectorAll('[data-testid="template-card"]');
			expect(cards.length).toBeGreaterThan(0);

			cards.forEach((card) => {
				// Should have cursor-not-allowed class or style
				expect(card).toHaveClass('cursor-not-allowed');
			});
		});

		it('shows toast notification when drag is attempted in read-only mode', async () => {
			vi.mocked(usePhaseTemplates).mockReturnValue(createStandardTemplates());

			render(<PhaseTemplatePalette readOnly={true} workflowId="test-wf" />);

			const card = document.querySelector('[data-testid="template-card"]');
			expect(card).toBeTruthy();

			// Attempt to start a drag operation
			fireEvent.dragStart(card!);

			// Should trigger a toast notification explaining why drag is disabled
			expect(mockToast).toHaveBeenCalledWith(
				expect.objectContaining({
					description: expect.stringMatching(/clone|read.?only|built.?in/i),
				}),
			);
		});

		it('does not have cursor-not-allowed when readOnly=false', () => {
			vi.mocked(usePhaseTemplates).mockReturnValue(createStandardTemplates());

			render(<PhaseTemplatePalette readOnly={false} workflowId="test-wf" />);

			const cards = document.querySelectorAll('[data-testid="template-card"]');
			expect(cards.length).toBeGreaterThan(0);

			cards.forEach((card) => {
				// Should NOT have cursor-not-allowed class when editable
				expect(card).not.toHaveClass('cursor-not-allowed');
			});
		});
	});

	// ─── SC-2: Prompt read-only determined by template.isBuiltin ────────────

	describe('SC-2: Prompt read-only determined by template.isBuiltin', () => {
		it('shows read-only prompt view (no textarea) for built-in template in custom workflow', async () => {
			// Custom workflow (isBuiltin: false) with built-in template (template.isBuiltin: true)
			const builtinTemplatePhase = createPhaseWithTemplate({
				templateId: 'implement',
				isBuiltin: true, // Template is built-in
				promptSource: PromptSource.EMBEDDED,
				promptContent: 'Built-in prompt content',
			});

			const customWorkflowWithBuiltinTemplate = createTestWorkflowDetails({
				workflowId: 'custom-wf',
				isBuiltin: false, // Workflow is custom
				phases: [builtinTemplatePhase],
			});

			render(
				<PhaseInspector
					phase={builtinTemplatePhase}
					workflowDetails={customWorkflowWithBuiltinTemplate}
					readOnly={false} // Workflow is editable
				/>,
			);

			// Even though workflow is custom (readOnly=false), the prompt should be
			// read-only because template.isBuiltin is true
			expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
			// Should see the prompt content in read-only view
			expect(screen.getByText(/built-in prompt content/i)).toBeInTheDocument();
		});

		it('shows editable textarea for custom template in custom workflow', async () => {
			// Custom workflow (isBuiltin: false) with custom template (template.isBuiltin: false)
			const customTemplatePhase = createPhaseWithTemplate({
				templateId: 'my-custom-phase',
				isBuiltin: false, // Template is custom
				promptSource: PromptSource.DB,
				promptContent: 'Custom prompt content',
			});

			const customWorkflowWithCustomTemplate = createTestWorkflowDetails({
				workflowId: 'custom-wf',
				isBuiltin: false, // Workflow is custom
				phases: [customTemplatePhase],
			});

			render(
				<PhaseInspector
					phase={customTemplatePhase}
					workflowDetails={customWorkflowWithCustomTemplate}
					readOnly={false} // Workflow is editable
				/>,
			);

			// Both workflow and template are custom, so prompt should be editable
			const textarea = screen.getByRole('textbox');
			expect(textarea).toBeInTheDocument();
			expect(textarea).not.toBeDisabled();
			expect(textarea).toHaveValue('Custom prompt content');
		});

		it('shows read-only prompt for built-in template even when workflow.readOnly is false', () => {
			// This tests the specific case where PhaseInspector receives readOnly=false
			// but the template itself is built-in - prompt should still be read-only
			const builtinTemplatePhase = createPhaseWithTemplate({
				templateId: 'spec',
				templateName: 'Full Spec',
				isBuiltin: true,
				promptSource: PromptSource.EMBEDDED,
				promptContent: 'You are a specification writer.',
			});

			const customWorkflow = createTestWorkflowDetails({
				workflowId: 'custom-wf',
				isBuiltin: false,
				phases: [builtinTemplatePhase],
			});

			render(
				<PhaseInspector
					phase={builtinTemplatePhase}
					workflowDetails={customWorkflow}
					readOnly={false}
				/>,
			);

			// No textarea should be present - prompt is determined by template.isBuiltin
			expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
			// Clone Template button should be visible for built-in templates
			expect(screen.getByRole('button', { name: /clone template/i })).toBeInTheDocument();
		});
	});

	// ─── SC-3: Settings always editable in custom workflows ─────────────────

	describe('SC-3: Settings always editable in custom workflows', () => {
		it('enables settings inputs for custom workflow with built-in template', async () => {
			const user = userEvent.setup();

			// Custom workflow with built-in template
			const builtinTemplatePhase = createPhaseWithTemplate({
				templateId: 'implement',
				isBuiltin: true,
			});

			const customWorkflow = createTestWorkflowDetails({
				workflowId: 'custom-wf',
				isBuiltin: false, // Workflow is custom - settings should be editable
				phases: [builtinTemplatePhase],
			});

			render(
				<PhaseInspector
					phase={builtinTemplatePhase}
					workflowDetails={customWorkflow}
					readOnly={false} // Workflow-level readOnly is false
				/>,
			);

			// Switch to Settings tab
			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Settings should be editable even though template is built-in
			// because the WORKFLOW is custom (allows phase override configuration)
			const maxIterationsInput = screen.getByLabelText(/max iterations/i);
			expect(maxIterationsInput).not.toBeDisabled();

			const modelSelect = screen.getByLabelText(/model/i);
			expect(modelSelect).not.toBeDisabled();

			const thinkingCheckbox = screen.getByLabelText(/thinking/i);
			expect(thinkingCheckbox).not.toBeDisabled();

			const gateTypeSelect = screen.getByLabelText(/gate type/i);
			expect(gateTypeSelect).not.toBeDisabled();
		});

		it('disables settings inputs for built-in workflow', async () => {
			const user = userEvent.setup();

			// Built-in workflow with built-in template
			const builtinTemplatePhase = createPhaseWithTemplate({
				templateId: 'implement',
				isBuiltin: true,
			});

			const builtinWorkflow = createTestWorkflowDetails({
				workflowId: 'medium',
				isBuiltin: true, // Workflow is built-in - settings should be disabled
				phases: [builtinTemplatePhase],
			});

			render(
				<PhaseInspector
					phase={builtinTemplatePhase}
					workflowDetails={builtinWorkflow}
					readOnly={true} // Workflow-level readOnly is true
				/>,
			);

			// Switch to Settings tab
			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// All settings should be disabled for built-in workflows
			const maxIterationsInput = screen.getByLabelText(/max iterations/i);
			expect(maxIterationsInput).toBeDisabled();

			const modelSelect = screen.getByLabelText(/model/i);
			expect(modelSelect).toBeDisabled();

			const thinkingCheckbox = screen.getByLabelText(/thinking/i);
			expect(thinkingCheckbox).toBeDisabled();

			const gateTypeSelect = screen.getByLabelText(/gate type/i);
			expect(gateTypeSelect).toBeDisabled();

			// Should show "Clone to customize" message
			expect(screen.getByText(/clone to customize/i)).toBeInTheDocument();
		});

		it('does NOT show "Clone to customize" in settings for custom workflow', async () => {
			const user = userEvent.setup();

			const builtinTemplatePhase = createPhaseWithTemplate({
				templateId: 'implement',
				isBuiltin: true,
			});

			const customWorkflow = createTestWorkflowDetails({
				workflowId: 'custom-wf',
				isBuiltin: false,
				phases: [builtinTemplatePhase],
			});

			render(
				<PhaseInspector
					phase={builtinTemplatePhase}
					workflowDetails={customWorkflow}
					readOnly={false}
				/>,
			);

			await user.click(screen.getByRole('tab', { name: /settings/i }));

			// Custom workflows should NOT show "Clone to customize" in settings
			// (they might show it elsewhere for built-in templates)
			expect(screen.queryByText(/clone to customize/i)).not.toBeInTheDocument();
		});
	});

	// ─── Edge Cases ─────────────────────────────────────────────────────────

	describe('Edge cases', () => {
		it('handles mixed scenario: custom workflow, built-in template, custom settings', async () => {
			const user = userEvent.setup();

			// Phase has custom overrides even though template is built-in
			const phase = createMockWorkflowPhase({
				id: 1,
				phaseTemplateId: 'implement',
				sequence: 1,
				modelOverride: 'claude-opus-4',
				maxIterationsOverride: 5,
				thinkingOverride: true,
				gateTypeOverride: GateType.HUMAN,
				template: createMockPhaseTemplate({
					id: 'implement',
					name: 'Implement',
					isBuiltin: true,
					promptSource: PromptSource.EMBEDDED,
					promptContent: 'Built-in prompt',
					maxIterations: 3,
					gateType: GateType.AUTO,
				}),
			});

			const customWorkflow = createTestWorkflowDetails({
				workflowId: 'custom-wf',
				isBuiltin: false,
				phases: [phase],
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={customWorkflow}
					readOnly={false}
				/>,
			);

			// Prompt should be read-only (built-in template)
			expect(screen.queryByRole('textbox')).not.toBeInTheDocument();

			// Settings should be editable (custom workflow)
			await user.click(screen.getByRole('tab', { name: /settings/i }));

			const maxIterationsInput = screen.getByLabelText(/max iterations/i);
			expect(maxIterationsInput).not.toBeDisabled();
			expect(maxIterationsInput).toHaveValue(5); // Shows override value

			const modelSelect = screen.getByLabelText(/model/i);
			expect(modelSelect).not.toBeDisabled();
		});

		it('correctly distinguishes prompt vs settings editability', async () => {
			const user = userEvent.setup();

			// Custom workflow with custom template
			const customPhase = createPhaseWithTemplate({
				templateId: 'my-phase',
				isBuiltin: false,
				promptSource: PromptSource.DB,
				promptContent: 'Editable prompt',
			});

			const customWorkflow = createTestWorkflowDetails({
				workflowId: 'custom-wf',
				isBuiltin: false,
				phases: [customPhase],
			});

			render(
				<PhaseInspector
					phase={customPhase}
					workflowDetails={customWorkflow}
					readOnly={false}
				/>,
			);

			// Prompt should be editable (custom template)
			expect(screen.getByRole('textbox')).toBeInTheDocument();

			// Settings should also be editable (custom workflow)
			await user.click(screen.getByRole('tab', { name: /settings/i }));
			const maxIterationsInput = screen.getByLabelText(/max iterations/i);
			expect(maxIterationsInput).not.toBeDisabled();
		});
	});
});
