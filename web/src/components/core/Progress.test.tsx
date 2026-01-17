import { describe, it, expect, vi } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import { createRef } from 'react';
import { Progress, type ProgressColor, type ProgressSize } from './Progress';

describe('Progress', () => {
	describe('rendering', () => {
		it('renders a progress bar', () => {
			render(<Progress value={50} />);
			expect(screen.getByRole('progressbar')).toBeInTheDocument();
		});

		it('renders track and fill elements', () => {
			const { container } = render(<Progress value={50} />);
			expect(container.querySelector('.progress-track')).toBeInTheDocument();
			expect(container.querySelector('.progress-fill')).toBeInTheDocument();
		});
	});

	describe('value handling', () => {
		it('sets correct width percentage', () => {
			const { container } = render(<Progress value={75} />);
			const fill = container.querySelector('.progress-fill') as HTMLElement;
			expect(fill.style.width).toBe('75%');
		});

		it('calculates percentage based on custom max', () => {
			const { container } = render(<Progress value={25} max={50} />);
			const fill = container.querySelector('.progress-fill') as HTMLElement;
			expect(fill.style.width).toBe('50%');
		});

		it('clamps value above max to 100%', () => {
			const { container } = render(<Progress value={150} max={100} />);
			const fill = container.querySelector('.progress-fill') as HTMLElement;
			expect(fill.style.width).toBe('100%');
		});

		it('clamps negative value to 0%', () => {
			const { container } = render(<Progress value={-10} />);
			const fill = container.querySelector('.progress-fill') as HTMLElement;
			expect(fill.style.width).toBe('0%');
		});

		it('handles max of 0 without division error', () => {
			const { container } = render(<Progress value={50} max={0} />);
			const fill = container.querySelector('.progress-fill') as HTMLElement;
			expect(fill.style.width).toBe('0%');
		});
	});

	describe('color variants', () => {
		const colors: ProgressColor[] = ['purple', 'green', 'amber', 'blue'];

		it.each(colors)('renders %s color variant with correct class', (color) => {
			const { container } = render(<Progress value={50} color={color} />);
			const progress = container.querySelector('.progress');
			expect(progress).toHaveClass(`progress-${color}`);
		});

		it('uses purple color by default', () => {
			const { container } = render(<Progress value={50} />);
			const progress = container.querySelector('.progress');
			expect(progress).toHaveClass('progress-purple');
		});
	});

	describe('size variants', () => {
		const sizes: ProgressSize[] = ['sm', 'md'];

		it.each(sizes)('renders %s size variant with correct class', (size) => {
			const { container } = render(<Progress value={50} size={size} />);
			const progress = container.querySelector('.progress');
			expect(progress).toHaveClass(`progress-${size}`);
		});

		it('uses md size by default', () => {
			const { container } = render(<Progress value={50} />);
			const progress = container.querySelector('.progress');
			expect(progress).toHaveClass('progress-md');
		});
	});

	describe('showLabel', () => {
		it('does not show label by default', () => {
			const { container } = render(<Progress value={50} />);
			expect(container.querySelector('.progress-label')).not.toBeInTheDocument();
		});

		it('shows label when showLabel is true', () => {
			const { container } = render(<Progress value={50} showLabel />);
			const label = container.querySelector('.progress-label');
			expect(label).toBeInTheDocument();
			expect(label).toHaveTextContent('50%');
		});

		it('rounds percentage in label', () => {
			const { container } = render(<Progress value={33} max={100} showLabel />);
			const label = container.querySelector('.progress-label');
			expect(label).toHaveTextContent('33%');
		});

		it('adds progress-with-label class when showLabel is true', () => {
			const { container } = render(<Progress value={50} showLabel />);
			const progress = container.querySelector('.progress');
			expect(progress).toHaveClass('progress-with-label');
		});
	});

	describe('accessibility', () => {
		it('has progressbar role', () => {
			render(<Progress value={50} />);
			expect(screen.getByRole('progressbar')).toBeInTheDocument();
		});

		it('sets aria-valuenow to clamped value', () => {
			render(<Progress value={75} />);
			const progressbar = screen.getByRole('progressbar');
			expect(progressbar).toHaveAttribute('aria-valuenow', '75');
		});

		it('sets aria-valuemin to 0', () => {
			render(<Progress value={50} />);
			const progressbar = screen.getByRole('progressbar');
			expect(progressbar).toHaveAttribute('aria-valuemin', '0');
		});

		it('sets aria-valuemax to max prop', () => {
			render(<Progress value={50} max={200} />);
			const progressbar = screen.getByRole('progressbar');
			expect(progressbar).toHaveAttribute('aria-valuemax', '200');
		});

		it('clamps aria-valuenow when value exceeds max', () => {
			render(<Progress value={150} max={100} />);
			const progressbar = screen.getByRole('progressbar');
			expect(progressbar).toHaveAttribute('aria-valuenow', '100');
		});
	});

	describe('animation', () => {
		it('has progress-animate class initially', () => {
			const { container } = render(<Progress value={50} />);
			const progress = container.querySelector('.progress');
			expect(progress).toHaveClass('progress-animate');
		});

		it('removes progress-animate class after mount animation', async () => {
			vi.useFakeTimers();
			const { container } = render(<Progress value={50} />);

			// Simulate requestAnimationFrame
			await act(async () => {
				vi.advanceTimersByTime(16); // One frame at ~60fps
			});

			const progress = container.querySelector('.progress');
			expect(progress).not.toHaveClass('progress-animate');

			vi.useRealTimers();
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref correctly', () => {
			const ref = createRef<HTMLDivElement>();
			render(<Progress ref={ref} value={50} />);
			expect(ref.current).toBeInstanceOf(HTMLDivElement);
		});
	});

	describe('custom className', () => {
		it('applies custom className', () => {
			const { container } = render(<Progress value={50} className="custom-class" />);
			const progress = container.querySelector('.progress');
			expect(progress).toHaveClass('custom-class');
			expect(progress).toHaveClass('progress');
		});

		it('merges multiple classes correctly', () => {
			const { container } = render(
				<Progress value={50} color="green" size="sm" className="custom-a custom-b" />
			);
			const progress = container.querySelector('.progress');
			expect(progress).toHaveClass('progress');
			expect(progress).toHaveClass('progress-sm');
			expect(progress).toHaveClass('progress-green');
			expect(progress).toHaveClass('custom-a');
			expect(progress).toHaveClass('custom-b');
		});
	});

	describe('HTML attributes', () => {
		it('passes through native div attributes', () => {
			render(<Progress value={50} data-testid="test-progress" title="Test tooltip" />);
			const progress = screen.getByTestId('test-progress');
			expect(progress).toHaveAttribute('title', 'Test tooltip');
		});

		it('supports aria attributes', () => {
			const { container } = render(<Progress value={50} aria-label="Upload progress" />);
			const progress = container.querySelector('.progress');
			expect(progress).toHaveAttribute('aria-label', 'Upload progress');
		});
	});

	describe('combined props', () => {
		it('handles all props together', () => {
			const { container } = render(
				<Progress
					value={60}
					max={200}
					color="amber"
					size="sm"
					showLabel
					className="custom"
					data-testid="test-progress"
				/>
			);
			const progress = container.querySelector('.progress');
			expect(progress).toHaveClass('progress');
			expect(progress).toHaveClass('progress-sm');
			expect(progress).toHaveClass('progress-amber');
			expect(progress).toHaveClass('progress-with-label');
			expect(progress).toHaveClass('custom');
			expect(progress).toHaveAttribute('data-testid', 'test-progress');
			expect(progress).toHaveAttribute('aria-valuenow', '60');
			expect(progress).toHaveAttribute('aria-valuemax', '200');

			const fill = container.querySelector('.progress-fill') as HTMLElement;
			expect(fill.style.width).toBe('30%'); // 60/200 = 30%

			const label = container.querySelector('.progress-label');
			expect(label).toHaveTextContent('30%');
		});
	});
});
