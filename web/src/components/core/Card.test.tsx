import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { createRef } from 'react';
import { Card, type CardPadding } from './Card';

describe('Card', () => {
	describe('rendering', () => {
		it('renders a div element', () => {
			render(<Card data-testid="card">Content</Card>);
			expect(screen.getByTestId('card')).toBeInTheDocument();
			expect(screen.getByTestId('card').tagName).toBe('DIV');
		});

		it('renders children content', () => {
			render(<Card>Test Content</Card>);
			expect(screen.getByText('Test Content')).toBeInTheDocument();
		});

		it('renders complex children', () => {
			render(
				<Card>
					<h2>Title</h2>
					<p>Description</p>
				</Card>
			);
			expect(screen.getByText('Title')).toBeInTheDocument();
			expect(screen.getByText('Description')).toBeInTheDocument();
		});
	});

	describe('padding variants', () => {
		const paddings: CardPadding[] = ['sm', 'md', 'lg'];

		it.each(paddings)('renders %s padding variant with correct class', (padding) => {
			const { container } = render(<Card padding={padding}>Content</Card>);
			const card = container.querySelector('.card');
			expect(card).toHaveClass(`card-padding-${padding}`);
		});

		it('uses md padding by default', () => {
			const { container } = render(<Card>Content</Card>);
			const card = container.querySelector('.card');
			expect(card).toHaveClass('card-padding-md');
		});
	});

	describe('hoverable variant', () => {
		it('applies hoverable class when hoverable is true', () => {
			const { container } = render(<Card hoverable>Content</Card>);
			const card = container.querySelector('.card');
			expect(card).toHaveClass('card-hoverable');
		});

		it('does not apply hoverable class by default', () => {
			const { container } = render(<Card>Content</Card>);
			const card = container.querySelector('.card');
			expect(card).not.toHaveClass('card-hoverable');
		});

		it('does not apply hoverable class when hoverable is false', () => {
			const { container } = render(<Card hoverable={false}>Content</Card>);
			const card = container.querySelector('.card');
			expect(card).not.toHaveClass('card-hoverable');
		});
	});

	describe('active variant', () => {
		it('applies active class when active is true', () => {
			const { container } = render(<Card active>Content</Card>);
			const card = container.querySelector('.card');
			expect(card).toHaveClass('card-active');
		});

		it('does not apply active class by default', () => {
			const { container } = render(<Card>Content</Card>);
			const card = container.querySelector('.card');
			expect(card).not.toHaveClass('card-active');
		});

		it('does not apply active class when active is false', () => {
			const { container } = render(<Card active={false}>Content</Card>);
			const card = container.querySelector('.card');
			expect(card).not.toHaveClass('card-active');
		});
	});

	describe('interactive (clickable) behavior', () => {
		it('applies interactive class when onClick is provided', () => {
			const handleClick = vi.fn();
			const { container } = render(<Card onClick={handleClick}>Content</Card>);
			const card = container.querySelector('.card');
			expect(card).toHaveClass('card-interactive');
		});

		it('does not apply interactive class when onClick is not provided', () => {
			const { container } = render(<Card>Content</Card>);
			const card = container.querySelector('.card');
			expect(card).not.toHaveClass('card-interactive');
		});

		it('calls onClick when clicked', () => {
			const handleClick = vi.fn();
			render(<Card onClick={handleClick}>Content</Card>);
			fireEvent.click(screen.getByText('Content'));
			expect(handleClick).toHaveBeenCalledTimes(1);
		});

		it('has cursor pointer via CSS class for interactive cards', () => {
			const handleClick = vi.fn();
			const { container } = render(<Card onClick={handleClick}>Content</Card>);
			const card = container.querySelector('.card');
			expect(card).toHaveClass('card-interactive');
		});
	});

	describe('keyboard accessibility', () => {
		it('sets role="button" when interactive', () => {
			const handleClick = vi.fn();
			const { container } = render(<Card onClick={handleClick}>Content</Card>);
			const card = container.querySelector('.card');
			expect(card).toHaveAttribute('role', 'button');
		});

		it('does not set role when not interactive', () => {
			const { container } = render(<Card>Content</Card>);
			const card = container.querySelector('.card');
			expect(card).not.toHaveAttribute('role');
		});

		it('sets tabIndex=0 when interactive', () => {
			const handleClick = vi.fn();
			const { container } = render(<Card onClick={handleClick}>Content</Card>);
			const card = container.querySelector('.card');
			expect(card).toHaveAttribute('tabIndex', '0');
		});

		it('does not set tabIndex when not interactive', () => {
			const { container } = render(<Card>Content</Card>);
			const card = container.querySelector('.card');
			expect(card).not.toHaveAttribute('tabIndex');
		});

		it('triggers onClick on Enter key press', () => {
			const handleClick = vi.fn();
			const { container } = render(<Card onClick={handleClick}>Content</Card>);
			const card = container.querySelector('.card')!;
			fireEvent.keyDown(card, { key: 'Enter' });
			expect(handleClick).toHaveBeenCalledTimes(1);
		});

		it('triggers onClick on Space key press', () => {
			const handleClick = vi.fn();
			const { container } = render(<Card onClick={handleClick}>Content</Card>);
			const card = container.querySelector('.card')!;
			fireEvent.keyDown(card, { key: ' ' });
			expect(handleClick).toHaveBeenCalledTimes(1);
		});

		it('does not trigger onClick on other key presses', () => {
			const handleClick = vi.fn();
			const { container } = render(<Card onClick={handleClick}>Content</Card>);
			const card = container.querySelector('.card')!;
			fireEvent.keyDown(card, { key: 'Escape' });
			fireEvent.keyDown(card, { key: 'Tab' });
			fireEvent.keyDown(card, { key: 'a' });
			expect(handleClick).not.toHaveBeenCalled();
		});

		it('calls existing onKeyDown handler before internal handler', () => {
			const handleClick = vi.fn();
			const handleKeyDown = vi.fn();
			const { container } = render(
				<Card onClick={handleClick} onKeyDown={handleKeyDown}>
					Content
				</Card>
			);
			const card = container.querySelector('.card')!;
			fireEvent.keyDown(card, { key: 'Enter' });
			expect(handleKeyDown).toHaveBeenCalled();
			expect(handleClick).toHaveBeenCalled();
		});
	});

	describe('ref forwarding', () => {
		it('forwards ref correctly', () => {
			const ref = createRef<HTMLDivElement>();
			render(<Card ref={ref}>Content</Card>);
			expect(ref.current).toBeInstanceOf(HTMLDivElement);
			expect(ref.current?.tagName).toBe('DIV');
		});
	});

	describe('custom className', () => {
		it('applies custom className', () => {
			const { container } = render(<Card className="custom-class">Content</Card>);
			const card = container.querySelector('.card');
			expect(card).toHaveClass('custom-class');
			expect(card).toHaveClass('card');
		});

		it('merges multiple classes correctly', () => {
			const { container } = render(
				<Card className="class-a class-b" hoverable active>
					Content
				</Card>
			);
			const card = container.querySelector('.card');
			expect(card).toHaveClass('card');
			expect(card).toHaveClass('card-hoverable');
			expect(card).toHaveClass('card-active');
			expect(card).toHaveClass('class-a');
			expect(card).toHaveClass('class-b');
		});
	});

	describe('HTML attributes', () => {
		it('passes through native div attributes', () => {
			render(<Card data-testid="test-card" title="Test tooltip">Content</Card>);
			const card = screen.getByTestId('test-card');
			expect(card).toHaveAttribute('title', 'Test tooltip');
		});

		it('supports aria attributes', () => {
			const { container } = render(
				<Card aria-label="Card description">Content</Card>
			);
			const card = container.querySelector('.card');
			expect(card).toHaveAttribute('aria-label', 'Card description');
		});

		it('supports id attribute', () => {
			const { container } = render(<Card id="my-card">Content</Card>);
			const card = container.querySelector('.card');
			expect(card).toHaveAttribute('id', 'my-card');
		});
	});

	describe('nested cards', () => {
		it('supports nested cards', () => {
			render(
				<Card data-testid="outer">
					<Card data-testid="inner">Nested content</Card>
				</Card>
			);
			const outer = screen.getByTestId('outer');
			const inner = screen.getByTestId('inner');
			expect(outer).toBeInTheDocument();
			expect(inner).toBeInTheDocument();
			expect(outer.contains(inner)).toBe(true);
		});

		it('maintains independent states for nested cards', () => {
			const handleOuterClick = vi.fn();
			const handleInnerClick = vi.fn();
			render(
				<Card data-testid="outer" onClick={handleOuterClick}>
					<Card data-testid="inner" onClick={handleInnerClick}>
						Nested content
					</Card>
				</Card>
			);
			const inner = screen.getByTestId('inner');
			fireEvent.click(inner);
			expect(handleInnerClick).toHaveBeenCalledTimes(1);
			// Note: outer will also receive the click due to event bubbling
			// This is expected browser behavior
		});
	});

	describe('combined props', () => {
		it('handles all props together', () => {
			const handleClick = vi.fn();
			const { container } = render(
				<Card
					hoverable
					active
					padding="lg"
					className="custom"
					onClick={handleClick}
					data-testid="full-card"
				>
					Full featured card
				</Card>
			);
			const card = container.querySelector('.card');
			expect(card).toHaveClass('card');
			expect(card).toHaveClass('card-padding-lg');
			expect(card).toHaveClass('card-hoverable');
			expect(card).toHaveClass('card-active');
			expect(card).toHaveClass('card-interactive');
			expect(card).toHaveClass('custom');
			expect(card).toHaveAttribute('role', 'button');
			expect(card).toHaveAttribute('tabIndex', '0');
		});

		it('handles minimal props', () => {
			const { container } = render(<Card>Minimal card</Card>);
			const card = container.querySelector('.card');
			expect(card).toHaveClass('card');
			expect(card).toHaveClass('card-padding-md');
			expect(card).not.toHaveClass('card-hoverable');
			expect(card).not.toHaveClass('card-active');
			expect(card).not.toHaveClass('card-interactive');
			expect(card).not.toHaveAttribute('role');
			expect(card).not.toHaveAttribute('tabIndex');
		});
	});
});
