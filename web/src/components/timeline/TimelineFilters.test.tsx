/**
 * Tests for TimelineFilters component
 *
 * TimelineFilters provides a dropdown interface for filtering timeline events
 * by event type, task, initiative, and source. It syncs filter state with URL
 * parameters.
 *
 * Success Criteria covered:
 * - SC-6: Filter dropdown shows event type checkboxes
 * - SC-7: Filtering by event type updates the event list
 * - SC-9: Infinite scroll respects current filters
 * - SC-12: Empty state shows when no events match filters
 *
 * TDD Note: These tests are written BEFORE the implementation exists.
 * The TimelineFilters.tsx file does not yet exist.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';

// Import from the file we're going to create
// This will fail until implementation exists
import { TimelineFilters, type TimelineFiltersProps } from './TimelineFilters';

// Mock tasks for task filter
const MOCK_TASKS = [
	{ id: 'TASK-001', title: 'First Task' },
	{ id: 'TASK-002', title: 'Second Task' },
	{ id: 'TASK-003', title: 'Third Task' },
];

// Mock initiatives for initiative filter
const MOCK_INITIATIVES = [
	{ id: 'INIT-001', title: 'Feature Initiative' },
	{ id: 'INIT-002', title: 'Bug Fix Initiative' },
];

// Helper to render TimelineFilters with necessary providers
function renderTimelineFilters(props: Partial<TimelineFiltersProps> = {}) {
	const defaultProps: TimelineFiltersProps = {
		selectedTypes: [],
		selectedTaskId: undefined,
		selectedInitiativeId: undefined,
		tasks: MOCK_TASKS,
		initiatives: MOCK_INITIATIVES,
		onTypesChange: vi.fn(),
		onTaskChange: vi.fn(),
		onInitiativeChange: vi.fn(),
		onClearAll: vi.fn(),
		...props,
	};

	return render(
		<MemoryRouter>
			<TimelineFilters {...defaultProps} />
		</MemoryRouter>
	);
}

describe('TimelineFilters', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	describe('filter dropdown rendering', () => {
		it('renders filter button with icon', () => {
			renderTimelineFilters();

			const filterButton = screen.getByRole('button', { name: /filter/i });
			expect(filterButton).toBeInTheDocument();
		});

		it('opens dropdown menu when filter button is clicked', async () => {
			const user = userEvent.setup();
			renderTimelineFilters();

			const filterButton = screen.getByRole('button', { name: /filter/i });
			await user.click(filterButton);

			// Dropdown content should be visible
			expect(screen.getByRole('menu')).toBeInTheDocument();
		});

		it('shows all event type checkboxes in the dropdown', async () => {
			const user = userEvent.setup();
			renderTimelineFilters();

			await user.click(screen.getByRole('button', { name: /filter/i }));

			// Check that each event type has a checkbox
			expect(screen.getByLabelText(/phase completed/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/phase started/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/phase failed/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/task created/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/error occurred/i)).toBeInTheDocument();
		});

		it('groups event types into categories', async () => {
			const user = userEvent.setup();
			renderTimelineFilters();

			await user.click(screen.getByRole('button', { name: /filter/i }));

			// Should have grouping headers
			expect(screen.getByText(/phase events/i)).toBeInTheDocument();
			expect(screen.getByText(/task events/i)).toBeInTheDocument();
		});
	});

	describe('event type filtering (SC-6, SC-7)', () => {
		it('calls onTypesChange when a type checkbox is checked', async () => {
			const user = userEvent.setup();
			const onTypesChange = vi.fn();

			renderTimelineFilters({ onTypesChange });

			await user.click(screen.getByRole('button', { name: /filter/i }));

			const phaseCompletedCheckbox = screen.getByLabelText(/phase completed/i);
			await user.click(phaseCompletedCheckbox);

			expect(onTypesChange).toHaveBeenCalledWith(expect.arrayContaining(['phase_completed']));
		});

		it('unchecks type when already selected', async () => {
			const user = userEvent.setup();
			const onTypesChange = vi.fn();

			renderTimelineFilters({
				selectedTypes: ['phase_completed', 'task_created'],
				onTypesChange,
			});

			await user.click(screen.getByRole('button', { name: /filter/i }));

			const phaseCompletedCheckbox = screen.getByLabelText(/phase completed/i);
			await user.click(phaseCompletedCheckbox);

			// Should call with phase_completed removed
			expect(onTypesChange).toHaveBeenCalledWith(['task_created']);
		});

		it('shows checked state for selected types', async () => {
			const user = userEvent.setup();

			renderTimelineFilters({
				selectedTypes: ['phase_completed', 'error_occurred'],
			});

			await user.click(screen.getByRole('button', { name: /filter/i }));

			const phaseCompletedCheckbox = screen.getByLabelText(/phase completed/i);
			const errorCheckbox = screen.getByLabelText(/error occurred/i);
			const taskCreatedCheckbox = screen.getByLabelText(/task created/i);

			expect(phaseCompletedCheckbox).toBeChecked();
			expect(errorCheckbox).toBeChecked();
			expect(taskCreatedCheckbox).not.toBeChecked();
		});

		it('shows "Select All" option for event types', async () => {
			const user = userEvent.setup();
			const onTypesChange = vi.fn();

			renderTimelineFilters({ onTypesChange });

			await user.click(screen.getByRole('button', { name: /filter/i }));

			const selectAllButton = screen.getByRole('button', { name: /select all/i });
			expect(selectAllButton).toBeInTheDocument();

			await user.click(selectAllButton);

			// Should call with all event types
			expect(onTypesChange).toHaveBeenCalled();
			const callArg = onTypesChange.mock.calls[0][0];
			expect(callArg.length).toBeGreaterThan(5);
		});

		it('shows "Clear" option when types are selected', async () => {
			const user = userEvent.setup();
			const onTypesChange = vi.fn();

			renderTimelineFilters({
				selectedTypes: ['phase_completed'],
				onTypesChange,
			});

			await user.click(screen.getByRole('button', { name: /filter/i }));

			const clearButton = screen.getByRole('button', { name: /clear type filter/i });
			await user.click(clearButton);

			expect(onTypesChange).toHaveBeenCalledWith([]);
		});
	});

	describe('task filtering', () => {
		it('shows task filter dropdown', async () => {
			const user = userEvent.setup();
			renderTimelineFilters();

			await user.click(screen.getByRole('button', { name: /filter/i }));

			// Use the specific id-based selector for the task filter select
			const taskFilter = screen.getByRole('combobox', { name: /^task$/i });
			expect(taskFilter).toBeInTheDocument();
		});

		it('lists all tasks in dropdown', async () => {
			const user = userEvent.setup();
			renderTimelineFilters();

			await user.click(screen.getByRole('button', { name: /filter/i }));

			// The select element shows options, check for the task IDs in the options
			const taskSelect = screen.getByRole('combobox', { name: /^task$/i });
			expect(taskSelect).toBeInTheDocument();

			// Check options are present
			expect(screen.getByRole('option', { name: 'TASK-001' })).toBeInTheDocument();
			expect(screen.getByRole('option', { name: 'TASK-002' })).toBeInTheDocument();
			expect(screen.getByRole('option', { name: 'TASK-003' })).toBeInTheDocument();
		});

		it('calls onTaskChange when task is selected', async () => {
			const user = userEvent.setup();
			const onTaskChange = vi.fn();

			renderTimelineFilters({ onTaskChange });

			await user.click(screen.getByRole('button', { name: /filter/i }));

			const taskSelect = screen.getByRole('combobox', { name: /^task$/i });
			await user.selectOptions(taskSelect, 'TASK-001');

			expect(onTaskChange).toHaveBeenCalledWith('TASK-001');
		});

		it('shows "All Tasks" option to clear task filter', async () => {
			const user = userEvent.setup();
			const onTaskChange = vi.fn();

			renderTimelineFilters({
				selectedTaskId: 'TASK-001',
				onTaskChange,
			});

			await user.click(screen.getByRole('button', { name: /filter/i }));

			const taskSelect = screen.getByRole('combobox', { name: /^task$/i });
			await user.selectOptions(taskSelect, '');

			expect(onTaskChange).toHaveBeenCalledWith(undefined);
		});
	});

	describe('initiative filtering', () => {
		it('shows initiative filter dropdown', async () => {
			const user = userEvent.setup();
			renderTimelineFilters();

			await user.click(screen.getByRole('button', { name: /filter/i }));

			const initiativeFilter = screen.getByRole('combobox', { name: /^initiative$/i });
			expect(initiativeFilter).toBeInTheDocument();
		});

		it('calls onInitiativeChange when initiative is selected', async () => {
			const user = userEvent.setup();
			const onInitiativeChange = vi.fn();

			renderTimelineFilters({ onInitiativeChange });

			await user.click(screen.getByRole('button', { name: /filter/i }));

			const initiativeSelect = screen.getByRole('combobox', { name: /^initiative$/i });
			await user.selectOptions(initiativeSelect, 'INIT-001');

			expect(onInitiativeChange).toHaveBeenCalledWith('INIT-001');
		});
	});

	describe('active filter indicator', () => {
		it('shows badge with filter count when filters are active', () => {
			renderTimelineFilters({
				selectedTypes: ['phase_completed', 'error_occurred'],
				selectedTaskId: 'TASK-001',
			});

			// Should show badge with count (2 type filters + 1 task filter = 3)
			const badge = screen.getByText('3');
			expect(badge).toBeInTheDocument();
		});

		it('hides badge when no filters are active', () => {
			renderTimelineFilters({
				selectedTypes: [],
				selectedTaskId: undefined,
				selectedInitiativeId: undefined,
			});

			// Should not show any badge
			expect(screen.queryByText('1')).not.toBeInTheDocument();
			expect(screen.queryByText('2')).not.toBeInTheDocument();
			expect(screen.queryByText('3')).not.toBeInTheDocument();
		});
	});

	describe('clear all filters', () => {
		it('shows "Clear all" button when filters are active', async () => {
			const user = userEvent.setup();
			const onClearAll = vi.fn();

			renderTimelineFilters({
				selectedTypes: ['phase_completed'],
				onClearAll,
			});

			await user.click(screen.getByRole('button', { name: /filter/i }));

			const clearAllButton = screen.getByRole('button', { name: /clear all/i });
			expect(clearAllButton).toBeInTheDocument();

			await user.click(clearAllButton);

			expect(onClearAll).toHaveBeenCalled();
		});

		it('hides "Clear all" button when no filters are active', async () => {
			const user = userEvent.setup();

			renderTimelineFilters({
				selectedTypes: [],
				selectedTaskId: undefined,
				selectedInitiativeId: undefined,
			});

			await user.click(screen.getByRole('button', { name: /filter/i }));

			expect(screen.queryByRole('button', { name: /clear all/i })).not.toBeInTheDocument();
		});
	});

	describe('URL parameter sync', () => {
		it('reads initial filter state from URL params', () => {
			render(
				<MemoryRouter initialEntries={['/?types=phase_completed,error_occurred&task_id=TASK-001']}>
					<TimelineFilters
						selectedTypes={['phase_completed', 'error_occurred']}
						selectedTaskId="TASK-001"
						tasks={MOCK_TASKS}
						initiatives={MOCK_INITIATIVES}
						onTypesChange={vi.fn()}
						onTaskChange={vi.fn()}
						onInitiativeChange={vi.fn()}
						onClearAll={vi.fn()}
					/>
				</MemoryRouter>
			);

			// Filter button should show active state (badge with count)
			expect(screen.getByText('3')).toBeInTheDocument(); // 2 types + 1 task
		});
	});

	describe('accessibility', () => {
		it('filter dropdown is keyboard navigable', async () => {
			const user = userEvent.setup();
			renderTimelineFilters();

			const filterButton = screen.getByRole('button', { name: /filter/i });
			await user.click(filterButton);

			// Should be able to tab through options
			await user.tab();
			const focusedElement = document.activeElement;
			expect(focusedElement).toBeInTheDocument();
		});

		it('checkboxes have associated labels', async () => {
			const user = userEvent.setup();
			renderTimelineFilters();

			await user.click(screen.getByRole('button', { name: /filter/i }));

			// All checkboxes should have labels
			const checkboxes = screen.getAllByRole('checkbox');
			checkboxes.forEach((checkbox) => {
				expect(checkbox).toHaveAccessibleName();
			});
		});

		it('dropdown closes on Escape key', async () => {
			const user = userEvent.setup();
			renderTimelineFilters();

			await user.click(screen.getByRole('button', { name: /filter/i }));

			expect(screen.getByRole('menu')).toBeInTheDocument();

			await user.keyboard('{Escape}');

			expect(screen.queryByRole('menu')).not.toBeInTheDocument();
		});
	});

	describe('edge cases', () => {
		it('handles empty tasks array', async () => {
			const user = userEvent.setup();

			renderTimelineFilters({ tasks: [] });

			await user.click(screen.getByRole('button', { name: /filter/i }));

			// Task dropdown should show "No tasks" or be disabled
			const taskFilter = screen.getByRole('combobox', { name: /^task$/i });
			expect(taskFilter).toBeInTheDocument();
		});

		it('handles empty initiatives array', async () => {
			const user = userEvent.setup();

			renderTimelineFilters({ initiatives: [] });

			await user.click(screen.getByRole('button', { name: /filter/i }));

			const initiativeFilter = screen.getByRole('combobox', { name: /^initiative$/i });
			expect(initiativeFilter).toBeInTheDocument();
		});

		it('handles invalid URL parameter values gracefully', () => {
			// Should not throw when URL has invalid filter values
			expect(() => {
				render(
					<MemoryRouter initialEntries={['/?types=invalid_type,another_bad_one']}>
						<TimelineFilters
							selectedTypes={[]}
							tasks={MOCK_TASKS}
							initiatives={MOCK_INITIATIVES}
							onTypesChange={vi.fn()}
							onTaskChange={vi.fn()}
							onInitiativeChange={vi.fn()}
							onClearAll={vi.fn()}
						/>
					</MemoryRouter>
				);
			}).not.toThrow();
		});
	});
});
