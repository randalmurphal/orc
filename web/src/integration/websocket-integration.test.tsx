/**
 * Integration tests for WebSocket event handling and store synchronization.
 *
 * Tests verify:
 * - WebSocket events correctly update stores
 * - Initiative events are handled
 * - Task events update task store
 * - Cross-component state stays synchronized
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import App from '@/App';
import {
	useProjectStore,
	useTaskStore,
	useInitiativeStore,
} from '@/stores';
import type { Task, Initiative, TaskState } from '@/lib/types';

// Store mock handler for WebSocket events
let mockWsEventHandler: ((event: unknown) => void) | null = null;

// Mock WebSocket to capture event handlers
vi.mock('@/lib/websocket', () => ({
	OrcWebSocket: vi.fn().mockImplementation(() => ({
		connect: vi.fn(),
		disconnect: vi.fn(),
		subscribe: vi.fn(),
		unsubscribe: vi.fn(),
		subscribeGlobal: vi.fn(),
		setPrimarySubscription: vi.fn(),
		on: vi.fn((eventType: string, callback: (event: unknown) => void) => {
			// Capture the 'all' event handler
			if (eventType === 'all' || eventType === '*') {
				mockWsEventHandler = callback;
			}
			return () => {
				if (eventType === 'all' || eventType === '*') {
					mockWsEventHandler = null;
				}
			};
		}),
		onStatusChange: vi.fn((callback: (status: string) => void) => {
			// Call immediately with connected status
			callback('connected');
			return () => {};
		}),
		isConnected: vi.fn().mockReturnValue(true),
		getTaskId: vi.fn().mockReturnValue('*'),
		command: vi.fn(),
	})),
	GLOBAL_TASK_ID: '*',
}));

// Mock API calls
vi.mock('@/lib/api', () => ({
	listProjects: vi.fn().mockResolvedValue([
		{
			id: 'project-1',
			name: 'Test Project',
			path: '/path/to/project',
			created_at: new Date().toISOString(),
		},
	]),
	listProjectTasks: vi.fn().mockResolvedValue([
		{
			id: 'TASK-001',
			title: 'Test Task 1',
			status: 'created',
			weight: 'small',
			branch: 'orc/TASK-001',
			created_at: new Date().toISOString(),
			updated_at: new Date().toISOString(),
		},
	]),
	listInitiatives: vi.fn().mockResolvedValue([
		{
			id: 'INIT-001',
			title: 'Test Initiative',
			status: 'active',
			version: 1,
			created_at: new Date().toISOString(),
			updated_at: new Date().toISOString(),
		},
	]),
	getDashboardStats: vi.fn().mockResolvedValue({
		running: 0,
		paused: 0,
		blocked: 0,
		completed: 0,
		failed: 0,
		today: 0,
		total: 1,
		tokens: 0,
		cost: 0,
	}),
	runProjectTask: vi.fn(),
	pauseProjectTask: vi.fn(),
	resumeProjectTask: vi.fn(),
	escalateProjectTask: vi.fn(),
	updateTask: vi.fn(),
	deleteProjectTask: vi.fn(),
}));

// Helper to simulate WebSocket event
function simulateWsEvent(
	eventType: string,
	taskId: string,
	data: unknown
): void {
	if (mockWsEventHandler) {
		mockWsEventHandler({
			type: 'event',
			event: eventType,
			task_id: taskId,
			data,
			time: new Date().toISOString(),
		});
	}
}

function renderApp(initialPath: string = '/') {
	return render(
		<MemoryRouter initialEntries={[initialPath]}>
			<App />
		</MemoryRouter>
	);
}

describe('WebSocket Integration', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mockWsEventHandler = null;

		// Reset stores
		useProjectStore.setState({
			projects: [],
			currentProjectId: null,
			loading: false,
			error: null,
			_isHandlingPopState: false,
		});
		useTaskStore.setState({
			tasks: [],
			taskStates: new Map(),
			loading: false,
			error: null,
		});
		useInitiativeStore.setState({
			initiatives: new Map(),
			currentInitiativeId: null,
			loading: false,
			error: null,
			hasLoaded: false,
			_isHandlingPopState: false,
		});
	});

	afterEach(() => {
		vi.clearAllMocks();
	});

	describe('Task Events', () => {
		it('handles task_created event by adding task to store', async () => {
			renderApp('/');

			await waitFor(() => {
				expect(mockWsEventHandler).not.toBeNull();
			});

			const newTask: Task = {
				id: 'TASK-002',
				title: 'New Task via WebSocket',
				status: 'created',
				weight: 'medium',
				branch: 'orc/TASK-002',
				created_at: new Date().toISOString(),
				updated_at: new Date().toISOString(),
			};

			await act(async () => {
				simulateWsEvent('task_created', 'TASK-002', newTask);
			});

			// Verify task was added to store
			const tasks = useTaskStore.getState().tasks;
			expect(tasks.find((t) => t.id === 'TASK-002')).toBeDefined();
		});

		it('handles task_updated event by updating task in store', async () => {
			// Set up initial task
			useTaskStore.setState({
				tasks: [
					{
						id: 'TASK-001',
						title: 'Original Title',
						status: 'created',
						weight: 'small',
						branch: 'orc/TASK-001',
						created_at: new Date().toISOString(),
						updated_at: new Date().toISOString(),
					},
				],
				taskStates: new Map(),
				loading: false,
				error: null,
			});

			renderApp('/');

			await waitFor(() => {
				expect(mockWsEventHandler).not.toBeNull();
			});

			const updatedTask: Partial<Task> = {
				id: 'TASK-001',
				title: 'Updated Title',
				status: 'running',
			};

			await act(async () => {
				simulateWsEvent('task_updated', 'TASK-001', updatedTask);
			});

			// Verify task was updated
			const tasks = useTaskStore.getState().tasks;
			const task = tasks.find((t) => t.id === 'TASK-001');
			expect(task?.title).toBe('Updated Title');
			expect(task?.status).toBe('running');
		});

		it('handles task_deleted event by removing task from store', async () => {
			// Set up initial task
			useTaskStore.setState({
				tasks: [
					{
						id: 'TASK-001',
						title: 'Task to Delete',
						status: 'created',
						weight: 'small',
						branch: 'orc/TASK-001',
						created_at: new Date().toISOString(),
						updated_at: new Date().toISOString(),
					},
				],
				taskStates: new Map(),
				loading: false,
				error: null,
			});

			renderApp('/');

			await waitFor(() => {
				expect(mockWsEventHandler).not.toBeNull();
			});

			await act(async () => {
				simulateWsEvent('task_deleted', 'TASK-001', {});
			});

			// Verify task was removed
			const tasks = useTaskStore.getState().tasks;
			expect(tasks.find((t) => t.id === 'TASK-001')).toBeUndefined();
		});

		it('handles state event by updating task state', async () => {
			renderApp('/');

			await waitFor(() => {
				expect(mockWsEventHandler).not.toBeNull();
			});

			const taskState: TaskState = {
				task_id: 'TASK-001',
				current_phase: 'implement',
				current_iteration: 1,
				status: 'running',
				started_at: new Date().toISOString(),
				updated_at: new Date().toISOString(),
				phases: {},
				gates: [],
				tokens: {
					input_tokens: 1000,
					output_tokens: 500,
					total_tokens: 1500,
				},
			};

			await act(async () => {
				simulateWsEvent('state', 'TASK-001', taskState);
			});

			// Verify task state was updated
			const state = useTaskStore.getState().taskStates.get('TASK-001');
			expect(state?.current_phase).toBe('implement');
			expect(state?.status).toBe('running');
		});

		it('handles complete event by updating task status', async () => {
			// Set up initial task
			useTaskStore.setState({
				tasks: [
					{
						id: 'TASK-001',
						title: 'Running Task',
						status: 'running',
						weight: 'small',
						branch: 'orc/TASK-001',
						created_at: new Date().toISOString(),
						updated_at: new Date().toISOString(),
					},
				],
				taskStates: new Map(),
				loading: false,
				error: null,
			});

			renderApp('/');

			await waitFor(() => {
				expect(mockWsEventHandler).not.toBeNull();
			});

			await act(async () => {
				simulateWsEvent('complete', 'TASK-001', { status: 'completed' });
			});

			// Verify task status was updated
			const tasks = useTaskStore.getState().tasks;
			const task = tasks.find((t) => t.id === 'TASK-001');
			expect(task?.status).toBe('completed');
		});
	});

	describe('Initiative Events', () => {
		it('handles initiative_created event by adding initiative to store', async () => {
			renderApp('/');

			await waitFor(() => {
				expect(mockWsEventHandler).not.toBeNull();
			});

			const newInitiative: Initiative = {
				id: 'INIT-002',
				title: 'New Initiative via WebSocket',
				status: 'active',
				version: 1,
				created_at: new Date().toISOString(),
				updated_at: new Date().toISOString(),
			};

			await act(async () => {
				simulateWsEvent('initiative_created', 'INIT-002', newInitiative);
			});

			// Verify initiative was added
			const initiatives = useInitiativeStore.getState().initiatives;
			expect(initiatives.get('INIT-002')).toBeDefined();
			expect(initiatives.get('INIT-002')?.title).toBe(
				'New Initiative via WebSocket'
			);
		});

		it('handles initiative_updated event by updating initiative in store', async () => {
			// Set up initial initiative
			const initMap = new Map<string, Initiative>();
			initMap.set('INIT-001', {
				id: 'INIT-001',
				title: 'Original Initiative',
				status: 'active',
				version: 1,
				created_at: new Date().toISOString(),
				updated_at: new Date().toISOString(),
			});
			useInitiativeStore.setState({
				initiatives: initMap,
				currentInitiativeId: null,
				loading: false,
				error: null,
				hasLoaded: true,
				_isHandlingPopState: false,
			});

			renderApp('/');

			await waitFor(() => {
				expect(mockWsEventHandler).not.toBeNull();
			});

			const updatedInitiative: Initiative = {
				id: 'INIT-001',
				title: 'Updated Initiative Title',
				status: 'completed',
				version: 2,
				created_at: new Date().toISOString(),
				updated_at: new Date().toISOString(),
			};

			await act(async () => {
				simulateWsEvent('initiative_updated', 'INIT-001', updatedInitiative);
			});

			// Verify initiative was updated
			const initiatives = useInitiativeStore.getState().initiatives;
			expect(initiatives.get('INIT-001')?.title).toBe(
				'Updated Initiative Title'
			);
			expect(initiatives.get('INIT-001')?.status).toBe('completed');
		});

		it('handles initiative_deleted event by removing initiative from store', async () => {
			// Set up initial initiative
			const initMap = new Map<string, Initiative>();
			initMap.set('INIT-001', {
				id: 'INIT-001',
				title: 'Initiative to Delete',
				status: 'active',
				version: 1,
				created_at: new Date().toISOString(),
				updated_at: new Date().toISOString(),
			});
			useInitiativeStore.setState({
				initiatives: initMap,
				currentInitiativeId: null,
				loading: false,
				error: null,
				hasLoaded: true,
				_isHandlingPopState: false,
			});

			renderApp('/');

			await waitFor(() => {
				expect(mockWsEventHandler).not.toBeNull();
			});

			await act(async () => {
				simulateWsEvent('initiative_deleted', 'INIT-001', {});
			});

			// Verify initiative was removed
			const initiatives = useInitiativeStore.getState().initiatives;
			expect(initiatives.has('INIT-001')).toBe(false);
		});
	});

	describe('Finalize Events', () => {
		it('handles finalize event with running status', async () => {
			// Set up initial task
			useTaskStore.setState({
				tasks: [
					{
						id: 'TASK-001',
						title: 'Completed Task',
						status: 'completed',
						weight: 'small',
						branch: 'orc/TASK-001',
						created_at: new Date().toISOString(),
						updated_at: new Date().toISOString(),
					},
				],
				taskStates: new Map(),
				loading: false,
				error: null,
			});

			renderApp('/');

			await waitFor(() => {
				expect(mockWsEventHandler).not.toBeNull();
			});

			await act(async () => {
				simulateWsEvent('finalize', 'TASK-001', {
					step: 'syncing',
					status: 'running',
					progress: 25,
				});
			});

			// Verify task status changed to finalizing
			const tasks = useTaskStore.getState().tasks;
			const task = tasks.find((t) => t.id === 'TASK-001');
			expect(task?.status).toBe('finalizing');
		});

		it('handles finalize event with completed status', async () => {
			// Set up initial task in finalizing state
			useTaskStore.setState({
				tasks: [
					{
						id: 'TASK-001',
						title: 'Finalizing Task',
						status: 'finalizing',
						weight: 'small',
						branch: 'orc/TASK-001',
						created_at: new Date().toISOString(),
						updated_at: new Date().toISOString(),
					},
				],
				taskStates: new Map(),
				loading: false,
				error: null,
			});

			renderApp('/');

			await waitFor(() => {
				expect(mockWsEventHandler).not.toBeNull();
			});

			await act(async () => {
				simulateWsEvent('finalize', 'TASK-001', {
					step: 'done',
					status: 'completed',
					progress: 100,
				});
			});

			// Verify task status changed to completed
			const tasks = useTaskStore.getState().tasks;
			const task = tasks.find((t) => t.id === 'TASK-001');
			expect(task?.status).toBe('completed');
		});

		it('handles finalize event with failed status', async () => {
			// Set up initial task in finalizing state
			useTaskStore.setState({
				tasks: [
					{
						id: 'TASK-001',
						title: 'Finalizing Task',
						status: 'finalizing',
						weight: 'small',
						branch: 'orc/TASK-001',
						created_at: new Date().toISOString(),
						updated_at: new Date().toISOString(),
					},
				],
				taskStates: new Map(),
				loading: false,
				error: null,
			});

			renderApp('/');

			await waitFor(() => {
				expect(mockWsEventHandler).not.toBeNull();
			});

			await act(async () => {
				simulateWsEvent('finalize', 'TASK-001', {
					step: 'syncing',
					status: 'failed',
					error: 'Merge conflict',
				});
			});

			// Verify task status changed to failed
			const tasks = useTaskStore.getState().tasks;
			const task = tasks.find((t) => t.id === 'TASK-001');
			expect(task?.status).toBe('failed');
		});
	});

	describe('Phase Events', () => {
		it('handles phase event by updating task current_phase', async () => {
			// Set up initial task
			useTaskStore.setState({
				tasks: [
					{
						id: 'TASK-001',
						title: 'Running Task',
						status: 'running',
						current_phase: 'spec',
						weight: 'large',
						branch: 'orc/TASK-001',
						created_at: new Date().toISOString(),
						updated_at: new Date().toISOString(),
					},
				],
				taskStates: new Map(),
				loading: false,
				error: null,
			});

			renderApp('/');

			await waitFor(() => {
				expect(mockWsEventHandler).not.toBeNull();
			});

			await act(async () => {
				simulateWsEvent('phase', 'TASK-001', {
					phase: 'implement',
					status: 'started',
				});
			});

			// Verify task current_phase was updated
			const tasks = useTaskStore.getState().tasks;
			const task = tasks.find((t) => t.id === 'TASK-001');
			expect(task?.current_phase).toBe('implement');
		});
	});
});
