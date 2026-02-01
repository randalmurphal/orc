/**
 * TDD Tests for ConditionEditor component
 *
 * Tests for TASK-694: Condition editor UI in phase inspector
 *
 * ConditionEditor is a self-contained visual condition builder that receives
 * a JSON condition string and emits JSON strings via onChange. It manages
 * visual builder rows, logic mode (all/any), and raw JSON editing mode.
 *
 * Success Criteria Coverage:
 * - SC-1: Visual builder renders field, operator, and value controls
 * - SC-2: Selecting field/operator/value produces correct JSON via onChange
 * - SC-3: Adding multiple conditions wraps them in compound all/any JSON
 * - SC-4: Logic toggle switches between all and any compound wrapper
 * - SC-5: Removing a condition row updates JSON correctly
 * - SC-7: Existing condition from phase.condition is loaded into editor on mount
 * - SC-9: Raw JSON editor toggle allows direct JSON editing with validation
 * - SC-10: Condition section renders as read-only for built-in workflows
 *
 * Failure Modes:
 * - Malformed JSON in condition falls back to raw JSON editor
 * - Invalid JSON in raw editor shows error, onChange NOT called
 * - Incomplete rows excluded from serialized JSON
 * - Nested compound conditions fall back to raw JSON mode
 *
 * These tests will FAIL until ConditionEditor is implemented.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, within, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ConditionEditor } from './ConditionEditor';

// ─── Test Helpers ───────────────────────────────────────────────────────────

/** Parse the JSON string from onChange to compare values structurally */
function parseConditionJSON(json: string): unknown {
	if (!json) return null;
	return JSON.parse(json);
}

// ─── Tests ──────────────────────────────────────────────────────────────────

