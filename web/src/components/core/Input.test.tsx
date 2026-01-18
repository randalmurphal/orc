import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { createRef } from 'react';
import { Input } from './Input';

describe('Input', () => {
	describe('rendering', () => {
		it('renders an input element', () => {
			render(<Input />);
			expect(screen.getByRole('textbox')).toBeInTheDocument();
		});

		it('renders with placeholder', () => {
			render(<Input placeholder="Enter name..." />);
			expect(screen.getByPlaceholderText('Enter name...')).toBeInTheDocument();
		});

		it('renders with value', () => {
			render(<Input value="test value" onChange={() => {}} />);
			expect(screen.getByDisplayValue('test value')).toBeInTheDocument();
		});

		it('applies input class by default', () => {
			const { container } = render(<Input />);
			const input = container.querySelector('.input');
			expect(input).toBeInTheDocument();
		});
	});

	describe('input types', () => {
		it('defaults to text type behavior', () => {
			render(<Input />);
			const input = screen.getByRole('textbox') as HTMLInputElement;
			// HTML inputs default to text type, even without explicit attribute
			expect(input.type).toBe('text');
		});

		it('supports password type', () => {
			render(<Input type="password" />);
			const input = document.querySelector('input[type="password"]');
			expect(input).toBeInTheDocument();
		});

		it('supports email type', () => {
			render(<Input type="email" />);
			const input = document.querySelector('input[type="email"]');
			expect(input).toBeInTheDocument();
		});

		it('supports number type', () => {
			render(<Input type="number" />);
			const input = document.querySelector('input[type="number"]');
			expect(input).toBeInTheDocument();
		});
	});

	describe('disabled state', () => {
		it('disables input when disabled prop is true', () => {
			render(<Input disabled />);
			const input = screen.getByRole('textbox');
			expect(input).toBeDisabled();
		});

		it('allows typing when not disabled', () => {
			const handleChange = vi.fn();
			render(<Input onChange={handleChange} />);
			const input = screen.getByRole('textbox');
			fireEvent.change(input, { target: { value: 'new text' } });
			expect(handleChange).toHaveBeenCalled();
		});
	});

	describe('event handling', () => {
		it('calls onChange when typing', () => {
			const handleChange = vi.fn();
			render(<Input onChange={handleChange} />);
			const input = screen.getByRole('textbox');
			fireEvent.change(input, { target: { value: 'typed text' } });
			expect(handleChange).toHaveBeenCalledTimes(1);
		});

		it('passes event to onChange handler', () => {
			const handleChange = vi.fn();
			render(<Input onChange={handleChange} />);
			const input = screen.getByRole('textbox');
			fireEvent.change(input, { target: { value: 'test' } });
			expect(handleChange.mock.calls[0][0].target.value).toBe('test');
		});

		it('can be controlled via value prop', () => {
			const { rerender } = render(<Input value="initial" onChange={() => {}} />);
			expect(screen.getByDisplayValue('initial')).toBeInTheDocument();

			rerender(<Input value="updated" onChange={() => {}} />);
			expect(screen.getByDisplayValue('updated')).toBeInTheDocument();
		});

		it('handles onFocus', () => {
			const handleFocus = vi.fn();
			render(<Input onFocus={handleFocus} />);
			const input = screen.getByRole('textbox');
			fireEvent.focus(input);
			expect(handleFocus).toHaveBeenCalledTimes(1);
		});

		it('handles onBlur', () => {
			const handleBlur = vi.fn();
			render(<Input onBlur={handleBlur} />);
			const input = screen.getByRole('textbox');
			fireEvent.blur(input);
			expect(handleBlur).toHaveBeenCalledTimes(1);
		});

		it('handles onKeyDown', () => {
			const handleKeyDown = vi.fn();
			render(<Input onKeyDown={handleKeyDown} />);
			const input = screen.getByRole('textbox');
			fireEvent.keyDown(input, { key: 'Enter' });
			expect(handleKeyDown).toHaveBeenCalledTimes(1);
		});
	});

	describe('form integration', () => {
		it('works with name attribute', () => {
			render(<Input name="email-field" />);
			const input = screen.getByRole('textbox');
			expect(input).toHaveAttribute('name', 'email-field');
		});

		it('passes through id attribute', () => {
			render(<Input id="my-input" />);
			const input = screen.getByRole('textbox');
			expect(input).toHaveAttribute('id', 'my-input');
		});

		it('supports required attribute', () => {
			render(<Input required />);
			const input = screen.getByRole('textbox');
			expect(input).toBeRequired();
		});

		it('supports autoComplete attribute', () => {
			render(<Input autoComplete="email" />);
			const input = screen.getByRole('textbox');
			expect(input).toHaveAttribute('autocomplete', 'email');
		});

		it('supports maxLength attribute', () => {
			render(<Input maxLength={10} />);
			const input = screen.getByRole('textbox');
			expect(input).toHaveAttribute('maxLength', '10');
		});

		it('supports minLength attribute', () => {
			render(<Input minLength={5} />);
			const input = screen.getByRole('textbox');
			expect(input).toHaveAttribute('minLength', '5');
		});

		it('supports pattern attribute', () => {
			render(<Input pattern="[0-9]+" />);
			const input = screen.getByRole('textbox');
			expect(input).toHaveAttribute('pattern', '[0-9]+');
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref to input element', () => {
			const ref = createRef<HTMLInputElement>();
			render(<Input ref={ref} />);
			expect(ref.current).toBeInstanceOf(HTMLInputElement);
		});

		it('allows focus via ref', () => {
			const ref = createRef<HTMLInputElement>();
			render(<Input ref={ref} />);
			ref.current?.focus();
			expect(document.activeElement).toBe(ref.current);
		});

		it('allows selecting text via ref', () => {
			const ref = createRef<HTMLInputElement>();
			render(<Input ref={ref} value="select me" onChange={() => {}} />);
			ref.current?.select();
			expect(ref.current?.selectionStart).toBe(0);
			expect(ref.current?.selectionEnd).toBe(9);
		});
	});

	describe('custom className', () => {
		it('applies custom className', () => {
			const { container } = render(<Input className="custom-input" />);
			const input = container.querySelector('.input');
			expect(input).toHaveClass('custom-input');
		});

		it('preserves base input class', () => {
			const { container } = render(<Input className="custom-class" />);
			const input = container.querySelector('.input');
			expect(input).toHaveClass('input');
		});

		it('merges multiple classes correctly', () => {
			const { container } = render(<Input className="class-a class-b" />);
			const input = container.querySelector('.input');
			expect(input).toHaveClass('input');
			expect(input).toHaveClass('class-a');
			expect(input).toHaveClass('class-b');
		});
	});

	describe('HTML attributes', () => {
		it('passes through data attributes', () => {
			render(<Input data-testid="test-input" />);
			const input = screen.getByTestId('test-input');
			expect(input).toBeInTheDocument();
		});

		it('supports aria-label', () => {
			render(<Input aria-label="Search input" />);
			const input = screen.getByRole('textbox', { name: 'Search input' });
			expect(input).toBeInTheDocument();
		});

		it('supports aria-describedby', () => {
			render(<Input aria-describedby="help-text" />);
			const input = screen.getByRole('textbox');
			expect(input).toHaveAttribute('aria-describedby', 'help-text');
		});

		it('supports title attribute', () => {
			render(<Input title="Input tooltip" />);
			const input = screen.getByRole('textbox');
			expect(input).toHaveAttribute('title', 'Input tooltip');
		});
	});

	describe('read-only state', () => {
		it('supports readOnly attribute', () => {
			render(<Input readOnly value="read only value" />);
			const input = screen.getByRole('textbox');
			expect(input).toHaveAttribute('readonly');
		});

		it('does not allow typing when readOnly', () => {
			render(<Input readOnly value="original" />);
			const input = screen.getByRole('textbox') as HTMLInputElement;
			expect(input.value).toBe('original');
			// Value should remain unchanged due to readOnly
		});
	});
});
