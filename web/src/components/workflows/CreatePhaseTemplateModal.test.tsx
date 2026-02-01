/**
 * TDD Tests for CreatePhaseTemplateModal - Create Phase Template from Scratch
 *
 * Tests for TASK-703: Create 'New Phase Template' modal with prompt editor and data flow
 *
 * Success Criteria Coverage:
 * - SC-2: CreatePhaseTemplateModal opens when "Create From Scratch" is clicked
 * - SC-3: Template ID auto-slugifies from Name field
 * - SC-4: Creating template with required fields succeeds and template appears in list
 * - SC-5: Prompt source toggle switches between Inline and File modes
 * - SC-6: Inline prompt editor renders `{{VARIABLE}}` patterns with visual highlighting
 * - SC-7: Input Variables tag input accepts variable names and shows suggestions
 * - SC-8: Output Variable Name field accepts input and is sent to API
 * - SC-9: Claude Config section renders same 7 collapsible sections as EditPhaseTemplateModal
 * - SC-10: Created template contains all configured values when retrieved via API
 *
 * Failure Modes:
 * - Duplicate template ID → API returns 409 Conflict
 * - Empty ID on submit → Form validation blocks submit
 * - Empty Name on submit → Form validation blocks submit
 * - API save failure → Modal stays open, error shown
 * - Invalid JSON in JSON Override → Field highlighted red
 *
 * Edge Cases:
 * - Name with special characters → ID slugifies correctly
 * - Empty Name field → ID field stays empty
 * - User manually edits ID then edits Name → Manual ID preserved
 * - Prompt with unclosed `{{` → Shows as plain text, no crash
 * - Very long prompt content → Textarea scrollable
 * - Mixed Inline and File (toggle back and forth) → Preserves content for each mode
 * - Create with minimal fields (ID + Name only) → Creates template with defaults
 */

import { describe, it, expect, vi, beforeEach, afterEach, beforeAll } from 'vitest';
import { render, screen, waitFor, cleanup, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CreatePhaseTemplateModal } from './CreatePhaseTemplateModal';
import { create } from '@bufbuild/protobuf';
import { ListHooksResponseSchema, ListSkillsResponseSchema, ListAgentsResponseSchema } from '@/gen/orc/v1/config_pb';
import { ListMCPServersResponseSchema } from '@/gen/orc/v1/mcp_pb';
import { CreatePhaseTemplateResponseSchema, PromptSource } from '@/gen/orc/v1/workflow_pb';
import {
	createMockPhaseTemplate,
	createMockHook,
	createMockSkill,
	createMockMCPServerInfo,
} from '@/test/factories';

