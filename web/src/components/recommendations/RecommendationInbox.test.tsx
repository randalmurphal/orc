import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent, act } from '@testing-library/react';
import { create } from '@bufbuild/protobuf';
import { RecommendationInbox } from './RecommendationInbox';
import {
	AcceptRecommendationResponseSchema,
	DiscussRecommendationResponseSchema,
	ListRecommendationHistoryResponseSchema,
	RecommendationKind,
	RecommendationHistoryEntrySchema,
	RecommendationSchema,
	RecommendationStatus,
	RejectRecommendationResponseSchema,
	ListRecommendationsResponseSchema,
	type RecommendationHistoryEntry,
	type Recommendation,
} from '@/gen/orc/v1/recommendation_pb';

vi.mock('@/stores/projectStore', () => ({
	useCurrentProjectId: () => 'proj-001',
}));

vi.mock('@/lib/api/recommendation', () => ({
	listRecommendations: vi.fn(),
	listRecommendationHistory: vi.fn(),
	acceptRecommendation: vi.fn(),
	rejectRecommendation: vi.fn(),
	discussRecommendation: vi.fn(),
}));

import {
	acceptRecommendation,
	discussRecommendation,
	listRecommendationHistory,
	listRecommendations,
	rejectRecommendation,
} from '@/lib/api/recommendation';
import { emitRecommendationSignal } from '@/lib/events/recommendationSignals';

