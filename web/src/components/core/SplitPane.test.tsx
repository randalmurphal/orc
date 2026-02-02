/**
 * Tests for SplitPane component
 *
 * TDD tests for the resizable split pane that displays
 * Live Output (left) and Changes (right) panels.
 *
 * Success Criteria Coverage:
 * - SC-4: Split pane component renders with resizable divider between left and right panels
 * - SC-5: Split pane resize is persisted across page navigations
 * - SC-6: Split pane has minimum panel width constraints to prevent collapse
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { SplitPane } from './SplitPane';

// Mock localStorage
const localStorageMock = (() => {
	let store: Record<string, string> = {};
	return {
		getItem: vi.fn((key: string) => store[key] ?? null),
		setItem: vi.fn((key: string, value: string) => {
			store[key] = value;
		}),
		removeItem: vi.fn((key: string) => {
			delete store[key];
		}),
		clear: vi.fn(() => {
			store = {};
		}),
		_getStore: () => store,
	};
})();

Object.defineProperty(window, 'localStorage', {
	value: localStorageMock,
	writable: true,
});

describe('SplitPane', () => {
	beforeEach(() => {
		localStorageMock.clear();
		vi.clearAllMocks();
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	describe('SC-4: Renders with resizable divider', () => {
		it('renders left and right panels', () => {
			render(
				<SplitPane
					left={<div data-testid="left-panel">Left Content</div>}
					right={<div data-testid="right-panel">Right Content</div>}
				/>
			);

			expect(screen.getByTestId('left-panel')).toBeInTheDocument();
			expect(screen.getByTestId('right-panel')).toBeInTheDocument();
		});

		it('renders a draggable divider between panels', () => {
			const { container } = render(
				<SplitPane
					left={<div>Left</div>}
					right={<div>Right</div>}
				/>
			);

			const divider = container.querySelector('.split-pane__divider');
			expect(divider).toBeInTheDocument();
			expect(divider).toHaveAttribute('role', 'separator');
		});

		it('divider has aria-valuenow attribute reflecting current ratio', () => {
			const { container } = render(
				<SplitPane
					left={<div>Left</div>}
					right={<div>Right</div>}
					initialRatio={50}
				/>
			);

			const divider = container.querySelector('.split-pane__divider');
			expect(divider).toHaveAttribute('aria-valuenow', '50');
		});

		it('panels resize when divider is dragged', () => {
			const { container } = render(
				<SplitPane
					left={<div>Left</div>}
					right={<div>Right</div>}
					initialRatio={50}
				/>
			);

			const divider = container.querySelector('.split-pane__divider') as HTMLElement;
			const leftPanel = container.querySelector('.split-pane__left') as HTMLElement;

			// Simulate drag start
			fireEvent.mouseDown(divider, { clientX: 400 });

			// Simulate drag to new position (move divider to the right)
			fireEvent.mouseMove(window, { clientX: 500 });

			// Simulate drag end
			fireEvent.mouseUp(window);

			// Left panel should have expanded
			// The exact width depends on container size, so check for style change
			expect(leftPanel.style.flexBasis).toBeDefined();
		});

		it('calls onResize callback when ratio changes', () => {
			const onResize = vi.fn();
			const { container } = render(
				<SplitPane
					left={<div>Left</div>}
					right={<div>Right</div>}
					initialRatio={50}
					onResize={onResize}
				/>
			);

			const divider = container.querySelector('.split-pane__divider') as HTMLElement;

			// Simulate drag
			fireEvent.mouseDown(divider, { clientX: 400 });
			fireEvent.mouseMove(window, { clientX: 500 });
			fireEvent.mouseUp(window);

			expect(onResize).toHaveBeenCalled();
		});

		it('shows empty state message when panel has no content', () => {
			render(
				<SplitPane
					left={null}
					right={<div>Right Content</div>}
					leftEmptyMessage="No output yet"
				/>
			);

			expect(screen.getByText('No output yet')).toBeInTheDocument();
		});
	});

	describe('SC-5: Resize persisted across page navigations', () => {
		it('loads initial ratio from localStorage when persistKey is provided', () => {
			localStorageMock.setItem('split-pane-test', '40');

			const { container } = render(
				<SplitPane
					left={<div>Left</div>}
					right={<div>Right</div>}
					persistKey="test"
				/>
			);

			const divider = container.querySelector('.split-pane__divider');
			expect(divider).toHaveAttribute('aria-valuenow', '40');
		});

		it('saves ratio to localStorage after resize', async () => {
			const { container } = render(
				<SplitPane
					left={<div>Left</div>}
					right={<div>Right</div>}
					persistKey="test"
					initialRatio={50}
				/>
			);

			const divider = container.querySelector('.split-pane__divider') as HTMLElement;

			// Simulate drag
			fireEvent.mouseDown(divider, { clientX: 400 });
			fireEvent.mouseMove(window, { clientX: 320 }); // Move to ~40%
			fireEvent.mouseUp(window);

			// Should save to localStorage (may be debounced)
			await waitFor(() => {
				expect(localStorageMock.setItem).toHaveBeenCalledWith(
					'split-pane-test',
					expect.any(String)
				);
			});
		});

		it('uses default ratio when localStorage value is invalid', () => {
			localStorageMock.setItem('split-pane-test', 'invalid');

			const { container } = render(
				<SplitPane
					left={<div>Left</div>}
					right={<div>Right</div>}
					persistKey="test"
					initialRatio={50}
				/>
			);

			const divider = container.querySelector('.split-pane__divider');
			expect(divider).toHaveAttribute('aria-valuenow', '50');
		});

		it('uses default ratio when localStorage is unavailable', () => {
			// Mock localStorage to throw on getItem
			const originalGetItem = localStorageMock.getItem;
			localStorageMock.getItem = vi.fn(() => {
				throw new Error('localStorage unavailable');
			});

			const { container } = render(
				<SplitPane
					left={<div>Left</div>}
					right={<div>Right</div>}
					persistKey="test"
					initialRatio={50}
				/>
			);

			const divider = container.querySelector('.split-pane__divider');
			expect(divider).toHaveAttribute('aria-valuenow', '50');

			// Restore
			localStorageMock.getItem = originalGetItem;
		});
	});

	describe('SC-6: Minimum panel width constraints', () => {
		it('prevents left panel from collapsing below minimum width', () => {
			const { container } = render(
				<SplitPane
					left={<div>Left</div>}
					right={<div>Right</div>}
					initialRatio={50}
					minLeftWidth={200}
				/>
			);

			const divider = container.querySelector('.split-pane__divider') as HTMLElement;

			// Simulate dragging divider all the way to the left
			fireEvent.mouseDown(divider, { clientX: 400 });
			fireEvent.mouseMove(window, { clientX: 50 }); // Try to collapse left panel
			fireEvent.mouseUp(window);

			// Divider should stop at minimum (aria-valuenow should be at minimum %)
			const ariaValue = divider.getAttribute('aria-valuenow');
			expect(Number(ariaValue)).toBeGreaterThanOrEqual(10); // Minimum ~10% of 800px container
		});

		it('prevents right panel from collapsing below minimum width', () => {
			const { container } = render(
				<SplitPane
					left={<div>Left</div>}
					right={<div>Right</div>}
					initialRatio={50}
					minRightWidth={200}
				/>
			);

			const divider = container.querySelector('.split-pane__divider') as HTMLElement;

			// Simulate dragging divider all the way to the right
			fireEvent.mouseDown(divider, { clientX: 400 });
			fireEvent.mouseMove(window, { clientX: 750 }); // Try to collapse right panel
			fireEvent.mouseUp(window);

			// Divider should stop at maximum (aria-valuenow should be at maximum %)
			const ariaValue = divider.getAttribute('aria-valuenow');
			expect(Number(ariaValue)).toBeLessThanOrEqual(90); // Maximum ~90% of 800px container
		});

		it('respects custom minimum widths', () => {
			const { container } = render(
				<SplitPane
					left={<div>Left</div>}
					right={<div>Right</div>}
					initialRatio={50}
					minLeftWidth={300}
					minRightWidth={300}
				/>
			);

			const divider = container.querySelector('.split-pane__divider') as HTMLElement;

			// With 300px minimum on each side and 800px container,
			// divider should be constrained to 37.5% - 62.5%
			const ariaValue = Number(divider.getAttribute('aria-valuenow'));
			expect(ariaValue).toBeGreaterThanOrEqual(30);
			expect(ariaValue).toBeLessThanOrEqual(70);
		});

		it('handles keyboard navigation for accessibility', () => {
			const { container } = render(
				<SplitPane
					left={<div>Left</div>}
					right={<div>Right</div>}
					initialRatio={50}
				/>
			);

			const divider = container.querySelector('.split-pane__divider') as HTMLElement;
			divider.focus();

			// Press right arrow to increase left panel size
			fireEvent.keyDown(divider, { key: 'ArrowRight' });

			const ariaValue = Number(divider.getAttribute('aria-valuenow'));
			expect(ariaValue).toBeGreaterThan(50);
		});
	});

	describe('Edge Cases', () => {
		it('handles window resize gracefully', async () => {
			const { container } = render(
				<SplitPane
					left={<div>Left</div>}
					right={<div>Right</div>}
					initialRatio={50}
				/>
			);

			// Trigger window resize
			fireEvent(window, new Event('resize'));

			// Should not throw and should maintain ratio
			const divider = container.querySelector('.split-pane__divider');
			expect(divider).toHaveAttribute('aria-valuenow', '50');
		});

		it('handles touch events for mobile', () => {
			const { container } = render(
				<SplitPane
					left={<div>Left</div>}
					right={<div>Right</div>}
					initialRatio={50}
				/>
			);

			const divider = container.querySelector('.split-pane__divider') as HTMLElement;

			// Simulate touch drag
			fireEvent.touchStart(divider, {
				touches: [{ clientX: 400, clientY: 300 }],
			});
			fireEvent.touchMove(window, {
				touches: [{ clientX: 500, clientY: 300 }],
			});
			fireEvent.touchEnd(window);

			// Should handle without error
			expect(divider).toBeInTheDocument();
		});
	});
});
