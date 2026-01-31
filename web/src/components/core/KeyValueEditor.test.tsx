/**
 * TDD Tests for KeyValueEditor
 *
 * Tests for TASK-669: Phase template claude_config editor with collapsible sections
 *
 * Success Criteria Coverage:
 * - SC-8: KeyValueEditor component allows adding/removing key-value rows for env vars
 *
 * Edge Cases:
 * - Empty key on save: row excluded from serialized output
 * - Empty value: included in output (empty string is valid)
 */

import { describe, it, expect, vi, afterEach, beforeAll } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { KeyValueEditor } from './KeyValueEditor';

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

describe('KeyValueEditor', () => {
	afterEach(() => {
		cleanup();
	});

	// SC-8: click Add → new empty row appears
	it('adds a new empty row when Add button is clicked', async () => {
		const user = userEvent.setup();
		const onChange = vi.fn();

		render(
			<KeyValueEditor
				entries={{}}
				onChange={onChange}
			/>
		);

		const addButton = screen.getByRole('button', { name: /add/i });
		await user.click(addButton);

		// Should call onChange with a new entry (key-value pair)
		// The component should add a new empty row
		expect(onChange).toHaveBeenCalled();
	});

	// SC-8: renders existing key-value rows
	it('renders existing key-value rows', () => {
		render(
			<KeyValueEditor
				entries={{ FOO: 'bar', BAZ: 'qux' }}
				onChange={vi.fn()}
			/>
		);

		// Key inputs should show FOO and BAZ
		const inputs = screen.getAllByRole('textbox');
		// Should have at least 4 inputs (2 keys + 2 values)
		expect(inputs.length).toBeGreaterThanOrEqual(4);

		// Check values are present
		expect(screen.getByDisplayValue('FOO')).toBeInTheDocument();
		expect(screen.getByDisplayValue('bar')).toBeInTheDocument();
		expect(screen.getByDisplayValue('BAZ')).toBeInTheDocument();
		expect(screen.getByDisplayValue('qux')).toBeInTheDocument();
	});

	// SC-8: editing a key updates state
	it('calls onChange when key is edited', async () => {
		const user = userEvent.setup();
		const onChange = vi.fn();

		render(
			<KeyValueEditor
				entries={{ FOO: 'bar' }}
				onChange={onChange}
			/>
		);

		const keyInput = screen.getByDisplayValue('FOO');
		await user.clear(keyInput);
		await user.type(keyInput, 'NEW_KEY');

		expect(onChange).toHaveBeenCalledWith(
			expect.objectContaining({ NEW_KEY: 'bar' })
		);
	});

	// SC-8: editing a value updates state
	it('calls onChange when value is edited', async () => {
		const user = userEvent.setup();
		const onChange = vi.fn();

		render(
			<KeyValueEditor
				entries={{ FOO: 'bar' }}
				onChange={onChange}
			/>
		);

		const valueInput = screen.getByDisplayValue('bar');
		await user.clear(valueInput);
		await user.type(valueInput, 'new_value');

		expect(onChange).toHaveBeenCalledWith(
			expect.objectContaining({ FOO: 'new_value' })
		);
	});

	// SC-8: click remove on row → row deleted
	it('removes a row when remove button is clicked', async () => {
		const user = userEvent.setup();
		const onChange = vi.fn();

		render(
			<KeyValueEditor
				entries={{ FOO: 'bar', BAZ: 'qux' }}
				onChange={onChange}
			/>
		);

		// Find remove buttons
		const removeButtons = screen.getAllByRole('button', { name: /remove/i });
		expect(removeButtons).toHaveLength(2);

		// Click remove on first row
		await user.click(removeButtons[0]);

		// Should call onChange without the removed entry
		expect(onChange).toHaveBeenCalledWith(
			expect.not.objectContaining({ FOO: 'bar' })
		);
	});

	// Edge case: empty key is excluded from serialized output
	it('excludes rows with empty keys from output', async () => {
		const user = userEvent.setup();
		const onChange = vi.fn();

		render(
			<KeyValueEditor
				entries={{ FOO: 'bar' }}
				onChange={onChange}
			/>
		);

		// Clear the key
		const keyInput = screen.getByDisplayValue('FOO');
		await user.clear(keyInput);

		// The onChange should exclude this row (empty key)
		// AMEND-002: toHaveProperty('') triggers vitest bug with empty string paths
		const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1];
		if (lastCall) {
			expect(Object.keys(lastCall[0])).not.toContain('');
		}
	});

	// Edge case: empty value is included (valid)
	it('includes rows with empty values in output', async () => {
		const user = userEvent.setup();
		const onChange = vi.fn();

		render(
			<KeyValueEditor
				entries={{ FOO: 'bar' }}
				onChange={onChange}
			/>
		);

		const valueInput = screen.getByDisplayValue('bar');
		await user.clear(valueInput);

		expect(onChange).toHaveBeenCalledWith(
			expect.objectContaining({ FOO: '' })
		);
	});

	// Edge case: disabled state for built-in templates
	it('disables all inputs when disabled prop is set', () => {
		render(
			<KeyValueEditor
				entries={{ FOO: 'bar' }}
				onChange={vi.fn()}
				disabled
			/>
		);

		const inputs = screen.getAllByRole('textbox');
		inputs.forEach((input) => {
			expect(input).toBeDisabled();
		});

		// Add button should also be disabled
		const addButton = screen.getByRole('button', { name: /add/i });
		expect(addButton).toBeDisabled();
	});

	// No rows initially when entries is empty
	it('shows empty state with Add button when no entries', () => {
		render(
			<KeyValueEditor
				entries={{}}
				onChange={vi.fn()}
			/>
		);

		// Only the Add button should be present, no textboxes
		expect(screen.queryAllByRole('textbox')).toHaveLength(0);
		expect(screen.getByRole('button', { name: /add/i })).toBeInTheDocument();
	});
});
