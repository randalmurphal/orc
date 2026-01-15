import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
	listTasks,
	getTask,
	createTask,
	getTaskState,
	getTaskPlan,
	runTask,
	pauseTask,
	resumeTask,
	deleteTask,
	getTranscripts,
	listPrompts,
	getPrompt,
	savePrompt,
	deletePrompt,
	listHooks,
	createHook,
	deleteHook,
	listSkills,
	getSkill,
	createSkill,
	deleteSkill,
	getConfig,
	updateConfig,
	listProjects,
	listProjectTasks,
	createProjectTask,
	subscribeToTask
} from './api';
import type { Task, Plan, TaskState, Phase } from './types';

// Mock fetch globally
const mockFetch = vi.fn();
global.fetch = mockFetch;

describe('API Client', () => {
	beforeEach(() => {
		mockFetch.mockReset();
	});

	afterEach(() => {
		vi.clearAllMocks();
	});

	// Helper to create mock response
	function mockResponse<T>(data: T, ok = true, status = 200) {
		return {
			ok,
			status,
			json: () => Promise.resolve(data),
			statusText: ok ? 'OK' : 'Error'
		};
	}

	describe('Tasks', () => {
		const mockTask: Task = {
			id: 'TASK-001',
			title: 'Test task',
			status: 'created',
			weight: 'small',
			branch: 'orc/TASK-001',
			created_at: '2025-01-01T00:00:00Z',
			updated_at: '2025-01-01T00:00:00Z'
		};

		it('listTasks returns array of tasks', async () => {
			mockFetch.mockResolvedValueOnce(mockResponse([mockTask]));
			const tasks = await listTasks();
			expect(tasks).toEqual([mockTask]);
			expect(mockFetch).toHaveBeenCalledWith('/api/tasks', expect.any(Object));
		});

		it('listTasks with pagination returns paginated response', async () => {
			const paginatedResponse = {
				tasks: [mockTask],
				total: 1,
				page: 1,
				limit: 10,
				total_pages: 1
			};
			mockFetch.mockResolvedValueOnce(mockResponse(paginatedResponse));
			const result = await listTasks({ page: 1, limit: 10 });
			expect(result).toEqual(paginatedResponse);
			expect(mockFetch).toHaveBeenCalledWith('/api/tasks?page=1&limit=10', expect.any(Object));
		});

		it('getTask returns single task', async () => {
			mockFetch.mockResolvedValueOnce(mockResponse(mockTask));
			const task = await getTask('TASK-001');
			expect(task).toEqual(mockTask);
			expect(mockFetch).toHaveBeenCalledWith('/api/tasks/TASK-001', expect.any(Object));
		});

		it('createTask sends POST with body', async () => {
			mockFetch.mockResolvedValueOnce(mockResponse(mockTask));
			const task = await createTask('Test task', 'Description', 'small');
			expect(task).toEqual(mockTask);
			expect(mockFetch).toHaveBeenCalledWith('/api/tasks', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ title: 'Test task', description: 'Description', weight: 'small' })
			});
		});

		it('deleteTask handles 204 response', async () => {
			mockFetch.mockResolvedValueOnce({ ok: true, status: 204 });
			await expect(deleteTask('TASK-001')).resolves.toBeUndefined();
			expect(mockFetch).toHaveBeenCalledWith('/api/tasks/TASK-001', { method: 'DELETE' });
		});

		it('deleteTask throws on error', async () => {
			mockFetch.mockResolvedValueOnce(mockResponse({ error: 'Not found' }, false, 404));
			await expect(deleteTask('TASK-999')).rejects.toThrow('Not found');
		});
	});

	describe('Task State and Plan', () => {
		it('getTaskState returns state', async () => {
			const mockState: TaskState = {
				task_id: 'TASK-001',
				current_phase: 'implement',
				current_iteration: 1,
				status: 'running',
				started_at: '2025-01-01T00:00:00Z',
				updated_at: '2025-01-01T00:00:00Z',
				phases: {},
				gates: [],
				tokens: { input_tokens: 0, output_tokens: 0, total_tokens: 0 }
			};
			mockFetch.mockResolvedValueOnce(mockResponse(mockState));
			const state = await getTaskState('TASK-001');
			expect(state).toEqual(mockState);
		});

		it('getTaskPlan returns plan', async () => {
			const mockPhase: Phase = {
				id: 'implement',
				name: 'Implementation',
				status: 'pending',
				iterations: 0
			};
			const mockPlan: Plan = {
				version: 1,
				weight: 'small',
				description: 'Test plan',
				phases: [mockPhase]
			};
			mockFetch.mockResolvedValueOnce(mockResponse(mockPlan));
			const plan = await getTaskPlan('TASK-001');
			expect(plan).toEqual(mockPlan);
		});
	});

	describe('Task Control', () => {
		const controlResponse = { status: 'ok', task_id: 'TASK-001' };

		it('runTask sends POST', async () => {
			mockFetch.mockResolvedValueOnce(mockResponse(controlResponse));
			const result = await runTask('TASK-001');
			expect(result).toEqual(controlResponse);
			expect(mockFetch).toHaveBeenCalledWith('/api/tasks/TASK-001/run', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' }
			});
		});

		it('pauseTask sends POST', async () => {
			mockFetch.mockResolvedValueOnce(mockResponse(controlResponse));
			const result = await pauseTask('TASK-001');
			expect(result).toEqual(controlResponse);
		});

		it('resumeTask sends POST', async () => {
			mockFetch.mockResolvedValueOnce(mockResponse(controlResponse));
			const result = await resumeTask('TASK-001');
			expect(result).toEqual(controlResponse);
		});
	});

	describe('Transcripts', () => {
		it('getTranscripts returns transcript files', async () => {
			const transcripts = [
				{ filename: 'implement-01.md', content: 'Transcript content', created_at: '2025-01-01T00:00:00Z' }
			];
			mockFetch.mockResolvedValueOnce(mockResponse(transcripts));
			const result = await getTranscripts('TASK-001');
			expect(result).toEqual(transcripts);
		});
	});

	describe('Prompts', () => {
		it('listPrompts returns prompt info', async () => {
			const prompts = [{ phase: 'implement', source: 'embedded', has_override: false, variables: [] }];
			mockFetch.mockResolvedValueOnce(mockResponse(prompts));
			const result = await listPrompts();
			expect(result).toEqual(prompts);
		});

		it('getPrompt returns prompt content', async () => {
			const prompt = { phase: 'implement', content: 'Do stuff', source: 'embedded', variables: [] };
			mockFetch.mockResolvedValueOnce(mockResponse(prompt));
			const result = await getPrompt('implement');
			expect(result).toEqual(prompt);
		});

		it('savePrompt sends PUT with content', async () => {
			const prompt = { phase: 'implement', content: 'New content', source: 'project', variables: [] };
			mockFetch.mockResolvedValueOnce(mockResponse(prompt));
			const result = await savePrompt('implement', 'New content');
			expect(result).toEqual(prompt);
			expect(mockFetch).toHaveBeenCalledWith('/api/prompts/implement', {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ content: 'New content' })
			});
		});

		it('deletePrompt handles 204 response', async () => {
			mockFetch.mockResolvedValueOnce({ ok: true, status: 204 });
			await expect(deletePrompt('implement')).resolves.toBeUndefined();
		});
	});

	describe('Hooks', () => {
		it('listHooks returns hooks map', async () => {
			const hooksMap = { PreToolUse: [{ matcher: '*', hooks: [{ type: 'command', command: 'echo test' }] }] };
			mockFetch.mockResolvedValueOnce(mockResponse(hooksMap));
			const result = await listHooks();
			expect(result).toEqual(hooksMap);
		});

		it('createHook sends POST with event and hook', async () => {
			const hook = { matcher: '*', hooks: [{ type: 'command', command: 'echo test' }] };
			const hooks = [hook];
			mockFetch.mockResolvedValueOnce(mockResponse(hooks));
			const result = await createHook('PreToolUse', hook);
			expect(result).toEqual(hooks);
			expect(mockFetch).toHaveBeenCalledWith('/api/hooks', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ event: 'PreToolUse', hook })
			});
		});

		it('deleteHook handles 204 response', async () => {
			mockFetch.mockResolvedValueOnce({ ok: true, status: 204 });
			await expect(deleteHook('PreToolUse')).resolves.toBeUndefined();
		});
	});

	describe('Skills', () => {
		const mockSkillInfo = { name: 'test', description: 'Test skill', path: '.claude/skills/test/' };
		const mockSkill = {
			name: 'test',
			description: 'Test skill',
			content: '# Test',
			prompt: 'Test prompt',
			path: '.claude/skills/test/'
		};

		it('listSkills returns skill info', async () => {
			const skills = [mockSkillInfo];
			mockFetch.mockResolvedValueOnce(mockResponse(skills));
			const result = await listSkills();
			expect(result).toEqual(skills);
		});

		it('getSkill returns skill', async () => {
			mockFetch.mockResolvedValueOnce(mockResponse(mockSkill));
			const result = await getSkill('test');
			expect(result).toEqual(mockSkill);
		});

		it('createSkill sends POST', async () => {
			mockFetch.mockResolvedValueOnce(mockResponse(mockSkill));
			const result = await createSkill(mockSkill);
			expect(result).toEqual(mockSkill);
		});

		it('deleteSkill handles 204 response', async () => {
			mockFetch.mockResolvedValueOnce({ ok: true, status: 204 });
			await expect(deleteSkill('test')).resolves.toBeUndefined();
		});
	});

	describe('Config', () => {
		const mockConfig = {
			version: '1.0.0',
			profile: 'auto',
			automation: { profile: 'auto', gates_default: 'auto', retry_enabled: true, retry_max: 3 },
			execution: { model: 'sonnet', max_iterations: 10, timeout: '30m' },
			git: { branch_prefix: 'orc/', commit_prefix: '[orc]' }
		};

		it('getConfig returns config', async () => {
			mockFetch.mockResolvedValueOnce(mockResponse(mockConfig));
			const result = await getConfig();
			expect(result).toEqual(mockConfig);
		});

		it('updateConfig sends PUT', async () => {
			mockFetch.mockResolvedValueOnce(mockResponse(mockConfig));
			const result = await updateConfig({ profile: 'safe' });
			expect(result).toEqual(mockConfig);
			expect(mockFetch).toHaveBeenCalledWith('/api/config', {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ profile: 'safe' })
			});
		});
	});

	describe('Projects', () => {
		const mockProject = { id: 'proj-123', name: 'test-project', path: '/home/test/project', created_at: '2025-01-01T00:00:00Z' };
		const mockTask: Task = {
			id: 'TASK-001',
			title: 'Project task',
			status: 'created',
			weight: 'small',
			branch: 'orc/TASK-001',
			created_at: '2025-01-01T00:00:00Z',
			updated_at: '2025-01-01T00:00:00Z'
		};

		it('listProjects returns projects', async () => {
			mockFetch.mockResolvedValueOnce(mockResponse([mockProject]));
			const result = await listProjects();
			expect(result).toEqual([mockProject]);
		});

		it('listProjectTasks returns tasks for project', async () => {
			mockFetch.mockResolvedValueOnce(mockResponse([mockTask]));
			const result = await listProjectTasks('proj-123');
			expect(result).toEqual([mockTask]);
			expect(mockFetch).toHaveBeenCalledWith('/api/projects/proj-123/tasks', expect.any(Object));
		});

		it('createProjectTask sends POST to project endpoint', async () => {
			mockFetch.mockResolvedValueOnce(mockResponse(mockTask));
			const result = await createProjectTask('proj-123', 'Project task');
			expect(result).toEqual(mockTask);
			expect(mockFetch).toHaveBeenCalledWith('/api/projects/proj-123/tasks', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ title: 'Project task', description: undefined, weight: undefined })
			});
		});
	});

	describe('Error Handling', () => {
		it('throws error with message from response', async () => {
			mockFetch.mockResolvedValueOnce(mockResponse({ error: 'Task not found' }, false, 404));
			await expect(getTask('TASK-999')).rejects.toThrow('Task not found');
		});

		it('throws generic error when response has no error message', async () => {
			mockFetch.mockResolvedValueOnce({
				ok: false,
				status: 500,
				statusText: 'Internal Server Error',
				json: () => Promise.reject(new Error('Invalid JSON'))
			});
			await expect(getTask('TASK-001')).rejects.toThrow('Internal Server Error');
		});

		it('throws "Request failed" when no error info available', async () => {
			mockFetch.mockResolvedValueOnce({
				ok: false,
				status: 500,
				statusText: '',
				json: () => Promise.resolve({})
			});
			await expect(getTask('TASK-001')).rejects.toThrow('Request failed');
		});
	});

	describe('SSE Subscription', () => {
		it('subscribeToTask creates EventSource and returns cleanup function', () => {
			// Mock EventSource
			const mockClose = vi.fn();
			const mockEventSource = {
				onmessage: null as unknown,
				onerror: null as unknown,
				addEventListener: vi.fn(),
				close: mockClose
			};
			vi.stubGlobal('EventSource', vi.fn(() => mockEventSource));

			const onEvent = vi.fn();
			const cleanup = subscribeToTask('TASK-001', onEvent);

			expect(EventSource).toHaveBeenCalledWith('/api/tasks/TASK-001/stream');
			expect(mockEventSource.addEventListener).toHaveBeenCalledWith('state', expect.any(Function));
			expect(mockEventSource.addEventListener).toHaveBeenCalledWith('transcript', expect.any(Function));
			expect(mockEventSource.addEventListener).toHaveBeenCalledWith('phase', expect.any(Function));

			// Test cleanup
			cleanup();
			expect(mockClose).toHaveBeenCalled();

			vi.unstubAllGlobals();
		});

		it('subscribeToTask handles message events', () => {
			const mockEventSource = {
				onmessage: null as ((e: MessageEvent) => void) | null,
				onerror: null as unknown,
				addEventListener: vi.fn(),
				close: vi.fn()
			};
			vi.stubGlobal('EventSource', vi.fn(() => mockEventSource));

			const onEvent = vi.fn();
			subscribeToTask('TASK-001', onEvent);

			// Simulate message event with JSON data
			mockEventSource.onmessage!({ data: '{"status":"running"}' } as MessageEvent);
			expect(onEvent).toHaveBeenCalledWith('message', { status: 'running' });

			// Simulate message event with non-JSON data
			mockEventSource.onmessage!({ data: 'plain text' } as MessageEvent);
			expect(onEvent).toHaveBeenCalledWith('message', 'plain text');

			vi.unstubAllGlobals();
		});

		it('subscribeToTask handles error events', () => {
			const mockEventSource = {
				onmessage: null as unknown,
				onerror: null as (() => void) | null,
				addEventListener: vi.fn(),
				close: vi.fn()
			};
			vi.stubGlobal('EventSource', vi.fn(() => mockEventSource));

			const onEvent = vi.fn();
			subscribeToTask('TASK-001', onEvent);

			// Simulate error
			mockEventSource.onerror!();
			expect(onEvent).toHaveBeenCalledWith('error', { message: 'Connection lost' });

			vi.unstubAllGlobals();
		});
	});
});
