import { describe, it, expect, beforeEach } from 'vitest';
import { useTaskStore } from './taskStore';
import type { Task, TaskState } from '@/lib/types';

// Factory for creating test tasks
function createTask(overrides: Partial<Task> = {}): Task {
	return {
		id: `TASK-${Math.random().toString(36).slice(2, 7)}`,
		title: 'Test Task',
		weight: 'medium',
		status: 'planned',
		branch: 'main',
		created_at: new Date().toISOString(),
		updated_at: new Date().toISOString(),
		...overrides,
	};
}

// Factory for creating test task states
function createTaskState(taskId: string, overrides: Partial<TaskState> = {}): TaskState {
	return {
		task_id: taskId,
		current_phase: 'implement',
		current_iteration: 1,
		status: 'running',
		started_at: new Date().toISOString(),
		updated_at: new Date().toISOString(),
		phases: {},
		gates: [],
		tokens: { input_tokens: 0, output_tokens: 0, total_tokens: 0 },
		...overrides,
	};
}

describe('TaskStore', () => {
	beforeEach(() => {
		// Reset store before each test
		useTaskStore.getState().reset();
	});

	describe('setTasks', () => {
		it('should set tasks array', () => {
			const tasks = [createTask({ id: 'TASK-001' }), createTask({ id: 'TASK-002' })];

			useTaskStore.getState().setTasks(tasks);

			expect(useTaskStore.getState().tasks).toHaveLength(2);
			expect(useTaskStore.getState().tasks[0].id).toBe('TASK-001');
		});

		it('should clear error when setting tasks', () => {
			useTaskStore.getState().setError('Some error');
			useTaskStore.getState().setTasks([]);

			expect(useTaskStore.getState().error).toBeNull();
		});
	});

	describe('addTask', () => {
		it('should add a task to the array', () => {
			const task = createTask({ id: 'TASK-001' });

			useTaskStore.getState().addTask(task);

			expect(useTaskStore.getState().tasks).toHaveLength(1);
			expect(useTaskStore.getState().tasks[0].id).toBe('TASK-001');
		});

		it('should prevent duplicate tasks', () => {
			const task = createTask({ id: 'TASK-001' });

			useTaskStore.getState().addTask(task);
			useTaskStore.getState().addTask(task);

			expect(useTaskStore.getState().tasks).toHaveLength(1);
		});
	});

	describe('updateTask', () => {
		it('should update task properties', () => {
			const task = createTask({ id: 'TASK-001', title: 'Original Title' });
			useTaskStore.getState().setTasks([task]);

			useTaskStore.getState().updateTask('TASK-001', { title: 'Updated Title' });

			expect(useTaskStore.getState().tasks[0].title).toBe('Updated Title');
		});

		it('should not affect other tasks', () => {
			const tasks = [
				createTask({ id: 'TASK-001', title: 'Task 1' }),
				createTask({ id: 'TASK-002', title: 'Task 2' }),
			];
			useTaskStore.getState().setTasks(tasks);

			useTaskStore.getState().updateTask('TASK-001', { title: 'Updated Task 1' });

			expect(useTaskStore.getState().tasks[0].title).toBe('Updated Task 1');
			expect(useTaskStore.getState().tasks[1].title).toBe('Task 2');
		});
	});

	describe('updateTaskStatus', () => {
		it('should update task status', () => {
			const task = createTask({ id: 'TASK-001', status: 'planned' });
			useTaskStore.getState().setTasks([task]);

			useTaskStore.getState().updateTaskStatus('TASK-001', 'running');

			expect(useTaskStore.getState().tasks[0].status).toBe('running');
		});

		it('should update current_phase when provided', () => {
			const task = createTask({ id: 'TASK-001', status: 'planned' });
			useTaskStore.getState().setTasks([task]);

			useTaskStore.getState().updateTaskStatus('TASK-001', 'running', 'implement');

			expect(useTaskStore.getState().tasks[0].status).toBe('running');
			expect(useTaskStore.getState().tasks[0].current_phase).toBe('implement');
		});
	});

	describe('removeTask', () => {
		it('should remove task from tasks array', () => {
			const tasks = [
				createTask({ id: 'TASK-001' }),
				createTask({ id: 'TASK-002' }),
			];
			useTaskStore.getState().setTasks(tasks);

			useTaskStore.getState().removeTask('TASK-001');

			expect(useTaskStore.getState().tasks).toHaveLength(1);
			expect(useTaskStore.getState().tasks[0].id).toBe('TASK-002');
		});

		it('should also remove associated task state', () => {
			const task = createTask({ id: 'TASK-001' });
			const taskState = createTaskState('TASK-001');
			useTaskStore.getState().setTasks([task]);
			useTaskStore.getState().updateTaskState('TASK-001', taskState);

			useTaskStore.getState().removeTask('TASK-001');

			expect(useTaskStore.getState().taskStates.has('TASK-001')).toBe(false);
		});
	});

	describe('updateTaskState', () => {
		it('should add task state to map', () => {
			const task = createTask({ id: 'TASK-001', status: 'planned' });
			const taskState = createTaskState('TASK-001', { status: 'running' });
			useTaskStore.getState().setTasks([task]);

			useTaskStore.getState().updateTaskState('TASK-001', taskState);

			expect(useTaskStore.getState().taskStates.get('TASK-001')).toEqual(taskState);
		});

		it('should sync status to task if task exists', () => {
			const task = createTask({ id: 'TASK-001', status: 'planned' });
			const taskState = createTaskState('TASK-001', {
				status: 'running',
				current_phase: 'test',
			});
			useTaskStore.getState().setTasks([task]);

			useTaskStore.getState().updateTaskState('TASK-001', taskState);

			expect(useTaskStore.getState().tasks[0].status).toBe('running');
			expect(useTaskStore.getState().tasks[0].current_phase).toBe('test');
		});
	});

	describe('removeTaskState', () => {
		it('should remove task state from map', () => {
			const taskState = createTaskState('TASK-001');
			useTaskStore.getState().updateTaskState('TASK-001', taskState);

			useTaskStore.getState().removeTaskState('TASK-001');

			expect(useTaskStore.getState().taskStates.has('TASK-001')).toBe(false);
		});
	});

	describe('getTask', () => {
		it('should return task by ID', () => {
			const task = createTask({ id: 'TASK-001', title: 'My Task' });
			useTaskStore.getState().setTasks([task]);

			const result = useTaskStore.getState().getTask('TASK-001');

			expect(result?.title).toBe('My Task');
		});

		it('should return undefined for non-existent task', () => {
			const result = useTaskStore.getState().getTask('TASK-999');

			expect(result).toBeUndefined();
		});
	});

	describe('getTaskState', () => {
		it('should return task state by ID', () => {
			const taskState = createTaskState('TASK-001', { current_phase: 'test' });
			useTaskStore.getState().updateTaskState('TASK-001', taskState);

			const result = useTaskStore.getState().getTaskState('TASK-001');

			expect(result?.current_phase).toBe('test');
		});

		it('should return undefined for non-existent task state', () => {
			const result = useTaskStore.getState().getTaskState('TASK-999');

			expect(result).toBeUndefined();
		});
	});

	describe('derived state: getActiveTasks', () => {
		it('should return tasks with active statuses', () => {
			const tasks = [
				createTask({ id: 'TASK-001', status: 'running' }),
				createTask({ id: 'TASK-002', status: 'blocked' }),
				createTask({ id: 'TASK-003', status: 'paused' }),
				createTask({ id: 'TASK-004', status: 'completed' }),
				createTask({ id: 'TASK-005', status: 'planned' }),
			];
			useTaskStore.getState().setTasks(tasks);

			const activeTasks = useTaskStore.getState().getActiveTasks();

			expect(activeTasks).toHaveLength(3);
			expect(activeTasks.map((t) => t.id)).toEqual(['TASK-001', 'TASK-002', 'TASK-003']);
		});
	});

	describe('derived state: getRecentTasks', () => {
		it('should return completed/failed/finished tasks sorted by updated_at', () => {
			const now = Date.now();
			const tasks = [
				createTask({
					id: 'TASK-001',
					status: 'completed',
					updated_at: new Date(now - 1000).toISOString(),
				}),
				createTask({
					id: 'TASK-002',
					status: 'failed',
					updated_at: new Date(now - 2000).toISOString(),
				}),
				createTask({
					id: 'TASK-003',
					status: 'finished',
					updated_at: new Date(now).toISOString(),
				}),
				createTask({ id: 'TASK-004', status: 'running' }),
			];
			useTaskStore.getState().setTasks(tasks);

			const recentTasks = useTaskStore.getState().getRecentTasks();

			expect(recentTasks).toHaveLength(3);
			// Most recent first
			expect(recentTasks[0].id).toBe('TASK-003');
			expect(recentTasks[1].id).toBe('TASK-001');
			expect(recentTasks[2].id).toBe('TASK-002');
		});

		it('should limit to 10 tasks', () => {
			const tasks = Array.from({ length: 15 }, (_, i) =>
				createTask({
					id: `TASK-${String(i + 1).padStart(3, '0')}`,
					status: 'completed',
					updated_at: new Date(Date.now() - i * 1000).toISOString(),
				})
			);
			useTaskStore.getState().setTasks(tasks);

			const recentTasks = useTaskStore.getState().getRecentTasks();

			expect(recentTasks).toHaveLength(10);
		});
	});

	describe('derived state: getRunningTasks', () => {
		it('should return only running tasks', () => {
			const tasks = [
				createTask({ id: 'TASK-001', status: 'running' }),
				createTask({ id: 'TASK-002', status: 'running' }),
				createTask({ id: 'TASK-003', status: 'blocked' }),
				createTask({ id: 'TASK-004', status: 'completed' }),
			];
			useTaskStore.getState().setTasks(tasks);

			const runningTasks = useTaskStore.getState().getRunningTasks();

			expect(runningTasks).toHaveLength(2);
			expect(runningTasks.map((t) => t.id)).toEqual(['TASK-001', 'TASK-002']);
		});
	});

	describe('derived state: getStatusCounts', () => {
		it('should count tasks by status', () => {
			const tasks = [
				createTask({ status: 'running' }),
				createTask({ status: 'running' }),
				createTask({ status: 'blocked' }),
				createTask({ status: 'completed' }),
				createTask({ status: 'finished' }),
				createTask({ status: 'failed' }),
				createTask({ status: 'paused' }),
			];
			useTaskStore.getState().setTasks(tasks);

			const counts = useTaskStore.getState().getStatusCounts();

			expect(counts.all).toBe(7);
			expect(counts.running).toBe(2);
			expect(counts.blocked).toBe(1);
			expect(counts.completed).toBe(2); // completed + finished
			expect(counts.failed).toBe(1);
			expect(counts.active).toBe(4); // running + blocked + paused
		});

		it('should return zeros when no tasks', () => {
			const counts = useTaskStore.getState().getStatusCounts();

			expect(counts.all).toBe(0);
			expect(counts.running).toBe(0);
			expect(counts.active).toBe(0);
		});
	});

	describe('loading and error states', () => {
		it('should set loading state', () => {
			useTaskStore.getState().setLoading(true);
			expect(useTaskStore.getState().loading).toBe(true);

			useTaskStore.getState().setLoading(false);
			expect(useTaskStore.getState().loading).toBe(false);
		});

		it('should set error state', () => {
			useTaskStore.getState().setError('Something went wrong');
			expect(useTaskStore.getState().error).toBe('Something went wrong');

			useTaskStore.getState().setError(null);
			expect(useTaskStore.getState().error).toBeNull();
		});
	});

	describe('reset', () => {
		it('should reset store to initial state', () => {
			const task = createTask({ id: 'TASK-001' });
			const taskState = createTaskState('TASK-001');
			useTaskStore.getState().setTasks([task]);
			useTaskStore.getState().updateTaskState('TASK-001', taskState);
			useTaskStore.getState().setLoading(true);
			useTaskStore.getState().setError('error');

			useTaskStore.getState().reset();

			expect(useTaskStore.getState().tasks).toHaveLength(0);
			expect(useTaskStore.getState().taskStates.size).toBe(0);
			expect(useTaskStore.getState().loading).toBe(false);
			expect(useTaskStore.getState().error).toBeNull();
		});
	});
});
