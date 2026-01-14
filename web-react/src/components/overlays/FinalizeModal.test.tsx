import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react';
import { FinalizeModal } from './FinalizeModal';
import type { Task } from '@/lib/types';

// Mock the WebSocket hook
vi.mock('@/hooks/useWebSocket', () => ({
	useWebSocket: vi.fn(() => ({
		status: 'connected',
		on: vi.fn(() => vi.fn()),
		subscribe: vi.fn(),
		unsubscribe: vi.fn(),
	})),
}));

// Mock the API
const mockTriggerFinalize = vi.fn().mockResolvedValue(undefined);
const mockGetFinalizeStatus = vi.fn();

vi.mock('@/lib/api', () => ({
	triggerFinalize: () => mockTriggerFinalize(),
	getFinalizeStatus: () => mockGetFinalizeStatus(),
}));

describe('FinalizeModal', () => {
	const mockOnClose = vi.fn();

	const mockTask: Task = {
		id: 'TASK-001',
		title: 'Task to Finalize',
		status: 'completed',
		weight: 'medium',
		branch: 'orc/TASK-001',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
	};

	beforeEach(() => {
		vi.clearAllMocks();
		mockGetFinalizeStatus.mockRejectedValue(new Error('Not found'));
	});

	afterEach(() => {
		cleanup();
		const portalContent = document.querySelector('.finalize-modal-backdrop');
		if (portalContent) {
			portalContent.remove();
		}
	});

	const renderModal = (props = {}) => {
		return render(
			<FinalizeModal open={true} task={mockTask} onClose={mockOnClose} {...props} />
		);
	};

	it('renders nothing when open is false', () => {
		render(<FinalizeModal open={false} task={mockTask} onClose={mockOnClose} />);
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('renders dialog when open is true', () => {
		renderModal();
		expect(screen.getByRole('dialog')).toBeInTheDocument();
	});

	it('displays task ID', () => {
		renderModal();
		expect(screen.getByText('TASK-001')).toBeInTheDocument();
	});

	it('displays "Finalize Task" title', () => {
		renderModal();
		expect(screen.getByRole('heading', { name: /finalize task/i })).toBeInTheDocument();
	});

	it('displays connection status', async () => {
		renderModal();
		await waitFor(() => {
			expect(screen.getByText('Live')).toBeInTheDocument();
		});
	});

	it('renders close button', () => {
		renderModal();
		const closeButton = document.querySelector('.close-btn');
		expect(closeButton).toBeInTheDocument();
	});

	it('calls onClose when close button is clicked', () => {
		renderModal();
		const closeButton = document.querySelector('.close-btn');
		fireEvent.click(closeButton!);
		expect(mockOnClose).toHaveBeenCalledTimes(1);
	});

	it('calls onClose when Escape key is pressed', () => {
		renderModal();
		fireEvent.keyDown(window, { key: 'Escape' });
		expect(mockOnClose).toHaveBeenCalledTimes(1);
	});

	it('calls onClose when backdrop is clicked', () => {
		renderModal();
		const backdrop = document.querySelector('.finalize-modal-backdrop');
		fireEvent.click(backdrop!);
		expect(mockOnClose).toHaveBeenCalledTimes(1);
	});

	it('renders Start Finalize button when not started', async () => {
		renderModal();
		await waitFor(() => {
			expect(screen.getByRole('button', { name: /start finalize/i })).toBeInTheDocument();
		});
	});

	it('renders informational text when not started', async () => {
		renderModal();
		await waitFor(() => {
			expect(screen.getByText(/sync your branch/i)).toBeInTheDocument();
		});
	});

	it('calls triggerFinalize when Start Finalize button is clicked', async () => {
		renderModal();
		await waitFor(() => {
			const startButton = screen.getByRole('button', { name: /start finalize/i });
			fireEvent.click(startButton);
		});
		expect(mockTriggerFinalize).toHaveBeenCalledTimes(1);
	});

	it('has proper accessibility attributes', () => {
		renderModal();
		const dialog = screen.getByRole('dialog');
		expect(dialog).toHaveAttribute('aria-modal', 'true');
	});

	it('renders close button in footer', async () => {
		renderModal();
		await waitFor(() => {
			// There are two close buttons - one in header (X icon) and one in footer
			const buttons = screen.getAllByRole('button');
			const footerCloseButton = buttons.find(b => b.classList.contains('btn-secondary') && b.textContent === 'Close');
			expect(footerCloseButton).toBeInTheDocument();
		});
	});

	describe('when finalize is running', () => {
		beforeEach(() => {
			mockGetFinalizeStatus.mockResolvedValue({
				task_id: 'TASK-001',
				status: 'running',
				step: 'Syncing branch',
				step_percent: 50,
			});
		});

		it('shows progress bar', async () => {
			renderModal();
			await waitFor(() => {
				expect(document.querySelector('.progress-bar')).toBeInTheDocument();
			});
		});

		it('shows step label', async () => {
			renderModal();
			await waitFor(() => {
				expect(screen.getByText('Syncing branch')).toBeInTheDocument();
			});
		});

		it('shows Running status', async () => {
			renderModal();
			await waitFor(() => {
				expect(screen.getByText('Running')).toBeInTheDocument();
			});
		});
	});

	describe('when finalize is completed', () => {
		beforeEach(() => {
			mockGetFinalizeStatus.mockResolvedValue({
				task_id: 'TASK-001',
				status: 'completed',
				step_percent: 100,
				result: {
					commit_sha: 'abc123def456',
					target_branch: 'main',
					files_changed: 5,
					conflicts_resolved: 2,
					tests_passed: true,
					risk_level: 'low',
				},
			});
		});

		it('shows completed status', async () => {
			renderModal();
			await waitFor(() => {
				expect(screen.getByText('Completed')).toBeInTheDocument();
			});
		});

		it('shows commit SHA', async () => {
			renderModal();
			await waitFor(() => {
				expect(screen.getByText('abc123d')).toBeInTheDocument();
			});
		});

		it('shows target branch', async () => {
			renderModal();
			await waitFor(() => {
				expect(screen.getByText('main')).toBeInTheDocument();
			});
		});

		it('shows files changed count', async () => {
			renderModal();
			await waitFor(() => {
				expect(screen.getByText('5')).toBeInTheDocument();
			});
		});

		it('shows test status', async () => {
			renderModal();
			await waitFor(() => {
				expect(screen.getByText('Passed')).toBeInTheDocument();
			});
		});

		it('shows risk level', async () => {
			renderModal();
			await waitFor(() => {
				expect(screen.getByText('low')).toBeInTheDocument();
			});
		});
	});

	describe('when finalize has failed', () => {
		beforeEach(() => {
			mockGetFinalizeStatus.mockResolvedValue({
				task_id: 'TASK-001',
				status: 'failed',
				error: 'Merge conflict could not be resolved',
			});
		});

		it('shows failed status', async () => {
			renderModal();
			await waitFor(() => {
				expect(screen.getByText('Failed')).toBeInTheDocument();
			});
		});

		it('shows error message', async () => {
			renderModal();
			await waitFor(() => {
				expect(screen.getByText('Merge conflict could not be resolved')).toBeInTheDocument();
			});
		});

		it('shows retry button', async () => {
			renderModal();
			await waitFor(() => {
				expect(screen.getByRole('button', { name: /retry finalize/i })).toBeInTheDocument();
			});
		});
	});
});
