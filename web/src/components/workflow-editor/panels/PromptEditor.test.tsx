/**
 * TDD Tests for PromptEditor component
 *
 * Tests for TASK-638: Prompt tab content with variable highlighting and debounced save
 *
 * Success Criteria Coverage:
 * - SC-5: Prompt tab fetches/displays prompt content based on PromptSource type
 * - SC-6: Template variables ({{VAR}}) highlighted as colored inline badges
 * - SC-11: Prompt editable for custom templates, read-only for built-in
 * - SC-12: Prompt edits debounce-save (500ms) via updatePhaseTemplate API
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PromptEditor } from './PromptEditor';
import { PromptSource } from '@/gen/orc/v1/workflow_pb';

// Mock the API client
vi.mock('@/lib/client', () => ({
	workflowClient: {
		getPromptContent: vi.fn(),
		updatePhaseTemplate: vi.fn(),
	},
}));

import { workflowClient } from '@/lib/client';

// ─── Tests ──────────────────────────────────────────────────────────────────

describe('PromptEditor', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		vi.useFakeTimers({ shouldAdvanceTime: true });
	});

	afterEach(() => {
		vi.useRealTimers();
		cleanup();
	});

	// ─── SC-5: Fetch and display prompt content ─────────────────────────────

	describe('SC-5: prompt content display', () => {
		it('displays promptContent directly for EMBEDDED source', async () => {
			render(
				<PromptEditor
					phaseTemplateId="spec"
					promptSource={PromptSource.EMBEDDED}
					promptContent="You are a spec writer. Generate a specification."
					readOnly={true}
				/>,
			);

			expect(
				screen.getByText(/you are a spec writer/i),
			).toBeInTheDocument();
		});

		it('displays promptContent directly for DB source', () => {
			render(
				<PromptEditor
					phaseTemplateId="custom-phase"
					promptSource={PromptSource.DB}
					promptContent="Custom prompt from database."
					readOnly={true}
				/>,
			);

			expect(
				screen.getByText(/custom prompt from database/i),
			).toBeInTheDocument();
		});

		it('calls getPromptContent API for FILE source', async () => {
			vi.mocked(workflowClient.getPromptContent).mockResolvedValue({
				content: 'Loaded from file: implement phase prompt.',
				source: PromptSource.FILE,
			} as any);

			render(
				<PromptEditor
					phaseTemplateId="implement"
					promptSource={PromptSource.FILE}
					readOnly={true}
				/>,
			);

			await waitFor(() => {
				expect(workflowClient.getPromptContent).toHaveBeenCalledWith(
					expect.objectContaining({
						phaseTemplateId: 'implement',
					}),
				);
			});

			await waitFor(() => {
				expect(
					screen.getByText(/loaded from file/i),
				).toBeInTheDocument();
			});
		});

		it('shows loading state during FILE prompt fetch', () => {
			// Never resolve the promise to keep loading state
			vi.mocked(workflowClient.getPromptContent).mockReturnValue(
				new Promise(() => {}),
			);

			render(
				<PromptEditor
					phaseTemplateId="implement"
					promptSource={PromptSource.FILE}
					readOnly={true}
				/>,
			);

			// Should show a loading indicator
			expect(
				screen.getByText(/loading/i) ||
				document.querySelector('.prompt-editor-loading'),
			).toBeTruthy();
		});

		it('shows error message with retry button when API fetch fails', async () => {
			vi.mocked(workflowClient.getPromptContent).mockRejectedValue(
				new Error('Network error'),
			);

			render(
				<PromptEditor
					phaseTemplateId="implement"
					promptSource={PromptSource.FILE}
					readOnly={true}
				/>,
			);

			await waitFor(() => {
				expect(
					screen.getByText(/failed to load prompt/i),
				).toBeInTheDocument();
			});

			expect(
				screen.getByRole('button', { name: /retry/i }),
			).toBeInTheDocument();
		});

		it('retries fetch when retry button is clicked after error', async () => {
			const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

			vi.mocked(workflowClient.getPromptContent)
				.mockRejectedValueOnce(new Error('Network error'))
				.mockResolvedValueOnce({
					content: 'Successfully loaded on retry.',
					source: PromptSource.FILE,
				} as any);

			render(
				<PromptEditor
					phaseTemplateId="implement"
					promptSource={PromptSource.FILE}
					readOnly={true}
				/>,
			);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
			});

			await user.click(screen.getByRole('button', { name: /retry/i }));

			await waitFor(() => {
				expect(workflowClient.getPromptContent).toHaveBeenCalledTimes(2);
			});

			await waitFor(() => {
				expect(
					screen.getByText(/successfully loaded on retry/i),
				).toBeInTheDocument();
			});
		});

		it('shows empty state when prompt content is empty', () => {
			render(
				<PromptEditor
					phaseTemplateId="empty-phase"
					promptSource={PromptSource.EMBEDDED}
					promptContent=""
					readOnly={true}
				/>,
			);

			expect(
				screen.getByText(/no prompt content/i),
			).toBeInTheDocument();
		});
	});

	// ─── SC-6: Template variable highlighting ───────────────────────────────

	describe('SC-6: variable badge highlighting', () => {
		it('highlights {{VARIABLE_NAME}} patterns as badges', () => {
			render(
				<PromptEditor
					phaseTemplateId="spec"
					promptSource={PromptSource.EMBEDDED}
					promptContent="Generate a spec for {{TASK_DESCRIPTION}} using {{CONSTITUTION_CONTENT}}."
					readOnly={true}
				/>,
			);

			// Variables should be rendered as badge elements
			const badges = screen.getAllByTestId('variable-badge');
			expect(badges).toHaveLength(2);
			expect(badges[0]).toHaveTextContent('TASK_DESCRIPTION');
			expect(badges[1]).toHaveTextContent('CONSTITUTION_CONTENT');
		});

		it('renders plain text between variable badges correctly', () => {
			render(
				<PromptEditor
					phaseTemplateId="spec"
					promptSource={PromptSource.EMBEDDED}
					promptContent="Start {{FOO}} middle {{BAR}} end"
					readOnly={true}
				/>,
			);

			expect(screen.getByText(/^Start\s*$/)).toBeInTheDocument();
			expect(screen.getByText(/middle/)).toBeInTheDocument();
			expect(screen.getByText(/end$/)).toBeInTheDocument();
		});

		it('renders malformed patterns like {{ without closing }} as plain text', () => {
			render(
				<PromptEditor
					phaseTemplateId="spec"
					promptSource={PromptSource.EMBEDDED}
					promptContent="This has {{ incomplete and {{VALID_VAR}} here."
					readOnly={true}
				/>,
			);

			// Only one badge for VALID_VAR
			const badges = screen.getAllByTestId('variable-badge');
			expect(badges).toHaveLength(1);
			expect(badges[0]).toHaveTextContent('VALID_VAR');

			// The malformed {{ should be plain text
			expect(screen.getByText(/\{\{ incomplete/)).toBeInTheDocument();
		});

		it('handles adjacent variable badges correctly', () => {
			render(
				<PromptEditor
					phaseTemplateId="spec"
					promptSource={PromptSource.EMBEDDED}
					promptContent="{{FOO}}{{BAR}}"
					readOnly={true}
				/>,
			);

			const badges = screen.getAllByTestId('variable-badge');
			expect(badges).toHaveLength(2);
			expect(badges[0]).toHaveTextContent('FOO');
			expect(badges[1]).toHaveTextContent('BAR');
		});

		it('does not highlight {literal} single-brace patterns', () => {
			render(
				<PromptEditor
					phaseTemplateId="spec"
					promptSource={PromptSource.EMBEDDED}
					promptContent="JSON example: {literal} and {{REAL_VAR}} here"
					readOnly={true}
				/>,
			);

			const badges = screen.getAllByTestId('variable-badge');
			expect(badges).toHaveLength(1);
			expect(badges[0]).toHaveTextContent('REAL_VAR');
		});

		it('highlights variables in prompt with no other text', () => {
			render(
				<PromptEditor
					phaseTemplateId="spec"
					promptSource={PromptSource.EMBEDDED}
					promptContent="{{ONLY_VAR}}"
					readOnly={true}
				/>,
			);

			const badges = screen.getAllByTestId('variable-badge');
			expect(badges).toHaveLength(1);
			expect(badges[0]).toHaveTextContent('ONLY_VAR');
		});
	});

	// ─── SC-11: Editable for custom, read-only for built-in ─────────────────

	describe('SC-11: editability', () => {
		it('renders editable textarea for custom phase templates', () => {
			render(
				<PromptEditor
					phaseTemplateId="custom-phase"
					promptSource={PromptSource.DB}
					promptContent="Editable prompt content"
					readOnly={false}
				/>,
			);

			const textarea = screen.getByRole('textbox');
			expect(textarea).not.toBeDisabled();
			expect(textarea).toHaveValue('Editable prompt content');
		});

		it('renders read-only display for built-in phase templates', () => {
			render(
				<PromptEditor
					phaseTemplateId="implement"
					promptSource={PromptSource.EMBEDDED}
					promptContent="Built-in prompt content"
					readOnly={true}
				/>,
			);

			// Should NOT have an editable textarea
			expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
		});

		it('shows "Clone Template" button for built-in templates', () => {
			render(
				<PromptEditor
					phaseTemplateId="implement"
					promptSource={PromptSource.EMBEDDED}
					promptContent="Built-in prompt"
					readOnly={true}
				/>,
			);

			expect(
				screen.getByRole('button', { name: /clone template/i }),
			).toBeInTheDocument();
		});
	});

	// ─── SC-12: Debounced save ──────────────────────────────────────────────

	describe('SC-12: debounced save', () => {
		it('does not call API immediately on keystroke', async () => {
			const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

			render(
				<PromptEditor
					phaseTemplateId="custom-phase"
					promptSource={PromptSource.DB}
					promptContent="Original content"
					readOnly={false}
				/>,
			);

			const textarea = screen.getByRole('textbox');
			await user.type(textarea, ' updated');

			// Should NOT have called API yet (within 500ms debounce)
			expect(workflowClient.updatePhaseTemplate).not.toHaveBeenCalled();
		});

		it('calls updatePhaseTemplate after 500ms idle', async () => {
			const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

			vi.mocked(workflowClient.updatePhaseTemplate).mockResolvedValue({
				template: {},
			} as any);

			render(
				<PromptEditor
					phaseTemplateId="custom-phase"
					promptSource={PromptSource.DB}
					promptContent="Original"
					readOnly={false}
				/>,
			);

			const textarea = screen.getByRole('textbox');
			await user.clear(textarea);
			await user.type(textarea, 'Updated content');

			// Advance past debounce period
			await act(async () => {
				vi.advanceTimersByTime(600);
			});

			await waitFor(() => {
				expect(workflowClient.updatePhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						id: 'custom-phase',
						promptContent: 'Updated content',
					}),
				);
			});
		});

		it('only makes single API call for rapid keystrokes', async () => {
			const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

			vi.mocked(workflowClient.updatePhaseTemplate).mockResolvedValue({
				template: {},
			} as any);

			render(
				<PromptEditor
					phaseTemplateId="custom-phase"
					promptSource={PromptSource.DB}
					promptContent=""
					readOnly={false}
				/>,
			);

			const textarea = screen.getByRole('textbox');

			// Type multiple characters rapidly
			await user.type(textarea, 'a');
			await act(async () => { vi.advanceTimersByTime(100); });
			await user.type(textarea, 'b');
			await act(async () => { vi.advanceTimersByTime(100); });
			await user.type(textarea, 'c');

			// Advance past debounce
			await act(async () => { vi.advanceTimersByTime(600); });

			await waitFor(() => {
				// Should have been called exactly once (debounced)
				expect(workflowClient.updatePhaseTemplate).toHaveBeenCalledTimes(1);
			});
		});

		it('shows save indicator after successful save', async () => {
			const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

			vi.mocked(workflowClient.updatePhaseTemplate).mockResolvedValue({
				template: {},
			} as any);

			render(
				<PromptEditor
					phaseTemplateId="custom-phase"
					promptSource={PromptSource.DB}
					promptContent="Original"
					readOnly={false}
				/>,
			);

			const textarea = screen.getByRole('textbox');
			await user.type(textarea, ' edited');

			await act(async () => { vi.advanceTimersByTime(600); });

			await waitFor(() => {
				expect(screen.getByText(/saved/i)).toBeInTheDocument();
			});
		});

		it('shows error indicator on save failure without losing edits', async () => {
			const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

			vi.mocked(workflowClient.updatePhaseTemplate).mockRejectedValue(
				new Error('Save failed'),
			);

			render(
				<PromptEditor
					phaseTemplateId="custom-phase"
					promptSource={PromptSource.DB}
					promptContent="Original"
					readOnly={false}
				/>,
			);

			const textarea = screen.getByRole('textbox');
			await user.clear(textarea);
			await user.type(textarea, 'My precious edits');

			await act(async () => { vi.advanceTimersByTime(600); });

			await waitFor(() => {
				expect(screen.getByText(/save failed|error/i)).toBeInTheDocument();
			});

			// User's edits should NOT be lost
			expect(textarea).toHaveValue('My precious edits');
		});

		it('shows "Saving..." indicator during save', async () => {
			const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

			// Return a promise that never resolves to keep "saving" state
			vi.mocked(workflowClient.updatePhaseTemplate).mockReturnValue(
				new Promise(() => {}),
			);

			render(
				<PromptEditor
					phaseTemplateId="custom-phase"
					promptSource={PromptSource.DB}
					promptContent="Original"
					readOnly={false}
				/>,
			);

			const textarea = screen.getByRole('textbox');
			await user.type(textarea, ' more');

			await act(async () => { vi.advanceTimersByTime(600); });

			await waitFor(() => {
				expect(screen.getByText(/saving/i)).toBeInTheDocument();
			});
		});
	});

	// ─── Edge Cases ─────────────────────────────────────────────────────────

	describe('edge cases', () => {
		it('renders prompt content with >5000 characters in scrollable container', () => {
			const longContent = 'A'.repeat(6000);

			const { container } = render(
				<PromptEditor
					phaseTemplateId="spec"
					promptSource={PromptSource.EMBEDDED}
					promptContent={longContent}
					readOnly={true}
				/>,
			);

			// Content should exist (rendered in scrollable container)
			expect(container.textContent).toContain('A'.repeat(100));
		});

		it('cancels fetch when phaseTemplateId changes (mounted guard)', async () => {
			// First render fetches for 'implement'
			vi.mocked(workflowClient.getPromptContent).mockResolvedValueOnce({
				content: 'Old content',
				source: PromptSource.FILE,
			} as any);

			const { rerender } = render(
				<PromptEditor
					phaseTemplateId="implement"
					promptSource={PromptSource.FILE}
					readOnly={true}
				/>,
			);

			// Immediately switch to different phase before first fetch completes
			vi.mocked(workflowClient.getPromptContent).mockResolvedValueOnce({
				content: 'New content for review phase',
				source: PromptSource.FILE,
			} as any);

			rerender(
				<PromptEditor
					phaseTemplateId="review"
					promptSource={PromptSource.FILE}
					readOnly={true}
				/>,
			);

			await waitFor(() => {
				// Should only show the latest phase's content, not the old one
				expect(
					screen.queryByText('Old content'),
				).not.toBeInTheDocument();
			});
		});

		it('handles prompt with variable not matching any input variable', () => {
			// Variable in prompt text that isn't in inputVariables — still highlighted
			render(
				<PromptEditor
					phaseTemplateId="spec"
					promptSource={PromptSource.EMBEDDED}
					promptContent="Use {{UNKNOWN_VAR}} in this prompt"
					readOnly={true}
				/>,
			);

			const badges = screen.getAllByTestId('variable-badge');
			expect(badges).toHaveLength(1);
			expect(badges[0]).toHaveTextContent('UNKNOWN_VAR');
		});
	});
});
