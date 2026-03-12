import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import type { Thread } from '@/gen/orc/v1/thread_pb';
import { ListThreadsRequestSchema, CreateThreadRequestSchema } from '@/gen/orc/v1/thread_pb';
import { create as createMsg } from '@bufbuild/protobuf';
import { threadClient } from '@/lib/client';

let latestThreadLoadRequestId = 0;

function beginThreadLoadRequest() {
	latestThreadLoadRequestId += 1;
	return latestThreadLoadRequestId;
}

function isCurrentThreadLoadRequest(requestId: number) {
	return requestId === latestThreadLoadRequestId;
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
			} catch {
				if (!isCurrentThreadLoadRequest(requestId)) {
					return;
				}
				set({ threads: [], error: 'Failed to load threads', loading: false });
			}
		},

		createThread: async (projectId: string, title: string) => {
			try {
				const response = await threadClient.createThread(
					createMsg(CreateThreadRequestSchema, { projectId, title })
				);
				const thread = response.thread;
				if (thread) {
					set((state) => ({
						threads: [...state.threads, thread],
						selectedThreadId: thread.id,
					}));
					return thread;
				}
				return null;
			} catch {
				set({ error: 'Failed to create thread' });
				return null;
			}
		},

		selectThread: (threadId: string | null) => {
			set({ selectedThreadId: threadId });
		},

		reset: () => {
			beginThreadLoadRequest();
			set(initialState);
		},
	}))
);

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