describe('RecommendationInbox', () => {
	beforeEach(() => {
		vi.clearAllMocks();
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
			.mockResolvedValueOnce(makeListResponse([makeRecommendation({
				status: RecommendationStatus.DISCUSSED,
				decisionReason: 'Needs a narrower plan.',
			})]));
		vi.mocked(discussRecommendation).mockResolvedValue(
			create(DiscussRecommendationResponseSchema, {
				recommendation: makeRecommendation({
					status: RecommendationStatus.DISCUSSED,
					decisionReason: 'Needs a narrower plan.',
				}),
				contextPack: 'Recommendation REC-001\nKind: cleanup',
			}),
		);

		render(<RecommendationInbox />);

		await screen.findByText('Clean up duplicate polling');
		fireEvent.change(screen.getByLabelText('Decision note'), {
			target: { value: 'Needs a narrower plan.' },
		});
		fireEvent.click(screen.getByRole('button', { name: 'Discuss' }));

		await screen.findByText(/Recommendation REC-001/);
		expect(discussRecommendation).toHaveBeenCalledWith('proj-001', 'REC-001', 'operator', 'Needs a narrower plan.');
	});

	it('accepts and rejects recommendations through the API, preserves decision notes, and shows promoted artifacts', async () => {
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
				makeRecommendation({
					status: RecommendationStatus.ACCEPTED,
					decisionReason: 'Looks worth shipping.',
					decidedBy: 'operator',
					promotedToType: 'task',
					promotedToId: 'TASK-099',
				}),
				makeRecommendation({
					id: 'REC-002',
					title: 'Reject me',
					dedupeKey: 'cleanup:task-001:reject-me',
				}),
			]))
			.mockResolvedValueOnce(makeListResponse([
				makeRecommendation({
					status: RecommendationStatus.ACCEPTED,
					decisionReason: 'Looks worth shipping.',
					decidedBy: 'operator',
					promotedToType: 'task',
					promotedToId: 'TASK-099',
				}),
				makeRecommendation({
					id: 'REC-002',
					title: 'Reject me',
					status: RecommendationStatus.REJECTED,
					decisionReason: 'Not worth the churn.',
					dedupeKey: 'cleanup:task-001:reject-me',
				}),
			]));
		vi.mocked(acceptRecommendation).mockResolvedValue(
			create(AcceptRecommendationResponseSchema, {
				recommendation: makeRecommendation({
					status: RecommendationStatus.ACCEPTED,
					decisionReason: 'Looks worth shipping.',
					decidedBy: 'operator',
					promotedToType: 'task',
					promotedToId: 'TASK-099',
				}),
			}),
		);
		vi.mocked(rejectRecommendation).mockResolvedValue(
			create(RejectRecommendationResponseSchema, {
				recommendation: makeRecommendation({
					id: 'REC-002',
					title: 'Reject me',
					status: RecommendationStatus.REJECTED,
					decisionReason: 'Not worth the churn.',
					dedupeKey: 'cleanup:task-001:reject-me',
				}),
			}),
		);

		render(<RecommendationInbox />);

		await screen.findByText('Clean up duplicate polling');
		fireEvent.change(screen.getAllByLabelText('Decision note')[0], {
			target: { value: 'Looks worth shipping.' },
		});
		fireEvent.click(screen.getAllByRole('button', { name: 'Accept' })[0]);

		await waitFor(() => {
			expect(acceptRecommendation).toHaveBeenCalledWith('proj-001', 'REC-001', 'operator', 'Looks worth shipping.');
		});
		await screen.findByText('Task TASK-099');
		expect(screen.getByText('Looks worth shipping.')).toBeInTheDocument();

		fireEvent.change(screen.getAllByLabelText('Decision note')[1], {
			target: { value: 'Not worth the churn.' },
		});
		fireEvent.click(screen.getAllByRole('button', { name: 'Reject' })[1]);

		await waitFor(() => {
			expect(rejectRecommendation).toHaveBeenCalledWith('proj-001', 'REC-002', 'operator', 'Not worth the churn.');
		});
		expect(listRecommendations).toHaveBeenCalledTimes(3);
	});

	it('loads recommendation history only when requested and renders the audit trail', async () => {
		vi.mocked(listRecommendations).mockResolvedValue(makeListResponse([
			makeRecommendation({
				status: RecommendationStatus.ACCEPTED,
				decisionReason: 'Looks worth shipping.',
				decidedBy: 'operator',
				promotedToType: 'task',
				promotedToId: 'TASK-099',
			}),
		]));
		vi.mocked(listRecommendationHistory).mockResolvedValue(makeHistoryResponse([
			makeHistoryEntry({
				id: 2n,
				fromStatus: RecommendationStatus.PENDING,
				toStatus: RecommendationStatus.ACCEPTED,
				decidedBy: 'operator',
				decisionReason: 'Looks worth shipping.',
			}),
			makeHistoryEntry({
				id: 1n,
				fromStatus: RecommendationStatus.UNSPECIFIED,
				toStatus: RecommendationStatus.PENDING,
			}),
		]));

		render(<RecommendationInbox />);

		await screen.findByText('Clean up duplicate polling');
		expect(listRecommendationHistory).not.toHaveBeenCalled();

		fireEvent.click(screen.getByRole('button', { name: 'Show history' }));

		await screen.findByText(/Accepted from pending by operator/);
		expect(screen.getByText('Pending')).toBeInTheDocument();
		expect(listRecommendationHistory).toHaveBeenCalledWith('proj-001', 'REC-001');

		fireEvent.click(screen.getByRole('button', { name: 'Hide history' }));
		expect(screen.queryByText(/Accepted from pending by operator/)).not.toBeInTheDocument();
	});

	it('invalidates cached history after a decision so reopened history refetches fresh entries', async () => {
		vi.mocked(listRecommendations)
			.mockResolvedValueOnce(makeListResponse([makeRecommendation()]))
			.mockResolvedValueOnce(makeListResponse([
				makeRecommendation({
					status: RecommendationStatus.ACCEPTED,
					decisionReason: 'Looks worth shipping.',
					decidedBy: 'operator',
					promotedToType: 'task',
					promotedToId: 'TASK-099',
				}),
			]));
		vi.mocked(listRecommendationHistory)
			.mockResolvedValueOnce(makeHistoryResponse([
				makeHistoryEntry({
					id: 1n,
					fromStatus: RecommendationStatus.UNSPECIFIED,
					toStatus: RecommendationStatus.PENDING,
				}),
			]))
			.mockResolvedValueOnce(makeHistoryResponse([
				makeHistoryEntry({
					id: 2n,
					fromStatus: RecommendationStatus.PENDING,
					toStatus: RecommendationStatus.ACCEPTED,
					decidedBy: 'operator',
					decisionReason: 'Looks worth shipping.',
				}),
				makeHistoryEntry({
					id: 1n,
					fromStatus: RecommendationStatus.UNSPECIFIED,
					toStatus: RecommendationStatus.PENDING,
				}),
			]));
		vi.mocked(acceptRecommendation).mockResolvedValue(
			create(AcceptRecommendationResponseSchema, {
				recommendation: makeRecommendation({
					status: RecommendationStatus.ACCEPTED,
					decisionReason: 'Looks worth shipping.',
					decidedBy: 'operator',
					promotedToType: 'task',
					promotedToId: 'TASK-099',
				}),
			}),
		);

		render(<RecommendationInbox />);

		await screen.findByText('Clean up duplicate polling');
		fireEvent.click(screen.getByRole('button', { name: 'Show history' }));
		await screen.findByText('Pending');
		expect(listRecommendationHistory).toHaveBeenCalledTimes(1);

		fireEvent.change(screen.getByLabelText('Decision note'), {
			target: { value: 'Looks worth shipping.' },
		});
		fireEvent.click(screen.getByRole('button', { name: 'Accept' }));

		await screen.findByText('Task TASK-099');
		expect(screen.queryByText(/Accepted from pending by operator/)).not.toBeInTheDocument();

		fireEvent.click(screen.getByRole('button', { name: 'Show history' }));
		await screen.findByText(/Accepted from pending by operator/);
		expect(listRecommendationHistory).toHaveBeenCalledTimes(2);
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
});

function makeListResponse(recommendations: Recommendation[]) {
	return create(ListRecommendationsResponseSchema, { recommendations });
}

function makeHistoryResponse(history: RecommendationHistoryEntry[]) {
	return create(ListRecommendationHistoryResponseSchema, { history });
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

function makeHistoryEntry(overrides: Record<string, unknown> = {}): RecommendationHistoryEntry {
	return create(RecommendationHistoryEntrySchema, {
		id: 1n,
		recommendationId: 'REC-001',
		fromStatus: RecommendationStatus.UNSPECIFIED,
		toStatus: RecommendationStatus.PENDING,
		decisionReason: '',
		decidedBy: '',
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
