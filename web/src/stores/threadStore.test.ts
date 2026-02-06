import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { ThreadSchema, ThreadMessageSchema } from '@/gen/orc/v1/thread_pb';
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

function createMockMessage(overrides: Record<string, unknown> = {}) {
	return create(ThreadMessageSchema, {
		id: BigInt(1),
		threadId: 'thread-001',
		role: 'user',
		content: 'Hello',
		createdAt: createTimestamp('2024-01-01T00:00:00Z'),
		...overrides,
	});
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
			expect(state.error).toBe('Failed to load threads');
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

			await useThreadStore.getState().createThread('proj-001', 'New Thread');

			expect(useThreadStore.getState().selectedThreadId).toBe('thread-new');
		});

		it('should return null and set error on API failure', async () => {
			vi.mocked(threadClient.createThread).mockRejectedValue(
				new Error('Creation failed')
			);

			const result = await useThreadStore.getState().createThread('proj-001', 'New Thread');

			expect(result).toBeNull();
			// Error should be accessible for UI to display toast
			expect(useThreadStore.getState().error).toBeTruthy();
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
