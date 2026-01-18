import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { createRef } from 'react';
import { Home, Check, AlertCircle, Search } from 'lucide-react';
import { Icon, type IconSize, type IconColor } from './Icon';

describe('Icon', () => {
	describe('rendering', () => {
		it('renders an SVG element', () => {
			const { container } = render(<Icon name={Home} aria-label="Home" />);
			const svg = container.querySelector('svg');
			expect(svg).toBeInTheDocument();
		});

		it('renders the correct Lucide icon', () => {
			const { container } = render(<Icon name={Home} aria-label="Home" />);
			const svg = container.querySelector('svg');
			// Lucide icons use 'lucide-house' class for the Home icon
			expect(svg).toHaveClass('lucide-house');
		});

		it('renders different icons correctly', () => {
			const { container: c1 } = render(<Icon name={Check} aria-hidden />);
			const { container: c2 } = render(<Icon name={AlertCircle} aria-hidden />);
			expect(c1.querySelector('.lucide-check')).toBeInTheDocument();
			expect(c2.querySelector('.lucide-circle-alert')).toBeInTheDocument();
		});
	});

	describe('sizes', () => {
		const sizes: { size: IconSize; pixels: number }[] = [
			{ size: 'xs', pixels: 12 },
			{ size: 'sm', pixels: 14 },
			{ size: 'md', pixels: 16 },
			{ size: 'lg', pixels: 18 },
			{ size: 'xl', pixels: 20 },
		];

		it.each(sizes)('renders $size size with $pixels pixels', ({ size, pixels }) => {
			const { container } = render(<Icon name={Home} size={size} aria-hidden />);
			const svg = container.querySelector('svg');
			expect(svg).toHaveAttribute('width', String(pixels));
			expect(svg).toHaveAttribute('height', String(pixels));
			expect(svg).toHaveClass(`icon--${size}`);
		});

		it('uses md size by default', () => {
			const { container } = render(<Icon name={Home} aria-hidden />);
			const svg = container.querySelector('svg');
			expect(svg).toHaveAttribute('width', '16');
			expect(svg).toHaveAttribute('height', '16');
			expect(svg).toHaveClass('icon--md');
		});
	});

	describe('colors', () => {
		const colors: IconColor[] = ['primary', 'secondary', 'muted', 'success', 'warning', 'error'];

		it.each(colors)('renders %s color with correct class', (color) => {
			const { container } = render(<Icon name={Home} color={color} aria-hidden />);
			const svg = container.querySelector('svg');
			expect(svg).toHaveClass(`icon--${color}`);
		});

		it('does not add color class when color is not specified', () => {
			const { container } = render(<Icon name={Home} aria-hidden />);
			const svg = container.querySelector('svg');
			expect(svg).not.toHaveClass('icon--primary');
			expect(svg).not.toHaveClass('icon--secondary');
			expect(svg).not.toHaveClass('icon--muted');
			expect(svg).not.toHaveClass('icon--success');
			expect(svg).not.toHaveClass('icon--warning');
			expect(svg).not.toHaveClass('icon--error');
		});
	});

	describe('accessibility', () => {
		it('sets aria-label when provided', () => {
			render(<Icon name={Home} aria-label="Go to home page" />);
			const svg = screen.getByRole('img', { name: 'Go to home page' });
			expect(svg).toBeInTheDocument();
		});

		it('sets aria-hidden when no aria-label is provided', () => {
			const { container } = render(<Icon name={Home} />);
			const svg = container.querySelector('svg');
			expect(svg).toHaveAttribute('aria-hidden', 'true');
		});

		it('sets aria-hidden to false when aria-label is provided', () => {
			const { container } = render(<Icon name={Home} aria-label="Home" />);
			const svg = container.querySelector('svg');
			expect(svg).toHaveAttribute('aria-hidden', 'false');
		});

		it('allows explicit aria-hidden override', () => {
			const { container } = render(<Icon name={Home} aria-hidden={true} />);
			const svg = container.querySelector('svg');
			expect(svg).toHaveAttribute('aria-hidden', 'true');
		});

		it('sets role=img when aria-label is provided', () => {
			const { container } = render(<Icon name={Home} aria-label="Home" />);
			const svg = container.querySelector('svg');
			expect(svg).toHaveAttribute('role', 'img');
		});

		it('does not set role when decorative (no aria-label)', () => {
			const { container } = render(<Icon name={Home} aria-hidden />);
			const svg = container.querySelector('svg');
			expect(svg).not.toHaveAttribute('role');
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref correctly', () => {
			const ref = createRef<SVGSVGElement>();
			render(<Icon name={Home} ref={ref} aria-hidden />);
			expect(ref.current).toBeInstanceOf(SVGSVGElement);
			expect(ref.current?.tagName).toBe('svg');
		});
	});

	describe('custom className', () => {
		it('applies custom className', () => {
			const { container } = render(<Icon name={Home} className="custom-class" aria-hidden />);
			const svg = container.querySelector('svg');
			expect(svg).toHaveClass('custom-class');
			expect(svg).toHaveClass('icon');
		});

		it('merges multiple classes correctly', () => {
			const { container } = render(
				<Icon name={Home} className="class-a class-b" size="lg" color="success" aria-hidden />
			);
			const svg = container.querySelector('svg');
			expect(svg).toHaveClass('icon');
			expect(svg).toHaveClass('icon--lg');
			expect(svg).toHaveClass('icon--success');
			expect(svg).toHaveClass('class-a');
			expect(svg).toHaveClass('class-b');
		});
	});

	describe('SVG attributes', () => {
		it('passes through native SVG attributes', () => {
			const { container } = render(
				<Icon name={Home} data-testid="test-icon" aria-hidden strokeWidth={3} />
			);
			const svg = container.querySelector('svg');
			expect(svg).toHaveAttribute('data-testid', 'test-icon');
			expect(svg).toHaveAttribute('stroke-width', '3');
		});

		it('supports style prop', () => {
			const { container } = render(
				<Icon name={Home} style={{ opacity: 0.5 }} aria-hidden />
			);
			const svg = container.querySelector('svg');
			expect(svg).toHaveStyle({ opacity: '0.5' });
		});
	});

	describe('combined props', () => {
		it('handles all props together', () => {
			const { container } = render(
				<Icon
					name={Search}
					size="lg"
					color="primary"
					className="custom"
					data-testid="search-icon"
					aria-label="Search"
				/>
			);
			const svg = container.querySelector('svg');
			expect(svg).toHaveClass('icon');
			expect(svg).toHaveClass('icon--lg');
			expect(svg).toHaveClass('icon--primary');
			expect(svg).toHaveClass('custom');
			expect(svg).toHaveAttribute('data-testid', 'search-icon');
			expect(svg).toHaveAttribute('aria-label', 'Search');
			expect(svg).toHaveAttribute('role', 'img');
			expect(svg).toHaveAttribute('width', '18');
			expect(svg).toHaveAttribute('height', '18');
		});
	});

	describe('usage examples from task', () => {
		it('renders Home icon with size lg and aria-label', () => {
			render(<Icon name={Home} size="lg" aria-label="Home" />);
			const svg = screen.getByRole('img', { name: 'Home' });
			expect(svg).toHaveAttribute('width', '18');
			expect(svg).toHaveClass('icon--lg');
		});

		it('renders Check icon with success color and aria-hidden', () => {
			const { container } = render(<Icon name={Check} color="success" aria-hidden />);
			const svg = container.querySelector('svg');
			expect(svg).toHaveClass('icon--success');
			expect(svg).toHaveAttribute('aria-hidden', 'true');
		});

		it('renders AlertCircle icon with error color and sm size', () => {
			const { container } = render(<Icon name={AlertCircle} color="error" size="sm" />);
			const svg = container.querySelector('svg');
			expect(svg).toHaveClass('icon--error');
			expect(svg).toHaveClass('icon--sm');
			expect(svg).toHaveAttribute('width', '14');
		});
	});
});
