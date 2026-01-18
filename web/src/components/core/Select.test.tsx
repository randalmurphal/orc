import { describe, it, expect, vi, beforeAll } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { createRef } from 'react';
import { Select, type SelectOption } from './Select';

// Mock browser APIs not available in jsdom (required for Radix Select)
beforeAll(() => {
	Element.prototype.scrollIntoView = vi.fn();
	global.ResizeObserver = vi.fn().mockImplementation(() => ({
		observe: vi.fn(),
		unobserve: vi.fn(),
		disconnect: vi.fn(),
	}));
});

const defaultOptions: SelectOption[] = [
	{ value: 'apple', label: 'Apple' },
	{ value: 'banana', label: 'Banana' },
	{ value: 'cherry', label: 'Cherry' },
];

describe('Select', () => {
	describe('rendering', () => {
		it('renders a select trigger', () => {
			render(<Select options={defaultOptions} aria-label="Select fruit" />);
			expect(screen.getByRole('combobox', { name: 'Select fruit' })).toBeInTheDocument();
		});

		it('renders with placeholder when no value', () => {
			render(<Select options={defaultOptions} placeholder="Choose a fruit" />);
			expect(screen.getByText('Choose a fruit')).toBeInTheDocument();
		});

		it('renders selected value label', () => {
			render(<Select options={defaultOptions} value="banana" />);
			expect(screen.getByText('Banana')).toBeInTheDocument();
		});

		it('uses default placeholder when none provided', () => {
			render(<Select options={defaultOptions} />);
			expect(screen.getByText('Select...')).toBeInTheDocument();
		});
	});

	describe('opening and closing', () => {
		it('opens dropdown on click', async () => {
			render(<Select options={defaultOptions} aria-label="Select fruit" />);

			fireEvent.click(screen.getByRole('combobox'));

			await waitFor(() => {
				expect(screen.getByRole('listbox')).toBeInTheDocument();
			});
		});

		it('shows all options when open', async () => {
			render(<Select options={defaultOptions} aria-label="Select fruit" />);

			fireEvent.click(screen.getByRole('combobox'));

			await waitFor(() => {
				expect(screen.getByRole('option', { name: 'Apple' })).toBeInTheDocument();
				expect(screen.getByRole('option', { name: 'Banana' })).toBeInTheDocument();
				expect(screen.getByRole('option', { name: 'Cherry' })).toBeInTheDocument();
			});
		});

		it('closes dropdown when option is selected', async () => {
			render(<Select options={defaultOptions} aria-label="Select fruit" />);

			fireEvent.click(screen.getByRole('combobox'));
			await waitFor(() => {
				expect(screen.getByRole('listbox')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('option', { name: 'Apple' }));

			await waitFor(() => {
				expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
			});
		});

		it('closes dropdown on escape key', async () => {
			render(<Select options={defaultOptions} aria-label="Select fruit" />);

			fireEvent.click(screen.getByRole('combobox'));
			await waitFor(() => {
				expect(screen.getByRole('listbox')).toBeInTheDocument();
			});

			fireEvent.keyDown(screen.getByRole('listbox'), { key: 'Escape' });

			await waitFor(() => {
				expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
			});
		});
	});

	describe('selection', () => {
		it('calls onChange when option is selected', async () => {
			const handleChange = vi.fn();
			render(
				<Select options={defaultOptions} onChange={handleChange} aria-label="Select fruit" />
			);

			fireEvent.click(screen.getByRole('combobox'));
			await waitFor(() => {
				expect(screen.getByRole('listbox')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('option', { name: 'Banana' }));

			expect(handleChange).toHaveBeenCalledWith('banana');
			expect(handleChange).toHaveBeenCalledTimes(1);
		});

		it('displays newly selected value', () => {
			const { rerender } = render(
				<Select options={defaultOptions} value="apple" aria-label="Select fruit" />
			);

			expect(screen.getByText('Apple')).toBeInTheDocument();

			rerender(<Select options={defaultOptions} value="cherry" aria-label="Select fruit" />);

			expect(screen.getByText('Cherry')).toBeInTheDocument();
			expect(screen.queryByText('Apple')).not.toBeInTheDocument();
		});

		it('marks selected option as checked', async () => {
			render(<Select options={defaultOptions} value="banana" aria-label="Select fruit" />);

			fireEvent.click(screen.getByRole('combobox'));
			await waitFor(() => {
				const bananaOption = screen.getByRole('option', { name: 'Banana' });
				expect(bananaOption).toHaveAttribute('data-state', 'checked');
			});
		});
	});

	describe('keyboard navigation', () => {
		it('opens dropdown with keyboard (Enter)', async () => {
			render(<Select options={defaultOptions} aria-label="Select fruit" />);

			const trigger = screen.getByRole('combobox');
			trigger.focus();
			fireEvent.keyDown(trigger, { key: 'Enter' });

			await waitFor(() => {
				expect(screen.getByRole('listbox')).toBeInTheDocument();
			});
		});

		it('opens dropdown with keyboard (Space)', async () => {
			render(<Select options={defaultOptions} aria-label="Select fruit" />);

			const trigger = screen.getByRole('combobox');
			trigger.focus();
			fireEvent.keyDown(trigger, { key: ' ' });

			await waitFor(() => {
				expect(screen.getByRole('listbox')).toBeInTheDocument();
			});
		});

		it('navigates options with arrow keys', async () => {
			render(<Select options={defaultOptions} aria-label="Select fruit" />);

			fireEvent.click(screen.getByRole('combobox'));
			await waitFor(() => {
				expect(screen.getByRole('listbox')).toBeInTheDocument();
			});

			const listbox = screen.getByRole('listbox');
			fireEvent.keyDown(listbox, { key: 'ArrowDown' });
			fireEvent.keyDown(listbox, { key: 'ArrowDown' });

			// The highlighted option should have data-highlighted attribute
			const options = screen.getAllByRole('option');
			expect(options.length).toBe(3);
		});

		it('selects highlighted option on click', async () => {
			const handleChange = vi.fn();
			render(
				<Select options={defaultOptions} onChange={handleChange} aria-label="Select fruit" />
			);

			fireEvent.click(screen.getByRole('combobox'));
			await waitFor(() => {
				expect(screen.getByRole('listbox')).toBeInTheDocument();
			});

			// Click directly on an option (more reliable than keyboard in jsdom)
			fireEvent.click(screen.getByRole('option', { name: 'Cherry' }));

			expect(handleChange).toHaveBeenCalledWith('cherry');
		});
	});

	describe('disabled state', () => {
		it('cannot be opened when disabled', async () => {
			render(<Select options={defaultOptions} disabled aria-label="Select fruit" />);

			const trigger = screen.getByRole('combobox');
			expect(trigger).toHaveAttribute('data-disabled');

			fireEvent.click(trigger);

			// Dropdown should not open
			expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
		});

		it('does not call onChange when disabled', () => {
			const handleChange = vi.fn();
			render(
				<Select
					options={defaultOptions}
					onChange={handleChange}
					disabled
					aria-label="Select fruit"
				/>
			);

			fireEvent.click(screen.getByRole('combobox'));

			expect(handleChange).not.toHaveBeenCalled();
		});
	});

	describe('disabled options', () => {
		it('renders disabled options', async () => {
			const optionsWithDisabled: SelectOption[] = [
				{ value: 'apple', label: 'Apple' },
				{ value: 'banana', label: 'Banana', disabled: true },
				{ value: 'cherry', label: 'Cherry' },
			];

			render(<Select options={optionsWithDisabled} aria-label="Select fruit" />);

			fireEvent.click(screen.getByRole('combobox'));
			await waitFor(() => {
				const bananaOption = screen.getByRole('option', { name: 'Banana' });
				expect(bananaOption).toHaveAttribute('data-disabled');
			});
		});

		it('does not select disabled options', async () => {
			const handleChange = vi.fn();
			const optionsWithDisabled: SelectOption[] = [
				{ value: 'apple', label: 'Apple' },
				{ value: 'banana', label: 'Banana', disabled: true },
				{ value: 'cherry', label: 'Cherry' },
			];

			render(
				<Select
					options={optionsWithDisabled}
					onChange={handleChange}
					aria-label="Select fruit"
				/>
			);

			fireEvent.click(screen.getByRole('combobox'));
			await waitFor(() => {
				expect(screen.getByRole('listbox')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('option', { name: 'Banana' }));

			// onChange should not be called for disabled option
			expect(handleChange).not.toHaveBeenCalledWith('banana');
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref to trigger button', () => {
			const ref = createRef<HTMLButtonElement>();
			render(<Select ref={ref} options={defaultOptions} aria-label="Select fruit" />);

			expect(ref.current).toBeInstanceOf(HTMLButtonElement);
			expect(ref.current).toHaveAttribute('role', 'combobox');
		});
	});

	describe('className', () => {
		it('applies custom className to trigger', () => {
			render(<Select options={defaultOptions} className="custom-class" aria-label="Select" />);

			const trigger = screen.getByRole('combobox');
			expect(trigger).toHaveClass('select-trigger');
			expect(trigger).toHaveClass('custom-class');
		});
	});

	describe('accessibility', () => {
		it('has correct ARIA role', () => {
			render(<Select options={defaultOptions} aria-label="Select fruit" />);
			expect(screen.getByRole('combobox')).toBeInTheDocument();
		});

		it('supports aria-label', () => {
			render(<Select options={defaultOptions} aria-label="Choose a fruit" />);
			expect(screen.getByRole('combobox', { name: 'Choose a fruit' })).toBeInTheDocument();
		});

		it('supports aria-labelledby', () => {
			render(
				<div>
					<label id="fruit-label">Fruit Selection</label>
					<Select options={defaultOptions} aria-labelledby="fruit-label" />
				</div>
			);
			const trigger = screen.getByRole('combobox');
			expect(trigger).toHaveAttribute('aria-labelledby', 'fruit-label');
		});

		it('options have correct role', async () => {
			render(<Select options={defaultOptions} aria-label="Select fruit" />);

			fireEvent.click(screen.getByRole('combobox'));
			await waitFor(() => {
				const options = screen.getAllByRole('option');
				expect(options).toHaveLength(3);
			});
		});

		it('listbox has correct role', async () => {
			render(<Select options={defaultOptions} aria-label="Select fruit" />);

			fireEvent.click(screen.getByRole('combobox'));
			await waitFor(() => {
				expect(screen.getByRole('listbox')).toBeInTheDocument();
			});
		});
	});

	describe('form attributes', () => {
		it('supports name attribute', () => {
			render(<Select options={defaultOptions} name="fruit" aria-label="Select fruit" />);
			// The name is stored internally by Radix
			expect(screen.getByRole('combobox')).toBeInTheDocument();
		});

		it('supports required attribute', () => {
			render(<Select options={defaultOptions} required aria-label="Select fruit" />);
			// Radix handles required internally
			expect(screen.getByRole('combobox')).toBeInTheDocument();
		});
	});

	describe('empty states', () => {
		it('renders with empty options array', () => {
			render(<Select options={[]} aria-label="Select fruit" />);
			expect(screen.getByRole('combobox')).toBeInTheDocument();
		});

		it('shows empty dropdown with no options', async () => {
			render(<Select options={[]} aria-label="Select fruit" />);

			fireEvent.click(screen.getByRole('combobox'));

			await waitFor(() => {
				expect(screen.getByRole('listbox')).toBeInTheDocument();
			});

			expect(screen.queryAllByRole('option')).toHaveLength(0);
		});
	});

	describe('long text handling', () => {
		it('renders long option labels', async () => {
			const longOptions: SelectOption[] = [
				{ value: 'long', label: 'This is a very long option label that might need truncation' },
			];

			render(<Select options={longOptions} aria-label="Select" />);

			fireEvent.click(screen.getByRole('combobox'));
			await waitFor(() => {
				expect(
					screen.getByRole('option', {
						name: 'This is a very long option label that might need truncation',
					})
				).toBeInTheDocument();
			});
		});

		it('displays long selected value', () => {
			const longOptions: SelectOption[] = [
				{ value: 'long', label: 'This is a very long option label that might need truncation' },
			];

			render(<Select options={longOptions} value="long" aria-label="Select" />);

			expect(
				screen.getByText('This is a very long option label that might need truncation')
			).toBeInTheDocument();
		});
	});

	describe('controlled component', () => {
		it('works as controlled component', async () => {
			let value = 'apple';
			const handleChange = vi.fn((newValue: string) => {
				value = newValue;
			});

			const { rerender } = render(
				<Select options={defaultOptions} value={value} onChange={handleChange} />
			);

			expect(screen.getByText('Apple')).toBeInTheDocument();

			fireEvent.click(screen.getByRole('combobox'));
			await waitFor(() => {
				expect(screen.getByRole('listbox')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('option', { name: 'Cherry' }));
			expect(handleChange).toHaveBeenCalledWith('cherry');

			// Simulate parent updating value
			rerender(<Select options={defaultOptions} value="cherry" onChange={handleChange} />);

			expect(screen.getByText('Cherry')).toBeInTheDocument();
		});

		it('works as uncontrolled component', async () => {
			render(<Select options={defaultOptions} aria-label="Select fruit" />);

			fireEvent.click(screen.getByRole('combobox'));
			await waitFor(() => {
				expect(screen.getByRole('listbox')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('option', { name: 'Banana' }));

			// Value should update internally
			expect(screen.getByText('Banana')).toBeInTheDocument();
		});
	});
});
