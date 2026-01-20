import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { TimelineEvent, type TimelineEventData, type EventType } from './TimelineEvent';
import { usePreferencesStore, defaultPreferences } from '@/stores';

// Mock date formatting for consistent test results
vi.mock('@/lib/formatDate', () => ({
	formatDate: vi.fn((_date, _format) => '2m ago'),
}));

// Helper to create event data
function createEvent(overrides: Partial<TimelineEventData> = {}): TimelineEventData {
	return {
		id: 1,
		task_id: 'TASK-001',
		task_title: 'Implement user authentication',
		phase: 'implement',
		iteration: 1,
		event_type: 'phase_completed',
		data: {},
		source: 'executor',
		created_at: '2024-01-15T10:30:00Z',
		...overrides,
	};
}

// Helper to render with router context
function renderTimelineEvent(
	event: TimelineEventData,
	props: Partial<Parameters<typeof TimelineEvent>[0]> = {}
) {
	return render(
		<MemoryRouter>
			<TimelineEvent event={event} {...props} />
		</MemoryRouter>
	);
}

describe('TimelineEvent', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		usePreferencesStore.setState(defaultPreferences);
	});

	describe('rendering', () => {
		it('renders event label based on event type', () => {
			renderTimelineEvent(createEvent({ event_type: 'phase_completed', phase: 'implement' }));
			expect(screen.getByText('Phase completed: implement')).toBeInTheDocument();
		});

		it('renders relative time', () => {
			renderTimelineEvent(createEvent());
			expect(screen.getByText('2m ago')).toBeInTheDocument();
		});

		it('renders task link when showTask is true (default)', () => {
			renderTimelineEvent(createEvent());

			const taskLink = screen.getByRole('link');
			expect(taskLink).toHaveAttribute('href', '/tasks/TASK-001');
			expect(screen.getByText('TASK-001')).toBeInTheDocument();
			expect(screen.getByText('Implement user authentication')).toBeInTheDocument();
		});

		it('does not render task link when showTask is false', () => {
			renderTimelineEvent(createEvent(), { showTask: false });

			expect(screen.queryByRole('link')).not.toBeInTheDocument();
			expect(screen.queryByText('TASK-001')).not.toBeInTheDocument();
		});
	});

	describe('event type styling', () => {
		const eventTypeTests: Array<{ type: EventType; expectedText: string; styleClass: string }> =
			[
				{
					type: 'phase_completed',
					expectedText: 'Phase completed:',
					styleClass: 'timeline-event--success',
				},
				{
					type: 'phase_failed',
					expectedText: 'Phase failed:',
					styleClass: 'timeline-event--error',
				},
				{
					type: 'phase_started',
					expectedText: 'Phase started:',
					styleClass: 'timeline-event--info',
				},
				{
					type: 'task_created',
					expectedText: 'Task created',
					styleClass: 'timeline-event--default',
				},
				{
					type: 'task_completed',
					expectedText: 'Task completed',
					styleClass: 'timeline-event--success',
				},
				{
					type: 'error_occurred',
					expectedText: 'Error occurred',
					styleClass: 'timeline-event--error',
				},
				{
					type: 'warning_issued',
					expectedText: 'Warning issued',
					styleClass: 'timeline-event--warning',
				},
				{
					type: 'task_started',
					expectedText: 'Task started',
					styleClass: 'timeline-event--info',
				},
				{
					type: 'task_paused',
					expectedText: 'Task paused',
					styleClass: 'timeline-event--warning',
				},
				{
					type: 'token_update',
					expectedText: 'Token usage updated',
					styleClass: 'timeline-event--default',
				},
			];

		eventTypeTests.forEach(({ type, expectedText, styleClass }) => {
			it(`renders correct icon and color for ${type}`, () => {
				const { container } = renderTimelineEvent(
					createEvent({ event_type: type, phase: 'test' })
				);

				expect(screen.getByText(expectedText, { exact: false })).toBeInTheDocument();
				expect(container.querySelector(`.${styleClass}`)).toBeInTheDocument();
			});
		});

		it('renders gate_decision with approval status', () => {
			renderTimelineEvent(
				createEvent({
					event_type: 'gate_decision',
					data: { approved: true },
				})
			);
			expect(screen.getByText('Gate: approved')).toBeInTheDocument();
		});

		it('renders gate_decision with rejection status', () => {
			renderTimelineEvent(
				createEvent({
					event_type: 'gate_decision',
					data: { approved: false },
				})
			);
			expect(screen.getByText('Gate: rejected')).toBeInTheDocument();
		});

		it('renders activity_changed with activity name', () => {
			renderTimelineEvent(
				createEvent({
					event_type: 'activity_changed',
					data: { activity: 'streaming' },
				})
			);
			expect(screen.getByText('Activity: streaming')).toBeInTheDocument();
		});
	});

	describe('expandable details', () => {
		it('shows expand hint when event has details', () => {
			const { container } = renderTimelineEvent(
				createEvent({
					data: { duration: 45 },
				})
			);

			expect(container.querySelector('.timeline-event-expand-hint')).toBeInTheDocument();
		});

		it('does not show expand hint when event has no details', () => {
			const { container } = renderTimelineEvent(
				createEvent({
					data: {},
					iteration: 1,
				})
			);

			expect(container.querySelector('.timeline-event-expand-hint')).not.toBeInTheDocument();
		});

		it('expands on click to show details', () => {
			const { container } = renderTimelineEvent(
				createEvent({
					data: { duration: 45, input_tokens: 5000, output_tokens: 2500 },
				})
			);

			const event = container.querySelector('.timeline-event')!;
			fireEvent.click(event);

			expect(screen.getByText('Duration:')).toBeInTheDocument();
			expect(screen.getByText('45s')).toBeInTheDocument();
			expect(screen.getByText('Tokens:')).toBeInTheDocument();
			expect(screen.getByText('5,000 input / 2,500 output')).toBeInTheDocument();
		});

		it('shows commit SHA when present', () => {
			const { container } = renderTimelineEvent(
				createEvent({
					data: { commit_sha: 'abc123def456' },
				})
			);

			const event = container.querySelector('.timeline-event')!;
			fireEvent.click(event);

			expect(screen.getByText('Commit:')).toBeInTheDocument();
			expect(screen.getByText('abc123d')).toBeInTheDocument(); // Truncated to 7 chars
		});

		it('shows error message when present', () => {
			const { container } = renderTimelineEvent(
				createEvent({
					event_type: 'error_occurred',
					data: { error: 'Connection timeout' },
				})
			);

			const event = container.querySelector('.timeline-event')!;
			fireEvent.click(event);

			expect(screen.getByText('Error:')).toBeInTheDocument();
			expect(screen.getByText('Connection timeout')).toBeInTheDocument();
		});

		it('shows iteration when > 1', () => {
			const { container } = renderTimelineEvent(
				createEvent({
					iteration: 3,
					data: { duration: 10 }, // Need some data to make it expandable
				})
			);

			const event = container.querySelector('.timeline-event')!;
			fireEvent.click(event);

			expect(screen.getByText('Iteration:')).toBeInTheDocument();
			expect(screen.getByText('3')).toBeInTheDocument();
		});

		it('collapses on second click', () => {
			const { container } = renderTimelineEvent(
				createEvent({
					data: { duration: 45 },
				})
			);

			const event = container.querySelector('.timeline-event')!;

			// First click expands
			fireEvent.click(event);
			expect(screen.getByText('Duration:')).toBeInTheDocument();

			// Second click collapses
			fireEvent.click(event);
			expect(screen.queryByText('Duration:')).not.toBeInTheDocument();
		});
	});

	describe('keyboard navigation', () => {
		it('expands on Enter key when has details', () => {
			const { container } = renderTimelineEvent(
				createEvent({
					data: { duration: 45 },
				})
			);

			const event = container.querySelector('.timeline-event')!;
			fireEvent.keyDown(event, { key: 'Enter' });

			expect(screen.getByText('Duration:')).toBeInTheDocument();
		});

		it('expands on Space key when has details', () => {
			const { container } = renderTimelineEvent(
				createEvent({
					data: { duration: 45 },
				})
			);

			const event = container.querySelector('.timeline-event')!;
			fireEvent.keyDown(event, { key: ' ' });

			expect(screen.getByText('Duration:')).toBeInTheDocument();
		});

		it('does not expand on other keys', () => {
			const { container } = renderTimelineEvent(
				createEvent({
					data: { duration: 45 },
				})
			);

			const event = container.querySelector('.timeline-event')!;
			fireEvent.keyDown(event, { key: 'Escape' });

			expect(screen.queryByText('Duration:')).not.toBeInTheDocument();
		});

		it('has no tabIndex when no details', () => {
			const { container } = renderTimelineEvent(
				createEvent({
					data: {},
					iteration: 1,
				})
			);

			const event = container.querySelector('.timeline-event')!;
			expect(event).not.toHaveAttribute('tabindex');
		});

		it('has tabIndex=0 when has details', () => {
			const { container } = renderTimelineEvent(
				createEvent({
					data: { duration: 45 },
				})
			);

			const event = container.querySelector('.timeline-event')!;
			expect(event).toHaveAttribute('tabindex', '0');
		});
	});

	describe('accessibility', () => {
		it('has correct aria-label with task info', () => {
			const { container } = renderTimelineEvent(createEvent());

			const event = container.querySelector('.timeline-event')!;
			expect(event).toHaveAttribute(
				'aria-label',
				'Phase completed: implement, TASK-001: Implement user authentication'
			);
		});

		it('has correct aria-label without task info', () => {
			const { container } = renderTimelineEvent(createEvent(), { showTask: false });

			const event = container.querySelector('.timeline-event')!;
			expect(event).toHaveAttribute('aria-label', 'Phase completed: implement');
		});

		it('has role=button and aria-expanded when expandable', () => {
			const { container } = renderTimelineEvent(
				createEvent({
					data: { duration: 45 },
				})
			);

			const event = container.querySelector('.timeline-event')!;
			expect(event).toHaveAttribute('role', 'button');
			expect(event).toHaveAttribute('aria-expanded', 'false');

			fireEvent.click(event);
			expect(event).toHaveAttribute('aria-expanded', 'true');
		});

		it('has no role or aria-expanded when not expandable', () => {
			const { container } = renderTimelineEvent(
				createEvent({
					data: {},
					iteration: 1,
				})
			);

			const event = container.querySelector('.timeline-event')!;
			expect(event).not.toHaveAttribute('role');
			expect(event).not.toHaveAttribute('aria-expanded');
		});
	});

	describe('task link interaction', () => {
		it('clicking task link does not expand details', () => {
			renderTimelineEvent(
				createEvent({
					data: { duration: 45 },
				})
			);

			const link = screen.getByRole('link');
			fireEvent.click(link);

			// Details should not be shown because click was on link
			expect(screen.queryByText('Duration:')).not.toBeInTheDocument();
		});
	});

	describe('duration formatting', () => {
		it('formats seconds correctly', () => {
			const { container } = renderTimelineEvent(
				createEvent({
					data: { duration: 30 },
				})
			);

			fireEvent.click(container.querySelector('.timeline-event')!);
			expect(screen.getByText('30s')).toBeInTheDocument();
		});

		it('formats minutes and seconds correctly', () => {
			const { container } = renderTimelineEvent(
				createEvent({
					data: { duration: 125 },
				})
			);

			fireEvent.click(container.querySelector('.timeline-event')!);
			expect(screen.getByText('2m 5s')).toBeInTheDocument();
		});

		it('formats hours and minutes correctly', () => {
			const { container } = renderTimelineEvent(
				createEvent({
					data: { duration: 7320 },
				})
			);

			fireEvent.click(container.querySelector('.timeline-event')!);
			expect(screen.getByText('2h 2m')).toBeInTheDocument();
		});
	});

	describe('token formatting', () => {
		it('formats large token counts with commas', () => {
			const { container } = renderTimelineEvent(
				createEvent({
					data: { input_tokens: 15000, output_tokens: 8500 },
				})
			);

			fireEvent.click(container.querySelector('.timeline-event')!);
			expect(screen.getByText('15,000 input / 8,500 output')).toBeInTheDocument();
		});
	});
});
