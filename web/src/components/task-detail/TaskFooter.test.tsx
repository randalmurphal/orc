import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { TaskFooter } from './TaskFooter';
import { TaskStatus } from '@/gen/orc/v1/task_pb';
import { createMockTask } from '@/test/factories';

const mockPauseTask = vi.fn();
const mockResumeTask = vi.fn();

vi.mock('@/lib/client', () => ({
	taskClient: {
		pauseTask: (...args: unknown[]) => mockPauseTask(...args),
		resumeTask: (...args: unknown[]) => mockResumeTask(...args),
	},
}));

vi.mock('@/stores/uiStore', () => ({
	toast: {
		success: vi.fn(),
		error: vi.fn(),
	},
}));

vi.mock('@/stores', () => ({
	useCurrentProjectId: () => 'test-project',
}));

describe('TaskFooter', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mockPauseTask.mockResolvedValue({ task: {} });
		mockResumeTask.mockResolvedValue({ task: {} });
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	describe('metrics rendering', () => {
		it('displays token count and cost when metrics are available', () => {
			const task = createMockTask({ status: TaskStatus.RUNNING });

			render(
				<TaskFooter
					task={task}
					metrics={{
						tokens: 45200,
						cost: 1.96,
						inputTokens: 30000,
						outputTokens: 15200,
					}}
				/>
			);

			expect(screen.getByText(/45\.2K/i)).toBeInTheDocument();
			expect(screen.getByText(/\$1\.96/)).toBeInTheDocument();
		});

		it('displays placeholder when metrics are unavailable', () => {
			const task = createMockTask({ status: TaskStatus.RUNNING });
			render(<TaskFooter task={task} metrics={null} />);
			expect(screen.getByText('—')).toBeInTheDocument();
		});
	});

	describe('running and paused actions', () => {
		it('shows Pause and Cancel buttons when task is running', () => {
			const task = createMockTask({ status: TaskStatus.RUNNING });
			render(<TaskFooter task={task} metrics={null} />);

			expect(screen.getByRole('button', { name: /pause/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();
		});

		it('clicking Pause calls pauseTask API', async () => {
			const task = createMockTask({ status: TaskStatus.RUNNING });
			render(<TaskFooter task={task} metrics={null} />);

			fireEvent.click(screen.getByRole('button', { name: /pause/i }));

			await waitFor(() => {
				expect(mockPauseTask).toHaveBeenCalled();
			});
		});

		it('clicking Cancel also calls pauseTask API', async () => {
			const task = createMockTask({ status: TaskStatus.RUNNING });
			render(<TaskFooter task={task} metrics={null} />);

			fireEvent.click(screen.getByRole('button', { name: /cancel/i }));

			await waitFor(() => {
				expect(mockPauseTask).toHaveBeenCalled();
			});
		});

		it('shows Resume button when task is paused', () => {
			const task = createMockTask({ status: TaskStatus.PAUSED });
			render(<TaskFooter task={task} metrics={null} />);

			expect(screen.getByRole('button', { name: /resume/i })).toBeInTheDocument();
		});

		it('clicking Resume calls resumeTask API', async () => {
			const task = createMockTask({ status: TaskStatus.PAUSED });
			render(<TaskFooter task={task} metrics={null} />);

			fireEvent.click(screen.getByRole('button', { name: /resume/i }));

			await waitFor(() => {
				expect(mockResumeTask).toHaveBeenCalled();
			});
		});
	});

	describe('terminal state footer', () => {
		it('shows completed status with compact footer', () => {
			const task = createMockTask({ status: TaskStatus.COMPLETED });
			render(<TaskFooter task={task} metrics={null} />);

			expect(screen.getByText(/completed/i)).toBeInTheDocument();
			expect(screen.queryByRole('button', { name: /pause/i })).not.toBeInTheDocument();
		});

		it('shows failed status without retry controls in footer', () => {
			const task = createMockTask({ status: TaskStatus.FAILED, currentPhase: 'implement' });
			render(<TaskFooter task={task} metrics={null} />);

			expect(screen.getByText(/failed/i)).toBeInTheDocument();
			expect(screen.queryByRole('button', { name: /retry/i })).not.toBeInTheDocument();
			expect(screen.queryByPlaceholderText(/guidance|feedback|retry/i)).not.toBeInTheDocument();
		});
	});
});
