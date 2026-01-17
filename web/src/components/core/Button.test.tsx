import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { createRef } from 'react';
import { Button, type ButtonVariant, type ButtonSize } from './Button';

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

		it('defaults to type="button"', () => {
			render(<Button>Button</Button>);
			expect(screen.getByRole('button')).toHaveAttribute('type', 'button');
		});

		it('can be used as submit button', () => {
			render(<Button type="submit">Submit</Button>);
			expect(screen.getByRole('button')).toHaveAttribute('type', 'submit');
		});
	});

	describe('variants', () => {
		const variants: ButtonVariant[] = ['primary', 'ghost', 'icon'];

		it.each(variants)('renders %s variant with correct class', (variant) => {
			const { container } = render(
				<Button variant={variant} aria-label={variant === 'icon' ? 'Icon button' : undefined}>
					{variant === 'icon' ? <span>X</span> : 'Button'}
				</Button>
			);
			const button = container.querySelector('button');
			expect(button).toHaveClass(`btn--${variant}`);
		});

		it('uses ghost variant by default', () => {
			const { container } = render(<Button>Button</Button>);
			const button = container.querySelector('button');
			expect(button).toHaveClass('btn--ghost');
		});
	});

	describe('sizes', () => {
		const sizes: ButtonSize[] = ['sm', 'md', 'lg'];

		it.each(sizes)('renders %s size with correct class', (size) => {
			const { container } = render(<Button size={size}>Button</Button>);
			const button = container.querySelector('button');
			expect(button).toHaveClass(`btn--${size}`);
		});

		it('uses medium size by default', () => {
			const { container } = render(<Button>Button</Button>);
			const button = container.querySelector('button');
			expect(button).toHaveClass('btn--md');
		});
	});

	describe('primary variant', () => {
		it('renders with primary class', () => {
			const { container } = render(<Button variant="primary">Primary</Button>);
			const button = container.querySelector('button');
			expect(button).toHaveClass('btn--primary');
		});
	});

	describe('ghost variant', () => {
		it('renders with ghost class', () => {
			const { container } = render(<Button variant="ghost">Ghost</Button>);
			const button = container.querySelector('button');
			expect(button).toHaveClass('btn--ghost');
		});
	});

	describe('icon variant', () => {
		it('renders with icon class', () => {
			const { container } = render(
				<Button variant="icon" aria-label="Icon button">
					<span>X</span>
				</Button>
			);
			const button = container.querySelector('button');
			expect(button).toHaveClass('btn--icon');
		});

		it('icon-only buttons require aria-label for accessibility', () => {
			render(
				<Button variant="icon" aria-label="Close dialog">
					<span>X</span>
				</Button>
			);
			const button = screen.getByRole('button', { name: 'Close dialog' });
			expect(button).toBeInTheDocument();
		});

		it('renders children directly in icon mode (not wrapped)', () => {
			const { container } = render(
				<Button variant="icon" aria-label="Test icon">
					<span data-testid="icon-child">X</span>
				</Button>
			);
			// In icon mode, children should NOT be wrapped in btn__content
			expect(container.querySelector('.btn__content')).not.toBeInTheDocument();
			expect(screen.getByTestId('icon-child')).toBeInTheDocument();
		});
	});

	describe('icon prop', () => {
		it('renders icon before text for non-icon variants', () => {
			const { container } = render(
				<Button icon={<span data-testid="btn-icon">+</span>}>Add</Button>
			);
			const iconContainer = container.querySelector('.btn__icon');
			expect(iconContainer).toBeInTheDocument();
			expect(screen.getByTestId('btn-icon')).toBeInTheDocument();
		});

		it('does not render icon prop for icon variant', () => {
			render(
				<Button variant="icon" icon={<span data-testid="ignored-icon">!</span>} aria-label="Test">
					<span data-testid="main-icon">X</span>
				</Button>
			);
			// icon prop should be ignored for icon variant
			expect(screen.queryByTestId('ignored-icon')).not.toBeInTheDocument();
			expect(screen.getByTestId('main-icon')).toBeInTheDocument();
		});
	});

	describe('loading state', () => {
		it('shows spinner when loading', () => {
			const { container } = render(<Button loading>Loading</Button>);
			const spinner = container.querySelector('.btn__spinner');
			expect(spinner).toBeInTheDocument();
		});

		it('renders content element when loading (hidden via CSS)', () => {
			const { container } = render(<Button loading>Loading</Button>);
			const content = container.querySelector('.btn__content');
			expect(content).toBeInTheDocument();
			const button = container.querySelector('button');
			expect(button).toHaveClass('btn--loading');
		});

		it('hides icon when loading', () => {
			const { container } = render(
				<Button loading icon={<span data-testid="test-icon">+</span>}>
					Loading
				</Button>
			);
			expect(container.querySelector('.btn__icon')).not.toBeInTheDocument();
		});

		it('applies btn--loading class', () => {
			const { container } = render(<Button loading>Loading</Button>);
			const button = container.querySelector('button');
			expect(button).toHaveClass('btn--loading');
		});

		it('sets aria-busy when loading', () => {
			render(<Button loading>Loading</Button>);
			const button = screen.getByRole('button');
			expect(button).toHaveAttribute('aria-busy', 'true');
		});

		it('is disabled when loading', () => {
			render(<Button loading>Loading</Button>);
			const button = screen.getByRole('button');
			expect(button).toBeDisabled();
		});

		it('disables click when loading', () => {
			const handleClick = vi.fn();
			render(
				<Button loading onClick={handleClick}>
					Loading
				</Button>
			);
			fireEvent.click(screen.getByRole('button'));
			expect(handleClick).not.toHaveBeenCalled();
		});
	});

	describe('disabled state', () => {
		it('is disabled when disabled prop is true', () => {
			render(<Button disabled>Disabled</Button>);
			const button = screen.getByRole('button');
			expect(button).toBeDisabled();
		});

		it('sets aria-disabled when disabled', () => {
			render(<Button disabled>Disabled</Button>);
			const button = screen.getByRole('button');
			expect(button).toHaveAttribute('aria-disabled', 'true');
		});

		it('has reduced opacity (verified via class)', () => {
			const { container } = render(<Button disabled>Disabled</Button>);
			const button = container.querySelector('button');
			expect(button).toBeDisabled();
		});

		it('prevents interaction when disabled', () => {
			const handleClick = vi.fn();
			render(
				<Button disabled onClick={handleClick}>
					Disabled
				</Button>
			);
			const button = screen.getByRole('button');
			fireEvent.click(button);
			expect(handleClick).not.toHaveBeenCalled();
		});
	});

	describe('click handling', () => {
		it('calls onClick when clicked', () => {
			const handleClick = vi.fn();
			render(<Button onClick={handleClick}>Click me</Button>);
			fireEvent.click(screen.getByRole('button'));
			expect(handleClick).toHaveBeenCalledTimes(1);
		});

		it('does NOT call onClick when disabled', () => {
			const handleClick = vi.fn();
			render(
				<Button disabled onClick={handleClick}>
					Click me
				</Button>
			);
			fireEvent.click(screen.getByRole('button'));
			expect(handleClick).not.toHaveBeenCalled();
		});

		it('does NOT call onClick when loading', () => {
			const handleClick = vi.fn();
			render(
				<Button loading onClick={handleClick}>
					Click me
				</Button>
			);
			fireEvent.click(screen.getByRole('button'));
			expect(handleClick).not.toHaveBeenCalled();
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
			const { container } = render(<Button className="custom-class">Button</Button>);
			const button = container.querySelector('button');
			expect(button).toHaveClass('custom-class');
			expect(button).toHaveClass('btn');
		});

		it('merges multiple classes correctly', () => {
			const { container } = render(
				<Button className="class-a class-b" variant="primary" size="lg">
					Button
				</Button>
			);
			const button = container.querySelector('button');
			expect(button).toHaveClass('btn');
			expect(button).toHaveClass('btn--primary');
			expect(button).toHaveClass('btn--lg');
			expect(button).toHaveClass('class-a');
			expect(button).toHaveClass('class-b');
		});
	});

	describe('HTML attributes', () => {
		it('passes through native button attributes', () => {
			render(
				<Button type="submit" name="test-button" data-testid="test-btn">
					Submit
				</Button>
			);
			const button = screen.getByTestId('test-btn');
			expect(button).toHaveAttribute('type', 'submit');
			expect(button).toHaveAttribute('name', 'test-button');
		});

		it('supports aria-label for icon-only buttons', () => {
			render(
				<Button variant="icon" aria-label="Delete item">
					<span>X</span>
				</Button>
			);
			const button = screen.getByRole('button', { name: 'Delete item' });
			expect(button).toBeInTheDocument();
		});
	});

	describe('accessibility', () => {
		it('has correct role', () => {
			render(<Button>Button</Button>);
			expect(screen.getByRole('button')).toBeInTheDocument();
		});

		it('spinner is aria-hidden', () => {
			const { container } = render(<Button loading>Loading</Button>);
			const spinner = container.querySelector('.btn__spinner');
			expect(spinner).toHaveAttribute('aria-hidden', 'true');
		});

		it('does not set aria-busy when not loading', () => {
			render(<Button>Button</Button>);
			const button = screen.getByRole('button');
			expect(button).not.toHaveAttribute('aria-busy');
		});

		it('does not set aria-disabled when not disabled', () => {
			render(<Button>Button</Button>);
			const button = screen.getByRole('button');
			expect(button).not.toHaveAttribute('aria-disabled');
		});
	});

	describe('form integration', () => {
		it('works as submit button in forms', () => {
			const handleSubmit = vi.fn((e) => e.preventDefault());
			render(
				<form onSubmit={handleSubmit}>
					<Button type="submit">Submit</Button>
				</form>
			);
			fireEvent.click(screen.getByRole('button'));
			expect(handleSubmit).toHaveBeenCalledTimes(1);
		});

		it('does not submit when disabled', () => {
			const handleSubmit = vi.fn((e) => e.preventDefault());
			render(
				<form onSubmit={handleSubmit}>
					<Button type="submit" disabled>
						Submit
					</Button>
				</form>
			);
			fireEvent.click(screen.getByRole('button'));
			expect(handleSubmit).not.toHaveBeenCalled();
		});
	});

	describe('combined states', () => {
		it('handles loading + primary correctly', () => {
			const { container } = render(
				<Button loading variant="primary">
					Loading
				</Button>
			);
			const button = container.querySelector('button');
			expect(button).toHaveClass('btn--primary');
			expect(button).toHaveClass('btn--loading');
			expect(button).toBeDisabled();
		});

		it('handles all props together', () => {
			const handleClick = vi.fn();
			const { container } = render(
				<Button
					variant="primary"
					size="lg"
					icon={<span data-testid="test-icon">+</span>}
					onClick={handleClick}
					className="custom"
				>
					Add Item
				</Button>
			);
			const button = container.querySelector('button');
			expect(button).toHaveClass('btn');
			expect(button).toHaveClass('btn--primary');
			expect(button).toHaveClass('btn--lg');
			expect(button).toHaveClass('custom');
			expect(container.querySelector('.btn__icon')).toBeInTheDocument();
			expect(screen.getByTestId('test-icon')).toBeInTheDocument();
			fireEvent.click(button!);
			expect(handleClick).toHaveBeenCalled();
		});
	});
});
