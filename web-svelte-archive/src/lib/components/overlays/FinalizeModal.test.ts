import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import FinalizeModal from './FinalizeModal.svelte';

// Mock the WebSocket module
vi.mock('$lib/websocket', () => ({
	getWebSocket: vi.fn(() => ({
		on: vi.fn(() => vi.fn()),
		onStatusChange: vi.fn(() => vi.fn())
	}))
}));

describe('FinalizeModal', () => {
	const mockTask = {
		id: 'TASK-001',
		title: 'Test Task',
		description: 'Test description',
		weight: 'medium' as const,
		status: 'completed' as const,
		branch: 'orc/TASK-001',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-02T00:00:00Z'
	};

	const mockOnClose = vi.fn();

	beforeEach(() => {
		vi.clearAllMocks();
		// Mock the API module with not_started status
		vi.mock('$lib/api', () => ({
			triggerFinalize: vi.fn().mockResolvedValue({ task_id: 'TASK-001', status: 'pending', message: 'Finalize started' }),
			getFinalizeStatus: vi.fn().mockResolvedValue({ task_id: 'TASK-001', status: 'not_started' })
		}));
	});

	afterEach(() => {
		vi.clearAllMocks();
	});

	it('renders nothing when not open', async () => {
		const { container } = render(FinalizeModal, {
			props: {
				open: false,
				task: mockTask,
				onClose: mockOnClose
			}
		});

		expect(container.querySelector('.modal-backdrop')).toBeNull();
	});

	it('renders modal when open', async () => {
		render(FinalizeModal, {
			props: {
				open: true,
				task: mockTask,
				onClose: mockOnClose
			}
		});

		expect(screen.getByRole('dialog')).toBeInTheDocument();
		expect(screen.getByText('TASK-001')).toBeInTheDocument();
		expect(screen.getByText('Finalize Task')).toBeInTheDocument();
	});

	it('calls onClose when close button is clicked', async () => {
		render(FinalizeModal, {
			props: {
				open: true,
				task: mockTask,
				onClose: mockOnClose
			}
		});

		const closeButton = screen.getByTitle('Close (Esc)');
		await fireEvent.click(closeButton);

		expect(mockOnClose).toHaveBeenCalled();
	});

	it('calls onClose when backdrop is clicked', async () => {
		render(FinalizeModal, {
			props: {
				open: true,
				task: mockTask,
				onClose: mockOnClose
			}
		});

		const backdrop = screen.getByRole('dialog');
		await fireEvent.click(backdrop);

		expect(mockOnClose).toHaveBeenCalled();
	});

	it('has footer with Close button', async () => {
		render(FinalizeModal, {
			props: {
				open: true,
				task: mockTask,
				onClose: mockOnClose
			}
		});

		// Close button should always be present
		const closeButton = screen.getByRole('button', { name: 'Close' });
		expect(closeButton).toBeInTheDocument();
	});

	it('displays task ID in header', async () => {
		render(FinalizeModal, {
			props: {
				open: true,
				task: mockTask,
				onClose: mockOnClose
			}
		});

		expect(screen.getByText('TASK-001')).toBeInTheDocument();
	});

	it('displays modal title', async () => {
		render(FinalizeModal, {
			props: {
				open: true,
				task: mockTask,
				onClose: mockOnClose
			}
		});

		expect(screen.getByText('Finalize Task')).toBeInTheDocument();
	});
});
