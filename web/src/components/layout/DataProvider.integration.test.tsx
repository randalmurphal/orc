/**
 * Integration tests: DataProvider → threadStore wiring
 *
 * Verifies that the production DataProvider component loads threads
 * via threadStore when the project changes. Without this wiring,
 * threads loaded by ThreadService never reach the UI — ProjectSidebar
 * reads from threadStore, but only DataProvider triggers the load.
 *
 * Production path: DataProvider → threadStore.loadThreads() → threadClient.listThreads()
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor, act } from '@testing-library/react';
import { DataProvider } from './DataProvider';
import {
	useProjectStore,
	useTaskStore,
	useInitiativeStore,
} from '@/stores';
import { useThreadStore } from '@/stores/threadStore';
import { create } from '@bufbuild/protobuf';
import { ProjectSchema } from '@/gen/orc/v1/project_pb';
import { ThreadSchema } from '@/gen/orc/v1/thread_pb';
import { createTimestamp } from '@/test/factories';

// Mock ALL API clients used by DataProvider and threadStore
vi.mock('@/lib/client', () => ({
	projectClient: {
		listProjects: vi.fn(),
	},
	taskClient: {
		listTasks: vi.fn(),
	},
	initiativeClient: {
		listInitiatives: vi.fn(),
	},
	threadClient: {
		listThreads: vi.fn(),
		createThread: vi.fn(),
		getThread: vi.fn(),
		sendMessage: vi.fn(),
	},
}));

import { threadClient, projectClient, taskClient, initiativeClient } from '@/lib/client';

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

// =============================================================================
// SETUP
// =============================================================================

beforeEach(() => {
	// Start with a project already selected (simulating normal app state)
	useProjectStore.setState({
		projects: [
			create(ProjectSchema, {
				id: 'proj-001',
				name: 'Test Project',
				path: '/test/project',
				createdAt: createTimestamp('2024-01-01T00:00:00Z'),
			}),
		],
		currentProjectId: 'proj-001',
		loading: false,
		error: null,
	});

	useTaskStore.getState().reset();
	useInitiativeStore.getState().reset();
	useThreadStore.getState().reset();

	// Setup API mocks with sensible defaults
	vi.mocked(projectClient.listProjects).mockResolvedValue({
		projects: [
			create(ProjectSchema, {
				id: 'proj-001',
				name: 'Test Project',
				path: '/test/project',
				createdAt: createTimestamp('2024-01-01T00:00:00Z'),
			}),
		],
	} as never);
	vi.mocked(taskClient.listTasks).mockResolvedValue({
		tasks: [],
		page: { hasMore: false },
	} as never);
	vi.mocked(initiativeClient.listInitiatives).mockResolvedValue({
		initiatives: [],
	} as never);
	vi.mocked(threadClient.listThreads).mockResolvedValue({
		threads: [],
	} as never);
});

afterEach(() => {
	vi.clearAllMocks();
});

// =============================================================================
// INTEGRATION: DataProvider → threadStore.loadThreads → threadClient.listThreads
// =============================================================================

describe('DataProvider thread loading integration', () => {
	it('should call threadClient.listThreads on initial load with current project', async () => {
		render(
			<DataProvider>
				<div>Content</div>
			</DataProvider>
		);

		// DataProvider should trigger thread loading for the current project
		// This goes: DataProvider → threadStore.loadThreads('proj-001') → threadClient.listThreads(...)
		await waitFor(() => {
			expect(threadClient.listThreads).toHaveBeenCalledWith(
				expect.objectContaining({ projectId: 'proj-001' })
			);
		});
	});

	it('should reload threads when project ID changes', async () => {
		render(
			<DataProvider>
				<div>Content</div>
			</DataProvider>
		);

		// Wait for initial load to complete
		await waitFor(() => {
			expect(threadClient.listThreads).toHaveBeenCalled();
		});

		vi.mocked(threadClient.listThreads).mockClear();

		// Simulate project change
		act(() => {
			useProjectStore.setState({ currentProjectId: 'proj-002' });
		});

		// DataProvider should reload threads for the new project
		await waitFor(() => {
			expect(threadClient.listThreads).toHaveBeenCalledWith(
				expect.objectContaining({ projectId: 'proj-002' })
			);
		});
	});

	it('should populate threadStore with loaded threads', async () => {
		const mockThreads = [
			createMockThread({ id: 'thread-001', title: 'Design Discussion' }),
			createMockThread({ id: 'thread-002', title: 'Bug Triage' }),
		];

		vi.mocked(threadClient.listThreads).mockResolvedValue({
			threads: mockThreads,
		} as never);

		render(
			<DataProvider>
				<div>Content</div>
			</DataProvider>
		);

		// threadStore should be populated by DataProvider's load
		await waitFor(() => {
			const state = useThreadStore.getState();
			expect(state.threads).toHaveLength(2);
			expect(state.threads[0].title).toBe('Design Discussion');
			expect(state.threads[1].title).toBe('Bug Triage');
		});
	});

	it('should reset threads when switching to a project with no threads', async () => {
		// First project has threads
		vi.mocked(threadClient.listThreads).mockResolvedValueOnce({
			threads: [createMockThread({ id: 'thread-001', title: 'Old Thread' })],
		} as never);

		render(
			<DataProvider>
				<div>Content</div>
			</DataProvider>
		);

		// Wait for initial threads to load
		await waitFor(() => {
			expect(useThreadStore.getState().threads).toHaveLength(1);
		});

		// Second project has no threads
		vi.mocked(threadClient.listThreads).mockResolvedValue({
			threads: [],
		} as never);

		// Switch project
		act(() => {
			useProjectStore.setState({ currentProjectId: 'proj-002' });
		});

		// Threads should be cleared/reloaded
		await waitFor(() => {
			expect(useThreadStore.getState().threads).toHaveLength(0);
		});
	});

	it('should clear threads when project becomes null', async () => {
		// Start with threads loaded
		vi.mocked(threadClient.listThreads).mockResolvedValue({
			threads: [createMockThread()],
		} as never);

		render(
			<DataProvider>
				<div>Content</div>
			</DataProvider>
		);

		await waitFor(() => {
			expect(useThreadStore.getState().threads).toHaveLength(1);
		});

		// Clear project selection
		act(() => {
			useProjectStore.setState({ currentProjectId: null });
		});

		// Threads should be cleared (no project = no threads)
		await waitFor(() => {
			expect(useThreadStore.getState().threads).toHaveLength(0);
		});
	});
});
