import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { createRef } from 'react';
import { Input } from './Input';
import { Icon } from './Icon';

describe('Input', () => {
	describe('rendering', () => {
		it('renders with default variant', () => {
			const { container } = render(<Input />);
			const wrapper = container.querySelector('.input-wrapper');
			expect(wrapper).toHaveClass('input-variant-default');
		});

		it('renders with error variant', () => {
			const { container } = render(<Input variant="error" />);
			const wrapper = container.querySelector('.input-wrapper');
			expect(wrapper).toHaveClass('input-variant-error');
		});

		it('renders placeholder text', () => {
			render(<Input placeholder="Enter your name" />);
			expect(screen.getByPlaceholderText('Enter your name')).toBeInTheDocument();
		});

		it('renders with value', () => {
			render(<Input value="test value" onChange={() => {}} />);
			expect(screen.getByDisplayValue('test value')).toBeInTheDocument();
		});
	});

	describe('sizes', () => {
		it('renders small size correctly', () => {
			const { container } = render(<Input size="sm" />);
			const wrapper = container.querySelector('.input-wrapper');
			expect(wrapper).toHaveClass('input-size-sm');
		});

		it('renders medium size correctly (default)', () => {
			const { container } = render(<Input />);
			const wrapper = container.querySelector('.input-wrapper');
			expect(wrapper).toHaveClass('input-size-md');
		});

		it('renders large size correctly', () => {
			const { container } = render(<Input size="lg" />);
			const wrapper = container.querySelector('.input-wrapper');
			expect(wrapper).toHaveClass('input-size-lg');
		});
	});

	describe('disabled state', () => {
		it('shows correct styling when disabled', () => {
			const { container } = render(<Input disabled />);
			const wrapper = container.querySelector('.input-wrapper');
			expect(wrapper).toHaveClass('input-disabled');
		});

		it('applies disabled attribute to input element', () => {
			render(<Input disabled />);
			const input = screen.getByRole('textbox');
			expect(input).toBeDisabled();
		});
	});

	describe('event handling', () => {
		it('calls onChange handler when value changes', () => {
			const handleChange = vi.fn();
			render(<Input onChange={handleChange} />);
			const input = screen.getByRole('textbox');
			fireEvent.change(input, { target: { value: 'new value' } });
			expect(handleChange).toHaveBeenCalledTimes(1);
		});

		it('can be controlled (value prop works)', () => {
			const { rerender } = render(<Input value="initial" onChange={() => {}} />);
			expect(screen.getByDisplayValue('initial')).toBeInTheDocument();

			rerender(<Input value="updated" onChange={() => {}} />);
			expect(screen.getByDisplayValue('updated')).toBeInTheDocument();
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref correctly', () => {
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
	});

	describe('standard input props', () => {
		it('passes through type prop', () => {
			render(<Input type="password" />);
			const input = document.querySelector('input');
			expect(input).toHaveAttribute('type', 'password');
		});

		it('passes through name prop', () => {
			render(<Input name="username" />);
			const input = document.querySelector('input');
			expect(input).toHaveAttribute('name', 'username');
		});

		it('passes through id prop', () => {
			render(<Input id="my-input" />);
			const input = document.querySelector('input');
			expect(input).toHaveAttribute('id', 'my-input');
		});

		it('passes through maxLength prop', () => {
			render(<Input maxLength={10} />);
			const input = document.querySelector('input');
			expect(input).toHaveAttribute('maxLength', '10');
		});

		it('passes through autoComplete prop', () => {
			render(<Input autoComplete="email" />);
			const input = document.querySelector('input');
			expect(input).toHaveAttribute('autoComplete', 'email');
		});
	});

	describe('icons', () => {
		it('renders left icon correctly', () => {
			const { container } = render(<Input leftIcon={<Icon name="search" />} />);
			const leftIcon = container.querySelector('.input-icon-left');
			expect(leftIcon).toBeInTheDocument();
			expect(container.querySelector('.input-wrapper')).toHaveClass('has-left-icon');
		});

		it('renders right icon correctly', () => {
			const { container } = render(<Input rightIcon={<Icon name="check" />} />);
			const rightIcon = container.querySelector('.input-icon-right');
			expect(rightIcon).toBeInTheDocument();
			expect(container.querySelector('.input-wrapper')).toHaveClass('has-right-icon');
		});

		it('renders both icons simultaneously', () => {
			const { container } = render(
				<Input leftIcon={<Icon name="search" />} rightIcon={<Icon name="close" />} />
			);
			expect(container.querySelector('.input-icon-left')).toBeInTheDocument();
			expect(container.querySelector('.input-icon-right')).toBeInTheDocument();
		});
	});

	describe('error message', () => {
		it('displays error message', () => {
			render(<Input error="This field is required" />);
			expect(screen.getByText('This field is required')).toBeInTheDocument();
		});

		it('sets aria-describedby linking to error element', () => {
			render(<Input id="test-input" error="Invalid input" />);
			const input = screen.getByRole('textbox');
			expect(input).toHaveAttribute('aria-describedby', 'test-input-error');
			expect(screen.getByText('Invalid input')).toHaveAttribute('id', 'test-input-error');
		});

		it('error prop overrides variant to error', () => {
			const { container } = render(<Input variant="default" error="Error message" />);
			const wrapper = container.querySelector('.input-wrapper');
			expect(wrapper).toHaveClass('input-variant-error');
		});

		it('error message has role="alert" for accessibility', () => {
			render(<Input error="Error occurred" />);
			const errorMessage = screen.getByRole('alert');
			expect(errorMessage).toHaveTextContent('Error occurred');
		});

		it('preserves existing aria-describedby when error is present', () => {
			render(
				<Input
					id="test-input"
					aria-describedby="help-text"
					error="Error message"
				/>
			);
			const input = screen.getByRole('textbox');
			expect(input.getAttribute('aria-describedby')).toContain('help-text');
			expect(input.getAttribute('aria-describedby')).toContain('test-input-error');
		});
	});

	describe('accessibility', () => {
		it('sets aria-invalid="true" for error state', () => {
			render(<Input variant="error" />);
			const input = screen.getByRole('textbox');
			expect(input).toHaveAttribute('aria-invalid', 'true');
		});

		it('does not set aria-invalid for default state', () => {
			render(<Input variant="default" />);
			const input = screen.getByRole('textbox');
			expect(input).not.toHaveAttribute('aria-invalid');
		});

		it('sets aria-required="true" for required inputs', () => {
			render(<Input required />);
			const input = screen.getByRole('textbox');
			expect(input).toHaveAttribute('aria-required', 'true');
		});

		it('does not set aria-required for non-required inputs', () => {
			render(<Input />);
			const input = screen.getByRole('textbox');
			expect(input).not.toHaveAttribute('aria-required');
		});
	});

	describe('className prop', () => {
		it('applies additional className to wrapper', () => {
			const { container } = render(<Input className="custom-class" />);
			const wrapper = container.querySelector('.input-wrapper');
			expect(wrapper).toHaveClass('custom-class');
		});
	});
});
