import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { TranscriptSection, type TranscriptSectionType } from './TranscriptSection';

describe('TranscriptSection', () => {
	describe('rendering', () => {
		it('renders a section element', () => {
			render(
				<TranscriptSection type="phase" title="test" testId="section">
					Content
				</TranscriptSection>
			);
			const section = screen.getByTestId('section');
			expect(section).toBeInTheDocument();
			expect(section.tagName).toBe('SECTION');
		});

		it('renders children content when expanded', () => {
			render(
				<TranscriptSection type="phase" title="test">
					Test Content
				</TranscriptSection>
			);
			expect(screen.getByText('Test Content')).toBeInTheDocument();
		});

		it('renders title', () => {
			render(
				<TranscriptSection type="phase" title="Phase Title">
					Content
				</TranscriptSection>
			);
			expect(screen.getByText('Phase Title')).toBeInTheDocument();
		});

		it('renders subtitle when provided', () => {
			render(
				<TranscriptSection type="tool_call" title="Read" subtitle="file.ts">
					Content
				</TranscriptSection>
			);
			expect(screen.getByText('file.ts')).toBeInTheDocument();
		});

		it('renders timestamp when provided', () => {
			render(
				<TranscriptSection type="response" title="Assistant" timestamp="12:34">
					Content
				</TranscriptSection>
			);
			expect(screen.getByText('12:34')).toBeInTheDocument();
		});

		it('renders badge when provided', () => {
			render(
				<TranscriptSection
					type="response"
					title="Assistant"
					badge={<span data-testid="badge">150 tokens</span>}
				>
					Content
				</TranscriptSection>
			);
			expect(screen.getByTestId('badge')).toBeInTheDocument();
			expect(screen.getByText('150 tokens')).toBeInTheDocument();
		});
	});

	describe('section types', () => {
		const sectionTypes: TranscriptSectionType[] = [
			'phase',
			'iteration',
			'prompt',
			'response',
			'tool_call',
			'tool_result',
			'error',
			'system',
		];

		it.each(sectionTypes)('renders %s section type with correct class', (type) => {
			render(
				<TranscriptSection type={type} title="Test" testId="section">
					Content
				</TranscriptSection>
			);
			const section = screen.getByTestId('section');
			expect(section).toHaveClass(`transcript-section--${type}`);
		});

		it.each(sectionTypes)('sets data-type attribute for %s', (type) => {
			render(
				<TranscriptSection type={type} title="Test" testId="section">
					Content
				</TranscriptSection>
			);
			const section = screen.getByTestId('section');
			expect(section).toHaveAttribute('data-type', type);
		});
	});

	describe('expand/collapse behavior', () => {
		it('is expanded by default', () => {
			render(
				<TranscriptSection type="phase" title="test" testId="section">
					Content
				</TranscriptSection>
			);
			const section = screen.getByTestId('section');
			expect(section).toHaveAttribute('data-expanded', 'true');
			expect(section).toHaveClass('transcript-section--expanded');
			expect(screen.getByText('Content')).toBeInTheDocument();
		});

		it('respects defaultExpanded=false', () => {
			render(
				<TranscriptSection
					type="phase"
					title="test"
					testId="section"
					defaultExpanded={false}
				>
					Content
				</TranscriptSection>
			);
			const section = screen.getByTestId('section');
			expect(section).toHaveAttribute('data-expanded', 'false');
			expect(section).not.toHaveClass('transcript-section--expanded');
			expect(screen.queryByText('Content')).not.toBeInTheDocument();
		});

		it('toggles on header click', () => {
			render(
				<TranscriptSection type="phase" title="test" testId="section">
					Content
				</TranscriptSection>
			);
			const header = screen.getByRole('button');
			const section = screen.getByTestId('section');

			// Initially expanded
			expect(section).toHaveAttribute('data-expanded', 'true');
			expect(screen.getByText('Content')).toBeInTheDocument();

			// Click to collapse
			fireEvent.click(header);
			expect(section).toHaveAttribute('data-expanded', 'false');
			expect(screen.queryByText('Content')).not.toBeInTheDocument();

			// Click to expand
			fireEvent.click(header);
			expect(section).toHaveAttribute('data-expanded', 'true');
			expect(screen.getByText('Content')).toBeInTheDocument();
		});

		it('toggles on Enter key', () => {
			render(
				<TranscriptSection type="phase" title="test" testId="section">
					Content
				</TranscriptSection>
			);
			const header = screen.getByRole('button');
			const section = screen.getByTestId('section');

			fireEvent.keyDown(header, { key: 'Enter' });
			expect(section).toHaveAttribute('data-expanded', 'false');

			fireEvent.keyDown(header, { key: 'Enter' });
			expect(section).toHaveAttribute('data-expanded', 'true');
		});

		it('toggles on Space key', () => {
			render(
				<TranscriptSection type="phase" title="test" testId="section">
					Content
				</TranscriptSection>
			);
			const header = screen.getByRole('button');
			const section = screen.getByTestId('section');

			fireEvent.keyDown(header, { key: ' ' });
			expect(section).toHaveAttribute('data-expanded', 'false');

			fireEvent.keyDown(header, { key: ' ' });
			expect(section).toHaveAttribute('data-expanded', 'true');
		});

		it('does not toggle on other keys', () => {
			render(
				<TranscriptSection type="phase" title="test" testId="section">
					Content
				</TranscriptSection>
			);
			const header = screen.getByRole('button');
			const section = screen.getByTestId('section');

			fireEvent.keyDown(header, { key: 'Escape' });
			fireEvent.keyDown(header, { key: 'Tab' });
			fireEvent.keyDown(header, { key: 'a' });

			expect(section).toHaveAttribute('data-expanded', 'true');
		});
	});

	describe('controlled mode', () => {
		it('uses controlled expanded value', () => {
			const { rerender } = render(
				<TranscriptSection
					type="phase"
					title="test"
					testId="section"
					expanded={false}
				>
					Content
				</TranscriptSection>
			);

			const section = screen.getByTestId('section');
			expect(section).toHaveAttribute('data-expanded', 'false');

			rerender(
				<TranscriptSection type="phase" title="test" testId="section" expanded={true}>
					Content
				</TranscriptSection>
			);
			expect(section).toHaveAttribute('data-expanded', 'true');
		});

		it('calls onExpandedChange when toggled', () => {
			const handleChange = vi.fn();
			render(
				<TranscriptSection
					type="phase"
					title="test"
					expanded={true}
					onExpandedChange={handleChange}
				>
					Content
				</TranscriptSection>
			);

			const header = screen.getByRole('button');
			fireEvent.click(header);

			expect(handleChange).toHaveBeenCalledWith(false);
		});

		it('does not change internal state when controlled', () => {
			const handleChange = vi.fn();
			render(
				<TranscriptSection
					type="phase"
					title="test"
					testId="section"
					expanded={true}
					onExpandedChange={handleChange}
				>
					Content
				</TranscriptSection>
			);

			const header = screen.getByRole('button');
			const section = screen.getByTestId('section');

			// Click should call handler but not change state
			fireEvent.click(header);
			expect(handleChange).toHaveBeenCalledWith(false);
			// State should still be controlled value
			expect(section).toHaveAttribute('data-expanded', 'true');
		});
	});

	describe('accessibility', () => {
		it('header has button role', () => {
			render(
				<TranscriptSection type="phase" title="test">
					Content
				</TranscriptSection>
			);
			expect(screen.getByRole('button')).toBeInTheDocument();
		});

		it('header has aria-expanded attribute', () => {
			render(
				<TranscriptSection type="phase" title="test">
					Content
				</TranscriptSection>
			);
			const header = screen.getByRole('button');
			expect(header).toHaveAttribute('aria-expanded', 'true');

			fireEvent.click(header);
			expect(header).toHaveAttribute('aria-expanded', 'false');
		});

		it('header has aria-controls pointing to content', () => {
			render(
				<TranscriptSection type="phase" title="Phase Test">
					Content
				</TranscriptSection>
			);
			const header = screen.getByRole('button');
			expect(header).toHaveAttribute('aria-controls', 'section-content-phase-test');
		});

		it('content has matching id for aria-controls', () => {
			render(
				<TranscriptSection type="phase" title="Phase Test">
					Content
				</TranscriptSection>
			);
			const content = screen.getByText('Content').closest('.transcript-section-content');
			expect(content).toHaveAttribute('id', 'section-content-phase-test');
		});
	});

	describe('depth/nesting', () => {
		it('applies depth-1 class for depth=1', () => {
			render(
				<TranscriptSection type="phase" title="test" testId="section" depth={1}>
					Content
				</TranscriptSection>
			);
			expect(screen.getByTestId('section')).toHaveClass('transcript-section--depth-1');
		});

		it('applies depth-2 class for depth=2', () => {
			render(
				<TranscriptSection type="phase" title="test" testId="section" depth={2}>
					Content
				</TranscriptSection>
			);
			expect(screen.getByTestId('section')).toHaveClass('transcript-section--depth-2');
		});

		it('applies depth-3 class for depth=3', () => {
			render(
				<TranscriptSection type="phase" title="test" testId="section" depth={3}>
					Content
				</TranscriptSection>
			);
			expect(screen.getByTestId('section')).toHaveClass('transcript-section--depth-3');
		});

		it('caps depth class at 3', () => {
			render(
				<TranscriptSection type="phase" title="test" testId="section" depth={5}>
					Content
				</TranscriptSection>
			);
			const section = screen.getByTestId('section');
			expect(section).toHaveClass('transcript-section--depth-3');
			expect(section).not.toHaveClass('transcript-section--depth-5');
		});

		it('does not apply depth class when depth=0', () => {
			render(
				<TranscriptSection type="phase" title="test" testId="section" depth={0}>
					Content
				</TranscriptSection>
			);
			const section = screen.getByTestId('section');
			expect(section).not.toHaveClass('transcript-section--depth-0');
			expect(section).not.toHaveClass('transcript-section--depth-1');
		});
	});

	describe('className', () => {
		it('applies custom className', () => {
			render(
				<TranscriptSection
					type="phase"
					title="test"
					testId="section"
					className="custom-class"
				>
					Content
				</TranscriptSection>
			);
			const section = screen.getByTestId('section');
			expect(section).toHaveClass('custom-class');
			expect(section).toHaveClass('transcript-section');
		});
	});

	describe('nested sections', () => {
		it('supports nested sections', () => {
			render(
				<TranscriptSection type="phase" title="Outer" testId="outer">
					<TranscriptSection type="iteration" title="Inner" testId="inner">
						Nested content
					</TranscriptSection>
				</TranscriptSection>
			);

			const outer = screen.getByTestId('outer');
			const inner = screen.getByTestId('inner');

			expect(outer).toBeInTheDocument();
			expect(inner).toBeInTheDocument();
			expect(outer.contains(inner)).toBe(true);
		});

		it('maintains independent expanded states for nested sections', () => {
			render(
				<TranscriptSection type="phase" title="Outer" testId="outer">
					<TranscriptSection type="iteration" title="Inner" testId="inner">
						Nested content
					</TranscriptSection>
				</TranscriptSection>
			);

			const outer = screen.getByTestId('outer');
			const inner = screen.getByTestId('inner');
			const innerHeader = inner.querySelector('.transcript-section-header')!;

			// Both start expanded
			expect(outer).toHaveAttribute('data-expanded', 'true');
			expect(inner).toHaveAttribute('data-expanded', 'true');

			// Collapse inner only
			fireEvent.click(innerHeader);
			expect(outer).toHaveAttribute('data-expanded', 'true');
			expect(inner).toHaveAttribute('data-expanded', 'false');
		});
	});

	describe('complex children', () => {
		it('renders pre elements', () => {
			render(
				<TranscriptSection type="tool_result" title="Output">
					<pre>console.log('hello')</pre>
				</TranscriptSection>
			);
			expect(screen.getByText("console.log('hello')")).toBeInTheDocument();
		});

		it('renders multiple children', () => {
			render(
				<TranscriptSection type="response" title="Assistant">
					<p>First paragraph</p>
					<p>Second paragraph</p>
				</TranscriptSection>
			);
			expect(screen.getByText('First paragraph')).toBeInTheDocument();
			expect(screen.getByText('Second paragraph')).toBeInTheDocument();
		});
	});
});
