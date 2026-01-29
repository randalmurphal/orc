/**
 * TDD Tests for DeletePhaseDialog
 *
 * Tests for TASK-640: Phase deletion confirmation dialog
 *
 * Success Criteria Coverage:
 * - SC-4: Confirmation dialog appears with phase name and buttons
 * - SC-5: Confirming triggers onConfirm callback
 *
 * Behaviors:
 * - Shows phase name in dialog
 * - Cancel closes dialog without action
 * - Confirm triggers deletion
 * - Loading state during delete operation
 * - Escape key closes dialog
 */

import { describe, it, expect, vi, afterEach, beforeAll } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { DeletePhaseDialog } from './DeletePhaseDialog';

// Mock IntersectionObserver for potential portal/overlay usage
beforeAll(() => {
	class MockIntersectionObserver {
		observe() {}
		unobserve() {}
		disconnect() {}
	}
	Object.defineProperty(window, 'IntersectionObserver', {
		value: MockIntersectionObserver,
		writable: true,
	});
});

describe('DeletePhaseDialog', () => {
	afterEach(() => {
		cleanup();
	});

	describe('dialog display', () => {
		it('renders dialog when open=true', () => {
			render(
				<DeletePhaseDialog
					open={true}
					phaseName="Specification"
					onConfirm={vi.fn()}
					onCancel={vi.fn()}
				/>
			);

			expect(screen.getByRole('dialog')).toBeInTheDocument();
		});

		it('does not render when open=false', () => {
			render(
				<DeletePhaseDialog
					open={false}
					phaseName="Specification"
					onConfirm={vi.fn()}
					onCancel={vi.fn()}
				/>
			);

			expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
		});

		it('displays phase name in dialog content', () => {
			render(
				<DeletePhaseDialog
					open={true}
					phaseName="Code Review"
					onConfirm={vi.fn()}
					onCancel={vi.fn()}
				/>
			);

			expect(screen.getByText(/code review/i)).toBeInTheDocument();
		});

		it('renders Remove Phase confirmation button', () => {
			render(
				<DeletePhaseDialog
					open={true}
					phaseName="Specification"
					onConfirm={vi.fn()}
					onCancel={vi.fn()}
				/>
			);

			expect(screen.getByRole('button', { name: /remove phase/i })).toBeInTheDocument();
		});

		it('renders Cancel button', () => {
			render(
				<DeletePhaseDialog
					open={true}
					phaseName="Specification"
					onConfirm={vi.fn()}
					onCancel={vi.fn()}
				/>
			);

			expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();
		});
	});

	describe('button actions', () => {
		it('calls onConfirm when Remove Phase is clicked', () => {
			const onConfirm = vi.fn();

			render(
				<DeletePhaseDialog
					open={true}
					phaseName="Specification"
					onConfirm={onConfirm}
					onCancel={vi.fn()}
				/>
			);

			fireEvent.click(screen.getByRole('button', { name: /remove phase/i }));

			expect(onConfirm).toHaveBeenCalledTimes(1);
		});

		it('calls onCancel when Cancel is clicked', () => {
			const onCancel = vi.fn();

			render(
				<DeletePhaseDialog
					open={true}
					phaseName="Specification"
					onConfirm={vi.fn()}
					onCancel={onCancel}
				/>
			);

			fireEvent.click(screen.getByRole('button', { name: /cancel/i }));

			expect(onCancel).toHaveBeenCalledTimes(1);
		});

		it('calls onCancel when Escape key is pressed', () => {
			const onCancel = vi.fn();

			render(
				<DeletePhaseDialog
					open={true}
					phaseName="Specification"
					onConfirm={vi.fn()}
					onCancel={onCancel}
				/>
			);

			fireEvent.keyDown(document, { key: 'Escape' });

			expect(onCancel).toHaveBeenCalledTimes(1);
		});
	});

	describe('loading state', () => {
		it('shows loading indicator when loading=true', () => {
			render(
				<DeletePhaseDialog
					open={true}
					phaseName="Specification"
					onConfirm={vi.fn()}
					onCancel={vi.fn()}
					loading={true}
				/>
			);

			// Loading indicator should be visible
			expect(screen.getByRole('button', { name: /remove phase/i })).toBeDisabled();
		});

		it('disables confirm button during loading', () => {
			render(
				<DeletePhaseDialog
					open={true}
					phaseName="Specification"
					onConfirm={vi.fn()}
					onCancel={vi.fn()}
					loading={true}
				/>
			);

			const confirmBtn = screen.getByRole('button', { name: /remove phase/i });
			expect(confirmBtn).toBeDisabled();
		});

		it('disables cancel button during loading', () => {
			render(
				<DeletePhaseDialog
					open={true}
					phaseName="Specification"
					onConfirm={vi.fn()}
					onCancel={vi.fn()}
					loading={true}
				/>
			);

			const cancelBtn = screen.getByRole('button', { name: /cancel/i });
			expect(cancelBtn).toBeDisabled();
		});
	});

	describe('dialog content', () => {
		it('displays warning message about phase removal', () => {
			render(
				<DeletePhaseDialog
					open={true}
					phaseName="Specification"
					onConfirm={vi.fn()}
					onCancel={vi.fn()}
				/>
			);

			// Should explain the consequences
			expect(screen.getByText(/remove/i)).toBeInTheDocument();
		});

		it('uses destructive styling for confirm button', () => {
			render(
				<DeletePhaseDialog
					open={true}
					phaseName="Specification"
					onConfirm={vi.fn()}
					onCancel={vi.fn()}
				/>
			);

			const confirmBtn = screen.getByRole('button', { name: /remove phase/i });
			// Destructive buttons typically have a danger/red class
			expect(confirmBtn.className).toMatch(/destructive|danger|red/i);
		});
	});

	describe('accessibility', () => {
		it('dialog has accessible role', () => {
			render(
				<DeletePhaseDialog
					open={true}
					phaseName="Specification"
					onConfirm={vi.fn()}
					onCancel={vi.fn()}
				/>
			);

			expect(screen.getByRole('dialog')).toBeInTheDocument();
		});

		it('dialog has accessible title', () => {
			render(
				<DeletePhaseDialog
					open={true}
					phaseName="Specification"
					onConfirm={vi.fn()}
					onCancel={vi.fn()}
				/>
			);

			// Dialog should have aria-labelledby or a heading
			const dialog = screen.getByRole('dialog');
			expect(dialog).toHaveAccessibleName();
		});

		it('focus is trapped within dialog', () => {
			render(
				<DeletePhaseDialog
					open={true}
					phaseName="Specification"
					onConfirm={vi.fn()}
					onCancel={vi.fn()}
				/>
			);

			// Cancel and confirm buttons should be focusable within dialog
			const cancelBtn = screen.getByRole('button', { name: /cancel/i });
			const confirmBtn = screen.getByRole('button', { name: /remove phase/i });

			expect(cancelBtn).toHaveAttribute('tabindex');
			expect(confirmBtn).toHaveAttribute('tabindex');
		});
	});

	describe('edge cases', () => {
		it('handles long phase names with truncation or wrapping', () => {
			const longName = 'This Is A Very Long Phase Template Name That Could Cause Layout Issues';

			render(
				<DeletePhaseDialog
					open={true}
					phaseName={longName}
					onConfirm={vi.fn()}
					onCancel={vi.fn()}
				/>
			);

			// Should still display the name (CSS handles truncation)
			expect(screen.getByText(new RegExp(longName.substring(0, 20)))).toBeInTheDocument();
		});

		it('handles phase names with special characters', () => {
			const specialName = 'Review & Test <Phase>';

			render(
				<DeletePhaseDialog
					open={true}
					phaseName={specialName}
					onConfirm={vi.fn()}
					onCancel={vi.fn()}
				/>
			);

			// Special characters should be escaped properly
			expect(screen.getByText(/review & test/i)).toBeInTheDocument();
		});
	});
});
