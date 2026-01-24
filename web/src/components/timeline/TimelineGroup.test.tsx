/**
 * Tests for TimelineGroup component
 *
 * TimelineGroup is a collapsible container that groups timeline events by date.
 * Each group has a header showing the date label (e.g., "Today (5 events)") and
 * can be collapsed/expanded.
 *
 * Success Criteria covered:
 * - SC-4: Events are grouped by date with collapsible headers
 * - SC-5: Collapse state persists across page navigation
 *
 * TDD Note: These tests are written BEFORE the implementation exists.
 * The TimelineGroup.tsx file does not yet exist.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';

// Import from the file we're going to create
// This will fail until implementation exists
import { TimelineGroup, type TimelineGroupProps } from './TimelineGroup';
import type { TimelineEventData } from './TimelineEvent';

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
		get length() {
			return Object.keys(store).length;
		},
		key: vi.fn((index: number) => Object.keys(store)[index] ?? null),
	};
})();

Object.defineProperty(window, 'localStorage', { value: localStorageMock });

// Helper to create test events
function createTestEvent(id: number, createdAt: string): TimelineEventData {
	return {
		id,
		task_id: `TASK-00${id}`,
		task_title: `Test Task ${id}`,
		event_type: 'phase_completed',
		data: {},
		source: 'executor',
		created_at: createdAt,
	};
}

// Helper to render TimelineGroup with necessary providers
function renderTimelineGroup(props: TimelineGroupProps) {
	return render(
		<MemoryRouter>
			<TimelineGroup {...props} />
		</MemoryRouter>
	);
}

describe('TimelineGroup', () => {
	beforeEach(() => {
		localStorageMock.clear();
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	describe('rendering', () => {
		it('renders the group label with event count', () => {
			const events = [
				createTestEvent(1, '2024-03-15T10:00:00Z'),
				createTestEvent(2, '2024-03-15T11:00:00Z'),
			];

			renderTimelineGroup({
				groupId: 'today',
				label: 'Today (2 events)',
				events,
			});

			expect(screen.getByText('Today (2 events)')).toBeInTheDocument();
		});

		it('renders all events in the group when expanded', () => {
			const events = [
				createTestEvent(1, '2024-03-15T10:00:00Z'),
				createTestEvent(2, '2024-03-15T11:00:00Z'),
				createTestEvent(3, '2024-03-15T12:00:00Z'),
			];

			renderTimelineGroup({
				groupId: 'today',
				label: 'Today (3 events)',
				events,
				defaultExpanded: true,
			});

			// All task IDs should be visible
			expect(screen.getByText('TASK-001')).toBeInTheDocument();
			expect(screen.getByText('TASK-002')).toBeInTheDocument();
			expect(screen.getByText('TASK-003')).toBeInTheDocument();
		});

		it('shows expand/collapse chevron icon', () => {
			const events = [createTestEvent(1, '2024-03-15T10:00:00Z')];

			const { container } = renderTimelineGroup({
				groupId: 'today',
				label: 'Today (1 event)',
				events,
			});

			// Should have a chevron icon (either chevron-down or chevron-right)
			const chevron = container.querySelector('[data-icon]');
			expect(chevron).toBeInTheDocument();
		});

		it('hides events when collapsed', () => {
			const events = [
				createTestEvent(1, '2024-03-15T10:00:00Z'),
				createTestEvent(2, '2024-03-15T11:00:00Z'),
			];

			renderTimelineGroup({
				groupId: 'today',
				label: 'Today (2 events)',
				events,
				defaultExpanded: false,
			});

			// Events should not be visible when collapsed
			expect(screen.queryByText('TASK-001')).not.toBeInTheDocument();
			expect(screen.queryByText('TASK-002')).not.toBeInTheDocument();
		});
	});

	describe('collapse/expand behavior', () => {
		it('toggles expand state when header is clicked', () => {
			const events = [createTestEvent(1, '2024-03-15T10:00:00Z')];

			renderTimelineGroup({
				groupId: 'today',
				label: 'Today (1 event)',
				events,
				defaultExpanded: true,
			});

			// Initially expanded - event visible
			expect(screen.getByText('TASK-001')).toBeInTheDocument();

			// Click header to collapse
			const header = screen.getByRole('button', { name: /Today/i });
			fireEvent.click(header);

			// Now collapsed - event not visible
			expect(screen.queryByText('TASK-001')).not.toBeInTheDocument();

			// Click again to expand
			fireEvent.click(header);

			// Expanded again - event visible
			expect(screen.getByText('TASK-001')).toBeInTheDocument();
		});

		it('supports keyboard navigation (Enter key)', () => {
			const events = [createTestEvent(1, '2024-03-15T10:00:00Z')];

			renderTimelineGroup({
				groupId: 'today',
				label: 'Today (1 event)',
				events,
				defaultExpanded: true,
			});

			const header = screen.getByRole('button', { name: /Today/i });

			// Press Enter to collapse
			fireEvent.keyDown(header, { key: 'Enter' });
			expect(screen.queryByText('TASK-001')).not.toBeInTheDocument();

			// Press Enter to expand
			fireEvent.keyDown(header, { key: 'Enter' });
			expect(screen.getByText('TASK-001')).toBeInTheDocument();
		});

		it('supports keyboard navigation (Space key)', () => {
			const events = [createTestEvent(1, '2024-03-15T10:00:00Z')];

			renderTimelineGroup({
				groupId: 'today',
				label: 'Today (1 event)',
				events,
				defaultExpanded: true,
			});

			const header = screen.getByRole('button', { name: /Today/i });

			// Press Space to collapse
			fireEvent.keyDown(header, { key: ' ' });
			expect(screen.queryByText('TASK-001')).not.toBeInTheDocument();
		});

		it('calls onToggle callback when expanded state changes', () => {
			const events = [createTestEvent(1, '2024-03-15T10:00:00Z')];
			const onToggle = vi.fn();

			renderTimelineGroup({
				groupId: 'today',
				label: 'Today (1 event)',
				events,
				defaultExpanded: true,
				onToggle,
			});

			const header = screen.getByRole('button', { name: /Today/i });
			fireEvent.click(header);

			expect(onToggle).toHaveBeenCalledWith('today', false);
		});

		it('rotates chevron icon based on expand state', () => {
			const events = [createTestEvent(1, '2024-03-15T10:00:00Z')];

			const { container } = renderTimelineGroup({
				groupId: 'today',
				label: 'Today (1 event)',
				events,
				defaultExpanded: true,
			});

			// When expanded, should have expanded class/state
			const headerExpanded = container.querySelector('.timeline-group--expanded');
			expect(headerExpanded).toBeInTheDocument();

			// Click to collapse
			const header = screen.getByRole('button', { name: /Today/i });
			fireEvent.click(header);

			// Rerender to get updated state
			// Collapsed state should not have expanded class
			expect(container.querySelector('.timeline-group--expanded')).not.toBeInTheDocument();
		});
	});

	describe('localStorage persistence (SC-5)', () => {
		it('saves collapsed state to localStorage when collapsed', () => {
			const events = [createTestEvent(1, '2024-03-15T10:00:00Z')];

			renderTimelineGroup({
				groupId: 'today',
				label: 'Today (1 event)',
				events,
				defaultExpanded: true,
			});

			const header = screen.getByRole('button', { name: /Today/i });
			fireEvent.click(header); // Collapse

			expect(localStorageMock.setItem).toHaveBeenCalled();
		});

		it('restores collapsed state from localStorage on mount', () => {
			// Pre-set localStorage to have 'today' collapsed
			localStorageMock.getItem.mockReturnValue(JSON.stringify(['today']));

			const events = [createTestEvent(1, '2024-03-15T10:00:00Z')];

			renderTimelineGroup({
				groupId: 'today',
				label: 'Today (1 event)',
				events,
				// No defaultExpanded - should use localStorage
			});

			// Events should be hidden because 'today' was collapsed in localStorage
			expect(screen.queryByText('TASK-001')).not.toBeInTheDocument();
		});

		it('uses defaultExpanded when localStorage is empty', () => {
			localStorageMock.getItem.mockReturnValue(null as unknown as string);

			const events = [createTestEvent(1, '2024-03-15T10:00:00Z')];

			renderTimelineGroup({
				groupId: 'today',
				label: 'Today (1 event)',
				events,
				defaultExpanded: true,
			});

			// Should be expanded by default
			expect(screen.getByText('TASK-001')).toBeInTheDocument();
		});

		it('gracefully handles invalid localStorage data', () => {
			localStorageMock.getItem.mockReturnValue('not-valid-json');

			const events = [createTestEvent(1, '2024-03-15T10:00:00Z')];

			// Should not throw, should fall back to defaultExpanded
			expect(() => {
				renderTimelineGroup({
					groupId: 'today',
					label: 'Today (1 event)',
					events,
					defaultExpanded: true,
				});
			}).not.toThrow();

			expect(screen.getByText('TASK-001')).toBeInTheDocument();
		});

		it('handles localStorage unavailable gracefully', () => {
			// Simulate localStorage error
			localStorageMock.setItem.mockImplementation(() => {
				throw new Error('localStorage is full');
			});

			const events = [createTestEvent(1, '2024-03-15T10:00:00Z')];

			renderTimelineGroup({
				groupId: 'today',
				label: 'Today (1 event)',
				events,
				defaultExpanded: true,
			});

			const header = screen.getByRole('button', { name: /Today/i });

			// Should not throw when trying to save
			expect(() => {
				fireEvent.click(header);
			}).not.toThrow();
		});
	});

	describe('accessibility', () => {
		it('has role="region" with aria-labelledby', () => {
			const events = [createTestEvent(1, '2024-03-15T10:00:00Z')];

			const { container } = renderTimelineGroup({
				groupId: 'today',
				label: 'Today (1 event)',
				events,
			});

			const region = container.querySelector('[role="region"]');
			expect(region).toBeInTheDocument();
			expect(region).toHaveAttribute('aria-labelledby');
		});

		it('header button has aria-expanded attribute', () => {
			const events = [createTestEvent(1, '2024-03-15T10:00:00Z')];

			renderTimelineGroup({
				groupId: 'today',
				label: 'Today (1 event)',
				events,
				defaultExpanded: true,
			});

			const header = screen.getByRole('button', { name: /Today/i });
			expect(header).toHaveAttribute('aria-expanded', 'true');

			fireEvent.click(header);
			expect(header).toHaveAttribute('aria-expanded', 'false');
		});

		it('content area has aria-hidden when collapsed', () => {
			const events = [createTestEvent(1, '2024-03-15T10:00:00Z')];

			const { container } = renderTimelineGroup({
				groupId: 'today',
				label: 'Today (1 event)',
				events,
				defaultExpanded: false,
			});

			const content = container.querySelector('.timeline-group-content');
			expect(content).toHaveAttribute('aria-hidden', 'true');
		});
	});

	describe('edge cases', () => {
		it('renders correctly with empty events array', () => {
			renderTimelineGroup({
				groupId: 'today',
				label: 'Today (0 events)',
				events: [],
			});

			expect(screen.getByText('Today (0 events)')).toBeInTheDocument();
		});

		it('handles single event correctly', () => {
			const events = [createTestEvent(1, '2024-03-15T10:00:00Z')];

			renderTimelineGroup({
				groupId: 'today',
				label: 'Today (1 event)',
				events,
				defaultExpanded: true,
			});

			expect(screen.getByText('TASK-001')).toBeInTheDocument();
		});

		it('handles many events efficiently', () => {
			const events = Array.from({ length: 50 }, (_, i) =>
				createTestEvent(i + 1, `2024-03-15T${String(i % 24).padStart(2, '0')}:00:00Z`)
			);

			const { container } = renderTimelineGroup({
				groupId: 'today',
				label: 'Today (50 events)',
				events,
				defaultExpanded: true,
			});

			// All events should be rendered
			const eventElements = container.querySelectorAll('.timeline-event');
			expect(eventElements.length).toBe(50);
		});
	});
});
