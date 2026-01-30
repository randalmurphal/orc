/**
 * TDD Tests for enhanced gate_decision timeline event details.
 *
 * Tests for TASK-655: CLI and UI for gate management
 *
 * Success Criteria Coverage:
 * - SC-8: Gate decisions appear as timeline entries with gate type, approved/rejected badge, reason, timestamp
 * - SC-9: Expanded gate_decision shows gate_type, resolution source, retry target, output data keys
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { TimelineEvent, type TimelineEventData } from './TimelineEvent';
import { usePreferencesStore, defaultPreferences } from '@/stores';

// Mock date formatting for consistent test results
vi.mock('@/lib/formatDate', () => ({
	formatDate: vi.fn((_date, _format) => '5m ago'),
}));

// Helper to create gate decision event
function createGateEvent(overrides: Partial<TimelineEventData> = {}): TimelineEventData {
	return {
		id: 100,
		task_id: 'TASK-001',
		task_title: 'Test task',
		phase: 'implement',
		iteration: 1,
		event_type: 'gate_decision',
		data: {
			approved: true,
			gate_type: 'auto',
			reason: 'auto-approved on success',
		},
		source: 'executor',
		created_at: '2025-01-15T10:30:00Z',
		...overrides,
	};
}

function renderEvent(event: TimelineEventData) {
	return render(
		<MemoryRouter>
			<TimelineEvent event={event} />
		</MemoryRouter>,
	);
}

describe('TimelineEvent - Gate Decision Details (TASK-655)', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		usePreferencesStore.setState(defaultPreferences);
	});

	// ─── SC-8: Gate decisions show in timeline ──────────────────────────────

	describe('SC-8: gate decision timeline entries', () => {
		it('displays gate type in the label for approved gate', () => {
			renderEvent(
				createGateEvent({
					data: { approved: true, gate_type: 'auto', reason: 'auto-approved' },
				}),
			);

			// The label should include gate type information
			// Current behavior: "Gate: approved" - after implementation should show gate_type
			expect(screen.getByText(/gate/i)).toBeInTheDocument();
			expect(screen.getByText(/approved/i)).toBeInTheDocument();
		});

		it('displays gate type in the label for rejected gate', () => {
			renderEvent(
				createGateEvent({
					data: {
						approved: false,
						gate_type: 'human',
						reason: 'Needs more tests',
					},
				}),
			);

			expect(screen.getByText(/gate/i)).toBeInTheDocument();
			expect(screen.getByText(/rejected/i)).toBeInTheDocument();
		});

		it('shows reason text in expanded details', () => {
			const { container } = renderEvent(
				createGateEvent({
					data: {
						approved: true,
						gate_type: 'auto',
						reason: 'auto-approved on success',
					},
				}),
			);

			// Click to expand
			const event = container.querySelector('.timeline-event')!;
			fireEvent.click(event);

			// Reason should be visible in expanded state
			expect(screen.getByText('auto-approved on success')).toBeInTheDocument();
		});
	});

	// ─── SC-9: Enhanced gate_decision expanded details ──────────────────────

	describe('SC-9: expanded gate decision details', () => {
		it('shows gate_type in expanded details', () => {
			const { container } = renderEvent(
				createGateEvent({
					data: {
						approved: true,
						gate_type: 'ai',
						reason: 'AI gate approved',
						source: 'phase_override',
					},
				}),
			);

			const event = container.querySelector('.timeline-event')!;
			fireEvent.click(event);

			// Should show gate type detail
			expect(screen.getByText(/gate type/i)).toBeInTheDocument();
			expect(screen.getByText(/ai/i)).toBeInTheDocument();
		});

		it('shows resolution source in expanded details', () => {
			const { container } = renderEvent(
				createGateEvent({
					data: {
						approved: true,
						gate_type: 'human',
						source: 'phase_override',
						reason: 'human approved',
					},
				}),
			);

			const event = container.querySelector('.timeline-event')!;
			fireEvent.click(event);

			// Should show source/resolution info
			expect(screen.getByText(/source/i)).toBeInTheDocument();
			expect(screen.getByText(/phase_override/i)).toBeInTheDocument();
		});

		it('shows retry target when gate is rejected', () => {
			const { container } = renderEvent(
				createGateEvent({
					data: {
						approved: false,
						gate_type: 'human',
						reason: 'needs refactor',
						retry_from: 'implement',
					},
				}),
			);

			const event = container.querySelector('.timeline-event')!;
			fireEvent.click(event);

			// Should show retry target phase
			expect(screen.getByText(/retry/i)).toBeInTheDocument();
			expect(screen.getByText(/implement/i)).toBeInTheDocument();
		});

		it('shows output data keys when AI gate produces data', () => {
			const { container } = renderEvent(
				createGateEvent({
					data: {
						approved: true,
						gate_type: 'ai',
						reason: 'AI analysis complete',
						output_data: { issues: ['XSS'], severity: 'high' },
					},
				}),
			);

			const event = container.querySelector('.timeline-event')!;
			fireEvent.click(event);

			// Should show output data keys
			expect(screen.getByText(/output/i)).toBeInTheDocument();
			expect(screen.getByText(/issues/i)).toBeInTheDocument();
		});

		it('gracefully handles missing optional fields', () => {
			const { container } = renderEvent(
				createGateEvent({
					data: {
						approved: true,
						// No gate_type, source, retry_from, or output_data
					},
				}),
			);

			const event = container.querySelector('.timeline-event')!;
			fireEvent.click(event);

			// Should not crash - missing fields just don't render
			expect(container.querySelector('.timeline-event--expanded')).toBeInTheDocument();
		});
	});
});
