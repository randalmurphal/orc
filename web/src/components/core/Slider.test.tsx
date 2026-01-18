import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { createRef } from 'react';
import { Slider } from './Slider';

describe('Slider', () => {
	describe('rendering', () => {
		it('renders a slider element', () => {
			render(<Slider value={50} onChange={() => {}} />);
			expect(screen.getByRole('slider')).toBeInTheDocument();
		});

		it('renders track, fill, and thumb elements', () => {
			const { container } = render(<Slider value={50} onChange={() => {}} />);
			expect(container.querySelector('.slider__track')).toBeInTheDocument();
			expect(container.querySelector('.slider__fill')).toBeInTheDocument();
			expect(container.querySelector('.slider__thumb')).toBeInTheDocument();
		});
	});

	describe('value and range', () => {
		it('sets aria-valuenow to current value', () => {
			render(<Slider value={42} onChange={() => {}} />);
			const slider = screen.getByRole('slider');
			expect(slider).toHaveAttribute('aria-valuenow', '42');
		});

		it('uses default min of 0', () => {
			render(<Slider value={50} onChange={() => {}} />);
			const slider = screen.getByRole('slider');
			expect(slider).toHaveAttribute('aria-valuemin', '0');
		});

		it('uses default max of 100', () => {
			render(<Slider value={50} onChange={() => {}} />);
			const slider = screen.getByRole('slider');
			expect(slider).toHaveAttribute('aria-valuemax', '100');
		});

		it('accepts custom min and max', () => {
			render(<Slider value={5} onChange={() => {}} min={0} max={10} />);
			const slider = screen.getByRole('slider');
			expect(slider).toHaveAttribute('aria-valuemin', '0');
			expect(slider).toHaveAttribute('aria-valuemax', '10');
		});

		it('positions thumb at correct percentage', () => {
			const { container } = render(<Slider value={50} onChange={() => {}} />);
			const thumb = container.querySelector('.slider__thumb') as HTMLElement;
			expect(thumb.style.left).toBe('50%');
		});

		it('positions fill at correct percentage', () => {
			const { container } = render(<Slider value={25} onChange={() => {}} />);
			const fill = container.querySelector('.slider__fill') as HTMLElement;
			expect(fill.style.width).toBe('25%');
		});

		it('handles custom range positioning', () => {
			const { container } = render(<Slider value={5} onChange={() => {}} min={0} max={10} />);
			const thumb = container.querySelector('.slider__thumb') as HTMLElement;
			expect(thumb.style.left).toBe('50%');
		});
	});

	describe('value display', () => {
		it('does not show value by default', () => {
			const { container } = render(<Slider value={50} onChange={() => {}} />);
			expect(container.querySelector('.slider__value')).not.toBeInTheDocument();
		});

		it('shows value when showValue is true', () => {
			const { container } = render(<Slider value={50} onChange={() => {}} showValue />);
			const valueDisplay = container.querySelector('.slider__value');
			expect(valueDisplay).toBeInTheDocument();
			expect(valueDisplay).toHaveTextContent('50');
		});

		it('uses custom formatValue function', () => {
			const { container } = render(
				<Slider value={50} onChange={() => {}} showValue formatValue={(v) => `$${v}`} />
			);
			const valueDisplay = container.querySelector('.slider__value');
			expect(valueDisplay).toHaveTextContent('$50');
		});

		it('updates value display in real-time', () => {
			const { container, rerender } = render(<Slider value={25} onChange={() => {}} showValue />);
			expect(container.querySelector('.slider__value')).toHaveTextContent('25');

			rerender(<Slider value={75} onChange={() => {}} showValue />);
			expect(container.querySelector('.slider__value')).toHaveTextContent('75');
		});
	});

	describe('keyboard accessibility', () => {
		it('is focusable', () => {
			render(<Slider value={50} onChange={() => {}} />);
			const slider = screen.getByRole('slider');
			slider.focus();
			expect(document.activeElement).toBe(slider);
		});

		it('increments value on ArrowRight', () => {
			const handleChange = vi.fn();
			render(<Slider value={50} onChange={handleChange} />);
			const slider = screen.getByRole('slider');
			fireEvent.keyDown(slider, { key: 'ArrowRight' });
			expect(handleChange).toHaveBeenCalledWith(51);
		});

		it('decrements value on ArrowLeft', () => {
			const handleChange = vi.fn();
			render(<Slider value={50} onChange={handleChange} />);
			const slider = screen.getByRole('slider');
			fireEvent.keyDown(slider, { key: 'ArrowLeft' });
			expect(handleChange).toHaveBeenCalledWith(49);
		});

		it('increments value on ArrowUp', () => {
			const handleChange = vi.fn();
			render(<Slider value={50} onChange={handleChange} />);
			const slider = screen.getByRole('slider');
			fireEvent.keyDown(slider, { key: 'ArrowUp' });
			expect(handleChange).toHaveBeenCalledWith(51);
		});

		it('decrements value on ArrowDown', () => {
			const handleChange = vi.fn();
			render(<Slider value={50} onChange={handleChange} />);
			const slider = screen.getByRole('slider');
			fireEvent.keyDown(slider, { key: 'ArrowDown' });
			expect(handleChange).toHaveBeenCalledWith(49);
		});

		it('goes to min on Home key', () => {
			const handleChange = vi.fn();
			render(<Slider value={50} onChange={handleChange} min={10} />);
			const slider = screen.getByRole('slider');
			fireEvent.keyDown(slider, { key: 'Home' });
			expect(handleChange).toHaveBeenCalledWith(10);
		});

		it('goes to max on End key', () => {
			const handleChange = vi.fn();
			render(<Slider value={50} onChange={handleChange} max={90} />);
			const slider = screen.getByRole('slider');
			fireEvent.keyDown(slider, { key: 'End' });
			expect(handleChange).toHaveBeenCalledWith(90);
		});

		it('respects step when using arrow keys', () => {
			const handleChange = vi.fn();
			render(<Slider value={50} onChange={handleChange} step={5} />);
			const slider = screen.getByRole('slider');
			fireEvent.keyDown(slider, { key: 'ArrowRight' });
			expect(handleChange).toHaveBeenCalledWith(55);
		});

		it('jumps by 10 steps on PageUp', () => {
			const handleChange = vi.fn();
			render(<Slider value={50} onChange={handleChange} step={1} />);
			const slider = screen.getByRole('slider');
			fireEvent.keyDown(slider, { key: 'PageUp' });
			expect(handleChange).toHaveBeenCalledWith(60);
		});

		it('jumps by 10 steps on PageDown', () => {
			const handleChange = vi.fn();
			render(<Slider value={50} onChange={handleChange} step={1} />);
			const slider = screen.getByRole('slider');
			fireEvent.keyDown(slider, { key: 'PageDown' });
			expect(handleChange).toHaveBeenCalledWith(40);
		});

		it('does not exceed max on keyboard navigation', () => {
			const handleChange = vi.fn();
			render(<Slider value={99} onChange={handleChange} max={100} />);
			const slider = screen.getByRole('slider');
			fireEvent.keyDown(slider, { key: 'ArrowRight' });
			expect(handleChange).toHaveBeenCalledWith(100);
		});

		it('does not go below min on keyboard navigation', () => {
			const handleChange = vi.fn();
			render(<Slider value={1} onChange={handleChange} min={0} />);
			const slider = screen.getByRole('slider');
			fireEvent.keyDown(slider, { key: 'ArrowLeft' });
			expect(handleChange).toHaveBeenCalledWith(0);
		});

		it('does not call onChange if value would not change', () => {
			const handleChange = vi.fn();
			render(<Slider value={0} onChange={handleChange} min={0} />);
			const slider = screen.getByRole('slider');
			fireEvent.keyDown(slider, { key: 'ArrowLeft' });
			expect(handleChange).not.toHaveBeenCalled();
		});
	});

	describe('step snapping', () => {
		it('snaps values to step increments', () => {
			const handleChange = vi.fn();
			render(<Slider value={50} onChange={handleChange} step={5} />);
			const slider = screen.getByRole('slider');
			fireEvent.keyDown(slider, { key: 'ArrowRight' });
			expect(handleChange).toHaveBeenCalledWith(55);
		});

		it('handles decimal steps', () => {
			const handleChange = vi.fn();
			render(<Slider value={0.5} onChange={handleChange} min={0} max={1} step={0.1} />);
			const slider = screen.getByRole('slider');
			fireEvent.keyDown(slider, { key: 'ArrowRight' });
			expect(handleChange).toHaveBeenCalledWith(0.6);
		});
	});

	describe('mouse interaction', () => {
		it('updates value on track click', () => {
			const handleChange = vi.fn();
			const { container } = render(<Slider value={50} onChange={handleChange} />);
			const track = container.querySelector('.slider__track') as HTMLElement;

			// Mock getBoundingClientRect
			track.getBoundingClientRect = vi.fn().mockReturnValue({
				left: 0,
				width: 100,
				top: 0,
				height: 4,
				right: 100,
				bottom: 4,
			});

			fireEvent.mouseDown(track, { clientX: 75 });
			expect(handleChange).toHaveBeenCalledWith(75);
		});

		it('applies dragging class during drag', () => {
			const { container } = render(<Slider value={50} onChange={() => {}} />);
			const track = container.querySelector('.slider__track') as HTMLElement;
			const wrapper = container.querySelector('.slider') as HTMLElement;

			track.getBoundingClientRect = vi.fn().mockReturnValue({
				left: 0,
				width: 100,
				top: 0,
				height: 4,
				right: 100,
				bottom: 4,
			});

			fireEvent.mouseDown(track, { clientX: 50 });
			expect(wrapper).toHaveClass('slider--dragging');

			fireEvent.mouseUp(document);
			expect(wrapper).not.toHaveClass('slider--dragging');
		});
	});

	describe('touch interaction', () => {
		it('updates value on touch', () => {
			const handleChange = vi.fn();
			const { container } = render(<Slider value={50} onChange={handleChange} />);
			const track = container.querySelector('.slider__track') as HTMLElement;

			track.getBoundingClientRect = vi.fn().mockReturnValue({
				left: 0,
				width: 100,
				top: 0,
				height: 4,
				right: 100,
				bottom: 4,
			});

			fireEvent.touchStart(track, { touches: [{ clientX: 25 }] });
			expect(handleChange).toHaveBeenCalledWith(25);
		});
	});

	describe('disabled state', () => {
		it('applies disabled class when disabled', () => {
			const { container } = render(<Slider value={50} onChange={() => {}} disabled />);
			const wrapper = container.querySelector('.slider');
			expect(wrapper).toHaveClass('slider--disabled');
		});

		it('sets aria-disabled when disabled', () => {
			render(<Slider value={50} onChange={() => {}} disabled />);
			const slider = screen.getByRole('slider');
			expect(slider).toHaveAttribute('aria-disabled', 'true');
		});

		it('removes focus from taborder when disabled', () => {
			render(<Slider value={50} onChange={() => {}} disabled />);
			const slider = screen.getByRole('slider');
			expect(slider).toHaveAttribute('tabIndex', '-1');
		});

		it('does not respond to keyboard when disabled', () => {
			const handleChange = vi.fn();
			render(<Slider value={50} onChange={handleChange} disabled />);
			const slider = screen.getByRole('slider');
			fireEvent.keyDown(slider, { key: 'ArrowRight' });
			expect(handleChange).not.toHaveBeenCalled();
		});

		it('does not respond to mouse when disabled', () => {
			const handleChange = vi.fn();
			const { container } = render(<Slider value={50} onChange={handleChange} disabled />);
			const track = container.querySelector('.slider__track') as HTMLElement;

			track.getBoundingClientRect = vi.fn().mockReturnValue({
				left: 0,
				width: 100,
				top: 0,
				height: 4,
				right: 100,
				bottom: 4,
			});

			fireEvent.mouseDown(track, { clientX: 75 });
			expect(handleChange).not.toHaveBeenCalled();
		});

		it('does not respond to touch when disabled', () => {
			const handleChange = vi.fn();
			const { container } = render(<Slider value={50} onChange={handleChange} disabled />);
			const track = container.querySelector('.slider__track') as HTMLElement;

			track.getBoundingClientRect = vi.fn().mockReturnValue({
				left: 0,
				width: 100,
				top: 0,
				height: 4,
				right: 100,
				bottom: 4,
			});

			fireEvent.touchStart(track, { touches: [{ clientX: 25 }] });
			expect(handleChange).not.toHaveBeenCalled();
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref correctly', () => {
			const ref = createRef<HTMLDivElement>();
			render(<Slider ref={ref} value={50} onChange={() => {}} />);
			expect(ref.current).toBeInstanceOf(HTMLDivElement);
			expect(ref.current).toHaveClass('slider');
		});
	});

	describe('custom className', () => {
		it('applies custom className', () => {
			const { container } = render(
				<Slider value={50} onChange={() => {}} className="custom-class" />
			);
			const wrapper = container.querySelector('.slider');
			expect(wrapper).toHaveClass('custom-class');
			expect(wrapper).toHaveClass('slider');
		});
	});

	describe('HTML attributes', () => {
		it('passes through native div attributes', () => {
			render(<Slider value={50} onChange={() => {}} data-testid="test-slider" />);
			const wrapper = screen.getByTestId('test-slider');
			expect(wrapper).toHaveClass('slider');
		});
	});

	describe('edge cases', () => {
		it('handles min equal to max gracefully', () => {
			const { container } = render(<Slider value={50} onChange={() => {}} min={50} max={50} />);
			const thumb = container.querySelector('.slider__thumb') as HTMLElement;
			expect(thumb.style.left).toBe('0%');
		});

		it('handles value at min boundary', () => {
			const { container } = render(<Slider value={0} onChange={() => {}} min={0} max={100} />);
			const thumb = container.querySelector('.slider__thumb') as HTMLElement;
			expect(thumb.style.left).toBe('0%');
		});

		it('handles value at max boundary', () => {
			const { container } = render(<Slider value={100} onChange={() => {}} min={0} max={100} />);
			const thumb = container.querySelector('.slider__thumb') as HTMLElement;
			expect(thumb.style.left).toBe('100%');
		});

		it('handles negative range', () => {
			const { container } = render(<Slider value={0} onChange={() => {}} min={-100} max={100} />);
			const thumb = container.querySelector('.slider__thumb') as HTMLElement;
			expect(thumb.style.left).toBe('50%');
		});
	});

	describe('combined props', () => {
		it('handles all props together', () => {
			const handleChange = vi.fn();
			const { container } = render(
				<Slider
					value={50}
					onChange={handleChange}
					min={0}
					max={100}
					step={10}
					showValue
					formatValue={(v) => `${v}%`}
					className="custom"
					data-testid="full-slider"
				/>
			);

			const wrapper = container.querySelector('.slider');
			expect(wrapper).toHaveClass('slider');
			expect(wrapper).toHaveClass('custom');

			const valueDisplay = container.querySelector('.slider__value');
			expect(valueDisplay).toHaveTextContent('50%');

			const slider = screen.getByRole('slider');
			expect(slider).toHaveAttribute('aria-valuenow', '50');
			expect(slider).toHaveAttribute('aria-valuemin', '0');
			expect(slider).toHaveAttribute('aria-valuemax', '100');

			fireEvent.keyDown(slider, { key: 'ArrowRight' });
			expect(handleChange).toHaveBeenCalledWith(60);
		});
	});
});
