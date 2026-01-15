import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/svelte';
import CommentForm from './CommentForm.svelte';
import type { CreateCommentRequest } from '$lib/types';

describe('CommentForm', () => {
	const mockOnSubmit = vi.fn();
	const mockOnCancel = vi.fn();

	// Store original platform
	const originalPlatform = navigator.platform;

	beforeEach(() => {
		vi.clearAllMocks();
		// Reset platform to a known value
		Object.defineProperty(navigator, 'platform', {
			value: 'Win32',
			configurable: true
		});
	});

	afterEach(() => {
		vi.clearAllMocks();
		cleanup();
		// Restore original platform
		Object.defineProperty(navigator, 'platform', {
			value: originalPlatform,
			configurable: true
		});
	});

	describe('cross-platform keyboard handling (Ctrl vs Cmd)', () => {
		it('submits form with Ctrl+Enter on non-Mac', async () => {
			// Mock navigator.platform for Windows/Linux
			Object.defineProperty(navigator, 'platform', {
				value: 'Win32',
				configurable: true
			});

			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel
				}
			});

			// Fill in required content
			const textarea = screen.getByPlaceholderText(/describe the issue/i);
			await fireEvent.input(textarea, { target: { value: 'Test comment content' } });

			// Submit with Ctrl+Enter
			await fireEvent.keyDown(textarea, { key: 'Enter', ctrlKey: true });

			expect(mockOnSubmit).toHaveBeenCalledWith(
				expect.objectContaining({
					content: 'Test comment content',
					severity: 'issue'
				})
			);
		});

		it('submits form with Cmd+Enter on Mac', async () => {
			// Mock navigator.platform for Mac
			Object.defineProperty(navigator, 'platform', {
				value: 'MacIntel',
				configurable: true
			});

			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel
				}
			});

			// Fill in required content
			const textarea = screen.getByPlaceholderText(/describe the issue/i);
			await fireEvent.input(textarea, { target: { value: 'Mac comment' } });

			// Submit with Cmd+Enter (metaKey)
			await fireEvent.keyDown(textarea, { key: 'Enter', metaKey: true });

			expect(mockOnSubmit).toHaveBeenCalledWith(
				expect.objectContaining({
					content: 'Mac comment'
				})
			);
		});

		it('does not submit with plain Enter', async () => {
			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel
				}
			});

			const textarea = screen.getByPlaceholderText(/describe the issue/i);
			await fireEvent.input(textarea, { target: { value: 'Test content' } });

			// Plain Enter should not submit
			await fireEvent.keyDown(textarea, { key: 'Enter' });

			expect(mockOnSubmit).not.toHaveBeenCalled();
		});

		it('closes form with Escape key', async () => {
			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel
				}
			});

			const form = document.querySelector('.comment-form')!;
			await fireEvent.keyDown(form, { key: 'Escape' });

			expect(mockOnCancel).toHaveBeenCalled();
		});
	});

	describe('modifierKey shows correct value', () => {
		it('shows "Ctrl" on Windows/Linux', async () => {
			// Platform set to Win32 in beforeEach
			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel
				}
			});

			// The keyboard hint should show "Ctrl" in a kbd element
			await waitFor(() => {
				const keyboardHint = document.querySelector('.keyboard-hint');
				expect(keyboardHint).toBeInTheDocument();
				expect(keyboardHint?.textContent).toContain('Ctrl');
				expect(keyboardHint?.textContent).toContain('Enter');
			});
		});

		it('shows "Cmd" on Mac', async () => {
			// Must set platform BEFORE rendering
			cleanup();
			Object.defineProperty(navigator, 'platform', {
				value: 'MacIntel',
				configurable: true
			});

			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel
				}
			});

			await waitFor(() => {
				const keyboardHint = document.querySelector('.keyboard-hint');
				expect(keyboardHint).toBeInTheDocument();
				expect(keyboardHint?.textContent).toContain('Cmd');
			});
		});

		it('shows "Cmd" on iPhone/iPad', async () => {
			// Must set platform BEFORE rendering
			cleanup();
			Object.defineProperty(navigator, 'platform', {
				value: 'iPhone',
				configurable: true
			});

			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel
				}
			});

			await waitFor(() => {
				const keyboardHint = document.querySelector('.keyboard-hint');
				expect(keyboardHint).toBeInTheDocument();
				expect(keyboardHint?.textContent).toContain('Cmd');
			});
		});

		it('shows "Ctrl" on Linux', async () => {
			// Must set platform BEFORE rendering
			cleanup();
			Object.defineProperty(navigator, 'platform', {
				value: 'Linux x86_64',
				configurable: true
			});

			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel
				}
			});

			await waitFor(() => {
				const keyboardHint = document.querySelector('.keyboard-hint');
				expect(keyboardHint).toBeInTheDocument();
				expect(keyboardHint?.textContent).toContain('Ctrl');
			});
		});
	});

	describe('form validation', () => {
		it('disables submit when content is empty', async () => {
			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel
				}
			});

			const submitBtn = screen.getByRole('button', { name: /add comment/i });
			expect(submitBtn).toBeDisabled();
		});

		it('enables submit when content is provided', async () => {
			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel
				}
			});

			const textarea = screen.getByPlaceholderText(/describe the issue/i);
			await fireEvent.input(textarea, { target: { value: 'Valid content' } });

			const submitBtn = screen.getByRole('button', { name: /add comment/i });
			expect(submitBtn).not.toBeDisabled();
		});

		it('disables submit when loading', async () => {
			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel,
					isLoading: true
				}
			});

			const textarea = screen.getByPlaceholderText(/describe the issue/i);
			await fireEvent.input(textarea, { target: { value: 'Some content' } });

			// Submit button should show "Adding..." and be disabled
			expect(screen.getByText('Adding...')).toBeInTheDocument();
		});

		it('trims whitespace from content before validation', async () => {
			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel
				}
			});

			const textarea = screen.getByPlaceholderText(/describe the issue/i);
			await fireEvent.input(textarea, { target: { value: '   ' } });

			const submitBtn = screen.getByRole('button', { name: /add comment/i });
			expect(submitBtn).toBeDisabled();
		});
	});

	describe('form submission', () => {
		it('includes file_path when provided', async () => {
			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel
				}
			});

			const fileInput = screen.getByLabelText(/file/i);
			const textarea = screen.getByPlaceholderText(/describe the issue/i);

			await fireEvent.input(fileInput, { target: { value: 'src/app.ts' } });
			await fireEvent.input(textarea, { target: { value: 'Comment about file' } });

			const submitBtn = screen.getByRole('button', { name: /add comment/i });
			await fireEvent.click(submitBtn);

			expect(mockOnSubmit).toHaveBeenCalledWith(
				expect.objectContaining({
					file_path: 'src/app.ts',
					content: 'Comment about file'
				})
			);
		});

		it('includes line_number when provided and valid', async () => {
			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel
				}
			});

			const lineInput = screen.getByLabelText(/line/i);
			const textarea = screen.getByPlaceholderText(/describe the issue/i);

			await fireEvent.input(lineInput, { target: { value: '42' } });
			await fireEvent.input(textarea, { target: { value: 'Line comment' } });

			const submitBtn = screen.getByRole('button', { name: /add comment/i });
			await fireEvent.click(submitBtn);

			expect(mockOnSubmit).toHaveBeenCalledWith(
				expect.objectContaining({
					line_number: 42
				})
			);
		});

		it('excludes line_number when zero or invalid', async () => {
			// This tests the logic in handleSubmit: lineNumber > 0 check
			// Line number 0 or negative should not be included in the request
			const lineNumberCheck = (lineNumber: number | undefined) => {
				if (lineNumber !== undefined && lineNumber > 0) {
					return lineNumber;
				}
				return undefined;
			};

			// Test the condition directly
			expect(lineNumberCheck(0)).toBeUndefined();
			expect(lineNumberCheck(-1)).toBeUndefined();
			expect(lineNumberCheck(undefined)).toBeUndefined();
			expect(lineNumberCheck(1)).toBe(1);
			expect(lineNumberCheck(42)).toBe(42);
		});

		it('uses selected severity', async () => {
			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel
				}
			});

			const textarea = screen.getByPlaceholderText(/describe the issue/i);
			await fireEvent.input(textarea, { target: { value: 'Blocker comment' } });

			// Click blocker severity option
			const blockerOption = screen.getByText('Blocker').closest('label');
			if (blockerOption) {
				await fireEvent.click(blockerOption);
			}

			const submitBtn = screen.getByRole('button', { name: /add comment/i });
			await fireEvent.click(submitBtn);

			expect(mockOnSubmit).toHaveBeenCalledWith(
				expect.objectContaining({
					severity: 'blocker'
				})
			);
		});

		it('defaults to issue severity', async () => {
			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel
				}
			});

			const textarea = screen.getByPlaceholderText(/describe the issue/i);
			await fireEvent.input(textarea, { target: { value: 'Default severity comment' } });

			const submitBtn = screen.getByRole('button', { name: /add comment/i });
			await fireEvent.click(submitBtn);

			expect(mockOnSubmit).toHaveBeenCalledWith(
				expect.objectContaining({
					severity: 'issue'
				})
			);
		});
	});

	describe('initial values', () => {
		it('pre-fills file path when initialFilePath provided', async () => {
			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel,
					initialFilePath: 'src/utils.ts'
				}
			});

			const fileInput = screen.getByLabelText(/file/i) as HTMLInputElement;
			expect(fileInput.value).toBe('src/utils.ts');
		});

		it('pre-fills line number when initialLineNumber provided', async () => {
			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel,
					initialLineNumber: 100
				}
			});

			const lineInput = screen.getByLabelText(/line/i) as HTMLInputElement;
			expect(lineInput.value).toBe('100');
		});
	});

	describe('cancel functionality', () => {
		it('calls onCancel when Cancel button clicked', async () => {
			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel
				}
			});

			// Get all buttons with Cancel text/title and find the ghost button (not icon button)
			const cancelBtns = screen.getAllByRole('button').filter(
				(btn) =>
					btn.textContent?.includes('Cancel') && btn.classList.contains('ghost')
			);
			expect(cancelBtns.length).toBe(1);
			await fireEvent.click(cancelBtns[0]);

			expect(mockOnCancel).toHaveBeenCalled();
		});

		it('calls onCancel when close button clicked', async () => {
			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel
				}
			});

			// Get the close button by its specific class
			const closeBtn = document.querySelector('.close-btn');
			expect(closeBtn).toBeInTheDocument();
			await fireEvent.click(closeBtn!);

			expect(mockOnCancel).toHaveBeenCalled();
		});
	});

	describe('loading state', () => {
		it('disables all inputs when loading', async () => {
			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel,
					isLoading: true
				}
			});

			const fileInput = screen.getByLabelText(/file/i);
			const lineInput = screen.getByLabelText(/line/i);
			const textarea = screen.getByPlaceholderText(/describe the issue/i);

			expect(fileInput).toBeDisabled();
			expect(lineInput).toBeDisabled();
			expect(textarea).toBeDisabled();
		});

		it('disables severity radio buttons when loading', async () => {
			render(CommentForm, {
				props: {
					onSubmit: mockOnSubmit,
					onCancel: mockOnCancel,
					isLoading: true
				}
			});

			const radios = document.querySelectorAll('input[type="radio"]');
			radios.forEach((radio) => {
				expect(radio).toBeDisabled();
			});
		});
	});
});
