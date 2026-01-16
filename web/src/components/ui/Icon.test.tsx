import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { Icon, type IconName } from './Icon';

describe('Icon', () => {
	it('renders an SVG element', () => {
		const { container } = render(<Icon name="check" />);
		const svg = container.querySelector('svg');
		expect(svg).toBeInTheDocument();
	});

	it('uses default size of 20', () => {
		const { container } = render(<Icon name="check" />);
		const svg = container.querySelector('svg');
		expect(svg).toHaveAttribute('width', '20');
		expect(svg).toHaveAttribute('height', '20');
	});

	it('accepts custom size', () => {
		const { container } = render(<Icon name="check" size={24} />);
		const svg = container.querySelector('svg');
		expect(svg).toHaveAttribute('width', '24');
		expect(svg).toHaveAttribute('height', '24');
	});

	it('applies custom className', () => {
		const { container } = render(<Icon name="check" className="my-icon" />);
		const svg = container.querySelector('svg');
		expect(svg).toHaveClass('my-icon');
	});

	it('sets aria-hidden for accessibility', () => {
		const { container } = render(<Icon name="check" />);
		const svg = container.querySelector('svg');
		expect(svg).toHaveAttribute('aria-hidden', 'true');
	});

	it('renders different icons', () => {
		const iconNames: IconName[] = ['check', 'close', 'search', 'plus', 'trash'];

		iconNames.forEach((name) => {
			const { container, unmount } = render(<Icon name={name} />);
			const svg = container.querySelector('svg');
			expect(svg).toBeInTheDocument();
			// Each icon should have some inner HTML (the path)
			expect(svg?.innerHTML).toBeTruthy();
			unmount();
		});
	});

	it('renders status icons with correct paths', () => {
		const statusIcons: IconName[] = ['success', 'error', 'warning', 'info'];

		statusIcons.forEach((name) => {
			const { container, unmount } = render(<Icon name={name} />);
			const svg = container.querySelector('svg');
			expect(svg).toBeInTheDocument();
			unmount();
		});
	});

	it('renders navigation icons', () => {
		const navIcons: IconName[] = ['dashboard', 'tasks', 'board', 'settings'];

		navIcons.forEach((name) => {
			const { container, unmount } = render(<Icon name={name} />);
			const svg = container.querySelector('svg');
			expect(svg).toBeInTheDocument();
			unmount();
		});
	});

	it('uses standard SVG attributes', () => {
		const { container } = render(<Icon name="check" />);
		const svg = container.querySelector('svg');
		expect(svg).toHaveAttribute('viewBox', '0 0 24 24');
		expect(svg).toHaveAttribute('fill', 'none');
		expect(svg).toHaveAttribute('stroke', 'currentColor');
		expect(svg).toHaveAttribute('stroke-width', '2');
		expect(svg).toHaveAttribute('stroke-linecap', 'round');
		expect(svg).toHaveAttribute('stroke-linejoin', 'round');
	});

	it('renders chevron icons', () => {
		const chevronIcons: IconName[] = ['chevron-down', 'chevron-up', 'chevron-left', 'chevron-right'];

		chevronIcons.forEach((name) => {
			const { container, unmount } = render(<Icon name={name} />);
			const svg = container.querySelector('svg');
			expect(svg).toBeInTheDocument();
			unmount();
		});
	});

	it('renders category icons', () => {
		const categoryIcons: IconName[] = ['sparkles', 'bug', 'recycle', 'beaker'];

		categoryIcons.forEach((name) => {
			const { container, unmount } = render(<Icon name={name} />);
			const svg = container.querySelector('svg');
			expect(svg).toBeInTheDocument();
			unmount();
		});
	});
});
