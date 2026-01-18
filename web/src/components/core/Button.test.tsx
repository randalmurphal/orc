import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { createRef } from 'react';
import { Plus, Settings, Pause } from 'lucide-react';
import { Button, type ButtonVariant } from './Button';

describe('Button', () => {
	describe('rendering', () => {
		it('renders a button element', () => {
			render(<Button>Click me</Button>);
			expect(screen.getByRole('button', { name: 'Click me' })).toBeInTheDocument();
		});

		it('renders children content', () => {
			render(<Button>Test Content</Button>);
			expect(screen.getByText('Test Content')).toBeInTheDocument();
		});

		it('renders with icon and text', () => {
			render(<Button icon={Plus}>New Task</Button>);
			const button = screen.getByRole('button', { name: 'New Task' });
			expect(button).toBeInTheDocument();
			expect(button.querySelector('svg')).toBeInTheDocument();
		});
	});

	describe('variants', () => {
		const variants: ButtonVariant[] = ['primary', 'secondary', 'ghost', 'danger', 'success', 'icon'];

		it.each(variants)('renders %s variant with correct class', (variant) => {
			render(
				<Button variant={variant} aria-label={variant === 'icon' ? 'Icon button' : undefined}>
					{variant !== 'icon' ? 'Button' : undefined}
				</Button>
			);
			const button = screen.getByRole('button');
			expect(button).toHaveClass(`btn-${variant}`);
		});

		it('uses primary variant by default', () => {
			render(<Button>Button</Button>);
			const button = screen.getByRole('button');
			expect(button).toHaveClass('btn-primary');
		});
	});

	describe('sizes', () => {
		it('applies sm size class', () => {
			render(<Button size="sm">Small</Button>);
			const button = screen.getByRole('button');
			expect(button).toHaveClass('btn-sm');
		});

		it('applies lg size class', () => {
			render(<Button size="lg">Large</Button>);
			const button = screen.getByRole('button');
			expect(button).toHaveClass('btn-lg');
		});

		it('does not apply size class for md (default)', () => {
			render(<Button size="md">Medium</Button>);
			const button = screen.getByRole('button');
			expect(button).not.toHaveClass('btn-md');
			expect(button).not.toHaveClass('btn-sm');
			expect(button).not.toHaveClass('btn-lg');
		});
	});

	describe('icon variant', () => {
		it('renders icon-only button', () => {
			render(<Button variant="icon" icon={Settings} aria-label="Settings" />);
			const button = screen.getByRole('button', { name: 'Settings' });
			expect(button).toHaveClass('btn-icon');
			expect(button.querySelector('svg')).toBeInTheDocument();
		});

		it('does not render children for icon variant', () => {
			render(
				<Button variant="icon" icon={Settings} aria-label="Settings">
					Should not appear
				</Button>
			);
			expect(screen.queryByText('Should not appear')).not.toBeInTheDocument();
		});

		it('requires aria-label for accessibility', () => {
			render(<Button variant="icon" icon={Settings} aria-label="Open settings" />);
			const button = screen.getByRole('button', { name: 'Open settings' });
			expect(button).toHaveAttribute('aria-label', 'Open settings');
		});
	});

	describe('icon in buttons', () => {
		it('renders icon before text', () => {
			const { container } = render(<Button icon={Plus}>Add Item</Button>);
			const button = container.querySelector('.btn');
			const svg = button?.querySelector('svg');
			expect(svg).toBeInTheDocument();
		});

		it('hides icon from screen readers', () => {
			const { container } = render(<Button icon={Plus}>Add</Button>);
			const svg = container.querySelector('svg');
			expect(svg).toHaveAttribute('aria-hidden', 'true');
		});
	});

	describe('loading state', () => {
		it('shows spinner when loading', () => {
			const { container } = render(<Button loading>Loading</Button>);
			const spinner = container.querySelector('.btn-spinner');
			expect(spinner).toBeInTheDocument();
		});

		it('applies loading class', () => {
			render(<Button loading>Loading</Button>);
			const button = screen.getByRole('button');
			expect(button).toHaveClass('btn-loading');
		});

		it('disables button when loading', () => {
			render(<Button loading>Loading</Button>);
			const button = screen.getByRole('button');
			expect(button).toBeDisabled();
		});

		it('sets aria-busy when loading', () => {
			render(<Button loading>Loading</Button>);
			const button = screen.getByRole('button');
			expect(button).toHaveAttribute('aria-busy', 'true');
		});

		it('does not trigger onClick when loading', () => {
			const onClick = vi.fn();
			render(
				<Button loading onClick={onClick}>
					Loading
				</Button>
			);
			const button = screen.getByRole('button');
			fireEvent.click(button);
			expect(onClick).not.toHaveBeenCalled();
		});
	});

	describe('disabled state', () => {
		it('disables button when disabled prop is true', () => {
			render(<Button disabled>Disabled</Button>);
			const button = screen.getByRole('button');
			expect(button).toBeDisabled();
		});

		it('applies disabled class', () => {
			render(<Button disabled>Disabled</Button>);
			const button = screen.getByRole('button');
			expect(button).toHaveClass('btn-disabled');
		});

		it('sets aria-disabled when disabled', () => {
			render(<Button disabled>Disabled</Button>);
			const button = screen.getByRole('button');
			expect(button).toHaveAttribute('aria-disabled', 'true');
		});

		it('reduces opacity via CSS class', () => {
			const { container } = render(<Button disabled>Disabled</Button>);
			const button = container.querySelector('.btn-disabled');
			expect(button).toBeInTheDocument();
		});

		it('does not trigger onClick when disabled', () => {
			const onClick = vi.fn();
			render(
				<Button disabled onClick={onClick}>
					Disabled
				</Button>
			);
			const button = screen.getByRole('button');
			fireEvent.click(button);
			expect(onClick).not.toHaveBeenCalled();
		});
	});

	describe('active state', () => {
		it('applies active class when active prop is true', () => {
			render(
				<Button variant="icon" icon={Settings} active aria-label="Settings (active)" />
			);
			const button = screen.getByRole('button');
			expect(button).toHaveClass('btn-active');
		});

		it('does not apply active class by default', () => {
			render(<Button variant="icon" icon={Settings} aria-label="Settings" />);
			const button = screen.getByRole('button');
			expect(button).not.toHaveClass('btn-active');
		});
	});

	describe('fullWidth', () => {
		it('applies full width class', () => {
			render(<Button fullWidth>Full Width</Button>);
			const button = screen.getByRole('button');
			expect(button).toHaveClass('btn-full');
		});
	});

	describe('button type', () => {
		it('defaults to type="button"', () => {
			render(<Button>Button</Button>);
			const button = screen.getByRole('button');
			expect(button).toHaveAttribute('type', 'button');
		});

		it('allows type="submit" for forms', () => {
			render(<Button type="submit">Submit</Button>);
			const button = screen.getByRole('button');
			expect(button).toHaveAttribute('type', 'submit');
		});

		it('allows type="reset"', () => {
			render(<Button type="reset">Reset</Button>);
			const button = screen.getByRole('button');
			expect(button).toHaveAttribute('type', 'reset');
		});
	});

	describe('click handling', () => {
		it('calls onClick when clicked', () => {
			const onClick = vi.fn();
			render(<Button onClick={onClick}>Click me</Button>);
			const button = screen.getByRole('button');
			fireEvent.click(button);
			expect(onClick).toHaveBeenCalledTimes(1);
		});

		it('passes event to onClick handler', () => {
			const onClick = vi.fn();
			render(<Button onClick={onClick}>Click me</Button>);
			const button = screen.getByRole('button');
			fireEvent.click(button);
			expect(onClick).toHaveBeenCalledWith(expect.any(Object));
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref correctly', () => {
			const ref = createRef<HTMLButtonElement>();
			render(<Button ref={ref}>Button</Button>);
			expect(ref.current).toBeInstanceOf(HTMLButtonElement);
			expect(ref.current?.tagName).toBe('BUTTON');
		});
	});

	describe('custom className', () => {
		it('applies custom className', () => {
			render(<Button className="custom-class">Button</Button>);
			const button = screen.getByRole('button');
			expect(button).toHaveClass('custom-class');
			expect(button).toHaveClass('btn');
		});

		it('merges multiple classes correctly', () => {
			render(
				<Button className="class-a class-b" variant="ghost" size="sm">
					Ghost
				</Button>
			);
			const button = screen.getByRole('button');
			expect(button).toHaveClass('btn');
			expect(button).toHaveClass('btn-ghost');
			expect(button).toHaveClass('btn-sm');
			expect(button).toHaveClass('class-a');
			expect(button).toHaveClass('class-b');
		});
	});

	describe('HTML attributes', () => {
		it('passes through native button attributes', () => {
			render(<Button data-testid="test-button" title="Test tooltip">Button</Button>);
			const button = screen.getByTestId('test-button');
			expect(button).toHaveAttribute('title', 'Test tooltip');
		});

		it('supports aria attributes', () => {
			render(<Button aria-label="Custom label" aria-describedby="desc">Button</Button>);
			const button = screen.getByRole('button');
			expect(button).toHaveAttribute('aria-label', 'Custom label');
			expect(button).toHaveAttribute('aria-describedby', 'desc');
		});
	});

	describe('form integration', () => {
		it('submits form when type="submit"', () => {
			const onSubmit = vi.fn((e: React.FormEvent) => e.preventDefault());
			render(
				<form onSubmit={onSubmit}>
					<Button type="submit">Submit</Button>
				</form>
			);
			const button = screen.getByRole('button');
			fireEvent.click(button);
			expect(onSubmit).toHaveBeenCalledTimes(1);
		});
	});

	describe('combined props', () => {
		it('handles all props together', () => {
			const onClick = vi.fn();
			render(
				<Button
					variant="ghost"
					size="sm"
					icon={Pause}
					className="custom"
					data-testid="test-button"
					onClick={onClick}
				>
					Pause
				</Button>
			);
			const button = screen.getByTestId('test-button');
			expect(button).toHaveClass('btn');
			expect(button).toHaveClass('btn-ghost');
			expect(button).toHaveClass('btn-sm');
			expect(button).toHaveClass('custom');
			expect(button.querySelector('svg')).toBeInTheDocument();
			fireEvent.click(button);
			expect(onClick).toHaveBeenCalled();
		});
	});
});
