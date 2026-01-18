import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { createRef } from 'react';
import { SearchInput } from './SearchInput';

describe('SearchInput', () => {
	describe('rendering', () => {
		it('renders with search icon', () => {
			const { container } = render(<SearchInput />);
			const icon = container.querySelector('.search-input__icon');
			expect(icon).toBeInTheDocument();
		});

		it('renders with default placeholder', () => {
			render(<SearchInput />);
			expect(screen.getByPlaceholderText('Search...')).toBeInTheDocument();
		});

		it('renders with custom placeholder', () => {
			render(<SearchInput placeholder="Find tasks..." />);
			expect(screen.getByPlaceholderText('Find tasks...')).toBeInTheDocument();
		});

		it('renders with value', () => {
			render(<SearchInput value="test query" onChange={() => {}} />);
			expect(screen.getByDisplayValue('test query')).toBeInTheDocument();
		});
	});

	describe('clear button visibility', () => {
		it('clear button is hidden when input is empty', () => {
			const { container } = render(<SearchInput value="" onChange={() => {}} />);
			const clearBtn = container.querySelector('.search-input__clear');
			expect(clearBtn).not.toHaveClass('search-input__clear--visible');
		});

		it('clear button is visible when input has value', () => {
			const { container } = render(
				<SearchInput value="some text" onChange={() => {}} />
			);
			const clearBtn = container.querySelector('.search-input__clear');
			expect(clearBtn).toHaveClass('search-input__clear--visible');
		});

		it('clear button becomes visible after typing', () => {
			const handleChange = vi.fn();
			const { container, rerender } = render(
				<SearchInput value="" onChange={handleChange} />
			);

			// Initially hidden
			let clearBtn = container.querySelector('.search-input__clear');
			expect(clearBtn).not.toHaveClass('search-input__clear--visible');

			// Simulate value update
			rerender(<SearchInput value="typed" onChange={handleChange} />);

			// Now visible
			clearBtn = container.querySelector('.search-input__clear');
			expect(clearBtn).toHaveClass('search-input__clear--visible');
		});
	});

	describe('clear functionality', () => {
		it('clicking clear button calls onChange with empty string', () => {
			const handleChange = vi.fn();
			const { container } = render(
				<SearchInput value="some text" onChange={handleChange} />
			);

			const clearBtn = container.querySelector('.search-input__clear');
			fireEvent.click(clearBtn!);

			expect(handleChange).toHaveBeenCalledWith('');
		});

		it('clicking clear button calls onClear callback', () => {
			const handleClear = vi.fn();
			const { container } = render(
				<SearchInput value="text" onChange={() => {}} onClear={handleClear} />
			);

			const clearBtn = container.querySelector('.search-input__clear');
			fireEvent.click(clearBtn!);

			expect(handleClear).toHaveBeenCalledTimes(1);
		});

		it('clear button is disabled when input is empty', () => {
			const { container } = render(<SearchInput value="" onChange={() => {}} />);
			const clearBtn = container.querySelector('.search-input__clear');
			expect(clearBtn).toBeDisabled();
		});

		it('clear button is enabled when input has value', () => {
			const { container } = render(
				<SearchInput value="text" onChange={() => {}} />
			);
			const clearBtn = container.querySelector('.search-input__clear');
			expect(clearBtn).not.toBeDisabled();
		});
	});

	describe('Escape key clears input', () => {
		it('pressing Escape clears the input when it has value', () => {
			const handleChange = vi.fn();
			render(<SearchInput value="some query" onChange={handleChange} />);

			const input = screen.getByRole('textbox');
			fireEvent.keyDown(input, { key: 'Escape' });

			expect(handleChange).toHaveBeenCalledWith('');
		});

		it('pressing Escape calls onClear callback', () => {
			const handleClear = vi.fn();
			render(
				<SearchInput value="text" onChange={() => {}} onClear={handleClear} />
			);

			const input = screen.getByRole('textbox');
			fireEvent.keyDown(input, { key: 'Escape' });

			expect(handleClear).toHaveBeenCalledTimes(1);
		});

		it('pressing Escape does nothing when input is empty', () => {
			const handleChange = vi.fn();
			const handleClear = vi.fn();
			render(
				<SearchInput value="" onChange={handleChange} onClear={handleClear} />
			);

			const input = screen.getByRole('textbox');
			fireEvent.keyDown(input, { key: 'Escape' });

			expect(handleChange).not.toHaveBeenCalled();
			expect(handleClear).not.toHaveBeenCalled();
		});
	});

	describe('form integration', () => {
		it('works with name attribute', () => {
			render(<SearchInput name="search-field" />);
			const input = screen.getByRole('textbox');
			expect(input).toHaveAttribute('name', 'search-field');
		});

		it('is of type text', () => {
			render(<SearchInput />);
			const input = screen.getByRole('textbox');
			expect(input).toHaveAttribute('type', 'text');
		});

		it('passes through id attribute', () => {
			render(<SearchInput id="my-search" />);
			const input = screen.getByRole('textbox');
			expect(input).toHaveAttribute('id', 'my-search');
		});
	});

	describe('disabled state', () => {
		it('applies disabled attribute to input', () => {
			render(<SearchInput disabled />);
			const input = screen.getByRole('textbox');
			expect(input).toBeDisabled();
		});

		it('clear button is disabled when input is disabled', () => {
			const { container } = render(
				<SearchInput value="text" onChange={() => {}} disabled />
			);
			const clearBtn = container.querySelector('.search-input__clear');
			expect(clearBtn).toBeDisabled();
		});
	});

	describe('event handling', () => {
		it('calls onChange with new value when typing', () => {
			const handleChange = vi.fn();
			render(<SearchInput value="" onChange={handleChange} />);

			const input = screen.getByRole('textbox');
			fireEvent.change(input, { target: { value: 'new search' } });

			expect(handleChange).toHaveBeenCalledWith('new search');
		});

		it('can be controlled via value prop', () => {
			const { rerender } = render(
				<SearchInput value="initial" onChange={() => {}} />
			);
			expect(screen.getByDisplayValue('initial')).toBeInTheDocument();

			rerender(<SearchInput value="updated" onChange={() => {}} />);
			expect(screen.getByDisplayValue('updated')).toBeInTheDocument();
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref to input element', () => {
			const ref = createRef<HTMLInputElement>();
			render(<SearchInput ref={ref} />);
			expect(ref.current).toBeInstanceOf(HTMLInputElement);
		});

		it('allows focus via ref', () => {
			const ref = createRef<HTMLInputElement>();
			render(<SearchInput ref={ref} />);
			ref.current?.focus();
			expect(document.activeElement).toBe(ref.current);
		});
	});

	describe('accessibility', () => {
		it('clear button has aria-label', () => {
			const { container } = render(
				<SearchInput value="text" onChange={() => {}} />
			);
			const clearBtn = container.querySelector('.search-input__clear');
			expect(clearBtn).toHaveAttribute('aria-label', 'Clear search');
		});

		it('search icon is hidden from screen readers', () => {
			const { container } = render(<SearchInput />);
			const icon = container.querySelector('.search-input__icon');
			expect(icon).toHaveAttribute('aria-hidden', 'true');
		});

		it('clear button is not focusable when hidden', () => {
			const { container } = render(<SearchInput value="" onChange={() => {}} />);
			const clearBtn = container.querySelector('.search-input__clear');
			expect(clearBtn).toHaveAttribute('tabIndex', '-1');
		});

		it('clear button is focusable when visible', () => {
			const { container } = render(
				<SearchInput value="text" onChange={() => {}} />
			);
			const clearBtn = container.querySelector('.search-input__clear');
			expect(clearBtn).toHaveAttribute('tabIndex', '0');
		});
	});

	describe('className prop', () => {
		it('applies additional className to wrapper', () => {
			const { container } = render(<SearchInput className="custom-search" />);
			const wrapper = container.querySelector('.search-input');
			expect(wrapper).toHaveClass('custom-search');
		});
	});
});
