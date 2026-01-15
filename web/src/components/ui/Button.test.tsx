import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { createRef } from 'react';
import { Button, type ButtonVariant, type ButtonSize } from './Button';
import { Icon } from './Icon';

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
	});

	describe('variants', () => {
		const variants: ButtonVariant[] = ['primary', 'secondary', 'danger', 'ghost', 'success'];

		it.each(variants)('renders %s variant with correct class', (variant) => {
			const { container } = render(<Button variant={variant}>Button</Button>);
			const button = container.querySelector('button');
			expect(button).toHaveClass(`btn-${variant}`);
		});

		it('uses secondary variant by default', () => {
			const { container } = render(<Button>Button</Button>);
			const button = container.querySelector('button');
			expect(button).toHaveClass('btn-secondary');
		});
	});

	describe('sizes', () => {
		const sizes: ButtonSize[] = ['sm', 'md', 'lg'];

		it.each(sizes)('renders %s size with correct class', (size) => {
			const { container } = render(<Button size={size}>Button</Button>);
			const button = container.querySelector('button');
			expect(button).toHaveClass(`btn-${size}`);
		});

		it('uses medium size by default', () => {
			const { container } = render(<Button>Button</Button>);
			const button = container.querySelector('button');
			expect(button).toHaveClass('btn-md');
		});
	});

	describe('icons', () => {
		it('renders with left icon', () => {
			const { container } = render(<Button leftIcon={<Icon name="plus" />}>Add</Button>);
			const leftIconContainer = container.querySelector('.btn-icon-left');
			expect(leftIconContainer).toBeInTheDocument();
			// Check that icon container has an SVG child
			expect(leftIconContainer?.querySelector('svg')).toBeInTheDocument();
		});

		it('renders with right icon', () => {
			const { container } = render(
				<Button rightIcon={<Icon name="chevron-right" />}>Next</Button>
			);
			const rightIconContainer = container.querySelector('.btn-icon-right');
			expect(rightIconContainer).toBeInTheDocument();
			// Check that icon container has an SVG child
			expect(rightIconContainer?.querySelector('svg')).toBeInTheDocument();
		});

		it('renders with both icons', () => {
			const { container } = render(
				<Button leftIcon={<Icon name="plus" />} rightIcon={<Icon name="chevron-right" />}>
					Action
				</Button>
			);
			const leftContainer = container.querySelector('.btn-icon-left');
			const rightContainer = container.querySelector('.btn-icon-right');
			expect(leftContainer).toBeInTheDocument();
			expect(rightContainer).toBeInTheDocument();
			expect(leftContainer?.querySelector('svg')).toBeInTheDocument();
			expect(rightContainer?.querySelector('svg')).toBeInTheDocument();
		});

		it('renders icon-only mode correctly', () => {
			const { container } = render(
				<Button iconOnly aria-label="Add item">
					<Icon name="plus" />
				</Button>
			);
			const button = container.querySelector('button');
			expect(button).toHaveClass('btn-icon-only');
			// Content should not be wrapped in btn-content span in icon-only mode
			expect(container.querySelector('.btn-content')).not.toBeInTheDocument();
		});
	});

	describe('loading state', () => {
		it('shows spinner when loading', () => {
			const { container } = render(<Button loading>Loading</Button>);
			const spinner = container.querySelector('.btn-spinner');
			expect(spinner).toBeInTheDocument();
		});

		it('renders content element when loading (hidden via CSS)', () => {
			const { container } = render(<Button loading>Loading</Button>);
			const content = container.querySelector('.btn-content');
			// Content is rendered but hidden via CSS .btn-loading .btn-content rule
			expect(content).toBeInTheDocument();
			// Button should have btn-loading class which applies visibility:hidden to content via CSS
			const button = container.querySelector('button');
			expect(button).toHaveClass('btn-loading');
		});

		it('hides icons when loading', () => {
			const { container } = render(
				<Button loading leftIcon={<Icon name="plus" />} rightIcon={<Icon name="check" />}>
					Loading
				</Button>
			);
			expect(container.querySelector('.btn-icon-left')).not.toBeInTheDocument();
			expect(container.querySelector('.btn-icon-right')).not.toBeInTheDocument();
		});

		it('applies btn-loading class', () => {
			const { container } = render(<Button loading>Loading</Button>);
			const button = container.querySelector('button');
			expect(button).toHaveClass('btn-loading');
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
			// Should also retain base classes
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
			expect(button).toHaveClass('btn-primary');
			expect(button).toHaveClass('btn-lg');
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
				<Button iconOnly aria-label="Delete item">
					<Icon name="trash" />
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
			const spinner = container.querySelector('.btn-spinner');
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

	describe('combined states', () => {
		it('handles loading + primary correctly', () => {
			const { container } = render(
				<Button loading variant="primary">
					Loading
				</Button>
			);
			const button = container.querySelector('button');
			expect(button).toHaveClass('btn-primary');
			expect(button).toHaveClass('btn-loading');
			expect(button).toBeDisabled();
		});

		it('handles all props together', () => {
			const handleClick = vi.fn();
			const { container } = render(
				<Button
					variant="danger"
					size="lg"
					leftIcon={<Icon name="trash" />}
					onClick={handleClick}
					className="custom"
				>
					Delete
				</Button>
			);
			const button = container.querySelector('button');
			expect(button).toHaveClass('btn');
			expect(button).toHaveClass('btn-danger');
			expect(button).toHaveClass('btn-lg');
			expect(button).toHaveClass('custom');
			expect(container.querySelector('.btn-icon-left')).toBeInTheDocument();
			fireEvent.click(button!);
			expect(handleClick).toHaveBeenCalled();
		});
	});
});
