import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent, act } from '@testing-library/react';
import { create } from '@bufbuild/protobuf';
import { RecommendationInbox } from './RecommendationInbox';
import {
	AcceptRecommendationResponseSchema,
	DiscussRecommendationResponseSchema,
	RecommendationKind,
	RecommendationSchema,
	RecommendationStatus,
	RejectRecommendationResponseSchema,
	ListRecommendationsResponseSchema,
	type Recommendation,
} from '@/gen/orc/v1/recommendation_pb';

let currentProjectId = 'proj-001';

vi.mock('@/stores/projectStore', () => ({
	useCurrentProjectId: () => currentProjectId,
}));

vi.mock('@/lib/api/recommendation', () => ({
	listRecommendations: vi.fn(),
	acceptRecommendation: vi.fn(),
	rejectRecommendation: vi.fn(),
	discussRecommendation: vi.fn(),
}));

import {
	acceptRecommendation,
	discussRecommendation,
	listRecommendations,
	rejectRecommendation,
} from '@/lib/api/recommendation';
import { emitRecommendationSignal } from '@/lib/events/recommendationSignals';

describe('RecommendationInbox', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		currentProjectId = 'proj-001';
	});

	it('renders loading and then the empty state', async () => {
		vi.mocked(listRecommendations).mockResolvedValue(makeListResponse([]));

		render(<RecommendationInbox />);

		expect(screen.getByText('Loading recommendations...')).toBeInTheDocument();
		await screen.findByText('No recommendations yet.');
	});

	it('shows an error state and retries loading', async () => {
		vi.mocked(listRecommendations)
			.mockRejectedValueOnce(new Error('load failed'))
			.mockResolvedValueOnce(makeListResponse([makeRecommendation()]));

		render(<RecommendationInbox />);

		await screen.findByText('load failed');
		fireEvent.click(screen.getByRole('button', { name: 'Retry' }));

		await screen.findByText('Clean up duplicate polling');
		expect(listRecommendations).toHaveBeenCalledTimes(2);
	});

	it('renders recommendations and allows follow-up decisions from discussed state', async () => {
		vi.mocked(listRecommendations).mockResolvedValue(makeListResponse([
			makeRecommendation(),
			makeRecommendation({
				id: 'REC-002',
				status: RecommendationStatus.DISCUSSED,
				title: 'Discussed follow-up',
				dedupeKey: 'cleanup:task-001:discussed',
			}),
		]));

		render(<RecommendationInbox />);

		await screen.findByText('Recommendation Inbox');
		expect(screen.getByText('1 pending recommendations need a human decision.')).toBeInTheDocument();

		const discussedCard = screen.getByText('Discussed follow-up').closest('.recommendation-card');
		expect(discussedCard).not.toBeNull();
		const acceptButton = withinCard(discussedCard!, 'Accept');
		const rejectButton = withinCard(discussedCard!, 'Reject');
		const discussButton = withinCard(discussedCard!, 'Discuss');

		expect(acceptButton).toBeEnabled();
		expect(rejectButton).toBeEnabled();
		expect(discussButton).toBeDisabled();
	});

	it('discusses a recommendation and renders the returned context pack', async () => {
		vi.mocked(listRecommendations)
			.mockResolvedValueOnce(makeListResponse([makeRecommendation()]))
			.mockResolvedValueOnce(makeListResponse([makeRecommendation({ status: RecommendationStatus.DISCUSSED })]));
		vi.mocked(discussRecommendation).mockResolvedValue(
			create(DiscussRecommendationResponseSchema, {
				recommendation: makeRecommendation({ status: RecommendationStatus.DISCUSSED }),
				contextPack: 'Recommendation REC-001\nKind: cleanup',
			}),
		);

		render(<RecommendationInbox />);

		await screen.findByText('Clean up duplicate polling');
		fireEvent.click(screen.getByRole('button', { name: 'Discuss' }));

		await screen.findByText(/Recommendation REC-001/);
		expect(discussRecommendation).toHaveBeenCalledWith('proj-001', 'REC-001', 'operator', '');
	});

	it('accepts and rejects recommendations through the API and refreshes the list', async () => {
		vi.mocked(listRecommendations)
			.mockResolvedValueOnce(makeListResponse([
				makeRecommendation(),
				makeRecommendation({
					id: 'REC-002',
					title: 'Reject me',
					dedupeKey: 'cleanup:task-001:reject-me',
				}),
			]))
			.mockResolvedValueOnce(makeListResponse([
				makeRecommendation({ status: RecommendationStatus.ACCEPTED }),
				makeRecommendation({
					id: 'REC-002',
					title: 'Reject me',
					dedupeKey: 'cleanup:task-001:reject-me',
				}),
			]))
			.mockResolvedValueOnce(makeListResponse([
				makeRecommendation({ status: RecommendationStatus.ACCEPTED }),
				makeRecommendation({
					id: 'REC-002',
					title: 'Reject me',
					status: RecommendationStatus.REJECTED,
					dedupeKey: 'cleanup:task-001:reject-me',
				}),
			]));
		vi.mocked(acceptRecommendation).mockResolvedValue(
			create(AcceptRecommendationResponseSchema, {
				recommendation: makeRecommendation({ status: RecommendationStatus.ACCEPTED }),
			}),
		);
		vi.mocked(rejectRecommendation).mockResolvedValue(
			create(RejectRecommendationResponseSchema, {
				recommendation: makeRecommendation({
					id: 'REC-002',
					title: 'Reject me',
					status: RecommendationStatus.REJECTED,
					dedupeKey: 'cleanup:task-001:reject-me',
				}),
			}),
		);

		render(<RecommendationInbox />);

		await screen.findByText('Clean up duplicate polling');
		fireEvent.click(screen.getAllByRole('button', { name: 'Accept' })[0]);

		await waitFor(() => {
			expect(acceptRecommendation).toHaveBeenCalledWith('proj-001', 'REC-001', 'operator', '');
		});

		fireEvent.click(screen.getAllByRole('button', { name: 'Reject' })[1]);

		await waitFor(() => {
			expect(rejectRecommendation).toHaveBeenCalledWith('proj-001', 'REC-002', 'operator', '');
		});
		expect(listRecommendations).toHaveBeenCalledTimes(3);
	});

	it('refreshes when an external recommendation event arrives for the current project', async () => {
		vi.mocked(listRecommendations)
			.mockResolvedValueOnce(makeListResponse([makeRecommendation()]))
			.mockResolvedValueOnce(makeListResponse([
				makeRecommendation(),
				makeRecommendation({
					id: 'REC-002',
					title: 'New external recommendation',
					dedupeKey: 'cleanup:task-001:new-external',
				}),
			]));

		render(<RecommendationInbox />);

		await screen.findByText('Clean up duplicate polling');
		await act(async () => {
			emitRecommendationSignal({
				projectId: 'proj-001',
				recommendationId: 'REC-002',
				type: 'created',
			});
		});

		await screen.findByText('New external recommendation');
		expect(listRecommendations).toHaveBeenCalledTimes(2);
	});

	it('does not leak context packs across project switches when recommendation ids overlap', async () => {
		vi.mocked(listRecommendations)
			.mockResolvedValueOnce(makeListResponse([makeRecommendation({ title: 'Project A recommendation' })]))
			.mockResolvedValueOnce(makeListResponse([makeRecommendation({ title: 'Project A recommendation', status: RecommendationStatus.DISCUSSED })]))
			.mockResolvedValueOnce(makeListResponse([makeRecommendation({
				title: 'Project B recommendation',
				summary: 'Different project, same local recommendation id.',
			})]));
		vi.mocked(discussRecommendation).mockResolvedValue(
			create(DiscussRecommendationResponseSchema, {
				recommendation: makeRecommendation({ status: RecommendationStatus.DISCUSSED }),
				contextPack: 'Project A context pack',
			}),
		);

		const view = render(<RecommendationInbox />);

		await screen.findByText('Project A recommendation');
		fireEvent.click(screen.getByRole('button', { name: 'Discuss' }));
		await screen.findByText('Project A context pack');

		currentProjectId = 'proj-002';
		view.rerender(<RecommendationInbox />);

		await screen.findByText('Project B recommendation');
		expect(screen.queryByText('Project A context pack')).not.toBeInTheDocument();
	});

	it('ignores stale recommendation loads after switching projects', async () => {
		const staleLoad = createDeferred<ReturnType<typeof makeListResponse>>();
		vi.mocked(listRecommendations)
			.mockImplementationOnce(() => staleLoad.promise as never)
			.mockResolvedValueOnce(makeListResponse([makeRecommendation({
				title: 'Project B recommendation',
				summary: 'This belongs to the active project.',
			})]));

		const view = render(<RecommendationInbox />);

		currentProjectId = 'proj-002';
		view.rerender(<RecommendationInbox />);

		await screen.findByText('Project B recommendation');
		await act(async () => {
			staleLoad.resolve(makeListResponse([makeRecommendation({
				title: 'Project A recommendation',
				summary: 'This response arrived late and must be ignored.',
			})]));
			await Promise.resolve();
		});

		expect(screen.queryByText('Project A recommendation')).not.toBeInTheDocument();
		expect(screen.getByText('Project B recommendation')).toBeInTheDocument();
	});

	it('does not trigger an old-project reload after a decision completes on a different active project', async () => {
		const acceptDeferred = createDeferred<ReturnType<typeof create<typeof AcceptRecommendationResponseSchema>>>();
		vi.mocked(listRecommendations)
			.mockResolvedValueOnce(makeListResponse([makeRecommendation({ title: 'Project A recommendation' })]))
			.mockResolvedValueOnce(makeListResponse([makeRecommendation({
				title: 'Project B recommendation',
				summary: 'Active project after the switch.',
			})]));
		vi.mocked(acceptRecommendation).mockReturnValue(acceptDeferred.promise as never);

		const view = render(<RecommendationInbox />);

		await screen.findByText('Project A recommendation');
		fireEvent.click(screen.getByRole('button', { name: 'Accept' }));

		currentProjectId = 'proj-002';
		view.rerender(<RecommendationInbox />);
		await screen.findByText('Project B recommendation');

		await act(async () => {
			acceptDeferred.resolve(create(AcceptRecommendationResponseSchema, {
				recommendation: makeRecommendation({ status: RecommendationStatus.ACCEPTED }),
			}));
			await Promise.resolve();
		});

		expect(screen.getByText('Project B recommendation')).toBeInTheDocument();
		expect(screen.queryByText('Project A recommendation')).not.toBeInTheDocument();
		expect(listRecommendations).toHaveBeenCalledTimes(2);
	});
});

function makeListResponse(recommendations: Recommendation[]) {
	return create(ListRecommendationsResponseSchema, { recommendations });
}

function makeRecommendation(overrides: Record<string, unknown> = {}): Recommendation {
	return create(RecommendationSchema, {
		id: 'REC-001',
		kind: RecommendationKind.CLEANUP,
		status: RecommendationStatus.PENDING,
		title: 'Clean up duplicate polling',
		summary: 'Two polling loops are doing the same work.',
		proposedAction: 'Remove the legacy loop after validating the new path.',
		evidence: 'Both loops hit the same endpoint every 5 seconds.',
		sourceTaskId: 'TASK-001',
		sourceRunId: 'RUN-001',
		sourceThreadId: 'THR-001',
		dedupeKey: 'cleanup:task-001:duplicate-polling',
		...overrides,
	});
}

function withinCard(card: Element, label: string): HTMLButtonElement {
	const button = Array.from(card.querySelectorAll('button')).find(
		(candidate) => candidate.textContent?.trim() === label,
	);
	if (!(button instanceof HTMLButtonElement)) {
		throw new Error(`button ${label} not found`);
	}
	return button;
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
