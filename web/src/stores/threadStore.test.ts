import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { ThreadSchema } from '@/gen/orc/v1/thread_pb';
import { createTimestamp } from '@/test/factories';

// Mock the threadClient from client.ts
// The store imports threadClient and calls its methods
vi.mock('@/lib/client', () => ({
	threadClient: {
		listThreads: vi.fn(),
		createThread: vi.fn(),
		getThread: vi.fn(),
		sendMessage: vi.fn(),
		archiveThread: vi.fn(),
		deleteThread: vi.fn(),
	},
}));

// Import after mock setup
import { threadClient } from '@/lib/client';
import { useThreadStore } from '@/stores/threadStore';

// =============================================================================
// HELPERS
// =============================================================================

function createMockThread(overrides: Record<string, unknown> = {}) {
	return create(ThreadSchema, {
		id: 'thread-001',
		title: 'Test Thread',
		status: 'open',
		taskId: '',
		initiativeId: '',
		sessionId: '',
		fileContext: '',
		createdAt: createTimestamp('2024-01-01T00:00:00Z'),
		updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
		messages: [],
		...overrides,
	});
}

function createDeferred<T>() {
	let resolvePromise: ((value: T | PromiseLike<T>) => void) | undefined;
	let rejectPromise: ((reason?: unknown) => void) | undefined;
	const promise = new Promise<T>((resolve, reject) => {
		resolvePromise = resolve;
		rejectPromise = reject;
	});
	return {
		promise,
		resolve(value: T) {
			if (resolvePromise === undefined) {
				throw new Error('Deferred promise resolved before initialization');
			}
			resolvePromise(value);
		},
		reject(reason?: unknown) {
			if (rejectPromise === undefined) {
				throw new Error('Deferred promise rejected before initialization');
			}
			rejectPromise(reason);
		},
	};
}

// =============================================================================
// TESTS
// =============================================================================

