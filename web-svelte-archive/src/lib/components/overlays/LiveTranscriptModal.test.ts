import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/svelte';
import LiveTranscriptModal from './LiveTranscriptModal.svelte';
import type { Task } from '$lib/types';

// Mock the API module
vi.mock('$lib/api', () => ({
	getTranscripts: vi.fn(() => Promise.resolve([])),
	getProjectTranscripts: vi.fn(() => Promise.resolve([])),
	getTaskState: vi.fn(() => Promise.resolve(null)),
	getProjectTaskState: vi.fn(() => Promise.resolve(null))
}));

// Mock the stores
vi.mock('$lib/stores/project', () => ({
	currentProjectId: {
		subscribe: vi.fn((cb) => {
			cb('test-project');
			return () => {};
		})
	}
}));

// Mock goto
vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

// Mock websocket
const mockEventListeners = new Map<string, Set<Function>>();
vi.mock('$lib/websocket', () => ({
	getWebSocket: vi.fn(() => ({
		on: vi.fn((eventType: string, callback: Function) => {
			if (!mockEventListeners.has(eventType)) {
				mockEventListeners.set(eventType, new Set());
			}
			mockEventListeners.get(eventType)!.add(callback);
			return () => mockEventListeners.get(eventType)?.delete(callback);
		}),
		onStatusChange: vi.fn((callback) => {
			callback('connected');
			return () => {};
		})
	}))
}));

import { getTranscripts, getProjectTranscripts } from '$lib/api';
import { goto } from '$app/navigation';

describe('LiveTranscriptModal', () => {
	const mockTask: Task = {
		id: 'TASK-001',
		title: 'Test Running Task',
		status: 'running',
		weight: 'medium',
		branch: 'orc/TASK-001',
		current_phase: 'implement',
		created_at: '2026-01-13T10:00:00Z',
		updated_at: '2026-01-13T10:30:00Z'
	};

	const mockOnClose = vi.fn();

	beforeEach(() => {
		vi.clearAllMocks();
		mockEventListeners.clear();
	});

	afterEach(() => {
		cleanup();
	});

	describe('rendering', () => {
		it('renders modal when open is true', async () => {
			render(LiveTranscriptModal, {
				props: {
					open: true,
					task: mockTask,
					onClose: mockOnClose
				}
			});

			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
				expect(screen.getByText('Test Running Task')).toBeInTheDocument();
			});
		});

		it('does not render modal when open is false', () => {
			render(LiveTranscriptModal, {
				props: {
					open: false,
					task: mockTask,
					onClose: mockOnClose
				}
			});

			expect(screen.queryByText('TASK-001')).not.toBeInTheDocument();
		});

		it('shows task status badge', async () => {
			render(LiveTranscriptModal, {
				props: {
					open: true,
					task: mockTask,
					onClose: mockOnClose
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Running')).toBeInTheDocument();
			});
		});

		it('shows current phase badge', async () => {
			render(LiveTranscriptModal, {
				props: {
					open: true,
					task: mockTask,
					onClose: mockOnClose
				}
			});

			await waitFor(() => {
				expect(screen.getByText('implement')).toBeInTheDocument();
			});
		});

		it('shows connection status', async () => {
			render(LiveTranscriptModal, {
				props: {
					open: true,
					task: mockTask,
					onClose: mockOnClose
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Live')).toBeInTheDocument();
			});
		});
	});

	describe('transcript loading', () => {
		it('loads transcripts from API', async () => {
			const mockTranscripts = [
				{
					filename: 'implement-001.md',
					content: '# implement - Iteration 1\n\n## Prompt\n\nTest prompt\n\n## Response\n\nTest response',
					created_at: '2026-01-13T10:15:00Z'
				}
			];
			vi.mocked(getProjectTranscripts).mockResolvedValue(mockTranscripts);

			render(LiveTranscriptModal, {
				props: {
					open: true,
					task: mockTask,
					onClose: mockOnClose
				}
			});

			await waitFor(() => {
				expect(getProjectTranscripts).toHaveBeenCalledWith('test-project', 'TASK-001');
			});
		});

		it('shows empty state when no transcripts', async () => {
			vi.mocked(getProjectTranscripts).mockResolvedValue([]);

			render(LiveTranscriptModal, {
				props: {
					open: true,
					task: mockTask,
					onClose: mockOnClose
				}
			});

			await waitFor(() => {
				expect(screen.getByText('No transcript yet')).toBeInTheDocument();
			});
		});
	});

	describe('close behavior', () => {
		it('calls onClose when close button is clicked', async () => {
			render(LiveTranscriptModal, {
				props: {
					open: true,
					task: mockTask,
					onClose: mockOnClose
				}
			});

			await waitFor(() => {
				expect(screen.getByTitle('Close (Esc)')).toBeInTheDocument();
			});

			const closeButton = screen.getByTitle('Close (Esc)');
			await fireEvent.click(closeButton);

			expect(mockOnClose).toHaveBeenCalled();
		});

		it('calls onClose when pressing Escape', async () => {
			render(LiveTranscriptModal, {
				props: {
					open: true,
					task: mockTask,
					onClose: mockOnClose
				}
			});

			await fireEvent.keyDown(window, { key: 'Escape' });

			expect(mockOnClose).toHaveBeenCalled();
		});

		it('calls onClose when clicking backdrop', async () => {
			render(LiveTranscriptModal, {
				props: {
					open: true,
					task: mockTask,
					onClose: mockOnClose
				}
			});

			await waitFor(() => {
				expect(screen.getByRole('dialog')).toBeInTheDocument();
			});

			const backdrop = screen.getByRole('dialog');
			await fireEvent.click(backdrop);

			expect(mockOnClose).toHaveBeenCalled();
		});
	});

	describe('open full view', () => {
		it('navigates to task page when "Open full view" is clicked', async () => {
			render(LiveTranscriptModal, {
				props: {
					open: true,
					task: mockTask,
					onClose: mockOnClose
				}
			});

			await waitFor(() => {
				expect(screen.getByTitle('Open full view')).toBeInTheDocument();
			});

			const openButton = screen.getByTitle('Open full view');
			await fireEvent.click(openButton);

			expect(goto).toHaveBeenCalledWith('/tasks/TASK-001?tab=transcript');
			expect(mockOnClose).toHaveBeenCalled();
		});
	});

	describe('different task statuses', () => {
		it('shows Paused status for paused tasks', async () => {
			const pausedTask = { ...mockTask, status: 'paused' as const };

			render(LiveTranscriptModal, {
				props: {
					open: true,
					task: pausedTask,
					onClose: mockOnClose
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Paused')).toBeInTheDocument();
			});
		});

		it('shows Blocked status for blocked tasks', async () => {
			const blockedTask = { ...mockTask, status: 'blocked' as const };

			render(LiveTranscriptModal, {
				props: {
					open: true,
					task: blockedTask,
					onClose: mockOnClose
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Blocked')).toBeInTheDocument();
			});
		});

		it('shows Completed status for completed tasks', async () => {
			const completedTask = { ...mockTask, status: 'completed' as const };

			render(LiveTranscriptModal, {
				props: {
					open: true,
					task: completedTask,
					onClose: mockOnClose
				}
			});

			await waitFor(() => {
				expect(screen.getByText('Completed')).toBeInTheDocument();
			});
		});
	});
});
