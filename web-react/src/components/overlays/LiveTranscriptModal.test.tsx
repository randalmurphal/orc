import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import { LiveTranscriptModal } from './LiveTranscriptModal';
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
vi.mock('@/lib/api', () => ({
	getTaskState: vi.fn().mockResolvedValue({
		task_id: 'TASK-001',
		status: 'running',
		current_phase: 'implement',
		tokens: {
			input_tokens: 1000,
			output_tokens: 500,
			cache_read_input_tokens: 200,
			total_tokens: 1500,
		},
	}),
}));

// Mock TranscriptTab component
vi.mock('@/components/task-detail/TranscriptTab', () => ({
	TranscriptTab: ({ taskId }: { taskId: string }) => (
		<div data-testid="transcript-tab">Transcript for {taskId}</div>
	),
}));

// Mock useNavigate
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
	const actual = await vi.importActual('react-router-dom');
	return {
		...actual,
		useNavigate: () => mockNavigate,
	};
});

describe('LiveTranscriptModal', () => {
	const mockOnClose = vi.fn();

	const mockTask: Task = {
		id: 'TASK-001',
		title: 'Running Task',
		status: 'running',
		current_phase: 'implement',
		weight: 'medium',
		branch: 'orc/TASK-001',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
	};

	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
		const portalContent = document.querySelector('.live-transcript-backdrop');
		if (portalContent) {
			portalContent.remove();
		}
	});

	const renderModal = (props = {}) => {
		return render(
			<BrowserRouter>
				<LiveTranscriptModal open={true} task={mockTask} onClose={mockOnClose} {...props} />
			</BrowserRouter>
		);
	};

	it('renders nothing when open is false', () => {
		render(
			<BrowserRouter>
				<LiveTranscriptModal open={false} task={mockTask} onClose={mockOnClose} />
			</BrowserRouter>
		);
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

	it('displays task title', () => {
		renderModal();
		expect(screen.getByText('Running Task')).toBeInTheDocument();
	});

	it('displays task status', () => {
		renderModal();
		expect(screen.getByText('Running')).toBeInTheDocument();
	});

	it('displays current phase', () => {
		renderModal();
		expect(screen.getByText('implement')).toBeInTheDocument();
	});

	it('displays connection status', async () => {
		renderModal();
		await waitFor(() => {
			expect(screen.getByText('Live')).toBeInTheDocument();
		});
	});

	it('renders TranscriptTab', async () => {
		renderModal();
		await waitFor(() => {
			expect(screen.getByTestId('transcript-tab')).toBeInTheDocument();
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
		const backdrop = document.querySelector('.live-transcript-backdrop');
		fireEvent.click(backdrop!);
		expect(mockOnClose).toHaveBeenCalledTimes(1);
	});

	it('renders full view button', () => {
		renderModal();
		const fullViewButton = document.querySelector('.header-btn:not(.close-btn)');
		expect(fullViewButton).toBeInTheDocument();
	});

	it('navigates to task detail on full view click', () => {
		renderModal();
		const fullViewButton = document.querySelector('.header-btn:not(.close-btn)');
		fireEvent.click(fullViewButton!);
		expect(mockNavigate).toHaveBeenCalledWith('/tasks/TASK-001?tab=transcript');
		expect(mockOnClose).toHaveBeenCalledTimes(1);
	});

	it('has proper accessibility attributes', () => {
		renderModal();
		const dialog = screen.getByRole('dialog');
		expect(dialog).toHaveAttribute('aria-modal', 'true');
	});

	it('displays different status for paused tasks', () => {
		const pausedTask = { ...mockTask, status: 'paused' as const };
		render(
			<BrowserRouter>
				<LiveTranscriptModal open={true} task={pausedTask} onClose={mockOnClose} />
			</BrowserRouter>
		);
		expect(screen.getByText('Paused')).toBeInTheDocument();
	});

	it('displays different status for completed tasks', () => {
		const completedTask = { ...mockTask, status: 'completed' as const };
		render(
			<BrowserRouter>
				<LiveTranscriptModal open={true} task={completedTask} onClose={mockOnClose} />
			</BrowserRouter>
		);
		expect(screen.getByText('Completed')).toBeInTheDocument();
	});
});
