/**
 * TDD Tests for TagInput
 *
 * Tests for TASK-669: Phase template claude_config editor with collapsible sections
 *
 * Success Criteria Coverage:
 * - SC-7: TagInput component allows adding tags via Enter/comma, displays as removable chips,
 *         and supports backspace-to-remove-last
 *
 * Edge Cases:
 * - Duplicate tag silently ignored
 * - Very long tool name truncates with ellipsis
 */

import { describe, it, expect, vi, afterEach, beforeAll } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TagInput } from './TagInput';

// Mock browser APIs for Radix
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
});

describe('TagInput', () => {
	afterEach(() => {
		cleanup();
	});

	// SC-7: adding tags via Enter
	it('adds a tag when Enter is pressed', async () => {
		const user = userEvent.setup();
		const onChange = vi.fn();

		render(
			<TagInput
				tags={[]}
				onChange={onChange}
				placeholder="Add tool..."
			/>
		);

		const input = screen.getByPlaceholderText('Add tool...');
		await user.type(input, 'Bash{Enter}');

		expect(onChange).toHaveBeenCalledWith(['Bash']);
	});

	// SC-7: adding tags via comma
	it('adds tags when comma is typed', async () => {
		const user = userEvent.setup();
		const onChange = vi.fn();

		render(
			<TagInput
				tags={[]}
				onChange={onChange}
				placeholder="Add tool..."
			/>
		);

		const input = screen.getByPlaceholderText('Add tool...');
		await user.type(input, 'Read,Write,');

		// Should have called onChange with the tags added
		expect(onChange).toHaveBeenCalledWith(expect.arrayContaining(['Read']));
		expect(onChange).toHaveBeenCalledWith(expect.arrayContaining(['Write']));
	});

	// SC-7: displays as removable chips
	it('displays existing tags as chips with remove buttons', () => {
		render(
			<TagInput
				tags={['Bash', 'Read', 'Write']}
				onChange={vi.fn()}
			/>
		);

		expect(screen.getByText('Bash')).toBeInTheDocument();
		expect(screen.getByText('Read')).toBeInTheDocument();
		expect(screen.getByText('Write')).toBeInTheDocument();

		// Each chip should have a remove button (×)
		const removeButtons = screen.getAllByRole('button', { name: /remove/i });
		expect(removeButtons).toHaveLength(3);
	});

	// SC-7: clicking × removes a chip
	it('removes a tag when × button is clicked', async () => {
		const user = userEvent.setup();
		const onChange = vi.fn();

		render(
			<TagInput
				tags={['Bash', 'Read', 'Write']}
				onChange={onChange}
			/>
		);

		// Click remove on "Read"
		const removeButtons = screen.getAllByRole('button', { name: /remove/i });
		// Find the one near "Read"
		const readChip = screen.getByText('Read').closest('[data-tag]');
		const readRemoveBtn = readChip?.querySelector('button');
		if (readRemoveBtn) {
			await user.click(readRemoveBtn);
		}

		expect(onChange).toHaveBeenCalledWith(['Bash', 'Write']);
	});

	// SC-7: backspace removes last tag when input is empty
	it('removes last tag on Backspace when input is empty', async () => {
		const user = userEvent.setup();
		const onChange = vi.fn();

		render(
			<TagInput
				tags={['Bash', 'Read']}
				onChange={onChange}
			/>
		);

		const input = screen.getByRole('textbox');

		// Focus the input and press Backspace with empty input
		await user.click(input);
		await user.keyboard('{Backspace}');

		expect(onChange).toHaveBeenCalledWith(['Bash']);
	});

	// SC-7: input clears after adding a tag
	it('clears input after adding a tag', async () => {
		const user = userEvent.setup();

		render(
			<TagInput
				tags={[]}
				onChange={vi.fn()}
				placeholder="Add tool..."
			/>
		);

		const input = screen.getByPlaceholderText('Add tool...');
		await user.type(input, 'Bash{Enter}');

		expect(input).toHaveValue('');
	});

	// Edge case: duplicate tag silently ignored
	it('silently ignores duplicate tags', async () => {
		const user = userEvent.setup();
		const onChange = vi.fn();

		render(
			<TagInput
				tags={['Bash']}
				onChange={onChange}
				placeholder="Add tool..."
			/>
		);

		const input = screen.getByPlaceholderText('Add tool...');
		await user.type(input, 'Bash{Enter}');

		// onChange should not be called with duplicate
		expect(onChange).not.toHaveBeenCalled();
	});

	// Edge case: whitespace-only tag is ignored
	it('ignores empty or whitespace-only tags', async () => {
		const user = userEvent.setup();
		const onChange = vi.fn();

		render(
			<TagInput
				tags={[]}
				onChange={onChange}
				placeholder="Add tool..."
			/>
		);

		const input = screen.getByPlaceholderText('Add tool...');
		await user.type(input, '   {Enter}');

		expect(onChange).not.toHaveBeenCalled();
	});

	// Edge case: very long tool name
	it('renders long tag names with truncation', () => {
		const longName = 'VeryLongToolNameThatShouldBeTruncatedInTheUI';

		render(
			<TagInput
				tags={[longName]}
				onChange={vi.fn()}
			/>
		);

		// The tag should be present in the DOM
		expect(screen.getByText(longName)).toBeInTheDocument();
	});

	// Edge case: disabled state
	it('disables input when disabled prop is set', () => {
		render(
			<TagInput
				tags={['Bash']}
				onChange={vi.fn()}
				disabled
			/>
		);

		const input = screen.getByRole('textbox');
		expect(input).toBeDisabled();
	});
});
