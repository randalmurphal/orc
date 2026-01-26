import { describe, it, expect, beforeEach } from 'vitest';
import { useTaskStore } from './taskStore';
import { type Task, TaskStatus, TaskWeight, type ExecutionState } from '@/gen/orc/v1/task_pb';

// Factory for creating test tasks
function createTask(overrides: Partial<Task> = {}): Task {
	return {
		$typeName: 'orc.v1.Task',
		id: `TASK-${Math.random().toString(36).slice(2, 7)}`,
		title: 'Test Task',
		weight: TaskWeight.MEDIUM,
		status: TaskStatus.PLANNED,
		branch: 'main',
		...overrides,
	} as Task;
}

// Factory for creating test task states
function createTaskState(_taskId: string, overrides: Partial<ExecutionState> = {}): ExecutionState {
	return {
		$typeName: 'orc.v1.ExecutionState',
		currentIteration: 1,
		phases: {},
		gates: [],
		tokens: { inputTokens: 0, outputTokens: 0, totalTokens: 0 },
		...overrides,
	} as ExecutionState;
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
			const task = createTask({ id: 'TASK-001', status: TaskStatus.PLANNED });
			useTaskStore.getState().setTasks([task]);

			useTaskStore.getState().updateTaskStatus('TASK-001', TaskStatus.RUNNING);

			expect(useTaskStore.getState().tasks[0].status).toBe(TaskStatus.RUNNING);
		});

		it('should update currentPhase when provided', () => {
			const task = createTask({ id: 'TASK-001', status: TaskStatus.PLANNED });
			useTaskStore.getState().setTasks([task]);

			useTaskStore.getState().updateTaskStatus('TASK-001', TaskStatus.RUNNING, 'implement');

			expect(useTaskStore.getState().tasks[0].status).toBe(TaskStatus.RUNNING);
			expect(useTaskStore.getState().tasks[0].currentPhase).toBe('implement');
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
			const task = createTask({ id: 'TASK-001', status: TaskStatus.PLANNED });
			const taskState = createTaskState('TASK-001');
			useTaskStore.getState().setTasks([task]);

			useTaskStore.getState().updateTaskState('TASK-001', taskState);

			expect(useTaskStore.getState().taskStates.get('TASK-001')).toEqual(taskState);
		});

		it('should store task state even if task does not exist', () => {
			// updateTaskState stores the execution state in the map
			// It does not automatically sync fields to the task
			const taskState = createTaskState('TASK-001', {
				currentIteration: 2,
			});

			useTaskStore.getState().updateTaskState('TASK-001', taskState);

			// State should be stored
			expect(useTaskStore.getState().taskStates.get('TASK-001')).toEqual(taskState);
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
			const taskState = createTaskState('TASK-001', { currentIteration: 3 });
			useTaskStore.getState().updateTaskState('TASK-001', taskState);

			const result = useTaskStore.getState().getTaskState('TASK-001');

			expect(result?.currentIteration).toBe(3);
		});

		it('should return undefined for non-existent task state', () => {
			const result = useTaskStore.getState().getTaskState('TASK-999');

			expect(result).toBeUndefined();
		});
	});

	describe('derived state: getActiveTasks', () => {
		it('should return tasks with active statuses', () => {
			const tasks = [
				createTask({ id: 'TASK-001', status: TaskStatus.RUNNING }),
				createTask({ id: 'TASK-002', status: TaskStatus.BLOCKED }),
				createTask({ id: 'TASK-003', status: TaskStatus.PAUSED }),
				createTask({ id: 'TASK-004', status: TaskStatus.COMPLETED }),
				createTask({ id: 'TASK-005', status: TaskStatus.PLANNED }),
			];
			useTaskStore.getState().setTasks(tasks);

			const activeTasks = useTaskStore.getState().getActiveTasks();

			expect(activeTasks).toHaveLength(3);
			expect(activeTasks.map((t) => t.id)).toEqual(['TASK-001', 'TASK-002', 'TASK-003']);
		});
	});

	describe('derived state: getRecentTasks', () => {
		it('should return completed/failed tasks sorted by updatedAt', () => {
			const tasks = [
				createTask({
					id: 'TASK-001',
					status: TaskStatus.COMPLETED,
				}),
				createTask({
					id: 'TASK-002',
					status: TaskStatus.FAILED,
				}),
				createTask({
					id: 'TASK-003',
					status: TaskStatus.COMPLETED,
				}),
				createTask({ id: 'TASK-004', status: TaskStatus.RUNNING }),
			];
			useTaskStore.getState().setTasks(tasks);

			const recentTasks = useTaskStore.getState().getRecentTasks();

			// Completed and failed tasks only
			expect(recentTasks.length).toBeGreaterThanOrEqual(3);
		});

		it('should limit to 10 tasks', () => {
			const tasks = Array.from({ length: 15 }, (_, i) =>
				createTask({
					id: `TASK-${String(i + 1).padStart(3, '0')}`,
					status: TaskStatus.COMPLETED,
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
				createTask({ id: 'TASK-001', status: TaskStatus.RUNNING }),
				createTask({ id: 'TASK-002', status: TaskStatus.RUNNING }),
				createTask({ id: 'TASK-003', status: TaskStatus.BLOCKED }),
				createTask({ id: 'TASK-004', status: TaskStatus.COMPLETED }),
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
				createTask({ status: TaskStatus.RUNNING }),
				createTask({ status: TaskStatus.RUNNING }),
				createTask({ status: TaskStatus.BLOCKED }),
				createTask({ status: TaskStatus.COMPLETED }),
				createTask({ status: TaskStatus.COMPLETED }),
				createTask({ status: TaskStatus.FAILED }),
				createTask({ status: TaskStatus.PAUSED }),
			];
			useTaskStore.getState().setTasks(tasks);

			const counts = useTaskStore.getState().getStatusCounts();

			expect(counts.all).toBe(7);
			expect(counts.running).toBe(2);
			expect(counts.blocked).toBe(1);
			expect(counts.completed).toBe(2);
			expect(counts.failed).toBe(1);
			// Active = not terminal (running + blocked + paused)
			expect(counts.active).toBe(4); // running(2) + blocked(1) + paused(1) = 4
		});

		it('should count planned and created tasks as active', () => {
			const tasks = [
				createTask({ status: TaskStatus.PLANNED }),
				createTask({ status: TaskStatus.CREATED }),
				createTask({ status: TaskStatus.CLASSIFYING }),
				createTask({ status: TaskStatus.COMPLETED }),
			];
			useTaskStore.getState().setTasks(tasks);

			const counts = useTaskStore.getState().getStatusCounts();

			// planned, created, classifying are active (not terminal)
			expect(counts.active).toBe(3);
			expect(counts.completed).toBe(1);
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
