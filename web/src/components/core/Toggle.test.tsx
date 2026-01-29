import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { createRef } from 'react';
import { Toggle, type ToggleSize } from './Toggle';

describe('Toggle', () => {
	describe('rendering', () => {
		it('renders a switch element', () => {
			render(<Toggle />);
			expect(screen.getByRole('switch')).toBeInTheDocument();
		});

		it('renders as checkbox input for form compatibility', () => {
			const { container } = render(<Toggle />);
			const input = container.querySelector('input[type="checkbox"]');
			expect(input).toBeInTheDocument();
		});

		it('renders track and knob elements', () => {
			const { container } = render(<Toggle />);
			expect(container.querySelector('.toggle__track')).toBeInTheDocument();
			expect(container.querySelector('.toggle__knob')).toBeInTheDocument();
		});

		it('role="switch" is on the label wrapper, not the input', () => {
			const { container } = render(<Toggle />);
			const label = container.querySelector('label.toggle');
			expect(label).toHaveAttribute('role', 'switch');
			const input = container.querySelector('input');
			expect(input).not.toHaveAttribute('role');
		});
	});

	describe('checked state', () => {
		it('is unchecked by default', () => {
			render(<Toggle />);
			const toggle = screen.getByRole('switch');
			expect(toggle).toHaveAttribute('aria-checked', 'false');
		});

		it('is checked when checked prop is true', () => {
			render(<Toggle checked />);
			const toggle = screen.getByRole('switch');
			expect(toggle).toHaveAttribute('aria-checked', 'true');
		});

		it('applies toggle--on class when checked', () => {
			const { container } = render(<Toggle checked />);
			const wrapper = container.querySelector('.toggle');
			expect(wrapper).toHaveClass('toggle--on');
		});

		it('does not apply toggle--on class when unchecked', () => {
			const { container } = render(<Toggle checked={false} />);
			const wrapper = container.querySelector('.toggle');
			expect(wrapper).not.toHaveClass('toggle--on');
		});
	});

	describe('onChange', () => {
		it('calls onChange with true when toggled on', () => {
			const handleChange = vi.fn();
			render(<Toggle checked={false} onChange={handleChange} />);
			fireEvent.click(screen.getByRole('switch'));
			expect(handleChange).toHaveBeenCalledTimes(1);
			expect(handleChange).toHaveBeenCalledWith(true, expect.any(Object));
		});

		it('calls onChange with false when toggled off', () => {
			const handleChange = vi.fn();
			render(<Toggle checked onChange={handleChange} />);
			fireEvent.click(screen.getByRole('switch'));
			expect(handleChange).toHaveBeenCalledTimes(1);
			expect(handleChange).toHaveBeenCalledWith(false, expect.any(Object));
		});

		it('does not call onChange when disabled', () => {
			const handleChange = vi.fn();
			render(<Toggle disabled onChange={handleChange} />);
			fireEvent.click(screen.getByRole('switch'));
			expect(handleChange).not.toHaveBeenCalled();
		});
	});

	describe('disabled state', () => {
		it('has aria-disabled when disabled', () => {
			render(<Toggle disabled />);
			const toggle = screen.getByRole('switch');
			expect(toggle).toHaveAttribute('aria-disabled', 'true');
		});

		it('applies toggle--disabled class when disabled', () => {
			const { container } = render(<Toggle disabled />);
			const wrapper = container.querySelector('.toggle');
			expect(wrapper).toHaveClass('toggle--disabled');
		});

		it('prevents interaction when disabled', () => {
			const handleChange = vi.fn();
			render(<Toggle disabled onChange={handleChange} />);
			const toggle = screen.getByRole('switch');
			fireEvent.click(toggle);
			expect(handleChange).not.toHaveBeenCalled();
		});

		it('has tabIndex -1 when disabled', () => {
			render(<Toggle disabled />);
			const toggle = screen.getByRole('switch');
			expect(toggle).toHaveAttribute('tabindex', '-1');
		});
	});

	describe('sizes', () => {
		const sizes: ToggleSize[] = ['sm', 'md'];

		it.each(sizes)('renders %s size with correct class', (size) => {
			const { container } = render(<Toggle size={size} />);
			const wrapper = container.querySelector('.toggle');
			expect(wrapper).toHaveClass(`toggle--${size}`);
		});

		it('uses medium size by default', () => {
			const { container } = render(<Toggle />);
			const wrapper = container.querySelector('.toggle');
			expect(wrapper).toHaveClass('toggle--md');
		});
	});

	describe('keyboard accessibility', () => {
		it('can be toggled with Space key', () => {
			const handleChange = vi.fn();
			render(<Toggle checked={false} onChange={handleChange} />);
			const toggle = screen.getByRole('switch');
			fireEvent.keyDown(toggle, { key: ' ' });
			expect(handleChange).toHaveBeenCalled();
		});

		it('can be toggled with Enter key', () => {
			const handleChange = vi.fn();
			render(<Toggle checked={false} onChange={handleChange} />);
			const toggle = screen.getByRole('switch');
			fireEvent.keyDown(toggle, { key: 'Enter' });
			expect(handleChange).toHaveBeenCalled();
		});

		it('is focusable', () => {
			render(<Toggle />);
			const toggle = screen.getByRole('switch');
			toggle.focus();
			expect(document.activeElement).toBe(toggle);
		});

		it('is not focusable when disabled', () => {
			render(<Toggle disabled />);
			const toggle = screen.getByRole('switch');
			expect(toggle).toHaveAttribute('tabindex', '-1');
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref correctly', () => {
			const ref = createRef<HTMLInputElement>();
			render(<Toggle ref={ref} />);
			expect(ref.current).toBeInstanceOf(HTMLInputElement);
			expect(ref.current?.type).toBe('checkbox');
		});
	});

	describe('custom className', () => {
		it('applies custom className', () => {
			const { container } = render(<Toggle className="custom-class" />);
			const wrapper = container.querySelector('.toggle');
			expect(wrapper).toHaveClass('custom-class');
			expect(wrapper).toHaveClass('toggle');
		});

		it('merges multiple classes correctly', () => {
			const { container } = render(<Toggle className="class-a class-b" size="sm" checked />);
			const wrapper = container.querySelector('.toggle');
			expect(wrapper).toHaveClass('toggle');
			expect(wrapper).toHaveClass('toggle--sm');
			expect(wrapper).toHaveClass('toggle--on');
			expect(wrapper).toHaveClass('class-a');
			expect(wrapper).toHaveClass('class-b');
		});
	});

	describe('form integration', () => {
		it('supports name attribute', () => {
			const { container } = render(<Toggle name="autoSave" />);
			const input = container.querySelector('input[name="autoSave"]');
			expect(input).toBeInTheDocument();
		});

		it('supports id attribute', () => {
			const { container } = render(<Toggle id="my-toggle" />);
			const input = container.querySelector('input[type="checkbox"]');
			expect(input).toHaveAttribute('id', 'my-toggle');
		});

		it('generates unique id when not provided', () => {
			const { container } = render(<Toggle />);
			const input = container.querySelector('input[type="checkbox"]');
			expect(input).toHaveAttribute('id');
			expect(input?.id).toBeTruthy();
		});

		it('works in a form with label', () => {
			render(
				<form>
					<Toggle id="test-toggle" name="feature" aria-label="Enable feature" />
				</form>
			);
			const toggle = screen.getByRole('switch');
			expect(toggle).toHaveAttribute('aria-label', 'Enable feature');
			const input = document.querySelector('input[name="feature"]');
			expect(input).toBeInTheDocument();
		});

		it('label wrapper is clickable', () => {
			const handleChange = vi.fn();
			const { container } = render(<Toggle onChange={handleChange} />);
			const label = container.querySelector('.toggle');
			fireEvent.click(label!);
			expect(handleChange).toHaveBeenCalled();
		});
	});

	describe('HTML attributes', () => {
		it('passes through aria-label to the label', () => {
			render(<Toggle aria-label="Toggle setting" />);
			const toggle = screen.getByRole('switch');
			expect(toggle).toHaveAttribute('aria-label', 'Toggle setting');
		});
	});

	describe('accessibility', () => {
		it('has role="switch"', () => {
			render(<Toggle />);
			expect(screen.getByRole('switch')).toBeInTheDocument();
		});

		it('has correct aria-checked when off', () => {
			render(<Toggle checked={false} />);
			const toggle = screen.getByRole('switch');
			expect(toggle).toHaveAttribute('aria-checked', 'false');
		});

		it('has correct aria-checked when on', () => {
			render(<Toggle checked />);
			const toggle = screen.getByRole('switch');
			expect(toggle).toHaveAttribute('aria-checked', 'true');
		});

		it('input is visually hidden but present in DOM', () => {
			const { container } = render(<Toggle />);
			const input = container.querySelector('.toggle__input');
			expect(input).toBeInTheDocument();
			expect(input).toHaveAttribute('aria-hidden', 'true');
		});
	});

	describe('combined states', () => {
		it('handles checked + disabled correctly', () => {
			const { container } = render(<Toggle checked disabled />);
			const wrapper = container.querySelector('.toggle');
			expect(wrapper).toHaveClass('toggle--on');
			expect(wrapper).toHaveClass('toggle--disabled');
			const toggle = screen.getByRole('switch');
			expect(toggle).toHaveAttribute('aria-checked', 'true');
			expect(toggle).toHaveAttribute('aria-disabled', 'true');
		});

		it('handles all props together', () => {
			const handleChange = vi.fn();
			const { container } = render(
				<Toggle
					checked
					size="sm"
					onChange={handleChange}
					className="custom"
					name="testToggle"
					id="test-id"
				/>
			);
			const wrapper = container.querySelector('.toggle');
			expect(wrapper).toHaveClass('toggle');
			expect(wrapper).toHaveClass('toggle--sm');
			expect(wrapper).toHaveClass('toggle--on');
			expect(wrapper).toHaveClass('custom');

			const input = container.querySelector('input[type="checkbox"]');
			expect(input).toHaveAttribute('name', 'testToggle');
			expect(input).toHaveAttribute('id', 'test-id');

			const toggle = screen.getByRole('switch');
			fireEvent.click(toggle);
			expect(handleChange).toHaveBeenCalledWith(false, expect.any(Object));
		});
	});
});
