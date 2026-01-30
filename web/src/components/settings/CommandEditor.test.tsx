import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CommandEditor, type EditableCommand } from './CommandEditor';

const mockCommand: EditableCommand = {
	id: 'cmd-1',
	name: '/review-pr',
	path: '.claude/commands/review-pr.md',
	content: `# Review PR

Run a comprehensive code review.

## Steps
- Check for bugs
- Review code style
- Suggest improvements

\`\`\`bash
gh pr view
\`\`\`
`,
};

describe('CommandEditor', () => {
	let onSave: ReturnType<typeof vi.fn>;
	let onCancel: ReturnType<typeof vi.fn>;

	beforeEach(() => {
		onSave = vi.fn().mockResolvedValue(undefined);
		onCancel = vi.fn();
	});

	describe('rendering', () => {
		it('renders command name in header', () => {
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			expect(screen.getByText('/review-pr')).toBeInTheDocument();
		});

		it('renders file path in header', () => {
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			expect(screen.getByText('.claude/commands/review-pr.md')).toBeInTheDocument();
		});

		it('loads initial content into editor', () => {
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			const textarea = screen.getByRole('textbox');
			expect(textarea).toHaveValue(mockCommand.content);
		});

		it('displays Save and Cancel buttons', () => {
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			expect(screen.getByRole('button', { name: /save/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();
		});

		it('displays line numbers', () => {
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			// Content has multiple lines, should see line numbers
			expect(screen.getByText('1')).toBeInTheDocument();
			expect(screen.getByText('2')).toBeInTheDocument();
		});
	});

	describe('dirty state', () => {
		it('does not show dirty indicator when content matches original', () => {
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			expect(screen.queryByText('Unsaved')).not.toBeInTheDocument();
		});

		it('shows dirty indicator when content is modified', async () => {
			const user = userEvent.setup();
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			const textarea = screen.getByRole('textbox');
			await user.type(textarea, 'x');

			expect(screen.getByText('Unsaved')).toBeInTheDocument();
		});

		it('hides dirty indicator when content returns to original', async () => {
			const user = userEvent.setup();
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			const textarea = screen.getByRole('textbox');
			await user.type(textarea, 'x');
			expect(screen.getByText('Unsaved')).toBeInTheDocument();

			// Delete the added character
			await user.type(textarea, '{Backspace}');
			expect(screen.queryByText('Unsaved')).not.toBeInTheDocument();
		});
	});

	describe('save functionality', () => {
		it('Save button calls onSave with current content', async () => {
			const user = userEvent.setup();
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			expect(onSave).toHaveBeenCalledTimes(1);
			expect(onSave).toHaveBeenCalledWith(mockCommand.content);
		});

		it('Save button calls onSave with modified content', async () => {
			const user = userEvent.setup();
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			const textarea = screen.getByRole('textbox');
			await user.clear(textarea);
			await user.type(textarea, 'New content');

			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			expect(onSave).toHaveBeenCalledWith('New content');
		});

		it('Save button is disabled during saving', async () => {
			// Make onSave hang to test loading state
			onSave = vi.fn().mockImplementation(() => new Promise(() => {}));
			const user = userEvent.setup();
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			expect(saveButton).toBeDisabled();
		});

		it('displays error state on save failure', async () => {
			const errorMessage = 'Network error';
			onSave = vi.fn().mockRejectedValue(new Error(errorMessage));
			const user = userEvent.setup();
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			await waitFor(() => {
				expect(screen.getByRole('alert')).toBeInTheDocument();
			});
			expect(screen.getByText(errorMessage)).toBeInTheDocument();
		});
	});

	describe('cancel functionality', () => {
		it('Cancel button calls onCancel', async () => {
			const user = userEvent.setup();
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			const cancelButton = screen.getByRole('button', { name: /cancel/i });
			await user.click(cancelButton);

			expect(onCancel).toHaveBeenCalledTimes(1);
		});

		it('Cancel does not call onSave', async () => {
			const user = userEvent.setup();
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			const cancelButton = screen.getByRole('button', { name: /cancel/i });
			await user.click(cancelButton);

			expect(onSave).not.toHaveBeenCalled();
		});
	});

	describe('keyboard shortcuts', () => {
		it('Ctrl+S triggers onSave when editor focused', async () => {
			const user = userEvent.setup();
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			const textarea = screen.getByRole('textbox');
			textarea.focus();

			await user.keyboard('{Control>}s{/Control}');

			expect(onSave).toHaveBeenCalledTimes(1);
		});

		it('Cmd+S triggers onSave on Mac', async () => {
			const user = userEvent.setup();
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			const textarea = screen.getByRole('textbox');
			textarea.focus();

			await user.keyboard('{Meta>}s{/Meta}');

			expect(onSave).toHaveBeenCalledTimes(1);
		});

		it('Escape triggers onCancel when editor focused', async () => {
			const user = userEvent.setup();
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			const textarea = screen.getByRole('textbox');
			textarea.focus();

			await user.keyboard('{Escape}');

			expect(onCancel).toHaveBeenCalledTimes(1);
		});

		it('keyboard shortcuts work from textarea', async () => {
			const user = userEvent.setup();
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			const textarea = screen.getByRole('textbox');
			textarea.focus();

			// Test Ctrl+S
			await user.keyboard('{Control>}s{/Control}');
			expect(onSave).toHaveBeenCalledTimes(1);

			// Test Escape
			await user.keyboard('{Escape}');
			expect(onCancel).toHaveBeenCalledTimes(1);
		});
	});

	describe('accessibility', () => {
		it('textarea has appropriate aria-label', () => {
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			const textarea = screen.getByRole('textbox');
			expect(textarea).toHaveAttribute('aria-label', 'Edit /review-pr');
		});

		it('Save button has aria-label', () => {
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			expect(screen.getByRole('button', { name: /save command/i })).toBeInTheDocument();
		});

		it('Cancel button has aria-label', () => {
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			expect(
				screen.getByRole('button', { name: /cancel editing/i })
			).toBeInTheDocument();
		});

		it('error message has role="alert"', async () => {
			onSave = vi.fn().mockRejectedValue(new Error('Failed'));
			const user = userEvent.setup();
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			await waitFor(() => {
				expect(screen.getByRole('alert')).toBeInTheDocument();
			});
		});

		it('dirty indicator has aria-label', async () => {
			const user = userEvent.setup();
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			const textarea = screen.getByRole('textbox');
			await user.type(textarea, 'x');

			// The aria-label is on the parent span.editor-dirty, not the text span
			const dirtyIndicator = screen.getByLabelText('Unsaved changes');
			expect(dirtyIndicator).toBeInTheDocument();
			expect(dirtyIndicator).toHaveClass('editor-dirty');
		});
	});

	describe('content editing', () => {
		it('allows typing in textarea', async () => {
			const user = userEvent.setup();
			render(
				<CommandEditor command={mockCommand} onSave={onSave} onCancel={onCancel} />
			);

			const textarea = screen.getByRole('textbox');
			await user.clear(textarea);
			await user.type(textarea, 'Hello World');

			expect(textarea).toHaveValue('Hello World');
		});

		it('updates line count when adding lines', async () => {
			const user = userEvent.setup();
			const shortCommand: EditableCommand = {
				id: 'cmd-short',
				name: '/test',
				path: '.claude/commands/test.md',
				content: 'Line 1',
			};

			render(
				<CommandEditor command={shortCommand} onSave={onSave} onCancel={onCancel} />
			);

			// Initially 1 line
			expect(screen.getByText('1')).toBeInTheDocument();
			expect(screen.queryByText('2')).not.toBeInTheDocument();

			// Add a newline
			const textarea = screen.getByRole('textbox');
			await user.type(textarea, '\nLine 2');

			// Now should have 2 lines
			expect(screen.getByText('2')).toBeInTheDocument();
		});
	});

	describe('syntax highlighting classes', () => {
		it('applies code-comment class to H1 headers', () => {
			const command: EditableCommand = {
				id: 'cmd-h1',
				name: '/test',
				path: '.claude/commands/test.md',
				content: '# Title\nsome text',
			};

			const { container } = render(
				<CommandEditor command={command} onSave={onSave} onCancel={onCancel} />
			);

			const overlay = container.querySelector('.editor-highlight');
			expect(overlay).not.toBeNull();
			expect(overlay!.innerHTML).toContain('<span class="code-comment">');
		});

		it('applies code-key class to H2+ headers', () => {
			const command: EditableCommand = {
				id: 'cmd-h2',
				name: '/test',
				path: '.claude/commands/test.md',
				content: '## Section\n### Subsection',
			};

			const { container } = render(
				<CommandEditor command={command} onSave={onSave} onCancel={onCancel} />
			);

			const overlay = container.querySelector('.editor-highlight');
			expect(overlay).not.toBeNull();
			const html = overlay!.innerHTML;
			// Both ## and ### should use code-key
			const codeKeyMatches = html.match(/<span class="code-key">/g);
			expect(codeKeyMatches).not.toBeNull();
			expect(codeKeyMatches!.length).toBeGreaterThanOrEqual(2);
		});

		it('applies code-string class to inline code', () => {
			const command: EditableCommand = {
				id: 'cmd-code',
				name: '/test',
				path: '.claude/commands/test.md',
				content: 'run `hello` now',
			};

			const { container } = render(
				<CommandEditor command={command} onSave={onSave} onCancel={onCancel} />
			);

			const overlay = container.querySelector('.editor-highlight');
			expect(overlay).not.toBeNull();
			expect(overlay!.innerHTML).toContain('<span class="code-string">');
		});

		it('does not use old md-header class', () => {
			const command: EditableCommand = {
				id: 'cmd-no-md-header',
				name: '/test',
				path: '.claude/commands/test.md',
				content: '# Title\n## Section',
			};

			const { container } = render(
				<CommandEditor command={command} onSave={onSave} onCancel={onCancel} />
			);

			const overlay = container.querySelector('.editor-highlight');
			expect(overlay).not.toBeNull();
			expect(overlay!.innerHTML).not.toContain('md-header');
		});

		it('does not use old md-code class', () => {
			const command: EditableCommand = {
				id: 'cmd-no-md-code',
				name: '/test',
				path: '.claude/commands/test.md',
				content: 'run `hello` now',
			};

			const { container } = render(
				<CommandEditor command={command} onSave={onSave} onCancel={onCancel} />
			);

			const overlay = container.querySelector('.editor-highlight');
			expect(overlay).not.toBeNull();
			expect(overlay!.innerHTML).not.toContain('md-code');
		});
	});
});
