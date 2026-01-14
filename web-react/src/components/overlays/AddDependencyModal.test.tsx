import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react';
import { AddDependencyModal } from './AddDependencyModal';

// Mock the API
vi.mock('@/lib/api', () => ({
	listTasks: vi.fn(() =>
		Promise.resolve({
			tasks: [
				{
					id: 'TASK-001',
					title: 'First Task',
					status: 'pending',
					weight: 'small',
					created_at: '2024-01-01T00:00:00Z',
					updated_at: '2024-01-01T00:00:00Z',
				},
				{
					id: 'TASK-002',
					title: 'Second Task',
					status: 'running',
					weight: 'medium',
					created_at: '2024-01-01T00:00:00Z',
					updated_at: '2024-01-01T00:00:00Z',
				},
				{
					id: 'TASK-003',
					title: 'Third Task',
					status: 'completed',
					weight: 'large',
					created_at: '2024-01-01T00:00:00Z',
					updated_at: '2024-01-01T00:00:00Z',
				},
				{
					id: 'TASK-010',
					title: 'Current Task',
					status: 'pending',
					weight: 'medium',
					created_at: '2024-01-01T00:00:00Z',
					updated_at: '2024-01-01T00:00:00Z',
				},
			],
		})
	),
}));

describe('AddDependencyModal', () => {
	const mockOnClose = vi.fn();
	const mockOnSelect = vi.fn();

	const defaultProps = {
		open: true,
		onClose: mockOnClose,
		onSelect: mockOnSelect,
		type: 'blocker' as const,
		currentTaskId: 'TASK-010',
		existingBlockers: ['TASK-001'],
		existingRelated: [] as string[],
	};

	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
		const portalContent = document.querySelector('.modal-backdrop');
		if (portalContent) {
			portalContent.remove();
		}
	});

	const renderModal = (props = {}) => {
		return render(<AddDependencyModal {...defaultProps} {...props} />);
	};

	it('renders nothing when open is false', () => {
		render(<AddDependencyModal {...defaultProps} open={false} />);
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('renders dialog when open is true', async () => {
		renderModal();
		await waitFor(() => {
			expect(screen.getByRole('dialog')).toBeInTheDocument();
		});
	});

	it('renders correct title for blocker type', async () => {
		renderModal({ type: 'blocker' });
		await waitFor(() => {
			expect(screen.getByRole('heading', { name: /add blocking task/i })).toBeInTheDocument();
		});
	});

	it('renders correct title for related type', async () => {
		renderModal({ type: 'related' });
		await waitFor(() => {
			expect(screen.getByRole('heading', { name: /add related task/i })).toBeInTheDocument();
		});
	});

	it('renders search input', async () => {
		renderModal();
		await waitFor(() => {
			expect(screen.getByPlaceholderText(/search by id or title/i)).toBeInTheDocument();
		});
	});

	it('shows loading state initially', () => {
		renderModal();
		expect(screen.getByText(/loading tasks/i)).toBeInTheDocument();
	});

	it('excludes current task from list', async () => {
		renderModal();
		await waitFor(() => {
			// TASK-010 should not appear in the list
			expect(screen.queryByText('Current Task')).not.toBeInTheDocument();
		});
	});

	it('excludes already blocked tasks when type is blocker', async () => {
		renderModal({ type: 'blocker', existingBlockers: ['TASK-001'] });
		await waitFor(() => {
			// TASK-001 is already in existingBlockers, should not appear
			expect(screen.queryByText('First Task')).not.toBeInTheDocument();
		});
	});

	it('excludes already related tasks when type is related', async () => {
		renderModal({ type: 'related', existingRelated: ['TASK-001'] });
		await waitFor(() => {
			// TASK-001 is already in existingRelated, should not appear
			expect(screen.queryByText('First Task')).not.toBeInTheDocument();
		});
	});

	it('shows available tasks', async () => {
		renderModal();
		await waitFor(() => {
			// TASK-002 and TASK-003 should be available
			expect(screen.getByText('Second Task')).toBeInTheDocument();
			expect(screen.getByText('Third Task')).toBeInTheDocument();
		});
	});

	it('filters tasks by search query', async () => {
		renderModal();
		await waitFor(() => {
			expect(screen.getByText('Second Task')).toBeInTheDocument();
		});

		const searchInput = screen.getByPlaceholderText(/search by id or title/i);
		fireEvent.change(searchInput, { target: { value: 'Second' } });

		await waitFor(() => {
			expect(screen.getByText('Second Task')).toBeInTheDocument();
			expect(screen.queryByText('Third Task')).not.toBeInTheDocument();
		});
	});

	it('shows no results when search has no matches', async () => {
		renderModal();
		await waitFor(() => {
			expect(screen.getByText('Second Task')).toBeInTheDocument();
		});

		const searchInput = screen.getByPlaceholderText(/search by id or title/i);
		fireEvent.change(searchInput, { target: { value: 'nonexistent' } });

		await waitFor(() => {
			expect(screen.getByText(/no tasks matching/i)).toBeInTheDocument();
		});
	});

	it('calls onSelect when task is clicked', async () => {
		renderModal();
		await waitFor(() => {
			expect(screen.getByText('Second Task')).toBeInTheDocument();
		});

		const taskButton = screen.getByText('Second Task').closest('button');
		fireEvent.click(taskButton!);

		expect(mockOnSelect).toHaveBeenCalledWith('TASK-002');
	});

	it('calls onClose when cancel button is clicked', async () => {
		renderModal();
		await waitFor(() => {
			expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();
		});
		fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
		expect(mockOnClose).toHaveBeenCalledTimes(1);
	});

	it('displays task status indicators', async () => {
		renderModal();
		await waitFor(() => {
			const statusElements = document.querySelectorAll('.task-status');
			expect(statusElements.length).toBeGreaterThan(0);
		});
	});

	it('displays task IDs', async () => {
		renderModal();
		await waitFor(() => {
			expect(screen.getByText('TASK-002')).toBeInTheDocument();
			expect(screen.getByText('TASK-003')).toBeInTheDocument();
		});
	});

	it('has proper accessibility attributes', async () => {
		renderModal();
		await waitFor(() => {
			const dialog = screen.getByRole('dialog');
			expect(dialog).toHaveAttribute('aria-modal', 'true');
		});
	});
});