describe('ConditionEditor', () => {
	let onChange: ReturnType<typeof vi.fn>;

	beforeEach(() => {
		onChange = vi.fn();
	});

	afterEach(() => {
		cleanup();
	});

	// ─── SC-1: Visual builder renders field, operator, and value controls ────

	describe('SC-1: visual builder controls', () => {
		it('renders empty state with only "+ Add condition" button when no condition set', () => {
			render(
				<ConditionEditor
					condition=""
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Empty state: only the add button, no condition rows
			expect(screen.getByRole('button', { name: /add condition/i })).toBeInTheDocument();

			// No field/operator/value controls should exist yet
			expect(screen.queryByRole('combobox', { name: /field/i })).not.toBeInTheDocument();
		});

		it('shows field, operator, and value controls after clicking "+ Add condition"', async () => {
			const user = userEvent.setup();

			render(
				<ConditionEditor
					condition=""
					onChange={onChange}
					disabled={false}
				/>,
			);

			await user.click(screen.getByRole('button', { name: /add condition/i }));

			// After adding a row, 3 controls should appear:
			// field select/combobox, operator select, value input
			expect(screen.getByLabelText(/field/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/operator/i)).toBeInTheDocument();
			// Value input may not be visible for some operators (exists), but the container should exist
		});

		it('field dropdown includes task.weight, task.category, task.priority suggestions', async () => {
			const user = userEvent.setup();

			render(
				<ConditionEditor
					condition=""
					onChange={onChange}
					disabled={false}
				/>,
			);

			await user.click(screen.getByRole('button', { name: /add condition/i }));

			const fieldSelect = screen.getByLabelText(/field/i);
			expect(fieldSelect).toBeInTheDocument();

			// The field dropdown should contain known task fields
			// These may be <option> elements or similar depending on implementation
			const options = within(fieldSelect.closest('[role="group"]') ?? fieldSelect.parentElement!)
				.getAllByRole('option');
			const optionValues = options.map((o) => o.getAttribute('value') ?? o.textContent);

			expect(optionValues).toContain('task.weight');
			expect(optionValues).toContain('task.category');
			expect(optionValues).toContain('task.priority');
		});

		it('does not show logic toggle with only one condition', async () => {
			const user = userEvent.setup();

			render(
				<ConditionEditor
					condition=""
					onChange={onChange}
					disabled={false}
				/>,
			);

			await user.click(screen.getByRole('button', { name: /add condition/i }));

			// Logic toggle (All/Any) should NOT be visible with a single condition
			expect(screen.queryByRole('button', { name: /all/i })).not.toBeInTheDocument();
			expect(screen.queryByRole('radio', { name: /all/i })).not.toBeInTheDocument();
		});

		it('hides value input when "exists" operator is selected', () => {
			render(
				<ConditionEditor
					condition='{"field":"task.weight","op":"exists"}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			// With "exists" operator, the value input should be hidden
			// The field and operator should be visible but no value control
			expect(screen.getByLabelText(/field/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/operator/i)).toBeInTheDocument();
			// There should be no value input for "exists"
			expect(screen.queryByLabelText(/value/i)).not.toBeInTheDocument();
		});

		it('shows multi-value input (TagInput) when "in" operator is selected', () => {
			render(
				<ConditionEditor
					condition='{"field":"task.weight","op":"in","value":["medium","large"]}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			// "in" operator should render a multi-value tag input, not a regular text input
			// Look for tag chips or multi-value indicator
			expect(screen.getByText('medium')).toBeInTheDocument();
			expect(screen.getByText('large')).toBeInTheDocument();
		});
	});

	// ─── SC-2: Field/operator/value produces correct JSON via onChange ────────

	describe('SC-2: JSON serialization from visual builder', () => {
		it('produces correct flat JSON for single condition: field=task.weight, op=eq, value=medium', async () => {
			const user = userEvent.setup();

			render(
				<ConditionEditor
					condition=""
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Add a condition row
			await user.click(screen.getByRole('button', { name: /add condition/i }));

			// Select field: task.weight
			const fieldSelect = screen.getByLabelText(/field/i);
			await user.selectOptions(fieldSelect, 'task.weight');

			// Select operator: eq (equals)
			const opSelect = screen.getByLabelText(/operator/i);
			await user.selectOptions(opSelect, 'eq');

			// Enter value: medium
			const valueInput = screen.getByLabelText(/value/i);
			await user.clear(valueInput);
			await user.type(valueInput, 'medium');

			// onChange should have been called with the correct JSON
			const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0];
			const parsed = parseConditionJSON(lastCall);
			expect(parsed).toEqual({
				field: 'task.weight',
				op: 'eq',
				value: 'medium',
			});
		});

		it('produces JSON with array value for "in" operator', async () => {
			const user = userEvent.setup();

			render(
				<ConditionEditor
					condition='{"field":"task.category","op":"in","value":[]}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			// The value input for "in" operator should accept multiple values
			// Add values via TagInput-style interaction
			const tagInput = screen.getByPlaceholderText(/add/i);
			await user.type(tagInput, 'feature{enter}');
			await user.type(tagInput, 'bug{enter}');

			// onChange should produce JSON with array value
			const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0];
			const parsed = parseConditionJSON(lastCall);
			expect(parsed).toEqual({
				field: 'task.category',
				op: 'in',
				value: expect.arrayContaining(['feature', 'bug']),
			});
		});

		it('excludes incomplete rows (missing field or operator) from serialized JSON', async () => {
			const user = userEvent.setup();

			render(
				<ConditionEditor
					condition=""
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Add a condition row but don't fill in field or operator
			await user.click(screen.getByRole('button', { name: /add condition/i }));

			// onChange should output empty string for incomplete rows
			const lastCall = onChange.mock.calls.length > 0
				? onChange.mock.calls[onChange.mock.calls.length - 1][0]
				: '';
			expect(lastCall === '' || lastCall === undefined).toBe(true);
		});
	});

	// ─── SC-3: Multiple conditions wrap in compound all/any JSON ─────────────

	describe('SC-3: compound condition wrapping', () => {
		it('wraps 2 conditions in {all: [...]} by default', async () => {
			const user = userEvent.setup();

			// Start with one condition already set
			render(
				<ConditionEditor
					condition='{"field":"task.weight","op":"eq","value":"medium"}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Add a second condition
			await user.click(screen.getByRole('button', { name: /add condition/i }));

			// Fill in the second condition
			// There should now be 2 rows - find the second row's fields
			const fieldSelects = screen.getAllByLabelText(/field/i);
			expect(fieldSelects.length).toBe(2);

			await user.selectOptions(fieldSelects[1], 'task.category');

			const opSelects = screen.getAllByLabelText(/operator/i);
			await user.selectOptions(opSelects[1], 'neq');

			const valueInputs = screen.getAllByLabelText(/value/i);
			await user.clear(valueInputs[1]);
			await user.type(valueInputs[1], 'docs');

			// onChange should produce compound JSON with "all" wrapper
			const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0];
			const parsed = parseConditionJSON(lastCall);
			expect(parsed).toEqual({
				all: [
					{ field: 'task.weight', op: 'eq', value: 'medium' },
					{ field: 'task.category', op: 'neq', value: 'docs' },
				],
			});
		});

		it('produces flat JSON (no wrapper) for single condition', () => {
			render(
				<ConditionEditor
					condition='{"field":"task.weight","op":"eq","value":"medium"}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			// The initial condition is a single flat condition - verify it displays
			// as a single row in the visual builder (not wrapped)
			const fieldSelects = screen.getAllByLabelText(/field/i);
			expect(fieldSelects.length).toBe(1);
		});

		it('produces empty string for zero conditions', async () => {
			const user = userEvent.setup();

			render(
				<ConditionEditor
					condition='{"field":"task.weight","op":"eq","value":"medium"}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Remove the only condition
			const removeButton = screen.getByRole('button', { name: /remove/i });
			await user.click(removeButton);

			// onChange should produce empty string
			const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0];
			expect(lastCall).toBe('');
		});
	});

	// ─── SC-4: Logic toggle switches between all and any ─────────────────────

	describe('SC-4: logic toggle (all/any)', () => {
		it('shows logic toggle when 2+ conditions exist', () => {
			render(
				<ConditionEditor
					condition='{"all":[{"field":"task.weight","op":"eq","value":"medium"},{"field":"task.category","op":"neq","value":"docs"}]}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Logic toggle should be visible with compound conditions
			// Look for All/Any toggle (could be radio buttons, segmented control, or toggle)
			expect(
				screen.getByRole('radio', { name: /all/i }) ??
				screen.getByRole('button', { name: /all/i }),
			).toBeInTheDocument();
			expect(
				screen.getByRole('radio', { name: /any/i }) ??
				screen.getByRole('button', { name: /any/i }),
			).toBeInTheDocument();
		});

		it('toggles from all to any, changing root key while preserving sub-conditions', async () => {
			const user = userEvent.setup();
			const conditions = {
				all: [
					{ field: 'task.weight', op: 'eq', value: 'medium' },
					{ field: 'task.category', op: 'neq', value: 'docs' },
				],
			};

			render(
				<ConditionEditor
					condition={JSON.stringify(conditions)}
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Click the "Any" toggle
			const anyToggle = screen.getByRole('radio', { name: /any/i }) ??
				screen.getByRole('button', { name: /any/i });
			await user.click(anyToggle);

			// onChange should produce JSON with "any" root key
			const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0];
			const parsed = parseConditionJSON(lastCall);
			expect(parsed).toEqual({
				any: [
					{ field: 'task.weight', op: 'eq', value: 'medium' },
					{ field: 'task.category', op: 'neq', value: 'docs' },
				],
			});
		});

		it('hides logic toggle when only 1 condition exists', () => {
			render(
				<ConditionEditor
					condition='{"field":"task.weight","op":"eq","value":"medium"}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Logic toggle should NOT be visible with single condition
			expect(screen.queryByRole('radio', { name: /all/i })).not.toBeInTheDocument();
			expect(screen.queryByRole('radio', { name: /any/i })).not.toBeInTheDocument();
			expect(screen.queryByRole('button', { name: /^all$/i })).not.toBeInTheDocument();
			expect(screen.queryByRole('button', { name: /^any$/i })).not.toBeInTheDocument();
		});
	});

	// ─── SC-5: Removing condition rows ───────────────────────────────────────

	describe('SC-5: removing conditions', () => {
		it('removes middle condition from 3, leaving 2 in compound JSON', async () => {
			const user = userEvent.setup();
			const conditions = {
				all: [
					{ field: 'task.weight', op: 'eq', value: 'medium' },
					{ field: 'task.category', op: 'eq', value: 'feature' },
					{ field: 'task.priority', op: 'eq', value: 'high' },
				],
			};

			render(
				<ConditionEditor
					condition={JSON.stringify(conditions)}
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Should have 3 condition rows
			const removeButtons = screen.getAllByRole('button', { name: /remove/i });
			expect(removeButtons.length).toBe(3);

			// Remove the middle condition (index 1)
			await user.click(removeButtons[1]);

			// onChange should produce compound JSON with 2 remaining conditions
			const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0];
			const parsed = parseConditionJSON(lastCall);
			expect(parsed).toEqual({
				all: [
					{ field: 'task.weight', op: 'eq', value: 'medium' },
					{ field: 'task.priority', op: 'eq', value: 'high' },
				],
			});
		});

		it('unwraps to flat JSON when removing to 1 condition', async () => {
			const user = userEvent.setup();
			const conditions = {
				all: [
					{ field: 'task.weight', op: 'eq', value: 'medium' },
					{ field: 'task.category', op: 'eq', value: 'feature' },
				],
			};

			render(
				<ConditionEditor
					condition={JSON.stringify(conditions)}
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Remove one condition, leaving 1
			const removeButtons = screen.getAllByRole('button', { name: /remove/i });
			await user.click(removeButtons[1]);

			// Should unwrap to flat JSON (no compound wrapper)
			const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0];
			const parsed = parseConditionJSON(lastCall);
			expect(parsed).toEqual({
				field: 'task.weight',
				op: 'eq',
				value: 'medium',
			});
		});

		it('produces empty string when removing the last condition', async () => {
			const user = userEvent.setup();

			render(
				<ConditionEditor
					condition='{"field":"task.weight","op":"eq","value":"medium"}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			const removeButton = screen.getByRole('button', { name: /remove/i });
			await user.click(removeButton);

			const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0];
			expect(lastCall).toBe('');
		});
	});

	// ─── SC-7: Loading existing conditions on mount ──────────────────────────

	describe('SC-7: loading existing conditions', () => {
		it('loads single flat condition into visual builder', () => {
			render(
				<ConditionEditor
					condition='{"field":"task.weight","op":"in","value":["medium","large"]}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Visual builder should show one row with the condition data
			expect(screen.getAllByLabelText(/field/i).length).toBe(1);

			// Field should show task.weight
			const fieldSelect = screen.getByLabelText(/field/i) as HTMLSelectElement;
			expect(fieldSelect.value).toBe('task.weight');

			// Operator should show "in" (displayed as "is one of" or similar)
			const opSelect = screen.getByLabelText(/operator/i) as HTMLSelectElement;
			expect(opSelect.value).toBe('in');

			// Values should be displayed as tags
			expect(screen.getByText('medium')).toBeInTheDocument();
			expect(screen.getByText('large')).toBeInTheDocument();
		});

		it('loads compound "all" condition with multiple rows', () => {
			const conditions = {
				all: [
					{ field: 'task.weight', op: 'eq', value: 'medium' },
					{ field: 'task.category', op: 'neq', value: 'docs' },
				],
			};

			render(
				<ConditionEditor
					condition={JSON.stringify(conditions)}
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Should render 2 condition rows
			const fieldSelects = screen.getAllByLabelText(/field/i);
			expect(fieldSelects.length).toBe(2);

			// Logic toggle should show "All" as active
			const allToggle = screen.getByRole('radio', { name: /all/i }) ??
				screen.getByRole('button', { name: /all/i });
			expect(allToggle).toBeInTheDocument();
		});

		it('loads compound "any" condition with correct logic mode', () => {
			const conditions = {
				any: [
					{ field: 'task.weight', op: 'eq', value: 'large' },
					{ field: 'task.priority', op: 'eq', value: 'critical' },
				],
			};

			render(
				<ConditionEditor
					condition={JSON.stringify(conditions)}
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Should render 2 rows with "Any" logic
			expect(screen.getAllByLabelText(/field/i).length).toBe(2);
		});

		it('falls back to raw JSON editor for malformed JSON', () => {
			render(
				<ConditionEditor
					condition='not valid json {'
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Should fall back to raw JSON editor mode showing the raw string
			const textarea = screen.getByRole('textbox');
			expect(textarea).toBeInTheDocument();
			expect((textarea as HTMLTextAreaElement).value).toContain('not valid json {');
		});

		it('falls back to raw JSON editor for nested compound conditions (BDD-5)', () => {
			const nested = {
				all: [
					{ any: [
						{ field: 'task.weight', op: 'eq', value: 'medium' },
						{ field: 'task.weight', op: 'eq', value: 'large' },
					]},
					{ field: 'task.category', op: 'neq', value: 'docs' },
				],
			};

			render(
				<ConditionEditor
					condition={JSON.stringify(nested)}
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Visual builder can't represent nested compounds → raw JSON editor
			const textarea = screen.getByRole('textbox');
			expect(textarea).toBeInTheDocument();
			// Should show the JSON in the textarea
			expect((textarea as HTMLTextAreaElement).value).toContain('"all"');
		});

		it('treats null condition as empty state', () => {
			render(
				<ConditionEditor
					condition={undefined as unknown as string}
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Should show empty state
			expect(screen.getByRole('button', { name: /add condition/i })).toBeInTheDocument();
			expect(screen.queryByLabelText(/field/i)).not.toBeInTheDocument();
		});

		it('treats empty string condition as empty state', () => {
			render(
				<ConditionEditor
					condition=""
					onChange={onChange}
					disabled={false}
				/>,
			);

			expect(screen.getByRole('button', { name: /add condition/i })).toBeInTheDocument();
			expect(screen.queryByLabelText(/field/i)).not.toBeInTheDocument();
		});
	});

	// ─── SC-9: Raw JSON editor toggle with validation ────────────────────────

	describe('SC-9: raw JSON editor toggle', () => {
		it('toggles from visual builder to raw JSON editor showing formatted JSON', async () => {
			const user = userEvent.setup();

			render(
				<ConditionEditor
					condition='{"field":"task.weight","op":"eq","value":"medium"}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Find and click the JSON/raw toggle
			const jsonToggle = screen.getByRole('button', { name: /json|raw/i });
			await user.click(jsonToggle);

			// Should show a textarea with the current condition JSON
			const textarea = screen.getByRole('textbox');
			expect(textarea).toBeInTheDocument();
			const textValue = (textarea as HTMLTextAreaElement).value;
			const parsed = JSON.parse(textValue);
			expect(parsed).toEqual({
				field: 'task.weight',
				op: 'eq',
				value: 'medium',
			});
		});

		it('syncs valid JSON back to visual builder when switching from JSON to visual mode', async () => {
			const user = userEvent.setup();

			render(
				<ConditionEditor
					condition='{"field":"task.weight","op":"eq","value":"medium"}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Switch to JSON mode
			const jsonToggle = screen.getByRole('button', { name: /json|raw/i });
			await user.click(jsonToggle);

			// Edit the JSON - use fireEvent to avoid curly brace issues with userEvent
			const textarea = screen.getByRole('textbox');
			fireEvent.change(textarea, {
				target: { value: '{"field":"task.category","op":"neq","value":"docs"}' },
			});
			fireEvent.blur(textarea);

			// onChange should have been called with the new JSON
			// Wait for async state updates to complete
			await waitFor(() => {
				expect(onChange).toHaveBeenCalled();
			});
			const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0];
			const parsed = parseConditionJSON(lastCall);
			expect(parsed).toEqual({
				field: 'task.category',
				op: 'neq',
				value: 'docs',
			});
		});

		it('shows error for invalid JSON and does NOT call onChange', async () => {
			const user = userEvent.setup();

			render(
				<ConditionEditor
					condition='{"field":"task.weight","op":"eq","value":"medium"}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Switch to JSON mode
			const jsonToggle = screen.getByRole('button', { name: /json|raw/i });
			await user.click(jsonToggle);

			// Clear onChange calls from mode switch
			onChange.mockClear();

			// Enter invalid JSON
			const textarea = screen.getByRole('textbox');
			await user.clear(textarea);
			await user.type(textarea, '{{invalid json');

			// Blur to trigger validation
			await user.tab();

			// Error message should be visible in the error element
			expect(screen.getByText('Invalid JSON')).toBeInTheDocument();

			// onChange should NOT have been called with invalid JSON
			expect(onChange).not.toHaveBeenCalled();
		});

		it('shows red border on textarea when JSON is invalid', async () => {
			const user = userEvent.setup();

			render(
				<ConditionEditor
					condition='{"field":"task.weight","op":"eq","value":"medium"}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Switch to JSON mode
			const jsonToggle = screen.getByRole('button', { name: /json|raw/i });
			await user.click(jsonToggle);

			// Enter invalid JSON and blur
			const textarea = screen.getByRole('textbox');
			await user.clear(textarea);
			await user.type(textarea, 'not json');
			await user.tab();

			// Textarea should have error styling
			expect(textarea.className).toMatch(/error/i);
		});

		it('stays in JSON mode when switching back to visual with invalid JSON (BDD-4)', async () => {
			const user = userEvent.setup();

			render(
				<ConditionEditor
					condition='{"field":"task.weight","op":"eq","value":"medium"}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Switch to JSON mode
			const jsonToggle = screen.getByRole('button', { name: /json|raw/i });
			await user.click(jsonToggle);

			// Enter invalid JSON
			const textarea = screen.getByRole('textbox');
			await user.clear(textarea);
			await user.type(textarea, '{{bad json');
			await user.tab();

			// Try to switch back to visual mode
			// The toggle should either be disabled, or clicking it should keep JSON mode
			const visualToggle = screen.getByRole('button', { name: /visual|builder/i });
			await user.click(visualToggle);

			// Should still show textarea (stayed in JSON mode due to invalid JSON)
			expect(screen.getByRole('textbox')).toBeInTheDocument();
			expect(screen.getByText(/invalid json/i)).toBeInTheDocument();
		});
	});

	// ─── SC-10: Read-only mode ──────────────────────────────────────────────

	describe('SC-10: read-only (disabled) mode', () => {
		it('does not show "+ Add condition" button when disabled', () => {
			render(
				<ConditionEditor
					condition=""
					onChange={onChange}
					disabled={true}
				/>,
			);

			expect(screen.queryByRole('button', { name: /add condition/i })).not.toBeInTheDocument();
		});

		it('displays existing conditions but all controls are disabled', () => {
			render(
				<ConditionEditor
					condition='{"field":"task.weight","op":"eq","value":"medium"}'
					onChange={onChange}
					disabled={true}
				/>,
			);

			// Field and operator selects should be disabled
			const fieldSelect = screen.getByLabelText(/field/i);
			expect(fieldSelect).toBeDisabled();

			const opSelect = screen.getByLabelText(/operator/i);
			expect(opSelect).toBeDisabled();

			// Value input should be disabled
			const valueInput = screen.getByLabelText(/value/i);
			expect(valueInput).toBeDisabled();
		});

		it('does not show remove buttons when disabled', () => {
			render(
				<ConditionEditor
					condition='{"field":"task.weight","op":"eq","value":"medium"}'
					onChange={onChange}
					disabled={true}
				/>,
			);

			expect(screen.queryByRole('button', { name: /remove/i })).not.toBeInTheDocument();
		});

		it('does not show logic toggle buttons when disabled with compound conditions', () => {
			const conditions = {
				all: [
					{ field: 'task.weight', op: 'eq', value: 'medium' },
					{ field: 'task.category', op: 'neq', value: 'docs' },
				],
			};

			render(
				<ConditionEditor
					condition={JSON.stringify(conditions)}
					onChange={onChange}
					disabled={true}
				/>,
			);

			// Should display conditions but logic toggle should be disabled/hidden
			expect(screen.getAllByLabelText(/field/i).length).toBe(2);

			// Logic toggle buttons should be disabled
			const allToggle = screen.queryByRole('radio', { name: /all/i });
			if (allToggle) {
				expect(allToggle).toBeDisabled();
			}
		});
	});

	// ─── Edge Cases ─────────────────────────────────────────────────────────

	describe('edge cases', () => {
		it('handles empty "in" value array', () => {
			render(
				<ConditionEditor
					condition='{"field":"task.weight","op":"in","value":[]}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Should render the row with empty tag input
			const fieldSelect = screen.getByLabelText(/field/i) as HTMLSelectElement;
			expect(fieldSelect.value).toBe('task.weight');
		});

		it('allows freeform var.* field entry', () => {
			render(
				<ConditionEditor
					condition='{"field":"var.MY_CUSTOM_VAR","op":"eq","value":"test"}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			// Should display the custom var.* field
			const fieldInput = screen.getByLabelText(/field/i);
			expect((fieldInput as HTMLSelectElement | HTMLInputElement).value).toContain('var.MY_CUSTOM_VAR');
		});

		it('allows freeform env.* field entry', async () => {
			render(
				<ConditionEditor
					condition='{"field":"env.SKIP_DOCS","op":"exists"}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			const fieldInput = screen.getByLabelText(/field/i);
			expect((fieldInput as HTMLSelectElement | HTMLInputElement).value).toContain('env.SKIP_DOCS');
		});

		it('renders known value dropdown for task.weight with eq/neq operator', () => {
			render(
				<ConditionEditor
					condition='{"field":"task.weight","op":"eq","value":"medium"}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			// For task.weight + eq operator, value should be a dropdown
			// with known values: trivial, small, medium, large
			const valueControl = screen.getByLabelText(/value/i);
			expect(valueControl).toBeInTheDocument();

			// Check that known values are available as options
			const options = valueControl.querySelectorAll?.('option') ??
				within(valueControl.parentElement!).queryAllByRole('option');
			if (options.length > 0) {
				const optionTexts = Array.from(options).map((o) => o.textContent?.toLowerCase());
				expect(optionTexts).toContain('trivial');
				expect(optionTexts).toContain('small');
				expect(optionTexts).toContain('medium');
				expect(optionTexts).toContain('large');
			}
		});

		it('renders known value dropdown for task.category with eq/neq operator', () => {
			render(
				<ConditionEditor
					condition='{"field":"task.category","op":"eq","value":"feature"}'
					onChange={onChange}
					disabled={false}
				/>,
			);

			const valueControl = screen.getByLabelText(/value/i);
			expect(valueControl).toBeInTheDocument();

			const options = valueControl.querySelectorAll?.('option') ??
				within(valueControl.parentElement!).queryAllByRole('option');
			if (options.length > 0) {
				const optionTexts = Array.from(options).map((o) => o.textContent?.toLowerCase());
				expect(optionTexts).toContain('feature');
				expect(optionTexts).toContain('bug');
				expect(optionTexts).toContain('refactor');
				expect(optionTexts).toContain('chore');
				expect(optionTexts).toContain('docs');
				expect(optionTexts).toContain('test');
			}
		});
	});
});
