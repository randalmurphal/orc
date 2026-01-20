import { describe, it, expect, vi } from 'vitest';
import { useState } from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { ConfigEditor, type ConfigEditorProps } from './ConfigEditor';

const defaultProps: ConfigEditorProps = {
	filePath: '.claude/commands/review.md',
	content: '# Code Review\n\nReview the code.',
	onChange: vi.fn(),
	onSave: vi.fn(),
	language: 'markdown',
};

function renderConfigEditor(props: Partial<ConfigEditorProps> = {}) {
	return render(<ConfigEditor {...defaultProps} {...props} />);
}

describe('ConfigEditor', () => {
	describe('rendering', () => {
		it('renders with file path in header', () => {
			renderConfigEditor();

			expect(screen.getByTestId('config-editor-path')).toHaveTextContent(
				'.claude/commands/review.md'
			);
		});

		it('renders Save button', () => {
			renderConfigEditor();

			const saveButton = screen.getByTestId('config-editor-save');
			expect(saveButton).toBeInTheDocument();
			expect(saveButton).toHaveTextContent('Save');
		});

		it('renders textarea with content', () => {
			renderConfigEditor();

			const textarea = screen.getByTestId('config-editor-textarea');
			expect(textarea).toBeInTheDocument();
			expect(textarea).toHaveValue('# Code Review\n\nReview the code.');
		});

		it('renders with aria-label for accessibility', () => {
			renderConfigEditor();

			const textarea = screen.getByTestId('config-editor-textarea');
			expect(textarea).toHaveAttribute('aria-label', 'Edit .claude/commands/review.md');
		});

		it('handles empty content gracefully', () => {
			renderConfigEditor({ content: '' });

			const textarea = screen.getByTestId('config-editor-textarea');
			expect(textarea).toBeInTheDocument();
			expect(textarea).toHaveValue('');
		});
	});

	describe('save functionality', () => {
		it('Save button calls onSave when clicked', () => {
			const handleSave = vi.fn();
			renderConfigEditor({ onSave: handleSave });

			const saveButton = screen.getByTestId('config-editor-save');
			fireEvent.click(saveButton);

			expect(handleSave).toHaveBeenCalledTimes(1);
		});

		it('Save button has aria-label', () => {
			renderConfigEditor();

			const saveButton = screen.getByTestId('config-editor-save');
			expect(saveButton).toHaveAttribute('aria-label', 'Save changes');
		});
	});

	describe('editing functionality', () => {
		it('content area is editable and calls onChange', () => {
			const handleChange = vi.fn();
			renderConfigEditor({ onChange: handleChange });

			const textarea = screen.getByTestId('config-editor-textarea');
			fireEvent.change(textarea, { target: { value: 'New content' } });

			expect(handleChange).toHaveBeenCalledTimes(1);
			expect(handleChange).toHaveBeenCalledWith('New content');
		});

		it('handles keyboard input correctly', () => {
			const handleChange = vi.fn();
			renderConfigEditor({
				content: 'Hello',
				onChange: handleChange,
			});

			const textarea = screen.getByTestId('config-editor-textarea');
			fireEvent.change(textarea, { target: { value: 'Hello World' } });

			expect(handleChange).toHaveBeenCalledWith('Hello World');
		});
	});

	describe('unsaved indicator', () => {
		it('unsaved indicator appears when content changes from initial value', () => {
			// Create a controlled wrapper that simulates parent state management
			const TestWrapper = () => {
				const [content, setContent] = useState('Original content');
				return (
					<ConfigEditor
						{...defaultProps}
						content={content}
						onChange={setContent}
						onSave={vi.fn()}
					/>
				);
			};

			render(<TestWrapper />);

			// Initially no unsaved indicator (content matches initial)
			expect(screen.queryByTestId('config-editor-unsaved')).not.toBeInTheDocument();

			// Simulate user typing in the textarea
			const textarea = screen.getByTestId('config-editor-textarea');
			fireEvent.change(textarea, { target: { value: 'Modified content' } });

			// Now unsaved indicator should appear
			expect(screen.getByTestId('config-editor-unsaved')).toBeInTheDocument();
			expect(screen.getByTestId('config-editor-unsaved')).toHaveTextContent('Modified');
		});

		it('unsaved indicator has correct aria-label', () => {
			const TestWrapper = () => {
				const [content, setContent] = useState('Original');
				return (
					<ConfigEditor
						{...defaultProps}
						content={content}
						onChange={setContent}
						onSave={vi.fn()}
					/>
				);
			};

			render(<TestWrapper />);

			// Change content
			const textarea = screen.getByTestId('config-editor-textarea');
			fireEvent.change(textarea, { target: { value: 'Changed' } });

			const unsavedIndicator = screen.getByTestId('config-editor-unsaved');
			expect(unsavedIndicator).toHaveAttribute('aria-label', 'Unsaved changes');
		});
	});

	describe('syntax highlighting', () => {
		it('applies correct highlighting classes for markdown', () => {
			const { container } = renderConfigEditor({
				content: '# Header\n\nSome text',
				language: 'markdown',
			});

			const highlightDiv = container.querySelector('.config-editor-highlight');
			expect(highlightDiv).toBeInTheDocument();
			// Check that the highlight div contains highlighted content
			expect(highlightDiv?.innerHTML).toContain('code-key');
		});

		it('applies correct highlighting classes for yaml', () => {
			const { container } = renderConfigEditor({
				content: 'name: test\n# comment',
				language: 'yaml',
			});

			const highlightDiv = container.querySelector('.config-editor-highlight');
			expect(highlightDiv).toBeInTheDocument();
			expect(highlightDiv?.innerHTML).toContain('code-key');
			expect(highlightDiv?.innerHTML).toContain('code-comment');
		});

		it('applies correct highlighting classes for json', () => {
			const { container } = renderConfigEditor({
				content: '{"key": "value"}',
				language: 'json',
			});

			const highlightDiv = container.querySelector('.config-editor-highlight');
			expect(highlightDiv).toBeInTheDocument();
			expect(highlightDiv?.innerHTML).toContain('code-key');
			expect(highlightDiv?.innerHTML).toContain('code-string');
		});

		it('defaults to markdown when no language specified', () => {
			const { container } = renderConfigEditor({
				content: '## Title',
				language: undefined,
			});

			const highlightDiv = container.querySelector('.config-editor-highlight');
			expect(highlightDiv?.innerHTML).toContain('code-key');
		});
	});

	describe('keyboard shortcuts', () => {
		it('Ctrl+S triggers onSave', () => {
			const handleSave = vi.fn();
			renderConfigEditor({ onSave: handleSave });

			const textarea = screen.getByTestId('config-editor-textarea');
			fireEvent.keyDown(textarea, { key: 's', ctrlKey: true });

			expect(handleSave).toHaveBeenCalledTimes(1);
		});

		it('Cmd+S triggers onSave on Mac', () => {
			const handleSave = vi.fn();
			renderConfigEditor({ onSave: handleSave });

			const textarea = screen.getByTestId('config-editor-textarea');
			fireEvent.keyDown(textarea, { key: 's', metaKey: true });

			expect(handleSave).toHaveBeenCalledTimes(1);
		});

		it('Tab key inserts tab character', () => {
			const handleChange = vi.fn();
			renderConfigEditor({
				content: 'test',
				onChange: handleChange,
			});

			const textarea = screen.getByTestId(
				'config-editor-textarea'
			) as HTMLTextAreaElement;
			textarea.setSelectionRange(4, 4);
			fireEvent.keyDown(textarea, { key: 'Tab' });

			expect(handleChange).toHaveBeenCalledWith('test\t');
		});
	});

	describe('scrolling', () => {
		it('content area has scroll support via CSS class', () => {
			const { container } = renderConfigEditor({
				content: 'Line 1\n'.repeat(100),
			});

			const contentArea = container.querySelector('.config-editor-content');
			expect(contentArea).toBeInTheDocument();

			// Verify the textarea exists with the correct class (CSS handles overflow)
			const textarea = container.querySelector('.config-editor-textarea');
			expect(textarea).toBeInTheDocument();
			expect(textarea).toHaveClass('config-editor-textarea');
		});
	});

	describe('accessibility', () => {
		it('has proper structure with data-testid attributes', () => {
			renderConfigEditor();

			expect(screen.getByTestId('config-editor')).toBeInTheDocument();
			expect(screen.getByTestId('config-editor-path')).toBeInTheDocument();
			expect(screen.getByTestId('config-editor-textarea')).toBeInTheDocument();
			expect(screen.getByTestId('config-editor-save')).toBeInTheDocument();
		});

		it('highlight layer is aria-hidden', () => {
			const { container } = renderConfigEditor();

			const highlightDiv = container.querySelector('.config-editor-highlight');
			expect(highlightDiv).toHaveAttribute('aria-hidden', 'true');
		});
	});
});
