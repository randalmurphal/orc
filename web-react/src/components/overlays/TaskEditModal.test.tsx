import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react';
import { TaskEditModal } from './TaskEditModal';
import type { Task } from '@/lib/types';

describe('TaskEditModal', () => {
	const mockOnClose = vi.fn();
	const mockOnSave = vi.fn().mockResolvedValue(undefined);

	const mockTask: Task = {
		id: 'TASK-001',
		title: 'Test Task',
		description: 'Test description',
		status: 'created',
		weight: 'medium',
		branch: 'orc/TASK-001',
		queue: 'active',
		priority: 'normal',
		category: 'feature',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
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
		return render(
			<TaskEditModal
				open={true}
				task={mockTask}
				onClose={mockOnClose}
				onSave={mockOnSave}
				{...props}
			/>
		);
	};

	it('renders nothing when open is false', () => {
		render(
			<TaskEditModal
				open={false}
				task={mockTask}
				onClose={mockOnClose}
				onSave={mockOnSave}
			/>
		);
		expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
	});

	it('renders dialog when open is true', () => {
		renderModal();
		expect(screen.getByRole('dialog')).toBeInTheDocument();
	});

	it('populates form with task values', () => {
		renderModal();
		expect(screen.getByDisplayValue('Test Task')).toBeInTheDocument();
		expect(screen.getByDisplayValue('Test description')).toBeInTheDocument();
	});

	it('renders weight options', () => {
		renderModal();
		expect(screen.getByText('Trivial')).toBeInTheDocument();
		expect(screen.getByText('Small')).toBeInTheDocument();
		expect(screen.getByText('Medium')).toBeInTheDocument();
		expect(screen.getByText('Large')).toBeInTheDocument();
		expect(screen.getByText('Greenfield')).toBeInTheDocument();
	});

	it('renders category options', () => {
		renderModal();
		expect(screen.getByText('Feature')).toBeInTheDocument();
		expect(screen.getByText('Bug')).toBeInTheDocument();
		expect(screen.getByText('Refactor')).toBeInTheDocument();
		expect(screen.getByText('Chore')).toBeInTheDocument();
		expect(screen.getByText('Docs')).toBeInTheDocument();
		expect(screen.getByText('Test')).toBeInTheDocument();
	});

	it('renders queue options', () => {
		renderModal();
		expect(screen.getByText('Active')).toBeInTheDocument();
		expect(screen.getByText('Backlog')).toBeInTheDocument();
	});

	it('renders priority options', () => {
		renderModal();
		expect(screen.getByText('Critical')).toBeInTheDocument();
		expect(screen.getByText('High')).toBeInTheDocument();
		expect(screen.getByText('Normal')).toBeInTheDocument();
		expect(screen.getByText('Low')).toBeInTheDocument();
	});

	it('disables save button when no changes', () => {
		renderModal();
		const saveButton = screen.getByRole('button', { name: /save changes/i });
		expect(saveButton).toBeDisabled();
	});

	it('enables save button when title changes', () => {
		renderModal();
		const titleInput = screen.getByDisplayValue('Test Task');
		fireEvent.change(titleInput, { target: { value: 'Updated Task' } });
		const saveButton = screen.getByRole('button', { name: /save changes/i });
		expect(saveButton).not.toBeDisabled();
	});

	it('calls onClose when cancel button is clicked', () => {
		renderModal();
		fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
		expect(mockOnClose).toHaveBeenCalledTimes(1);
	});

	it('calls onSave with changed values when save is clicked', async () => {
		renderModal();
		const titleInput = screen.getByDisplayValue('Test Task');
		fireEvent.change(titleInput, { target: { value: 'Updated Task Title' } });

		const saveButton = screen.getByRole('button', { name: /save changes/i });
		fireEvent.click(saveButton);

		await waitFor(() => {
			expect(mockOnSave).toHaveBeenCalledWith({
				title: 'Updated Task Title',
			});
		});
	});

	it('changes weight selection', () => {
		renderModal();
		const smallButton = screen.getByText('Small').closest('label');
		if (smallButton) {
			fireEvent.click(smallButton);
		}
		const saveButton = screen.getByRole('button', { name: /save changes/i });
		expect(saveButton).not.toBeDisabled();
	});

	it('changes category selection', () => {
		renderModal();
		const bugLabel = screen.getByText('Bug').closest('label');
		if (bugLabel) {
			fireEvent.click(bugLabel);
		}
		const saveButton = screen.getByRole('button', { name: /save changes/i });
		expect(saveButton).not.toBeDisabled();
	});

	it('shows keyboard hint', () => {
		renderModal();
		expect(screen.getByText(/enter/i)).toBeInTheDocument();
	});

	it('resets form when task changes', () => {
		const { rerender } = renderModal();

		// Change title
		const titleInput = screen.getByDisplayValue('Test Task');
		fireEvent.change(titleInput, { target: { value: 'Changed' } });
		expect(screen.getByDisplayValue('Changed')).toBeInTheDocument();

		// Rerender with new task
		const newTask = { ...mockTask, title: 'New Task' };
		rerender(
			<TaskEditModal
				open={true}
				task={newTask}
				onClose={mockOnClose}
				onSave={mockOnSave}
			/>
		);

		// Should show new task's title
		expect(screen.getByDisplayValue('New Task')).toBeInTheDocument();
	});
});
