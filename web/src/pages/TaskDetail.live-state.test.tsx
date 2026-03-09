import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { TooltipProvider } from '@/components/ui/Tooltip';
import { useProjectStore, useTaskStore } from '@/stores';
import { TaskDetail } from './TaskDetail';
import { TaskStatus } from '@/gen/orc/v1/task_pb';
import { createMockTask } from '@/test/factories';

const mockGetTask = vi.fn();
const mockGetTaskPlan = vi.fn();
const mockListTaskGeneratedNotes = vi.fn();
const mockTranscriptTab = vi.fn();
const mockTaskFooter = vi.fn();

vi.mock('@/lib/client', () => ({
	taskClient: {
		getTask: (...args: unknown[]) => mockGetTask(...args),
		getTaskPlan: (...args: unknown[]) => mockGetTaskPlan(...args),
	},
	initiativeClient: {
		listTaskGeneratedNotes: (...args: unknown[]) => mockListTaskGeneratedNotes(...args),
	},
}));

vi.mock('@/hooks', () => ({
	useTaskSubscription: vi.fn(() => ({
		state: undefined,
		transcript: [],
	})),
	useDocumentTitle: vi.fn(),
}));

vi.mock('@/components/task-detail/TranscriptTab', () => ({
	TranscriptTab: (props: unknown) => {
		mockTranscriptTab(props);
		return <div data-testid="transcript-tab" />;
	},
}));

vi.mock('@/components/task-detail/TaskFooter', () => ({
	TaskFooter: (props: unknown) => {
		mockTaskFooter(props);
		return <div data-testid="task-footer" />;
	},
}));

vi.mock('@/components/task-detail/ChangesTabEnhanced', () => ({
	ChangesTabEnhanced: () => <div data-testid="changes-tab" />,
}));

vi.mock('@/components/task-detail/FeedbackPanel', () => ({
	FeedbackPanel: () => <div data-testid="feedback-panel" />,
}));

function renderTaskDetail(taskId: string = 'TASK-001') {
	return render(
		<TooltipProvider delayDuration={0}>
			<MemoryRouter initialEntries={[`/tasks/${taskId}`]}>
				<Routes>
					<Route path="/tasks/:id" element={<TaskDetail />} />
					<Route path="/board" element={<div>Board</div>} />
				</Routes>
			</MemoryRouter>
		</TooltipProvider>
	);
}

describe('TaskDetail live state wiring', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		useTaskStore.getState().reset();
		useProjectStore.setState({ currentProjectId: 'test-project' });
		mockListTaskGeneratedNotes.mockResolvedValue({ notes: [] });
		mockGetTaskPlan.mockResolvedValue({ plan: null });
	});

	it('passes running state through to TranscriptTab', async () => {
		mockGetTask.mockResolvedValue({
			task: createMockTask({
				id: 'TASK-001',
				status: TaskStatus.RUNNING,
				currentPhase: 'implement_codex',
			}),
		});

		renderTaskDetail();

		await waitFor(() => {
			expect(screen.getByRole('heading', { name: /test task/i })).toBeInTheDocument();
		});

		expect(mockTranscriptTab).toHaveBeenCalled();
		expect(mockTranscriptTab.mock.calls[0][0]).toEqual(
			expect.objectContaining({ isRunning: true })
		);
	});

	it('passes live task session metrics to TaskFooter', async () => {
		mockGetTask.mockResolvedValue({
			task: createMockTask({
				id: 'TASK-001',
				status: TaskStatus.RUNNING,
			}),
		});

		useTaskStore.getState().updateSessionMetrics('TASK-001', {
			totalTokens: 45200,
			estimatedCostUSD: 1.96,
			inputTokens: 30000,
			outputTokens: 15200,
			durationSeconds: 120,
			tasksRunning: 1,
		});

		renderTaskDetail();

		await waitFor(() => {
			expect(screen.getByRole('heading', { name: /test task/i })).toBeInTheDocument();
		});

		expect(mockTaskFooter).toHaveBeenCalled();
		expect(mockTaskFooter.mock.calls[0][0]).toEqual(
			expect.objectContaining({
				metrics: expect.objectContaining({
					tokens: 45200,
					cost: 1.96,
					inputTokens: 30000,
					outputTokens: 15200,
				}),
			})
		);
	});
});
