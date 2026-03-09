import { create } from '@bufbuild/protobuf';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { KnowledgePage } from './KnowledgePage';
import { knowledgeClient, threadClient } from '@/lib/client';
import { useProjectStore } from '@/stores/projectStore';
import { useThreadStore } from '@/stores/threadStore';
import {
	GetKnowledgeInsightsResponseSchema,
	GetKnowledgeStatusResponseSchema,
	KnowledgeResultSchema,
	KnowledgeResultType,
	QueryKnowledgeResponseSchema,
} from '@/gen/orc/v1/knowledge_pb';
import { CreateThreadResponseSchema, SendThreadMessageResponseSchema } from '@/gen/orc/v1/thread_pb';

vi.mock('@/lib/client', () => ({
	knowledgeClient: {
		getStatus: vi.fn(),
		getInsights: vi.fn(),
		query: vi.fn(),
	},
	threadClient: {
		createThread: vi.fn(),
		sendMessage: vi.fn(),
		listThreads: vi.fn(),
		getThread: vi.fn(),
	},
}));

function renderPage() {
	return render(
		<MemoryRouter>
			<KnowledgePage />
		</MemoryRouter>
	);
}

describe('KnowledgePage', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		useProjectStore.getState().reset();
		useProjectStore.setState({
			projects: [],
			currentProjectId: 'proj-001',
			loading: false,
			error: null,
			_isHandlingPopState: false,
		});
		useThreadStore.getState().reset();
	});

	it('renders search UI when knowledge is running', async () => {
		vi.mocked(knowledgeClient.getStatus).mockResolvedValue(
			create(GetKnowledgeStatusResponseSchema, {
				status: {
					enabled: true,
					running: true,
					neo4j: true,
					qdrant: true,
					redis: true,
				},
			})
		);
		vi.mocked(knowledgeClient.getInsights).mockResolvedValue(
			create(GetKnowledgeInsightsResponseSchema, {})
		);

		renderPage();

		await waitFor(() => {
			expect(screen.getByPlaceholderText('Ask the knowledge graph about this codebase')).toBeInTheDocument();
		});

		const preset = screen.getByLabelText('Knowledge search preset');
		expect(preset).toBeInTheDocument();
		expect(screen.getByRole('option', { name: 'standard' })).toBeInTheDocument();
		expect(screen.getByRole('option', { name: 'fast' })).toBeInTheDocument();
		expect(screen.getByRole('option', { name: 'deep' })).toBeInTheDocument();
		expect(screen.getByRole('option', { name: 'graph-first' })).toBeInTheDocument();
		expect(screen.getByRole('option', { name: 'recency' })).toBeInTheDocument();
	});

	it('renders empty state when knowledge is not running', async () => {
		vi.mocked(knowledgeClient.getStatus).mockResolvedValue(
			create(GetKnowledgeStatusResponseSchema, {
				status: {
					enabled: true,
					running: false,
				},
			})
		);

		renderPage();

		await waitFor(() => {
			expect(screen.getByText('Knowledge Layer Not Running')).toBeInTheDocument();
		});
		expect(screen.getByText(/orc knowledge start && orc index/)).toBeInTheDocument();
		expect(screen.queryByPlaceholderText('Ask the knowledge graph about this codebase')).not.toBeInTheDocument();
	});

	it('submits query on Enter and renders result cards', async () => {
		vi.mocked(knowledgeClient.getStatus).mockResolvedValue(
			create(GetKnowledgeStatusResponseSchema, {
				status: {
					enabled: true,
					running: true,
					neo4j: true,
					qdrant: true,
					redis: true,
				},
			})
		);
		vi.mocked(knowledgeClient.getInsights).mockResolvedValue(
			create(GetKnowledgeInsightsResponseSchema, {})
		);
		vi.mocked(knowledgeClient.query).mockResolvedValue(
			create(QueryKnowledgeResponseSchema, {
				tokensUsed: 222,
				results: [
					create(KnowledgeResultSchema, {
						id: 'result-1',
						type: KnowledgeResultType.CODE,
						title: 'Gate Flow',
						content: 'Executor gate logic',
						summary: 'Gate summary',
						filePath: 'internal/executor/workflow_executor.go',
						startLine: 15,
						endLine: 28,
						score: 0.91,
					}),
				],
			})
		);

		renderPage();

		await waitFor(() => {
			expect(screen.getByPlaceholderText('Ask the knowledge graph about this codebase')).toBeInTheDocument();
		});

		const input = screen.getByLabelText('Knowledge query input');
		fireEvent.change(input, { target: { value: 'how does gate evaluation work' } });
		fireEvent.keyDown(input, { key: 'Enter', code: 'Enter' });

		await waitFor(() => {
			expect(knowledgeClient.query).toHaveBeenCalledTimes(1);
		});
		expect(screen.getByText('Gate Flow')).toBeInTheDocument();
		expect(screen.getByText('internal/executor/workflow_executor.go:15-28')).toBeInTheDocument();
		expect(screen.getByText('Score 0.91')).toBeInTheDocument();
	});

	it('Discuss action creates a thread and sends result context', async () => {
		vi.mocked(knowledgeClient.getStatus).mockResolvedValue(
			create(GetKnowledgeStatusResponseSchema, {
				status: {
					enabled: true,
					running: true,
				},
			})
		);
		vi.mocked(knowledgeClient.getInsights).mockResolvedValue(
			create(GetKnowledgeInsightsResponseSchema, {})
		);
		vi.mocked(knowledgeClient.query).mockResolvedValue(
			create(QueryKnowledgeResponseSchema, {
				results: [
					create(KnowledgeResultSchema, {
						id: 'result-2',
						type: KnowledgeResultType.CODE,
						title: 'Executor Runtime',
						content: 'Runtime details',
						filePath: 'internal/executor/workflow_executor.go',
						startLine: 200,
						score: 0.82,
					}),
				],
			})
		);
		vi.mocked(threadClient.createThread).mockResolvedValue(
			create(CreateThreadResponseSchema, {
				thread: {
					id: 'thread-001',
					title: 'Knowledge: Executor Runtime',
					status: 'active',
				},
			})
		);
		vi.mocked(threadClient.sendMessage).mockResolvedValue(
			create(SendThreadMessageResponseSchema, {})
		);

		renderPage();

		await waitFor(() => {
			expect(screen.getByPlaceholderText('Ask the knowledge graph about this codebase')).toBeInTheDocument();
		});

		const input = screen.getByLabelText('Knowledge query input');
		fireEvent.change(input, { target: { value: 'executor runtime' } });
		fireEvent.keyDown(input, { key: 'Enter', code: 'Enter' });

		await waitFor(() => {
			expect(screen.getByRole('button', { name: 'Discuss' })).toBeInTheDocument();
		});

		fireEvent.click(screen.getByRole('button', { name: 'Discuss' }));

		await waitFor(() => {
			expect(threadClient.createThread).toHaveBeenCalledTimes(1);
			expect(threadClient.sendMessage).toHaveBeenCalledTimes(1);
		});
		expect(useThreadStore.getState().selectedThreadId).toBe('thread-001');

		const sendCall = vi.mocked(threadClient.sendMessage).mock.calls[0]?.[0];
		expect(sendCall.content).toContain('Executor Runtime');
		expect(sendCall.content).toContain('internal/executor/workflow_executor.go');
	});
});
