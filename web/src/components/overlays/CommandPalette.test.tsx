import { create } from '@bufbuild/protobuf';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { CommandPalette } from './CommandPalette';
import { useProjectStore } from '@/stores/projectStore';
import { useTaskStore } from '@/stores/taskStore';
import { useThreadStore } from '@/stores/threadStore';
import { createMockTask, createMockThread, createTimestamp } from '@/test/factories';
import { ProjectSchema } from '@/gen/orc/v1/project_pb';
import { TaskStatus } from '@/gen/orc/v1/task_pb';

const mockNavigate = vi.fn();
const mockResumeTask = vi.fn();
const mockUpdateTask = vi.fn();
const mockFinalizeTask = vi.fn();
const toastSuccess = vi.fn();
const toastError = vi.fn();

vi.mock('react-router-dom', async () => {
	const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
	return {
		...actual,
		useNavigate: () => mockNavigate,
	};
});

vi.mock('@/lib/client', () => ({
	taskClient: {
		resumeTask: (...args: unknown[]) => mockResumeTask(...args),
		updateTask: (...args: unknown[]) => mockUpdateTask(...args),
		finalizeTask: (...args: unknown[]) => mockFinalizeTask(...args),
	},
}));

vi.mock('@/stores/uiStore', () => ({
	toast: {
		success: (...args: unknown[]) => toastSuccess(...args),
		error: (...args: unknown[]) => toastError(...args),
	},
}));

function renderPalette() {
	return render(
		<MemoryRouter>
			<CommandPalette open={true} onClose={vi.fn()} />
		</MemoryRouter>
	);
}

