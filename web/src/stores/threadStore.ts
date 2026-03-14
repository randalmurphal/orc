import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import type { Thread } from '@/gen/orc/v1/thread_pb';
import { ListThreadsRequestSchema, CreateThreadRequestSchema } from '@/gen/orc/v1/thread_pb';
import { create as createMsg } from '@bufbuild/protobuf';
import { threadClient } from '@/lib/client';

let latestThreadLoadRequestId = 0;
let threadStoreGeneration = 0;

function beginThreadLoadRequest() {
	latestThreadLoadRequestId += 1;
	return latestThreadLoadRequestId;
}

function isCurrentThreadLoadRequest(requestId: number) {
	return requestId === latestThreadLoadRequestId;
}

function currentThreadStoreGeneration() {
	return threadStoreGeneration;
}

function resetThreadStoreGeneration() {
	threadStoreGeneration += 1;
	return threadStoreGeneration;
}

function isCurrentThreadStoreGeneration(generation: number) {
	return generation === threadStoreGeneration;
}

function withErrorDetails(prefix: string, err: unknown): string {
	if (err instanceof Error && err.message) {
		return `${prefix} ${err.message}`;
	}
	return prefix;
}

interface ThreadStore {
	// State
	threads: Thread[];
	selectedThreadId: string | null;
	loading: boolean;
	error: string | null;

	// Derived
	getSelectedThread: () => Thread | undefined;

	// Actions
	loadThreads: (projectId: string) => Promise<void>;
	createThread: (projectId: string, title: string) => Promise<Thread | null>;
	refreshThreadList: (projectId: string) => Promise<void>;
	selectThread: (threadId: string | null) => void;
	reset: () => void;
}

const initialState = {
	threads: [] as Thread[],
	selectedThreadId: null as string | null,
	loading: false,
	error: null as string | null,
};

export const useThreadStore = create<ThreadStore>()(
	subscribeWithSelector((set, get) => ({
		...initialState,

		getSelectedThread: () => {
			const { threads, selectedThreadId } = get();
			if (!selectedThreadId) return undefined;
			return threads.find((t) => t.id === selectedThreadId);
		},

		loadThreads: async (projectId: string) => {
			const requestId = beginThreadLoadRequest();
			set({ loading: true, error: null });
			try {
				const response = await threadClient.listThreads(
					createMsg(ListThreadsRequestSchema, { projectId })
				);
				if (!isCurrentThreadLoadRequest(requestId)) {
					return;
				}
				set({ threads: response.threads, loading: false });
			} catch (err) {
				if (!isCurrentThreadLoadRequest(requestId)) {
					return;
				}
				set({
					threads: [],
					error: withErrorDetails('Failed to load threads', err),
					loading: false,
				});
			}
		},

		createThread: async (projectId: string, title: string) => {
			const generation = currentThreadStoreGeneration();
			try {
				const response = await threadClient.createThread(
					createMsg(CreateThreadRequestSchema, { projectId, title })
				);
				if (!isCurrentThreadStoreGeneration(generation)) {
					return null;
				}
				const thread = response.thread;
				if (thread) {
					set((state) => ({
						threads: upsertThread(state.threads, thread),
						selectedThreadId: thread.id,
					}));
					void get().loadThreads(projectId).catch(() => {});
					return thread;
				}
				return null;
			} catch (err) {
				if (!isCurrentThreadStoreGeneration(generation)) {
					return null;
				}
				set({ error: withErrorDetails('Failed to create thread', err) });
				return null;
			}
		},

		refreshThreadList: async (projectId: string) => {
			await get().loadThreads(projectId);
		},

		selectThread: (threadId: string | null) => {
			set({ selectedThreadId: threadId });
		},

		reset: () => {
			beginThreadLoadRequest();
			resetThreadStoreGeneration();
			set(initialState);
		},
	}))
);

function upsertThread(threads: Thread[], thread: Thread): Thread[] {
	const existingIndex = threads.findIndex((candidate) => candidate.id === thread.id);
	if (existingIndex === -1) {
		return [...threads, thread];
	}
	return threads.map((candidate, index) => (index === existingIndex ? thread : candidate));
}

// Selector hooks
export const useThreads = () => useThreadStore((state) => state.threads);
export const useSelectedThread = () =>
	useThreadStore((state) =>
		state.selectedThreadId
			? state.threads.find((t) => t.id === state.selectedThreadId)
			: undefined
	);
export const useThreadLoading = () => useThreadStore((state) => state.loading);
export const useThreadError = () => useThreadStore((state) => state.error);
