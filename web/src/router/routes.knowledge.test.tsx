import { create } from '@bufbuild/protobuf';
import { describe, expect, it, vi, beforeEach } from 'vitest';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { render, screen, waitFor } from '@testing-library/react';
import { routes } from './routes';
import { useProjectStore } from '@/stores/projectStore';
import { GetKnowledgeInsightsResponseSchema, GetKnowledgeStatusResponseSchema } from '@/gen/orc/v1/knowledge_pb';
import { knowledgeClient } from '@/lib/client';

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

describe('/knowledge route', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		useProjectStore.setState({
			projects: [],
			currentProjectId: 'proj-knowledge',
			loading: false,
			error: null,
			_isHandlingPopState: false,
		});
		vi.mocked(knowledgeClient.getStatus).mockResolvedValue(
			create(GetKnowledgeStatusResponseSchema, {
				status: { enabled: true, running: true, neo4j: true, qdrant: true, redis: true },
			})
		);
		vi.mocked(knowledgeClient.getInsights).mockResolvedValue(
			create(GetKnowledgeInsightsResponseSchema, {})
		);
	});

	it('mounts KnowledgePage at /knowledge', async () => {
		const root = routes[0];
		if (!root.children) {
			throw new Error('root route is missing children');
		}
		const knowledgeChild = root.children.find((route) => route.path === 'knowledge');
		if (!knowledgeChild?.element) {
			throw new Error('/knowledge route is not registered');
		}

		const router = createMemoryRouter(
			[
				{
					path: '/knowledge',
					element: knowledgeChild.element,
				},
			],
			{ initialEntries: ['/knowledge'] }
		);

		render(
			<RouterProvider router={router} />
		);

		await waitFor(() => {
			expect(screen.getByText('Knowledge Exploration')).toBeInTheDocument();
		});
	});
});