describe('CommandPalette', () => {
	beforeEach(() => {
		mockNavigate.mockReset();
		mockResumeTask.mockReset();
		mockUpdateTask.mockReset();
		mockFinalizeTask.mockReset();
		toastSuccess.mockReset();
		toastError.mockReset();

		Object.defineProperty(navigator, 'clipboard', {
			configurable: true,
			value: {
				writeText: vi.fn().mockResolvedValue(undefined),
			},
		});

		useProjectStore.setState({
			projects: [
				create(ProjectSchema, {
					id: 'P1',
					name: 'Project One',
					path: '/projects/one',
					createdAt: createTimestamp('2024-01-01T00:00:00Z'),
				}),
				create(ProjectSchema, {
					id: 'P2',
					name: 'Project Two',
					path: '/projects/two',
					createdAt: createTimestamp('2024-01-02T00:00:00Z'),
				}),
			],
			currentProjectId: 'P1',
			loading: false,
			error: null,
			_isHandlingPopState: false,
		});
		useTaskStore.setState({
			tasks: [],
			taskStates: new Map(),
			taskActivities: new Map(),
			taskOutputLines: new Map(),
			taskSessionMetrics: new Map(),
			taskPhaseProgress: new Map(),
			loading: false,
			error: null,
		});
		useThreadStore.setState({
			threads: [],
			selectedThreadId: null,
			loading: false,
			error: null,
		});
	});

	it('filters commands, supports arrow navigation, and executes the selected item', async () => {
		useTaskStore.setState({
			tasks: [createMockTask({ id: 'TASK-001', title: 'Paused task', status: TaskStatus.PAUSED })],
		});
		mockResumeTask.mockResolvedValue({
			task: createMockTask({ id: 'TASK-001', status: TaskStatus.RUNNING }),
		});

		renderPalette();

		const input = screen.getByLabelText('Search commands');
		await userEvent.type(input, 'resume');

		expect(screen.getByRole('option', { name: /^resume task-001/i })).toBeInTheDocument();
		expect(screen.getByRole('option', { name: /copy: orc resume task-001/i })).toBeInTheDocument();
		expect(screen.queryByRole('option', { name: /^show task-001/i })).not.toBeInTheDocument();

		fireEvent.keyDown(screen.getByRole('dialog', { name: 'Command palette' }), { key: 'ArrowDown' });
		fireEvent.keyDown(screen.getByRole('dialog', { name: 'Command palette' }), { key: 'Enter' });

		await waitFor(() => {
			expect(navigator.clipboard.writeText).toHaveBeenCalledWith('orc resume TASK-001');
		});
	});

	it('shows task actions only when the task status allows them', () => {
		useTaskStore.setState({
			tasks: [
				createMockTask({ id: 'TASK-CREATED', status: TaskStatus.CREATED }),
				createMockTask({ id: 'TASK-PLANNED', status: TaskStatus.PLANNED }),
				createMockTask({ id: 'TASK-PAUSED', status: TaskStatus.PAUSED }),
				createMockTask({ id: 'TASK-FAILED', status: TaskStatus.FAILED }),
				createMockTask({ id: 'TASK-BLOCKED', status: TaskStatus.BLOCKED }),
				createMockTask({ id: 'TASK-COMPLETED', status: TaskStatus.COMPLETED }),
				createMockTask({ id: 'TASK-RUNNING', status: TaskStatus.RUNNING }),
			],
		});

		renderPalette();

		expect(screen.getByRole('option', { name: /^resume task-paused/i })).toBeInTheDocument();
		expect(screen.getByRole('option', { name: /^resume task-failed/i })).toBeInTheDocument();
		expect(screen.getByRole('option', { name: /^resume task-blocked/i })).toBeInTheDocument();
		expect(screen.queryByRole('option', { name: /resume task-running/i })).not.toBeInTheDocument();
		expect(screen.getByRole('option', { name: /^approve task-blocked/i })).toBeInTheDocument();
		expect(screen.queryByRole('option', { name: /approve task-failed/i })).not.toBeInTheDocument();
		expect(screen.getByRole('option', { name: /^finalize task-completed/i })).toBeInTheDocument();
		expect(screen.queryByRole('option', { name: /finalize task-created/i })).not.toBeInTheDocument();
		expect(screen.queryByRole('option', { name: /finalize task-planned/i })).not.toBeInTheDocument();
		expect(screen.queryByRole('option', { name: /finalize task-running/i })).not.toBeInTheDocument();
		expect(screen.queryByRole('option', { name: /finalize task-failed/i })).not.toBeInTheDocument();
		expect(screen.getByRole('option', { name: /^close task-failed/i })).toBeInTheDocument();
		expect(screen.queryByRole('option', { name: /close task-completed/i })).not.toBeInTheDocument();
	});

	it('copies the exact CLI command and shows a toast', async () => {
		useTaskStore.setState({
			tasks: [createMockTask({ id: 'TASK-001', status: TaskStatus.PAUSED })],
		});

		renderPalette();

		await userEvent.click(screen.getByRole('option', { name: /copy: orc resume task-001/i }));

		await waitFor(() => {
			expect(navigator.clipboard.writeText).toHaveBeenCalledWith('orc resume TASK-001');
			expect(toastSuccess).toHaveBeenCalledWith('Copied orc resume TASK-001');
		});
	});

	it('revalidates task status before running state-changing actions', async () => {
		const resumeTask = createMockTask({ id: 'TASK-RESUME', status: TaskStatus.PAUSED });
		const approveTask = createMockTask({ id: 'TASK-APPROVE', status: TaskStatus.BLOCKED });
		const finalizeTask = createMockTask({ id: 'TASK-FINALIZE', status: TaskStatus.COMPLETED });
		const closeTask = createMockTask({ id: 'TASK-CLOSE', status: TaskStatus.FAILED });

		useTaskStore.setState({
			tasks: [resumeTask, approveTask, finalizeTask, closeTask],
		});

		renderPalette();

		const resumeAction = screen.getByRole('option', { name: /^resume task-resume/i });
		const approveAction = screen.getByRole('option', { name: /^approve task-approve/i });
		const finalizeAction = screen.getByRole('option', { name: /^finalize task-finalize/i });
		const closeAction = screen.getByRole('option', { name: /^close task-close/i });

		resumeTask.status = TaskStatus.COMPLETED;
		approveTask.status = TaskStatus.PAUSED;
		finalizeTask.status = TaskStatus.PAUSED;
		closeTask.status = TaskStatus.COMPLETED;

		await userEvent.click(resumeAction);
		await userEvent.click(approveAction);
		await userEvent.click(finalizeAction);
		await userEvent.click(closeAction);

		expect(mockResumeTask).not.toHaveBeenCalled();
		expect(mockUpdateTask).not.toHaveBeenCalled();
		expect(mockFinalizeTask).not.toHaveBeenCalled();
		expect(toastError).toHaveBeenNthCalledWith(
			1,
			'Task TASK-RESUME cannot be resumed from its current status'
		);
		expect(toastError).toHaveBeenNthCalledWith(2, 'Task TASK-APPROVE is not blocked');
		expect(toastError).toHaveBeenNthCalledWith(
			3,
			'Task TASK-FINALIZE cannot be finalized from its current status'
		);
		expect(toastError).toHaveBeenNthCalledWith(4, 'Task TASK-CLOSE is not failed');
	});

	it('routes navigation actions and dispatches global actions', async () => {
		const newTaskListener = vi.fn();
		window.addEventListener('orc:new-task', newTaskListener);

		renderPalette();

		await userEvent.click(screen.getByRole('option', { name: /go to board/i }));
		expect(mockNavigate).toHaveBeenCalledWith('/board');

		await userEvent.click(screen.getByRole('option', { name: /^new task/i }));
		expect(newTaskListener).toHaveBeenCalledTimes(1);

		window.removeEventListener('orc:new-task', newTaskListener);
	});

	it('updates available task actions when the current project changes', async () => {
		const projectOneTask = {
			...createMockTask({ id: 'TASK-P1', title: 'Project One Task', status: TaskStatus.PAUSED }),
			projectId: 'P1',
		};
		const projectTwoTask = {
			...createMockTask({ id: 'TASK-P2', title: 'Project Two Task', status: TaskStatus.BLOCKED }),
			projectId: 'P2',
		};

		useTaskStore.setState({
			tasks: [projectOneTask, projectTwoTask],
		});

		renderPalette();

		expect(screen.getByRole('option', { name: /^resume task-p1/i })).toBeInTheDocument();
		expect(screen.queryByRole('option', { name: /^resume task-p2/i })).not.toBeInTheDocument();

		act(() => {
			useProjectStore.setState({ currentProjectId: 'P2' });
		});

		await waitFor(() => {
			expect(screen.queryByRole('option', { name: /^resume task-p1/i })).not.toBeInTheDocument();
			expect(screen.getByRole('option', { name: /^resume task-p2/i })).toBeInTheDocument();
		});
	});

	it('opens active threads in the current project and ignores archived ones', async () => {
		const selectThread = vi.fn();
		const openThread = {
			...createMockThread({ id: 'thread-open', title: 'Design Discussion', status: 'active' }),
			projectId: 'P1',
		};
		const otherProjectThread = {
			...createMockThread({ id: 'thread-other-project', title: 'Other Project Thread', status: 'active' }),
			projectId: 'P2',
		};
		const archivedThread = {
			...createMockThread({ id: 'thread-archived', title: 'Old Thread', status: 'archived' }),
			projectId: 'P1',
		};

		useThreadStore.setState({
			threads: [openThread, otherProjectThread, archivedThread],
			selectThread,
		});

		renderPalette();

		expect(screen.getByRole('option', { name: /open thread: design discussion/i })).toBeInTheDocument();
		expect(screen.queryByRole('option', { name: /open thread: other project thread/i })).not.toBeInTheDocument();
		expect(screen.queryByRole('option', { name: /open thread: old thread/i })).not.toBeInTheDocument();

		await userEvent.click(screen.getByRole('option', { name: /open thread: design discussion/i }));
		expect(selectThread).toHaveBeenCalledWith('thread-open');
	});

	it('closes on Escape without letting other Escape handlers fire', () => {
		const onClose = vi.fn();
		const externalEscapeHandler = vi.fn();
		document.addEventListener('keydown', externalEscapeHandler);

		render(
			<MemoryRouter>
				<CommandPalette open={true} onClose={onClose} />
			</MemoryRouter>
		);

		fireEvent.keyDown(screen.getByLabelText('Search commands'), { key: 'Escape' });

		expect(onClose).toHaveBeenCalledTimes(1);
		expect(externalEscapeHandler).not.toHaveBeenCalled();

		document.removeEventListener('keydown', externalEscapeHandler);
	});
});
