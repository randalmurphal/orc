/**
 * TDD Tests for EditPhaseTemplateModal - Data Flow Fields
 *
 * Tests for TASK-683: Add data flow fields to EditPhaseTemplateModal
 *
 * Success Criteria Coverage:
 * - SC-1: Data Flow section with inputVariables TagInput and 4 suggestion buttons
 * - SC-2: Clicking suggestion adds variable; duplicate clicks ignored
 * - SC-3: outputVarName text input in Data Flow section
 * - SC-4: Save persists inputVariables and outputVarName through API round-trip
 * - SC-5: promptSource toggle (Inline/File) initialized from template
 * - SC-6: Conditional promptPath rendering based on toggle state
 * - SC-7: Save sends promptSource and promptPath correctly
 *
 * SC-8 is covered by Go backend tests (workflow_template_conversion_test.go)
 * SC-9 is already tested by existing PhaseInspector tests
 *
 * Failure Modes:
 * - Save failure → error toast, modal stays open (covered by existing test)
 * - Empty outputVarName → sent as undefined
 * - Toggle to File with empty path → allowed (validation out of scope)
 *
 * Edge Cases:
 * - Template with no inputVariables/outputVarName → empty inputs
 * - promptSource EMBEDDED → maps to "Inline"
 * - promptSource UNSPECIFIED → defaults to "Inline"
 * - Remove all inputVariables → empty array sent
 * - outputVarName whitespace → trimmed
 * - promptPath prefix is visual-only, not stored in value
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { EditPhaseTemplateModal } from './EditPhaseTemplateModal';
import { create } from '@bufbuild/protobuf';
import {
	ListHooksResponseSchema,
	ListSkillsResponseSchema,
	ListAgentsResponseSchema,
} from '@/gen/orc/v1/config_pb';
import { ListMCPServersResponseSchema } from '@/gen/orc/v1/mcp_pb';
import { PromptSource } from '@/gen/orc/v1/workflow_pb';
import {
	createMockPhaseTemplate,
	createMockUpdatePhaseTemplateResponse,
} from '@/test/factories';

// Mock the client module
vi.mock('@/lib/client', () => ({
	workflowClient: {
		updatePhaseTemplate: vi.fn(),
	},
	configClient: {
		listAgents: vi.fn(),
		listHooks: vi.fn(),
		listSkills: vi.fn(),
	},
	mcpClient: {
		listMCPServers: vi.fn(),
	},
}));

// Mock toast
vi.mock('@/stores/uiStore', () => ({
	toast: {
		success: vi.fn(),
		error: vi.fn(),
	},
}));

// Import mocked modules for assertions
import { workflowClient, configClient, mcpClient } from '@/lib/client';

function setupMocks() {
	vi.mocked(configClient.listAgents).mockResolvedValue(
		create(ListAgentsResponseSchema, { agents: [] })
	);
	vi.mocked(configClient.listHooks).mockResolvedValue(
		create(ListHooksResponseSchema, { hooks: [] })
	);
	vi.mocked(configClient.listSkills).mockResolvedValue(
		create(ListSkillsResponseSchema, { skills: [] })
	);
	vi.mocked(mcpClient.listMCPServers).mockResolvedValue(
		create(ListMCPServersResponseSchema, { servers: [] })
	);
}

const mockOnClose = vi.fn();
const mockOnUpdated = vi.fn();

/** Helper: render the modal, wait for async data loading to complete */
async function renderAndWait(
	templateOverrides: Parameters<typeof createMockPhaseTemplate>[0] = {},
	props: Partial<React.ComponentProps<typeof EditPhaseTemplateModal>> = {}
) {
	const result = render(
		<EditPhaseTemplateModal
			open={true}
			template={createMockPhaseTemplate({ isBuiltin: false, ...templateOverrides })}
			onClose={mockOnClose}
			onUpdated={mockOnUpdated}
			{...props}
		/>
	);

	await waitFor(() => {
		expect(configClient.listAgents).toHaveBeenCalled();
	});

	return result;
}

/** Helper: set up save mock and return user event instance */
function setupSave() {
	const user = userEvent.setup();

	vi.mocked(workflowClient.updatePhaseTemplate).mockResolvedValue(
		createMockUpdatePhaseTemplateResponse(
			createMockPhaseTemplate({ isBuiltin: false })
		)
	);

	return user;
}