// Mock the client module
vi.mock('@/lib/client', () => ({
	workflowClient: {
		createPhaseTemplate: vi.fn(),
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
import { toast } from '@/stores/uiStore';

// Mock browser APIs for Radix components
beforeAll(() => {
	Element.prototype.scrollIntoView = vi.fn();
	Element.prototype.hasPointerCapture = vi.fn().mockReturnValue(false);
	Element.prototype.setPointerCapture = vi.fn();
	Element.prototype.releasePointerCapture = vi.fn();
	global.ResizeObserver = vi.fn().mockImplementation(() => ({
		observe: vi.fn(),
		unobserve: vi.fn(),
		disconnect: vi.fn(),
	}));
	window.confirm = vi.fn().mockReturnValue(true);
});

// Standard mock data for library pickers
const mockHooks = [
	createMockHook({ name: 'pre-guard', eventType: 'PreToolUse' }),
	createMockHook({ name: 'post-log', eventType: 'PostToolUse' }),
];
const mockSkills = [
	createMockSkill({ name: 'python-style', description: 'Python coding standards' }),
	createMockSkill({ name: 'tdd', description: 'TDD workflow' }),
];
const mockMCPServers = [
	createMockMCPServerInfo({ name: 'filesystem', command: 'npx fs-server' }),
];

function setupMocks() {
	vi.mocked(configClient.listAgents).mockResolvedValue(create(ListAgentsResponseSchema, { agents: [] }));
	vi.mocked(configClient.listHooks).mockResolvedValue(create(ListHooksResponseSchema, { hooks: mockHooks }));
	vi.mocked(configClient.listSkills).mockResolvedValue(create(ListSkillsResponseSchema, { skills: mockSkills }));
	vi.mocked(mcpClient.listMCPServers).mockResolvedValue(create(ListMCPServersResponseSchema, { servers: mockMCPServers }));
}

/**
 * Helper to create a successful CreatePhaseTemplateResponse
 */
function createMockCreatePhaseTemplateResponse(overrides: Parameters<typeof createMockPhaseTemplate>[0] = {}) {
	return create(CreatePhaseTemplateResponseSchema, {
		template: createMockPhaseTemplate({
			isBuiltin: false,
			...overrides,
		}),
	});
}

const mockOnClose = vi.fn();
const mockOnCreated = vi.fn();

describe('CreatePhaseTemplateModal', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		setupMocks();
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-2: Modal opens and renders correctly', () => {
		it('renders modal with "Create Phase Template" title when open', async () => {
			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(screen.getByRole('dialog')).toBeInTheDocument();
			});

			expect(screen.getByText('Create Phase Template')).toBeInTheDocument();
		});

		it('does not render when open is false', () => {
			render(
				<CreatePhaseTemplateModal
					open={false}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
		});

		it('renders required form fields: ID, Name, Description', async () => {
			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			expect(screen.getByLabelText(/template id/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/^name/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/description/i)).toBeInTheDocument();
		});

		it('starts with empty form fields', async () => {
			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			const idInput = screen.getByLabelText(/template id/i) as HTMLInputElement;
			const nameInput = screen.getByLabelText(/^name/i) as HTMLInputElement;

			expect(idInput.value).toBe('');
			expect(nameInput.value).toBe('');
		});
	});

	describe('SC-3: Template ID auto-slugifies from Name field', () => {
		it('auto-generates ID from Name when typing', async () => {
			const user = userEvent.setup();

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			const nameInput = screen.getByLabelText(/^name/i);
			await user.type(nameInput, 'My Custom Phase');

			const idInput = screen.getByLabelText(/template id/i) as HTMLInputElement;
			expect(idInput.value).toBe('my-custom-phase');
		});

		it('slugifies special characters correctly', async () => {
			const user = userEvent.setup();

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			const nameInput = screen.getByLabelText(/^name/i);
			await user.type(nameInput, 'Test Phase!@#');

			const idInput = screen.getByLabelText(/template id/i) as HTMLInputElement;
			expect(idInput.value).toBe('test-phase');
		});

		it('produces empty ID when Name is empty', async () => {
			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			const idInput = screen.getByLabelText(/template id/i) as HTMLInputElement;
			expect(idInput.value).toBe('');
		});

		it('preserves manually edited ID when Name changes', async () => {
			const user = userEvent.setup();

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// First type a name to auto-generate ID
			const nameInput = screen.getByLabelText(/^name/i);
			await user.type(nameInput, 'Initial Name');

			// Now manually edit the ID
			const idInput = screen.getByLabelText(/template id/i);
			await user.clear(idInput);
			await user.type(idInput, 'custom-manual-id');

			// Change the name again - ID should NOT change
			await user.clear(nameInput);
			await user.type(nameInput, 'New Name');

			expect((idInput as HTMLInputElement).value).toBe('custom-manual-id');
		});
	});

	describe('SC-4: Creating template with required fields succeeds', () => {
		it('calls createPhaseTemplate API with correct data on submit', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.createPhaseTemplate).mockResolvedValue(
				createMockCreatePhaseTemplateResponse({
					id: 'my-phase',
					name: 'My Phase',
				})
			);

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Fill required fields
			const nameInput = screen.getByLabelText(/^name/i);
			await user.type(nameInput, 'My Phase');

			// Submit the form
			const createButton = screen.getByRole('button', { name: /create/i });
			await user.click(createButton);

			await waitFor(() => {
				expect(workflowClient.createPhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						id: 'my-phase',
						name: 'My Phase',
					})
				);
			});
		});

		it('shows success toast and calls onCreated callback on success', async () => {
			const user = userEvent.setup();

			const createdTemplate = createMockPhaseTemplate({
				id: 'test-phase',
				name: 'Test Phase',
				isBuiltin: false,
			});

			vi.mocked(workflowClient.createPhaseTemplate).mockResolvedValue(
				create(CreatePhaseTemplateResponseSchema, { template: createdTemplate })
			);

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			const nameInput = screen.getByLabelText(/^name/i);
			await user.type(nameInput, 'Test Phase');

			const createButton = screen.getByRole('button', { name: /create/i });
			await user.click(createButton);

			await waitFor(() => {
				expect(toast.success).toHaveBeenCalledWith(
					expect.stringContaining('Phase template created')
				);
				expect(mockOnCreated).toHaveBeenCalledWith(createdTemplate);
			});
		});

		it('closes modal on successful creation', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.createPhaseTemplate).mockResolvedValue(
				createMockCreatePhaseTemplateResponse({ id: 'test', name: 'Test' })
			);

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			const nameInput = screen.getByLabelText(/^name/i);
			await user.type(nameInput, 'Test');

			const createButton = screen.getByRole('button', { name: /create/i });
			await user.click(createButton);

			await waitFor(() => {
				expect(mockOnClose).toHaveBeenCalled();
			});
		});

		it('creates template with minimal required fields (ID + Name)', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.createPhaseTemplate).mockResolvedValue(
				createMockCreatePhaseTemplateResponse({
					id: 'minimal',
					name: 'Minimal',
				})
			);

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			const nameInput = screen.getByLabelText(/^name/i);
			await user.type(nameInput, 'Minimal');

			const createButton = screen.getByRole('button', { name: /create/i });
			await user.click(createButton);

			await waitFor(() => {
				expect(workflowClient.createPhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						id: 'minimal',
						name: 'Minimal',
					})
				);
			});
		});
	});

	describe('SC-5: Prompt source toggle switches between Inline and File modes', () => {
		it('renders Inline and File toggle buttons', async () => {
			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			expect(screen.getByRole('button', { name: /inline/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /file/i })).toBeInTheDocument();
		});

		it('shows code editor when Inline is selected', async () => {
			const user = userEvent.setup();

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Click Inline button
			const inlineButton = screen.getByRole('button', { name: /inline/i });
			await user.click(inlineButton);

			// Should show a textarea/editor for prompt content
			expect(screen.getByLabelText(/prompt content/i) || screen.getByPlaceholderText(/enter your prompt/i)).toBeInTheDocument();
		});

		it('shows file path input with .orc/prompts/ prefix when File is selected', async () => {
			const user = userEvent.setup();

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Click File button
			const fileButton = screen.getByRole('button', { name: /file/i });
			await user.click(fileButton);

			// Should show file path input
			const pathInput = screen.getByLabelText(/prompt path/i) || screen.getByPlaceholderText(/\.orc\/prompts/i);
			expect(pathInput).toBeInTheDocument();
		});

		it('preserves content when toggling between modes', async () => {
			const user = userEvent.setup();

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Select Inline and type content
			const inlineButton = screen.getByRole('button', { name: /inline/i });
			await user.click(inlineButton);

			const promptEditor = screen.getByLabelText(/prompt content/i) || screen.getByPlaceholderText(/enter your prompt/i);
			await user.type(promptEditor, 'My inline prompt');

			// Switch to File mode
			const fileButton = screen.getByRole('button', { name: /file/i });
			await user.click(fileButton);

			// Switch back to Inline mode
			await user.click(inlineButton);

			// Content should be preserved
			const editorAfter = screen.getByLabelText(/prompt content/i) || screen.getByPlaceholderText(/enter your prompt/i);
			expect((editorAfter as HTMLTextAreaElement).value).toContain('My inline prompt');
		});

		it('sends correct prompt_source to API based on toggle', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.createPhaseTemplate).mockResolvedValue(
				createMockCreatePhaseTemplateResponse({ id: 'test', name: 'Test' })
			);

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Fill name
			const nameInput = screen.getByLabelText(/^name/i);
			await user.type(nameInput, 'Test');

			// Select Inline and add content
			const inlineButton = screen.getByRole('button', { name: /inline/i });
			await user.click(inlineButton);

			const promptEditor = screen.getByLabelText(/prompt content/i) || screen.getByPlaceholderText(/enter your prompt/i);
			await user.type(promptEditor, 'Test prompt');

			// Submit
			const createButton = screen.getByRole('button', { name: /create/i });
			await user.click(createButton);

			await waitFor(() => {
				expect(workflowClient.createPhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						promptSource: PromptSource.DB,
						promptContent: 'Test prompt',
					})
				);
			});
		});
	});

	describe('SC-6: Inline prompt editor renders {{VARIABLE}} patterns with visual highlighting', () => {
		it('highlights {{VARIABLE}} patterns in prompt editor', async () => {
			const user = userEvent.setup();

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Select Inline mode
			const inlineButton = screen.getByRole('button', { name: /inline/i });
			await user.click(inlineButton);

			// Type content with variable
			const promptEditor = screen.getByLabelText(/prompt content/i) || screen.getByPlaceholderText(/enter your prompt/i);
			fireEvent.change(promptEditor, { target: { value: 'Analyze {{SPEC_CONTENT}} for issues' } });

			// Look for highlighted variable (either via class or aria-label)
			// The highlight overlay should contain a span with the variable
			// Implementation will add .prompt-editor-highlight or [data-variable-highlight] elements
			const hasHighlight = document.querySelector('.prompt-editor-highlight') ||
				document.querySelector('[data-variable-highlight]') ||
				document.querySelector('.variable-highlight');
			expect(hasHighlight).toBeTruthy();

			// At minimum, the variable text should be present
			expect(screen.getByDisplayValue(/\{\{SPEC_CONTENT\}\}/)).toBeInTheDocument();
		});

		it('shows unclosed {{ as plain text without crash', async () => {
			const user = userEvent.setup();

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Select Inline mode
			const inlineButton = screen.getByRole('button', { name: /inline/i });
			await user.click(inlineButton);

			// Type content with unclosed variable - should not crash
			const promptEditor = screen.getByLabelText(/prompt content/i) || screen.getByPlaceholderText(/enter your prompt/i);
			fireEvent.change(promptEditor, { target: { value: 'Incomplete {{VAR text here' } });

			// Should still render without error
			expect(screen.getByDisplayValue(/Incomplete \{\{VAR text here/)).toBeInTheDocument();
		});

		it('handles very long prompt content without truncation', async () => {
			const user = userEvent.setup();

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Select Inline mode
			const inlineButton = screen.getByRole('button', { name: /inline/i });
			await user.click(inlineButton);

			// Type very long content
			const longContent = 'A'.repeat(10000);
			const promptEditor = screen.getByLabelText(/prompt content/i) || screen.getByPlaceholderText(/enter your prompt/i);
			fireEvent.change(promptEditor, { target: { value: longContent } });

			// Content should not be truncated
			expect((promptEditor as HTMLTextAreaElement).value.length).toBe(10000);
		});
	});

	describe('SC-7: Input Variables tag input accepts variable names and shows suggestions', () => {
		it('renders input variables field', async () => {
			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			expect(screen.getByText(/input variables/i)).toBeInTheDocument();
		});

		it('shows variable suggestions when typing', async () => {
			const user = userEvent.setup();

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Find input variables field and focus it
			const inputVarsField = screen.getByLabelText(/input variables/i) ||
				screen.getByPlaceholderText(/add variable/i);
			await user.click(inputVarsField);

			// Type partial variable name
			await user.type(inputVarsField, 'SPEC');

			// Should show suggestions
			await waitFor(() => {
				expect(screen.getByText(/SPEC_CONTENT/)).toBeInTheDocument();
			});
		});

		it('shows common variable suggestions: SPEC_CONTENT, PROJECT_ROOT, TASK_DESCRIPTION, WORKTREE_PATH', async () => {
			const user = userEvent.setup();

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Find and focus input variables field
			const inputVarsField = screen.getByLabelText(/input variables/i) ||
				screen.getByPlaceholderText(/add variable/i);
			await user.click(inputVarsField);

			// Should show common suggestions
			await waitFor(() => {
				const suggestions = document.querySelectorAll('[role="option"]');
				const suggestionTexts = Array.from(suggestions).map(s => s.textContent);

				expect(suggestionTexts).toEqual(
					expect.arrayContaining([
						expect.stringContaining('SPEC_CONTENT'),
						expect.stringContaining('PROJECT_ROOT'),
						expect.stringContaining('TASK_DESCRIPTION'),
						expect.stringContaining('WORKTREE_PATH'),
					])
				);
			});
		});

		it('accepts custom variable names not in suggestions', async () => {
			const user = userEvent.setup();

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Find input variables field
			const inputVarsField = screen.getByLabelText(/input variables/i) ||
				screen.getByPlaceholderText(/add variable/i);

			// Type custom variable and add it
			await user.type(inputVarsField, 'MY_CUSTOM_VAR{enter}');

			// Should show the custom variable as a tag
			expect(screen.getByText('MY_CUSTOM_VAR')).toBeInTheDocument();
		});
	});

	describe('SC-8: Output Variable Name field accepts input and is sent to API', () => {
		it('renders output variable name field', async () => {
			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			expect(screen.getByLabelText(/output variable/i)).toBeInTheDocument();
		});

		it('sends output_var_name to API when filled', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.createPhaseTemplate).mockResolvedValue(
				createMockCreatePhaseTemplateResponse({
					id: 'test',
					name: 'Test',
					outputVarName: 'MY_OUTPUT',
				})
			);

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Fill required fields
			const nameInput = screen.getByLabelText(/^name/i);
			await user.type(nameInput, 'Test');

			// Fill output variable name
			const outputVarInput = screen.getByLabelText(/output variable/i);
			await user.type(outputVarInput, 'MY_OUTPUT');

			// Submit
			const createButton = screen.getByRole('button', { name: /create/i });
			await user.click(createButton);

			await waitFor(() => {
				expect(workflowClient.createPhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						outputVarName: 'MY_OUTPUT',
					})
				);
			});
		});

		it('omits output_var_name from API call when empty', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.createPhaseTemplate).mockResolvedValue(
				createMockCreatePhaseTemplateResponse({ id: 'test', name: 'Test' })
			);

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Fill only required fields
			const nameInput = screen.getByLabelText(/^name/i);
			await user.type(nameInput, 'Test');

			// Submit without filling output variable
			const createButton = screen.getByRole('button', { name: /create/i });
			await user.click(createButton);

			await waitFor(() => {
				const call = vi.mocked(workflowClient.createPhaseTemplate).mock.calls[0][0];
				expect(call.outputVarName).toBeFalsy();
			});
		});
	});

	describe('SC-9: Claude Config section renders same 7 collapsible sections as EditPhaseTemplateModal', () => {
		it('renders all 7 Claude Config section headers', async () => {
			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// All 7 section headers should be visible
			expect(screen.getByText(/^Hooks$/i)).toBeInTheDocument();
			expect(screen.getByText(/^MCP Servers$/i)).toBeInTheDocument();
			expect(screen.getByText(/^Skills$/i)).toBeInTheDocument();
			expect(screen.getByText(/^Allowed Tools$/i)).toBeInTheDocument();
			expect(screen.getByText(/^Disallowed Tools$/i)).toBeInTheDocument();
			expect(screen.getByText(/^Env Vars$/i)).toBeInTheDocument();
			expect(screen.getByText(/^JSON Override$/i)).toBeInTheDocument();
		});

		it('fetches hooks, skills, and MCP servers on mount', async () => {
			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listHooks).toHaveBeenCalled();
				expect(configClient.listSkills).toHaveBeenCalled();
				expect(mcpClient.listMCPServers).toHaveBeenCalled();
			});
		});

		it('shows badge count 0 for all sections initially', async () => {
			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// All sections should start with 0 items (badges showing 0)
			const zeroBadges = screen.getAllByText('0');
			expect(zeroBadges.length).toBeGreaterThanOrEqual(1);
		});
	});

	describe('SC-10: Created template contains all configured values', () => {
		it('sends all configured values to API including claude_config', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.createPhaseTemplate).mockResolvedValue(
				createMockCreatePhaseTemplateResponse({
					id: 'full-config',
					name: 'Full Config',
				})
			);

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Fill all fields
			const nameInput = screen.getByLabelText(/^name/i);
			await user.type(nameInput, 'Full Config');

			const descInput = screen.getByLabelText(/description/i);
			await user.type(descInput, 'A fully configured template');

			// Set prompt content
			const inlineButton = screen.getByRole('button', { name: /inline/i });
			await user.click(inlineButton);
			const promptEditor = screen.getByLabelText(/prompt content/i) || screen.getByPlaceholderText(/enter your prompt/i);
			fireEvent.change(promptEditor, { target: { value: 'Test prompt content' } });

			// Set output variable
			const outputVarInput = screen.getByLabelText(/output variable/i);
			await user.type(outputVarInput, 'OUTPUT_VAR');

			// Submit
			const createButton = screen.getByRole('button', { name: /create/i });
			await user.click(createButton);

			await waitFor(() => {
				expect(workflowClient.createPhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						id: 'full-config',
						name: 'Full Config',
						description: 'A fully configured template',
						promptSource: PromptSource.DB,
						promptContent: 'Test prompt content',
						outputVarName: 'OUTPUT_VAR',
					})
				);
			});
		});
	});

	describe('Failure Modes', () => {
		it('validates required ID field - shows error when empty', async () => {
			const user = userEvent.setup();

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Try to submit without filling anything
			const createButton = screen.getByRole('button', { name: /create/i });
			await user.click(createButton);

			// Should show validation error or button should be disabled
			// Check that API was NOT called
			expect(workflowClient.createPhaseTemplate).not.toHaveBeenCalled();
		});

		it('validates required Name field - shows error when empty', async () => {
			const user = userEvent.setup();

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Only fill ID, not name
			const idInput = screen.getByLabelText(/template id/i);
			await user.type(idInput, 'test-id');

			const createButton = screen.getByRole('button', { name: /create/i });
			await user.click(createButton);

			// Should show validation error or API should not be called
			expect(workflowClient.createPhaseTemplate).not.toHaveBeenCalled();
		});

		it('shows error toast for duplicate template ID (409 Conflict)', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.createPhaseTemplate).mockRejectedValue(
				new Error('Phase template test-id already exists')
			);

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			const nameInput = screen.getByLabelText(/^name/i);
			await user.type(nameInput, 'Test ID');

			const createButton = screen.getByRole('button', { name: /create/i });
			await user.click(createButton);

			await waitFor(() => {
				expect(toast.error).toHaveBeenCalledWith(
					expect.stringContaining('already exists')
				);
			});
		});

		it('shows error toast and keeps modal open on API failure', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.createPhaseTemplate).mockRejectedValue(
				new Error('Network error')
			);

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			const nameInput = screen.getByLabelText(/^name/i);
			await user.type(nameInput, 'Test');

			const createButton = screen.getByRole('button', { name: /create/i });
			await user.click(createButton);

			await waitFor(() => {
				expect(toast.error).toHaveBeenCalledWith(
					expect.stringContaining('Failed')
				);
			});

			// Modal should stay open
			expect(screen.getByRole('dialog')).toBeInTheDocument();
			expect(mockOnClose).not.toHaveBeenCalled();
		});

		it('shows validation error for invalid JSON in JSON Override', async () => {
			const user = userEvent.setup();

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Expand JSON Override section
			const jsonHeader = screen.getByText(/json override/i);
			await user.click(jsonHeader);

			// Enter invalid JSON
			const jsonTextarea = screen.getByRole('textbox', { name: /json/i });
			fireEvent.change(jsonTextarea, { target: { value: '{invalid json' } });
			fireEvent.blur(jsonTextarea);

			// Should show "Invalid JSON" error
			await waitFor(() => {
				const errorEl = document.querySelector('.create-template-json-error') ||
					screen.queryByText(/invalid json/i);
				expect(errorEl).toBeTruthy();
			});
		});

		it('handles network errors during library fetch gracefully', async () => {
			vi.mocked(configClient.listHooks).mockRejectedValue(new Error('API error'));

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Other sections should still render
			expect(screen.getByText(/^Skills$/i)).toBeInTheDocument();
			expect(screen.getByText(/^Allowed Tools$/i)).toBeInTheDocument();
		});
	});

	describe('Execution Section', () => {
		it('renders agent dropdown', async () => {
			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			expect(screen.getByLabelText(/agent/i) || screen.getByText(/agent/i)).toBeInTheDocument();
		});

		it('renders gate type selector', async () => {
			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			expect(screen.getByLabelText(/gate type/i) || screen.getByText(/gate type/i)).toBeInTheDocument();
		});

		it('renders max iterations field', async () => {
			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			expect(screen.getByLabelText(/max iterations/i) || screen.getByText(/max iterations/i)).toBeInTheDocument();
		});

		it('renders thinking toggle', async () => {
			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			expect(screen.getByLabelText(/thinking/i) || screen.getByText(/thinking/i)).toBeInTheDocument();
		});

		it('renders checkpoint toggle', async () => {
			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			expect(screen.getByLabelText(/checkpoint/i) || screen.getByText(/checkpoint/i)).toBeInTheDocument();
		});

		it('sends execution settings to API', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.createPhaseTemplate).mockResolvedValue(
				createMockCreatePhaseTemplateResponse({
					id: 'with-execution',
					name: 'With Execution',
					maxIterations: 5,
					checkpoint: true,
				})
			);

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Fill name
			const nameInput = screen.getByLabelText(/^name/i);
			await user.type(nameInput, 'With Execution');

			// Set max iterations
			const maxIterInput = screen.getByLabelText(/max iterations/i) as HTMLInputElement;
			await user.clear(maxIterInput);
			await user.type(maxIterInput, '5');

			// Toggle checkpoint
			const checkpointToggle = screen.getByLabelText(/checkpoint/i);
			await user.click(checkpointToggle);

			// Submit
			const createButton = screen.getByRole('button', { name: /create/i });
			await user.click(createButton);

			await waitFor(() => {
				expect(workflowClient.createPhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						maxIterations: 5,
						checkpoint: true,
					})
				);
			});
		});
	});

	describe('Modal behavior', () => {
		it('calls onClose when Cancel button is clicked', async () => {
			const user = userEvent.setup();

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			const cancelButton = screen.getByRole('button', { name: /cancel/i });
			await user.click(cancelButton);

			expect(mockOnClose).toHaveBeenCalled();
		});

		it('disables Create button while saving', async () => {
			const user = userEvent.setup();

			// Make API call hang
			vi.mocked(workflowClient.createPhaseTemplate).mockImplementation(
				() => new Promise(() => { /* never resolves */ })
			);

			render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			const nameInput = screen.getByLabelText(/^name/i);
			await user.type(nameInput, 'Test');

			const createButton = screen.getByRole('button', { name: /create/i });
			await user.click(createButton);

			// Button should be disabled while saving
			await waitFor(() => {
				expect(createButton).toBeDisabled();
			});
		});

		it('resets form when modal is reopened', async () => {
			const user = userEvent.setup();

			const { rerender } = render(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Fill some fields
			const nameInput = screen.getByLabelText(/^name/i);
			await user.type(nameInput, 'Test Name');

			// Close modal
			rerender(
				<CreatePhaseTemplateModal
					open={false}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			// Reopen modal
			rerender(
				<CreatePhaseTemplateModal
					open={true}
					onClose={mockOnClose}
					onCreated={mockOnCreated}
				/>
			);

			await waitFor(() => {
				const newNameInput = screen.getByLabelText(/^name/i) as HTMLInputElement;
				expect(newNameInput.value).toBe('');
			});
		});
	});
});
