import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { DecisionsPanel } from './DecisionsPanel';
import type { PendingDecision } from '@/lib/types';

// Sample decision for testing
const createDecision = (overrides: Partial<PendingDecision> = {}): PendingDecision => ({
	id: 'decision-001',
	task_id: 'TASK-001',
	question: 'Which test framework?',
	options: [
		{ id: 'jest', label: 'Jest' },
		{ id: 'vitest', label: 'Vitest' },
		{ id: 'both', label: 'Both' },
	],
	created_at: '2024-01-01T00:00:00Z',
	...overrides,
});

describe('DecisionsPanel', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	describe('empty state', () => {
		it('returns null when decisions array is empty', () => {
			const onDecide = vi.fn();
			const { container } = render(<DecisionsPanel decisions={[]} onDecide={onDecide} />);

			// Component should render nothing
			expect(container.firstChild).toBeNull();
		});

		it('does not render when decisions is empty', () => {
			const onDecide = vi.fn();
			render(<DecisionsPanel decisions={[]} onDecide={onDecide} />);

			// No section should be present
			expect(screen.queryByText('Decisions')).not.toBeInTheDocument();
		});
	});

	describe('rendering with decisions', () => {
		it('renders section header with correct count', () => {
			const decisions = [createDecision(), createDecision({ id: 'decision-002' })];
			const onDecide = vi.fn();

			render(<DecisionsPanel decisions={decisions} onDecide={onDecide} />);

			// Should show "Decisions" title and count badge
			expect(screen.getByText('Decisions')).toBeInTheDocument();
			expect(screen.getByText('2')).toBeInTheDocument();
		});

		it('renders decision question text', () => {
			const decisions = [createDecision({ question: 'Select a database' })];
			const onDecide = vi.fn();

			render(<DecisionsPanel decisions={decisions} onDecide={onDecide} />);

			expect(screen.getByText('Select a database')).toBeInTheDocument();
		});

		it('renders task ID context', () => {
			const decisions = [createDecision({ task_id: 'TASK-123' })];
			const onDecide = vi.fn();

			render(<DecisionsPanel decisions={decisions} onDecide={onDecide} />);

			expect(screen.getByText('TASK-123')).toBeInTheDocument();
		});

		it('renders all option buttons', () => {
			const decisions = [
				createDecision({
					options: [
						{ id: 'opt1', label: 'Option A' },
						{ id: 'opt2', label: 'Option B' },
						{ id: 'opt3', label: 'Option C' },
					],
				}),
			];
			const onDecide = vi.fn();

			render(<DecisionsPanel decisions={decisions} onDecide={onDecide} />);

			expect(screen.getByText('Option A')).toBeInTheDocument();
			expect(screen.getByText('Option B')).toBeInTheDocument();
			expect(screen.getByText('Option C')).toBeInTheDocument();
		});

		it('renders multiple decisions', () => {
			const decisions = [
				createDecision({ id: 'dec-1', question: 'Question 1' }),
				createDecision({ id: 'dec-2', question: 'Question 2' }),
				createDecision({ id: 'dec-3', question: 'Question 3' }),
			];
			const onDecide = vi.fn();

			render(<DecisionsPanel decisions={decisions} onDecide={onDecide} />);

			expect(screen.getByText('Question 1')).toBeInTheDocument();
			expect(screen.getByText('Question 2')).toBeInTheDocument();
			expect(screen.getByText('Question 3')).toBeInTheDocument();
		});
	});

	describe('recommended option highlighting', () => {
		it('highlights first option as recommended when none explicitly marked', () => {
			const decisions = [
				createDecision({
					options: [
						{ id: 'opt1', label: 'First Option' },
						{ id: 'opt2', label: 'Second Option' },
					],
				}),
			];
			const onDecide = vi.fn();

			render(<DecisionsPanel decisions={decisions} onDecide={onDecide} />);

			const firstButton = screen.getByText('First Option');
			const secondButton = screen.getByText('Second Option');

			expect(firstButton.closest('button')).toHaveClass('recommended');
			expect(secondButton.closest('button')).not.toHaveClass('recommended');
		});

		it('highlights explicitly recommended option', () => {
			const decisions = [
				createDecision({
					options: [
						{ id: 'opt1', label: 'Not Recommended' },
						{ id: 'opt2', label: 'Recommended', recommended: true },
						{ id: 'opt3', label: 'Also Not' },
					],
				}),
			];
			const onDecide = vi.fn();

			render(<DecisionsPanel decisions={decisions} onDecide={onDecide} />);

			const recommendedButton = screen.getByText('Recommended');
			const notRecommendedButton = screen.getByText('Not Recommended');

			expect(recommendedButton.closest('button')).toHaveClass('recommended');
			expect(notRecommendedButton.closest('button')).not.toHaveClass('recommended');
		});

		it('does not highlight first option when another is explicitly recommended', () => {
			const decisions = [
				createDecision({
					options: [
						{ id: 'opt1', label: 'First' },
						{ id: 'opt2', label: 'Second', recommended: true },
					],
				}),
			];
			const onDecide = vi.fn();

			render(<DecisionsPanel decisions={decisions} onDecide={onDecide} />);

			const firstButton = screen.getByText('First');
			expect(firstButton.closest('button')).not.toHaveClass('recommended');
		});
	});

	describe('option click handling', () => {
		it('calls onDecide with correct arguments when option clicked', async () => {
			const onDecide = vi.fn();
			const decisions = [
				createDecision({
					id: 'dec-123',
					options: [
						{ id: 'option-a', label: 'Option A' },
						{ id: 'option-b', label: 'Option B' },
					],
				}),
			];

			render(<DecisionsPanel decisions={decisions} onDecide={onDecide} />);

			const optionB = screen.getByText('Option B');
			fireEvent.click(optionB);

			await waitFor(() => {
				expect(onDecide).toHaveBeenCalledWith('dec-123', 'option-b');
			});
		});

		it('calls onDecide for each clicked option', async () => {
			const onDecide = vi.fn();
			const decisions = [
				createDecision({
					id: 'dec-1',
					options: [{ id: 'opt-1', label: 'First' }],
				}),
				createDecision({
					id: 'dec-2',
					options: [{ id: 'opt-2', label: 'Second' }],
				}),
			];

			render(<DecisionsPanel decisions={decisions} onDecide={onDecide} />);

			fireEvent.click(screen.getByText('First'));
			fireEvent.click(screen.getByText('Second'));

			await waitFor(() => {
				expect(onDecide).toHaveBeenCalledTimes(2);
				expect(onDecide).toHaveBeenCalledWith('dec-1', 'opt-1');
				expect(onDecide).toHaveBeenCalledWith('dec-2', 'opt-2');
			});
		});
	});

	describe('loading state', () => {
		it('disables buttons while submitting', async () => {
			let resolveOnDecide: () => void;
			const onDecide = vi.fn(
				() =>
					new Promise<void>((resolve) => {
						resolveOnDecide = resolve;
					})
			);

			const decisions = [
				createDecision({
					options: [
						{ id: 'opt1', label: 'Option 1' },
						{ id: 'opt2', label: 'Option 2' },
					],
				}),
			];

			render(<DecisionsPanel decisions={decisions} onDecide={onDecide} />);

			const option1 = screen.getByText('Option 1');
			fireEvent.click(option1);

			// Both buttons should be disabled while submitting
			await waitFor(() => {
				expect(option1.closest('button')).toBeDisabled();
				expect(screen.getByText('Option 2').closest('button')).toBeDisabled();
			});

			// Resolve the promise
			resolveOnDecide!();

			// Buttons should be enabled again
			await waitFor(() => {
				expect(option1.closest('button')).not.toBeDisabled();
			});
		});

		it('adds submitting class to decision item during submission', async () => {
			let resolveOnDecide: () => void;
			const onDecide = vi.fn(
				() =>
					new Promise<void>((resolve) => {
						resolveOnDecide = resolve;
					})
			);

			const decisions = [createDecision()];
			const { container } = render(<DecisionsPanel decisions={decisions} onDecide={onDecide} />);

			const button = screen.getByText('Jest');
			fireEvent.click(button);

			await waitFor(() => {
				const decisionItem = container.querySelector('.decision-item');
				expect(decisionItem).toHaveClass('submitting');
			});

			resolveOnDecide!();

			await waitFor(() => {
				const decisionItem = container.querySelector('.decision-item');
				expect(decisionItem).not.toHaveClass('submitting');
			});
		});
	});

	describe('long text wrapping', () => {
		it('renders long question text without overflow', () => {
			const longQuestion =
				'This is a very long question that should wrap properly within the decision panel without causing any horizontal overflow issues';
			const decisions = [createDecision({ question: longQuestion })];
			const onDecide = vi.fn();

			const { container } = render(<DecisionsPanel decisions={decisions} onDecide={onDecide} />);

			const questionElement = container.querySelector('.decision-question');
			expect(questionElement).toHaveTextContent(longQuestion);
		});

		it('renders long option labels correctly', () => {
			const decisions = [
				createDecision({
					options: [
						{ id: 'opt1', label: 'A very long option label that should wrap' },
						{ id: 'opt2', label: 'Another lengthy option text' },
					],
				}),
			];
			const onDecide = vi.fn();

			render(<DecisionsPanel decisions={decisions} onDecide={onDecide} />);

			expect(screen.getByText('A very long option label that should wrap')).toBeInTheDocument();
			expect(screen.getByText('Another lengthy option text')).toBeInTheDocument();
		});
	});

	describe('option description tooltip', () => {
		it('sets title attribute for option with description', () => {
			const decisions = [
				createDecision({
					options: [
						{
							id: 'opt1',
							label: 'Option',
							description: 'This is the description for this option',
						},
					],
				}),
			];
			const onDecide = vi.fn();

			render(<DecisionsPanel decisions={decisions} onDecide={onDecide} />);

			const button = screen.getByText('Option');
			expect(button.closest('button')).toHaveAttribute(
				'title',
				'This is the description for this option'
			);
		});
	});

	describe('accessibility', () => {
		it('has aria-busy on decision item during submission', async () => {
			let resolveOnDecide: () => void;
			const onDecide = vi.fn(
				() =>
					new Promise<void>((resolve) => {
						resolveOnDecide = resolve;
					})
			);

			const decisions = [createDecision()];
			const { container } = render(<DecisionsPanel decisions={decisions} onDecide={onDecide} />);

			const button = screen.getByText('Jest');
			fireEvent.click(button);

			await waitFor(() => {
				const decisionItem = container.querySelector('.decision-item');
				expect(decisionItem).toHaveAttribute('aria-busy', 'true');
			});

			resolveOnDecide!();

			await waitFor(() => {
				const decisionItem = container.querySelector('.decision-item');
				expect(decisionItem).not.toHaveAttribute('aria-busy', 'true');
			});
		});

		it('has appropriate aria-label for recommended options', () => {
			const decisions = [
				createDecision({
					options: [
						{ id: 'opt1', label: 'Recommended Option', recommended: true },
						{ id: 'opt2', label: 'Normal Option' },
					],
				}),
			];
			const onDecide = vi.fn();

			render(<DecisionsPanel decisions={decisions} onDecide={onDecide} />);

			const recommendedButton = screen.getByText('Recommended Option');
			expect(recommendedButton.closest('button')).toHaveAttribute(
				'aria-label',
				'Recommended Option (recommended)'
			);

			const normalButton = screen.getByText('Normal Option');
			expect(normalButton.closest('button')).toHaveAttribute('aria-label', 'Normal Option');
		});
	});
});
