import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { createRef } from 'react';
import { Textarea } from './Textarea';

describe('Textarea', () => {
	describe('rendering', () => {
		it('renders with default variant', () => {
			const { container } = render(<Textarea />);
			const wrapper = container.querySelector('.textarea-wrapper');
			expect(wrapper).toHaveClass('textarea-variant-default');
		});

		it('renders with error variant', () => {
			const { container } = render(<Textarea variant="error" />);
			const wrapper = container.querySelector('.textarea-wrapper');
			expect(wrapper).toHaveClass('textarea-variant-error');
		});

		it('renders placeholder text', () => {
			render(<Textarea placeholder="Enter description..." />);
			expect(screen.getByPlaceholderText('Enter description...')).toBeInTheDocument();
		});

		it('renders with value', () => {
			render(<Textarea value="test value" onChange={() => {}} />);
			expect(screen.getByDisplayValue('test value')).toBeInTheDocument();
		});
	});

	describe('sizes', () => {
		it('renders small size correctly', () => {
			const { container } = render(<Textarea size="sm" />);
			const wrapper = container.querySelector('.textarea-wrapper');
			expect(wrapper).toHaveClass('textarea-size-sm');
		});

		it('renders medium size correctly (default)', () => {
			const { container } = render(<Textarea />);
			const wrapper = container.querySelector('.textarea-wrapper');
			expect(wrapper).toHaveClass('textarea-size-md');
		});

		it('renders large size correctly', () => {
			const { container } = render(<Textarea size="lg" />);
			const wrapper = container.querySelector('.textarea-wrapper');
			expect(wrapper).toHaveClass('textarea-size-lg');
		});
	});

	describe('resize behavior', () => {
		it('renders with default vertical resize', () => {
			const { container } = render(<Textarea />);
			const wrapper = container.querySelector('.textarea-wrapper');
			expect(wrapper).toHaveClass('textarea-resize-vertical');
		});

		it('renders with resize="none"', () => {
			const { container } = render(<Textarea resize="none" />);
			const wrapper = container.querySelector('.textarea-wrapper');
			expect(wrapper).toHaveClass('textarea-resize-none');
		});

		it('renders with resize="horizontal"', () => {
			const { container } = render(<Textarea resize="horizontal" />);
			const wrapper = container.querySelector('.textarea-wrapper');
			expect(wrapper).toHaveClass('textarea-resize-horizontal');
		});

		it('renders with resize="both"', () => {
			const { container } = render(<Textarea resize="both" />);
			const wrapper = container.querySelector('.textarea-wrapper');
			expect(wrapper).toHaveClass('textarea-resize-both');
		});
	});

	describe('disabled state', () => {
		it('shows correct styling when disabled', () => {
			const { container } = render(<Textarea disabled />);
			const wrapper = container.querySelector('.textarea-wrapper');
			expect(wrapper).toHaveClass('textarea-disabled');
		});

		it('applies disabled attribute to textarea element', () => {
			render(<Textarea disabled />);
			const textarea = screen.getByRole('textbox');
			expect(textarea).toBeDisabled();
		});
	});

	describe('event handling', () => {
		it('calls onChange handler when value changes', () => {
			const handleChange = vi.fn();
			render(<Textarea onChange={handleChange} />);
			const textarea = screen.getByRole('textbox');
			fireEvent.change(textarea, { target: { value: 'new value' } });
			expect(handleChange).toHaveBeenCalledTimes(1);
		});

		it('can be controlled (value prop works)', () => {
			const { rerender } = render(<Textarea value="initial" onChange={() => {}} />);
			expect(screen.getByDisplayValue('initial')).toBeInTheDocument();

			rerender(<Textarea value="updated" onChange={() => {}} />);
			expect(screen.getByDisplayValue('updated')).toBeInTheDocument();
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref correctly', () => {
			const ref = createRef<HTMLTextAreaElement>();
			render(<Textarea ref={ref} />);
			expect(ref.current).toBeInstanceOf(HTMLTextAreaElement);
		});

		it('allows focus via ref', () => {
			const ref = createRef<HTMLTextAreaElement>();
			render(<Textarea ref={ref} />);
			ref.current?.focus();
			expect(document.activeElement).toBe(ref.current);
		});
	});

	describe('standard textarea props', () => {
		it('passes through name prop', () => {
			render(<Textarea name="description" />);
			const textarea = document.querySelector('textarea');
			expect(textarea).toHaveAttribute('name', 'description');
		});

		it('passes through id prop', () => {
			render(<Textarea id="my-textarea" />);
			const textarea = document.querySelector('textarea');
			expect(textarea).toHaveAttribute('id', 'my-textarea');
		});

		it('passes through maxLength prop', () => {
			render(<Textarea maxLength={500} />);
			const textarea = document.querySelector('textarea');
			expect(textarea).toHaveAttribute('maxLength', '500');
		});

		it('passes through rows prop', () => {
			render(<Textarea rows={5} />);
			const textarea = document.querySelector('textarea');
			expect(textarea).toHaveAttribute('rows', '5');
		});

		it('passes through cols prop', () => {
			render(<Textarea cols={40} />);
			const textarea = document.querySelector('textarea');
			expect(textarea).toHaveAttribute('cols', '40');
		});
	});

	describe('error message', () => {
		it('displays error message', () => {
			render(<Textarea error="This field is required" />);
			expect(screen.getByText('This field is required')).toBeInTheDocument();
		});

		it('sets aria-describedby linking to error element', () => {
			render(<Textarea id="test-textarea" error="Invalid input" />);
			const textarea = screen.getByRole('textbox');
			expect(textarea).toHaveAttribute('aria-describedby', 'test-textarea-error');
			expect(screen.getByText('Invalid input')).toHaveAttribute('id', 'test-textarea-error');
		});

		it('error prop overrides variant to error', () => {
			const { container } = render(<Textarea variant="default" error="Error message" />);
			const wrapper = container.querySelector('.textarea-wrapper');
			expect(wrapper).toHaveClass('textarea-variant-error');
		});

		it('error message has role="alert" for accessibility', () => {
			render(<Textarea error="Error occurred" />);
			const errorMessage = screen.getByRole('alert');
			expect(errorMessage).toHaveTextContent('Error occurred');
		});

		it('preserves existing aria-describedby when error is present', () => {
			render(
				<Textarea
					id="test-textarea"
					aria-describedby="help-text"
					error="Error message"
				/>
			);
			const textarea = screen.getByRole('textbox');
			expect(textarea.getAttribute('aria-describedby')).toContain('help-text');
			expect(textarea.getAttribute('aria-describedby')).toContain('test-textarea-error');
		});
	});

	describe('accessibility', () => {
		it('sets aria-invalid="true" for error state', () => {
			render(<Textarea variant="error" />);
			const textarea = screen.getByRole('textbox');
			expect(textarea).toHaveAttribute('aria-invalid', 'true');
		});

		it('does not set aria-invalid for default state', () => {
			render(<Textarea variant="default" />);
			const textarea = screen.getByRole('textbox');
			expect(textarea).not.toHaveAttribute('aria-invalid');
		});

		it('sets aria-required="true" for required textareas', () => {
			render(<Textarea required />);
			const textarea = screen.getByRole('textbox');
			expect(textarea).toHaveAttribute('aria-required', 'true');
		});

		it('does not set aria-required for non-required textareas', () => {
			render(<Textarea />);
			const textarea = screen.getByRole('textbox');
			expect(textarea).not.toHaveAttribute('aria-required');
		});
	});

	describe('className prop', () => {
		it('applies additional className to wrapper', () => {
			const { container } = render(<Textarea className="custom-class" />);
			const wrapper = container.querySelector('.textarea-wrapper');
			expect(wrapper).toHaveClass('custom-class');
		});
	});
});