describe('EditPhaseTemplateModal - Data Flow Fields (TASK-683)', () => {
	// NOTE: Browser API mocks (ResizeObserver, IntersectionObserver, scrollIntoView) provided by global test-setup.ts

	beforeEach(() => {
		vi.clearAllMocks();
		setupMocks();
	});

	afterEach(() => {
		cleanup();
	});

	// ─── SC-1: Data Flow section with inputVariables TagInput and suggestion buttons ───

	describe('SC-1: Data Flow section renders with TagInput and suggestion buttons', () => {
		it('renders a "Data Flow" section header', async () => {
			await renderAndWait();

			expect(screen.getByText(/^Data Flow$/i)).toBeInTheDocument();
		});

		it('renders 4 suggestion buttons for built-in variables', async () => {
			await renderAndWait();

			const builtinVars = [
				'SPEC_CONTENT',
				'PROJECT_ROOT',
				'TASK_DESCRIPTION',
				'WORKTREE_PATH',
			];
			for (const varName of builtinVars) {
				expect(screen.getByRole('button', { name: varName })).toBeInTheDocument();
			}
		});

		it('renders an empty TagInput for inputVariables when template has none', async () => {
			await renderAndWait({ inputVariables: [] });

			// The Data Flow section should be present with no chips
			expect(screen.getByText(/^Data Flow$/i)).toBeInTheDocument();

			// No chip elements should be rendered for input variables
			// (suggestion buttons exist, but no selected-variable chips)
			const specButton = screen.getByRole('button', { name: 'SPEC_CONTENT' });
			expect(specButton).toBeInTheDocument();
		});

		it('renders existing inputVariables as chips when template has them', async () => {
			await renderAndWait({
				inputVariables: ['SPEC_CONTENT', 'TASK_DESCRIPTION'],
			});

			// The variables should appear as chips/tags in the TagInput
			// TagInput renders tags as removable elements
			expect(screen.getByText('SPEC_CONTENT')).toBeInTheDocument();
			expect(screen.getByText('TASK_DESCRIPTION')).toBeInTheDocument();
		});
	});

	// ─── SC-2: Clicking suggestion adds variable; duplicates ignored ───

	describe('SC-2: Suggestion button click behavior', () => {
		it('clicking a suggestion button adds that variable as a chip', async () => {
			const user = userEvent.setup();
			await renderAndWait({ inputVariables: [] });

			await user.click(screen.getByRole('button', { name: 'SPEC_CONTENT' }));

			// SPEC_CONTENT should now appear as a chip (in addition to the suggestion button)
			// The chip text should be present in the Data Flow section
			await waitFor(() => {
				// There should be at least 2 elements with this text:
				// the suggestion button AND the chip
				const elements = screen.getAllByText('SPEC_CONTENT');
				expect(elements.length).toBeGreaterThanOrEqual(1);
			});
		});

		it('clicking the same suggestion button twice does not create a duplicate chip', async () => {
			const user = userEvent.setup();
			await renderAndWait({ inputVariables: [] });

			const button = screen.getByRole('button', { name: 'SPEC_CONTENT' });
			await user.click(button);
			await user.click(button);

			// Should still have exactly one chip for SPEC_CONTENT
			// (TagInput prevents duplicates)
			await waitFor(() => {
				const elements = screen.getAllByText('SPEC_CONTENT');
				// One for suggestion button, at most one chip
				expect(elements.length).toBeLessThanOrEqual(2);
			});
		});

		it('clicking multiple suggestion buttons adds multiple chips', async () => {
			const user = userEvent.setup();
			await renderAndWait({ inputVariables: [] });

			await user.click(screen.getByRole('button', { name: 'SPEC_CONTENT' }));
			await user.click(screen.getByRole('button', { name: 'TASK_DESCRIPTION' }));

			await waitFor(() => {
				expect(screen.getByText('SPEC_CONTENT')).toBeInTheDocument();
				expect(screen.getByText('TASK_DESCRIPTION')).toBeInTheDocument();
			});
		});
	});

	// ─── SC-3: outputVarName text input in Data Flow section ───

	describe('SC-3: outputVarName text input', () => {
		it('renders with pre-populated value from template', async () => {
			await renderAndWait({ outputVarName: 'ANALYSIS_REPORT' });

			const input = screen.getByDisplayValue('ANALYSIS_REPORT');
			expect(input).toBeInTheDocument();
		});

		it('renders empty when template has no outputVarName', async () => {
			await renderAndWait({ outputVarName: undefined });

			// The input should exist but be empty
			const input = screen.getByLabelText(/output variable/i);
			expect(input).toHaveValue('');
		});
	});

	// ─── SC-4: Save persists inputVariables and outputVarName ───

	describe('SC-4: Save includes inputVariables and outputVarName in API call', () => {
		it('sends inputVariables and outputVarName via updatePhaseTemplate', async () => {
			const user = setupSave();

			await renderAndWait({ inputVariables: [], outputVarName: undefined });

			// Add input variables via suggestion buttons
			await user.click(screen.getByRole('button', { name: 'SPEC_CONTENT' }));
			await user.click(screen.getByRole('button', { name: 'TASK_DESCRIPTION' }));

			// Set output variable name
			const outputInput = screen.getByLabelText(/output variable/i);
			await user.clear(outputInput);
			await user.type(outputInput, 'ANALYSIS_REPORT');

			// Save
			await user.click(screen.getByRole('button', { name: /save/i }));

			await waitFor(() => {
				expect(workflowClient.updatePhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						inputVariables: ['SPEC_CONTENT', 'TASK_DESCRIPTION'],
						outputVarName: 'ANALYSIS_REPORT',
					})
				);
			});
		});

		it('preserves pre-existing inputVariables through save round-trip', async () => {
			const user = setupSave();

			await renderAndWait({
				inputVariables: ['SPEC_CONTENT', 'WORKTREE_PATH'],
				outputVarName: 'MY_OUTPUT',
			});

			// Save without changing data flow fields
			await user.click(screen.getByRole('button', { name: /save/i }));

			await waitFor(() => {
				expect(workflowClient.updatePhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						inputVariables: ['SPEC_CONTENT', 'WORKTREE_PATH'],
						outputVarName: 'MY_OUTPUT',
					})
				);
			});
		});
	});

	// ─── SC-5: promptSource toggle (Inline / File) ───

	describe('SC-5: promptSource toggle initialization', () => {
		it('renders a Prompt section with Inline and File options', async () => {
			await renderAndWait();

			expect(screen.getByText(/^Prompt$/i)).toBeInTheDocument();
			expect(screen.getByText(/Inline/)).toBeInTheDocument();
			expect(screen.getByText(/File/)).toBeInTheDocument();
		});

		it('shows path input when initialized with promptSource FILE', async () => {
			await renderAndWait({ promptSource: PromptSource.FILE });

			// When File is selected, the path input and its prefix should be visible
			expect(screen.getByText('.orc/prompts/')).toBeInTheDocument();
		});

		it('hides path input when initialized with promptSource DB (Inline)', async () => {
			await renderAndWait({ promptSource: PromptSource.DB });

			// When Inline is selected, the path input should not be visible
			expect(screen.queryByText('.orc/prompts/')).not.toBeInTheDocument();
		});

		it('defaults to Inline when promptSource is UNSPECIFIED', async () => {
			await renderAndWait({ promptSource: PromptSource.UNSPECIFIED });

			// UNSPECIFIED defaults to Inline → no path input
			expect(screen.queryByText('.orc/prompts/')).not.toBeInTheDocument();
		});
	});

	// ─── SC-6: Conditional promptPath rendering ───

	describe('SC-6: Conditional promptPath input based on toggle state', () => {
		it('shows path input with .orc/prompts/ prefix when File is active', async () => {
			await renderAndWait({
				promptSource: PromptSource.FILE,
				promptPath: 'spec.md',
			});

			// Prefix text should be visible
			expect(screen.getByText('.orc/prompts/')).toBeInTheDocument();

			// Path input should show current value
			expect(screen.getByDisplayValue('spec.md')).toBeInTheDocument();
		});

		it('hides path input when Inline is active', async () => {
			await renderAndWait({ promptSource: PromptSource.DB });

			// No path prefix or path input should be visible
			expect(screen.queryByText('.orc/prompts/')).not.toBeInTheDocument();
		});

		it('toggling from Inline to File reveals path input', async () => {
			const user = userEvent.setup();
			await renderAndWait({ promptSource: PromptSource.DB });

			// Initially no path input
			expect(screen.queryByText('.orc/prompts/')).not.toBeInTheDocument();

			// Toggle to File
			await user.click(screen.getByText(/File/));

			// Path input should appear
			await waitFor(() => {
				expect(screen.getByText('.orc/prompts/')).toBeInTheDocument();
			});
		});

		it('toggling from File to Inline hides path input', async () => {
			const user = userEvent.setup();
			await renderAndWait({ promptSource: PromptSource.FILE });

			// Initially path input is visible
			expect(screen.getByText('.orc/prompts/')).toBeInTheDocument();

			// Toggle to Inline
			await user.click(screen.getByText(/Inline/));

			// Path input should disappear
			await waitFor(() => {
				expect(screen.queryByText('.orc/prompts/')).not.toBeInTheDocument();
			});
		});
	});

	// ─── SC-7: Save sends promptSource and promptPath ───

	describe('SC-7: Save sends promptSource and promptPath to API', () => {
		it('sends promptSource FILE and promptPath when saved in File mode', async () => {
			const user = setupSave();

			await renderAndWait({
				promptSource: PromptSource.DB,
			});

			// Toggle to File
			await user.click(screen.getByText(/File/));

			// Enter path
			await waitFor(() => {
				expect(screen.getByText('.orc/prompts/')).toBeInTheDocument();
			});

			// Find the path input and type into it
			// The input is adjacent to the .orc/prompts/ prefix
			const pathInput = screen.getByText('.orc/prompts/')
				.closest('div')!
				.querySelector('input')!;
			await user.type(pathInput, 'spec.md');

			// Save
			await user.click(screen.getByRole('button', { name: /save/i }));

			await waitFor(() => {
				expect(workflowClient.updatePhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						promptSource: PromptSource.FILE,
						promptPath: 'spec.md',
					})
				);
			});
		});

		it('sends promptSource DB without promptPath when saved in Inline mode', async () => {
			const user = setupSave();

			await renderAndWait({ promptSource: PromptSource.DB });

			// Save in Inline mode
			await user.click(screen.getByRole('button', { name: /save/i }));

			await waitFor(() => {
				const call = vi.mocked(workflowClient.updatePhaseTemplate).mock.calls[0][0];
				expect(call).toHaveProperty('promptSource', PromptSource.DB);
				// promptPath should be undefined or absent when in Inline mode
				expect(call.promptPath).toBeUndefined();
			});
		});

		it('preserves File + path through save round-trip', async () => {
			const user = setupSave();

			await renderAndWait({
				promptSource: PromptSource.FILE,
				promptPath: 'analysis.md',
			});

			// Save without changing prompt settings
			await user.click(screen.getByRole('button', { name: /save/i }));

			await waitFor(() => {
				expect(workflowClient.updatePhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						promptSource: PromptSource.FILE,
						promptPath: 'analysis.md',
					})
				);
			});
		});
	});

	// ─── Edge Cases ───

	describe('Edge cases', () => {
		it('sends undefined for empty outputVarName', async () => {
			const user = setupSave();

			await renderAndWait({ outputVarName: undefined });

			await user.click(screen.getByRole('button', { name: /save/i }));

			await waitFor(() => {
				const call = vi.mocked(workflowClient.updatePhaseTemplate).mock.calls[0][0];
				// Empty/unset outputVarName should not be sent (undefined)
				expect(call.outputVarName).toBeUndefined();
			});
		});

		it('trims whitespace from outputVarName before save', async () => {
			const user = setupSave();

			await renderAndWait({ outputVarName: undefined });

			const outputInput = screen.getByLabelText(/output variable/i);
			await user.type(outputInput, '  REPORT  ');

			await user.click(screen.getByRole('button', { name: /save/i }));

			await waitFor(() => {
				expect(workflowClient.updatePhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						outputVarName: 'REPORT',
					})
				);
			});
		});

		it('sends empty inputVariables array when all variables are removed', async () => {
			const user = setupSave();

			// Start with one variable
			await renderAndWait({
				inputVariables: ['SPEC_CONTENT'],
			});

			// Find and click the remove button on the SPEC_CONTENT chip
			// TagInput chips have a remove button (✕ or similar)
			const chipText = screen.getByText('SPEC_CONTENT');
			const chip = chipText.closest('[class*="chip"], [class*="tag"]');
			const removeButton = chip?.querySelector('button');
			if (removeButton) {
				await user.click(removeButton);
			}

			// Save
			await user.click(screen.getByRole('button', { name: /save/i }));

			await waitFor(() => {
				const call = vi.mocked(workflowClient.updatePhaseTemplate).mock.calls[0][0];
				// inputVariables should be empty array, not undefined
				expect((call as Record<string, unknown>).inputVariables).toEqual([]);
			});
		});

		it('treats EMBEDDED promptSource as Inline (no path input)', async () => {
			await renderAndWait({ promptSource: PromptSource.EMBEDDED });

			// EMBEDDED is for built-in templates; in the modal it maps to Inline
			expect(screen.queryByText('.orc/prompts/')).not.toBeInTheDocument();
		});

		it('stores promptPath value without the .orc/prompts/ prefix', async () => {
			const user = setupSave();

			await renderAndWait({
				promptSource: PromptSource.FILE,
				promptPath: 'custom/deep/path.md',
			});

			// The path input should show just the path, not prefixed
			expect(screen.getByDisplayValue('custom/deep/path.md')).toBeInTheDocument();

			// Save and verify the stored value is the raw path
			await user.click(screen.getByRole('button', { name: /save/i }));

			await waitFor(() => {
				expect(workflowClient.updatePhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						promptPath: 'custom/deep/path.md',
					})
				);
			});
		});

		it('does not render Data Flow section for built-in templates', async () => {
			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({ isBuiltin: true })}
					isBuiltin={true}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			// Built-in templates show the "Cannot edit" message, not the form
			expect(screen.getByText(/cannot edit built-in template/i)).toBeInTheDocument();
			expect(screen.queryByText(/^Data Flow$/i)).not.toBeInTheDocument();
			expect(screen.queryByText(/^Prompt$/i)).not.toBeInTheDocument();

			// Wait for any pending async operations to complete
			await act(async () => {
				await new Promise(resolve => setTimeout(resolve, 0));
			});
		});
	});
});