describe('threadStore', () => {
	beforeEach(() => {
		// Reset store to initial state
		useThreadStore.getState().reset();
		vi.clearAllMocks();
	});

	afterEach(() => {
		vi.clearAllMocks();
	});

	describe('initial state', () => {
		it('should start with empty threads list', () => {
			const state = useThreadStore.getState();
			expect(state.threads).toEqual([]);
		});

		it('should start with no selected thread', () => {
			const state = useThreadStore.getState();
			expect(state.selectedThreadId).toBeNull();
		});

		it('should start with loading false', () => {
			const state = useThreadStore.getState();
			expect(state.loading).toBe(false);
		});

		it('should start with no error', () => {
			const state = useThreadStore.getState();
			expect(state.error).toBeNull();
		});
	});

	describe('loadThreads (SC-2)', () => {
		it('should load threads from API and update state', async () => {
			const mockThreads = [
				createMockThread({ id: 'thread-001', title: 'First Thread' }),
				createMockThread({ id: 'thread-002', title: 'Second Thread', status: 'archived' }),
			];

			vi.mocked(threadClient.listThreads).mockResolvedValue({
				threads: mockThreads,
			} as never);

			await useThreadStore.getState().loadThreads('proj-001');

			const state = useThreadStore.getState();
			expect(state.threads).toHaveLength(2);
			expect(state.threads[0].id).toBe('thread-001');
			expect(state.threads[1].id).toBe('thread-002');
			expect(state.loading).toBe(false);
			expect(state.error).toBeNull();
		});

		it('should set loading true while fetching', async () => {
			let resolvePromise: (value: unknown) => void;
			const pending = new Promise((resolve) => {
				resolvePromise = resolve;
			});

			vi.mocked(threadClient.listThreads).mockReturnValue(pending as never);

			const loadPromise = useThreadStore.getState().loadThreads('proj-001');

			// Loading should be true while awaiting
			expect(useThreadStore.getState().loading).toBe(true);

			// Resolve the promise
			resolvePromise!({ threads: [] });
			await loadPromise;

			expect(useThreadStore.getState().loading).toBe(false);
		});

		it('should set error on API failure', async () => {
			vi.mocked(threadClient.listThreads).mockRejectedValue(
				new Error('Network error')
			);

			await useThreadStore.getState().loadThreads('proj-001');

			const state = useThreadStore.getState();
			expect(state.error).toBe('Failed to load threads Network error');
			expect(state.loading).toBe(false);
			expect(state.threads).toEqual([]);
		});

		it('should clear previous threads when loading for a new project', async () => {
			// Set up initial threads
			useThreadStore.setState({
				threads: [createMockThread({ id: 'old-thread' })],
			});

			vi.mocked(threadClient.listThreads).mockResolvedValue({
				threads: [createMockThread({ id: 'new-thread' })],
			} as never);

			await useThreadStore.getState().loadThreads('proj-002');

			const state = useThreadStore.getState();
			expect(state.threads).toHaveLength(1);
			expect(state.threads[0].id).toBe('new-thread');
		});

		it('should ignore stale responses after switching projects', async () => {
			const firstLoad = createDeferred<{ threads: ReturnType<typeof createMockThread>[] }>();
			const secondLoad = createDeferred<{ threads: ReturnType<typeof createMockThread>[] }>();

			vi.mocked(threadClient.listThreads)
				.mockReturnValueOnce(firstLoad.promise as never)
				.mockReturnValueOnce(secondLoad.promise as never);

			const firstPromise = useThreadStore.getState().loadThreads('proj-001');
			useThreadStore.getState().reset();
			const secondPromise = useThreadStore.getState().loadThreads('proj-002');

			secondLoad.resolve({
				threads: [createMockThread({ id: 'thread-b', title: 'Project B thread' })],
			});
			await secondPromise;
			expect(useThreadStore.getState().threads[0]?.id).toBe('thread-b');

			firstLoad.resolve({
				threads: [createMockThread({ id: 'thread-a', title: 'Project A thread' })],
			});
			await firstPromise;
			expect(useThreadStore.getState().threads[0]?.id).toBe('thread-b');
		});
	});

	describe('createThread (SC-4)', () => {
		it('should create a thread via API and add to state', async () => {
			const newThread = createMockThread({
				id: 'thread-new',
				title: 'New Thread',
			});

			vi.mocked(threadClient.createThread).mockResolvedValue({
				thread: newThread,
			} as never);
			vi.mocked(threadClient.listThreads).mockResolvedValue({
				threads: [newThread],
			} as never);

			const result = await useThreadStore.getState().createThread('proj-001', 'New Thread');

			const state = useThreadStore.getState();
			expect(state.threads).toHaveLength(1);
			expect(state.threads[0].id).toBe('thread-new');
			expect(result).toBeDefined();
			expect(result!.id).toBe('thread-new');
		});

		it('should select the newly created thread', async () => {
			const newThread = createMockThread({
				id: 'thread-new',
				title: 'New Thread',
			});

			vi.mocked(threadClient.createThread).mockResolvedValue({
				thread: newThread,
			} as never);
			vi.mocked(threadClient.listThreads).mockResolvedValue({
				threads: [newThread],
			} as never);

			await useThreadStore.getState().createThread('proj-001', 'New Thread');

			expect(useThreadStore.getState().selectedThreadId).toBe('thread-new');
		});

		it('should return null and set error on API failure', async () => {
			vi.mocked(threadClient.createThread).mockRejectedValue(
				new Error('Creation failed')
			);

			const result = await useThreadStore.getState().createThread('proj-001', 'New Thread');

			expect(result).toBeNull();
			expect(useThreadStore.getState().error).toBe('Failed to create thread Creation failed');
		});

		it('should ignore stale create results after switching projects', async () => {
			const createDeferredResponse = createDeferred<{ thread?: ReturnType<typeof createMockThread> }>();

			vi.mocked(threadClient.createThread).mockReturnValue(createDeferredResponse.promise as never);

			const createPromise = useThreadStore.getState().createThread('proj-001', 'Project A thread');
			useThreadStore.getState().reset();

			createDeferredResponse.resolve({
				thread: createMockThread({ id: 'thread-a', title: 'Project A thread' }),
			});

			const result = await createPromise;
			const state = useThreadStore.getState();
			expect(result).toBeNull();
			expect(state.threads).toEqual([]);
			expect(state.selectedThreadId).toBeNull();
			expect(state.error).toBeNull();
		});

		it('keeps the created thread selected when a same-project list refresh overlaps the create response', async () => {
			const staleListLoad = createDeferred<{ threads: ReturnType<typeof createMockThread>[] }>();
			const refreshAfterCreate = createDeferred<{ threads: ReturnType<typeof createMockThread>[] }>();
			const createdThread = createMockThread({
				id: 'thread-new',
				title: 'New Thread',
			});

			vi.mocked(threadClient.createThread).mockResolvedValue({
				thread: createdThread,
			} as never);
			vi.mocked(threadClient.listThreads)
				.mockReturnValueOnce(staleListLoad.promise as never)
				.mockReturnValueOnce(refreshAfterCreate.promise as never);

			const createPromise = useThreadStore.getState().createThread('proj-001', 'New Thread');
			const overlappingLoadPromise = useThreadStore.getState().loadThreads('proj-001');

			await expect(createPromise).resolves.toMatchObject({ id: 'thread-new' });
			expect(useThreadStore.getState().selectedThreadId).toBe('thread-new');
			expect(useThreadStore.getState().threads).toEqual([createdThread]);

			staleListLoad.resolve({
				threads: [createMockThread({ id: 'thread-old', title: 'Old Thread' })],
			});
			await overlappingLoadPromise;

			expect(useThreadStore.getState().selectedThreadId).toBe('thread-new');
			expect(useThreadStore.getState().threads).toEqual([createdThread]);

			refreshAfterCreate.resolve({
				threads: [
					createMockThread({ id: 'thread-old', title: 'Old Thread' }),
					createdThread,
				],
			});
			await refreshAfterCreate.promise;
			await Promise.resolve();

			expect(useThreadStore.getState().threads.map((thread) => thread.id)).toEqual(['thread-old', 'thread-new']);
			expect(useThreadStore.getState().selectedThreadId).toBe('thread-new');
		});
	});

	describe('selectThread (SC-3)', () => {
		it('should set selectedThreadId', () => {
			useThreadStore.setState({
				threads: [
					createMockThread({ id: 'thread-001' }),
					createMockThread({ id: 'thread-002' }),
				],
			});

			useThreadStore.getState().selectThread('thread-002');

			expect(useThreadStore.getState().selectedThreadId).toBe('thread-002');
		});

		it('should allow selecting null (deselect)', () => {
			useThreadStore.setState({ selectedThreadId: 'thread-001' });

			useThreadStore.getState().selectThread(null);

			expect(useThreadStore.getState().selectedThreadId).toBeNull();
		});
	});

	describe('reset', () => {
		it('should reset all state to initial values', () => {
			useThreadStore.setState({
				threads: [createMockThread()],
				selectedThreadId: 'thread-001',
				loading: true,
				error: 'some error',
			});

			useThreadStore.getState().reset();

			const state = useThreadStore.getState();
			expect(state.threads).toEqual([]);
			expect(state.selectedThreadId).toBeNull();
			expect(state.loading).toBe(false);
			expect(state.error).toBeNull();
		});
	});

	describe('getSelectedThread', () => {
		it('should return the selected thread when one is selected', () => {
			const thread = createMockThread({ id: 'thread-001', title: 'Selected' });
			useThreadStore.setState({
				threads: [thread, createMockThread({ id: 'thread-002' })],
				selectedThreadId: 'thread-001',
			});

			const selected = useThreadStore.getState().getSelectedThread();
			expect(selected).toBeDefined();
			expect(selected!.id).toBe('thread-001');
		});

		it('should return undefined when no thread is selected', () => {
			useThreadStore.setState({
				threads: [createMockThread()],
				selectedThreadId: null,
			});

			expect(useThreadStore.getState().getSelectedThread()).toBeUndefined();
		});
	});
});

// =============================================================================
// SELECTOR HOOK TESTS
// These test that exported selector hooks work correctly
// =============================================================================

describe('threadStore selector hooks', () => {
	beforeEach(() => {
		useThreadStore.getState().reset();
	});

	it('should export useThreads selector', async () => {
		// Verify the selector is exported and usable
		const { useThreads } = await import('@/stores/threadStore');
		expect(typeof useThreads).toBe('function');
	});

	it('should export useSelectedThread selector', async () => {
		const { useSelectedThread } = await import('@/stores/threadStore');
		expect(typeof useSelectedThread).toBe('function');
	});

	it('should export useThreadLoading selector', async () => {
		const { useThreadLoading } = await import('@/stores/threadStore');
		expect(typeof useThreadLoading).toBe('function');
	});

	it('should export useThreadError selector', async () => {
		const { useThreadError } = await import('@/stores/threadStore');
		expect(typeof useThreadError).toBe('function');
	});
});
