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

	describe('character count', () => {
		it('displays character count when showCount and maxLength are provided', () => {
			render(<Textarea showCount maxLength={100} />);
			expect(screen.getByText('0/100')).toBeInTheDocument();
		});

		it('does not display character count without showCount', () => {
			render(<Textarea maxLength={100} />);
			expect(screen.queryByText('0/100')).not.toBeInTheDocument();
		});

		it('does not display character count without maxLength', () => {
			const { container } = render(<Textarea showCount />);
			expect(container.querySelector('.textarea-char-count')).not.toBeInTheDocument();
		});

		it('updates character count on input', () => {
			render(<Textarea showCount maxLength={100} value="hello" onChange={() => {}} />);
			expect(screen.getByText('5/100')).toBeInTheDocument();
		});

		it('applies warning class at 90% capacity', () => {
			const { container } = render(
				<Textarea showCount maxLength={10} value="123456789" onChange={() => {}} />
			);
			const countElement = container.querySelector('.textarea-char-count');
			expect(countElement).toHaveClass('textarea-char-count-warning');
		});

		it('does not apply warning class below 90% capacity', () => {
			const { container } = render(
				<Textarea showCount maxLength={10} value="12345678" onChange={() => {}} />
			);
			const countElement = container.querySelector('.textarea-char-count');
			expect(countElement).not.toHaveClass('textarea-char-count-warning');
		});

		it('applies warning class at exactly 90% capacity', () => {
			const { container } = render(
				<Textarea showCount maxLength={100} value={'a'.repeat(90)} onChange={() => {}} />
			);
			const countElement = container.querySelector('.textarea-char-count');
			expect(countElement).toHaveClass('textarea-char-count-warning');
		});

		it('links character count via aria-describedby', () => {
			render(<Textarea id="test-textarea" showCount maxLength={100} />);
			const textarea = screen.getByRole('textbox');
			expect(textarea.getAttribute('aria-describedby')).toContain('test-textarea-count');
		});
	});

	describe('auto-resize', () => {
		it('applies auto-resize class when enabled', () => {
			const { container } = render(<Textarea autoResize />);
			const wrapper = container.querySelector('.textarea-wrapper');
			expect(wrapper).toHaveClass('textarea-auto-resize');
		});

		it('disables manual resize when autoResize is enabled', () => {
			const { container } = render(<Textarea autoResize resize="both" />);
			const wrapper = container.querySelector('.textarea-wrapper');
			expect(wrapper).toHaveClass('textarea-resize-none');
		});

		it('calls onChange when text is entered with autoResize', () => {
			const handleChange = vi.fn();
			render(<Textarea autoResize onChange={handleChange} />);
			const textarea = screen.getByRole('textbox');
			fireEvent.change(textarea, { target: { value: 'new value' } });
			expect(handleChange).toHaveBeenCalledTimes(1);
		});

		it('adjusts textarea height on value change', () => {
			const { rerender } = render(<Textarea autoResize value="" onChange={() => {}} />);
			const textarea = screen.getByRole('textbox');

			// Mock scrollHeight to simulate content height
			Object.defineProperty(textarea, 'scrollHeight', { value: 100, configurable: true });

			rerender(<Textarea autoResize value="Some longer text" onChange={() => {}} />);

			// Check that height style was set
			expect(textarea.style.height).toBeTruthy();
		});

		it('respects maxHeight prop', () => {
			render(<Textarea autoResize maxHeight={150} value="" onChange={() => {}} />);
			const textarea = screen.getByRole('textbox');

			// Mock scrollHeight larger than maxHeight
			Object.defineProperty(textarea, 'scrollHeight', { value: 200, configurable: true });

			fireEvent.change(textarea, { target: { value: 'lots of text' } });

			// Height should be capped at maxHeight
			const height = parseInt(textarea.style.height || '0', 10);
			expect(height).toBeLessThanOrEqual(150);
		});

		it('sets overflow-y to auto when content exceeds maxHeight', () => {
			render(<Textarea autoResize maxHeight={100} value="" onChange={() => {}} />);
			const textarea = screen.getByRole('textbox');

			// Mock scrollHeight larger than maxHeight
			Object.defineProperty(textarea, 'scrollHeight', { value: 200, configurable: true });

			fireEvent.change(textarea, { target: { value: 'lots of text' } });

			expect(textarea.style.overflowY).toBe('auto');
		});

		it('sets overflow-y to hidden when content is within maxHeight', () => {
			render(<Textarea autoResize maxHeight={300} value="" onChange={() => {}} />);
			const textarea = screen.getByRole('textbox');

			// Mock scrollHeight smaller than maxHeight
			Object.defineProperty(textarea, 'scrollHeight', { value: 100, configurable: true });

			fireEvent.change(textarea, { target: { value: 'short text' } });

			expect(textarea.style.overflowY).toBe('hidden');
		});
	});

	describe('combined features', () => {
		it('can have both error message and character count', () => {
			render(
				<Textarea
					id="test-textarea"
					showCount
					maxLength={100}
					error="Error message"
					value="hello"
					onChange={() => {}}
				/>
			);
			expect(screen.getByText('5/100')).toBeInTheDocument();
			expect(screen.getByText('Error message')).toBeInTheDocument();
		});

		it('links both error and count via aria-describedby', () => {
			render(
				<Textarea
					id="test-textarea"
					showCount
					maxLength={100}
					error="Error message"
				/>
			);
			const textarea = screen.getByRole('textbox');
			const describedBy = textarea.getAttribute('aria-describedby') || '';
			expect(describedBy).toContain('test-textarea-error');
			expect(describedBy).toContain('test-textarea-count');
		});

		it('preserves existing aria-describedby with error and count', () => {
			render(
				<Textarea
					id="test-textarea"
					aria-describedby="help-text"
					showCount
					maxLength={100}
					error="Error message"
				/>
			);
			const textarea = screen.getByRole('textbox');
			const describedBy = textarea.getAttribute('aria-describedby') || '';
			expect(describedBy).toContain('help-text');
			expect(describedBy).toContain('test-textarea-error');
			expect(describedBy).toContain('test-textarea-count');
		});
	});
});
