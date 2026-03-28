import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Button, Icon } from '@/components/ui';
import { useCurrentProjectId } from '@/stores/projectStore';
import {
	acceptRecommendation,
	discussRecommendation,
	listRecommendationHistory,
	listRecommendations,
	rejectRecommendation,
} from '@/lib/api/recommendation';
import type { Recommendation, RecommendationHistoryEntry } from '@/gen/orc/v1/recommendation_pb';
import { RecommendationStatus } from '@/gen/orc/v1/recommendation_pb';
import { onRecommendationSignal } from '@/lib/events/recommendationSignals';
import { recommendationKindLabel } from '@/lib/recommendations';
import { timestampToDate } from '@/lib/time';
import './RecommendationInbox.css';

export function RecommendationInbox() {
	const projectId = useCurrentProjectId() || '';
	const [recommendations, setRecommendations] = useState<Recommendation[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [busyId, setBusyId] = useState<string | null>(null);
	const [contextPacks, setContextPacks] = useState<Record<string, string>>({});
	const currentProjectIdRef = useRef(projectId);
	const latestLoadRequestIdRef = useRef(0);
	currentProjectIdRef.current = projectId;

	const isCurrentProjectRequest = useCallback((requestId: number, requestProjectId: string) => (
		currentProjectIdRef.current === requestProjectId && latestLoadRequestIdRef.current === requestId
	), []);

	const [decisionNotes, setDecisionNotes] = useState<Record<string, string>>({});
	const [expandedHistory, setExpandedHistory] = useState<Record<string, boolean>>({});
	const [historyById, setHistoryById] = useState<Record<string, RecommendationHistoryEntry[]>>({});
	const [historyLoadingId, setHistoryLoadingId] = useState<string | null>(null);

	const invalidateHistory = useCallback((recommendationId: string) => {
		setHistoryById((current) => {
			if (!(recommendationId in current)) {
				return current;
			}
			const next = { ...current };
			delete next[recommendationId];
			return next;
		});
		setExpandedHistory((current) => {
			if (!current[recommendationId]) {
				return current;
			}
			return {
				...current,
				[recommendationId]: false,
			};
		});
	}, []);

	const loadRecommendations = useCallback(async () => {
		const requestId = latestLoadRequestIdRef.current + 1;
		latestLoadRequestIdRef.current = requestId;
		const requestProjectId = projectId;
		setLoading(true);
		setError(null);
		try {
			const response = await listRecommendations(requestProjectId);
			if (!isCurrentProjectRequest(requestId, requestProjectId)) {
				return;
			}
			setRecommendations(response.recommendations);
		} catch (err) {
			if (!isCurrentProjectRequest(requestId, requestProjectId)) {
				return;
			}
			setError(err instanceof Error ? err.message : 'Failed to load recommendations');
		} finally {
			if (isCurrentProjectRequest(requestId, requestProjectId)) {
				setLoading(false);
			}
		}
	}, [isCurrentProjectRequest, projectId]);

	useEffect(() => {
		loadRecommendations();
	}, [loadRecommendations]);

	useEffect(() => {
		setRecommendations([]);
		setContextPacks({});
		setBusyId(null);
		setError(null);
	}, [projectId]);

	useEffect(() => {
		return onRecommendationSignal((signal) => {
			if (signal.projectId !== projectId) {
				return;
			}
			invalidateHistory(signal.recommendationId);
			void loadRecommendations();
		});
	}, [invalidateHistory, loadRecommendations, projectId]);

	const pendingRecommendations = useMemo(
		() => recommendations.filter((recommendation) => recommendation.status === RecommendationStatus.PENDING),
		[recommendations],
	);

	const handleDecision = useCallback(async (
		recommendation: Recommendation,
		action: 'accept' | 'reject' | 'discuss',
	) => {
		const decidedBy = 'operator';
		const decisionReason = (decisionNotes[recommendation.id] ?? '').trim();
		const decisionProjectId = projectId;
		const stateKey = recommendationStateKey(decisionProjectId, recommendation.id);
		setBusyId(stateKey);
		setError(null);
		try {
			if (action === 'accept') {
				await acceptRecommendation(decisionProjectId, recommendation.id, decidedBy, decisionReason);
			} else if (action === 'reject') {
				await rejectRecommendation(decisionProjectId, recommendation.id, decidedBy, decisionReason);
			} else {
				const response = await discussRecommendation(decisionProjectId, recommendation.id, decidedBy, decisionReason);
				if (currentProjectIdRef.current !== decisionProjectId) {
					return;
				}
				setContextPacks((current) => ({
					...current,
					[stateKey]: response.contextPack,
				}));
			}
			if (currentProjectIdRef.current !== decisionProjectId) {
				return;
			}
			setDecisionNotes((current) => ({
				...current,
				[recommendation.id]: '',
			}));
			invalidateHistory(recommendation.id);
			await loadRecommendations();
		} catch (err) {
			if (currentProjectIdRef.current === decisionProjectId) {
				setError(err instanceof Error ? err.message : 'Failed to update recommendation');
			}
		} finally {
			if (currentProjectIdRef.current === decisionProjectId) {
				setBusyId(null);
			}
		}
	}, [decisionNotes, invalidateHistory, loadRecommendations, projectId]);

	const toggleHistory = useCallback(async (recommendationId: string) => {
		if (expandedHistory[recommendationId]) {
			setExpandedHistory((current) => ({
				...current,
				[recommendationId]: false,
			}));
			return;
		}

		if (historyById[recommendationId]) {
			setExpandedHistory((current) => ({
				...current,
				[recommendationId]: true,
			}));
			return;
		}

		setHistoryLoadingId(recommendationId);
		setError(null);
		try {
			const response = await listRecommendationHistory(projectId, recommendationId);
			setHistoryById((current) => ({
				...current,
				[recommendationId]: response.history,
			}));
			setExpandedHistory((current) => ({
				...current,
				[recommendationId]: true,
			}));
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load recommendation history');
		} finally {
			setHistoryLoadingId(null);
		}
	}, [expandedHistory, historyById, projectId]);

	if (loading) {
		return <div className="recommendation-inbox__state">Loading recommendations...</div>;
	}

	if (error) {
		return (
			<div className="recommendation-inbox__state">
				<p>{error}</p>
				<Button variant="secondary" size="sm" onClick={loadRecommendations}>Retry</Button>
			</div>
		);
	}

	if (recommendations.length === 0) {
		return <div className="recommendation-inbox__state">No recommendations yet.</div>;
	}

	return (
		<div className="recommendation-inbox">
			<div className="recommendation-inbox__header">
				<div>
					<h1>Recommendation Inbox</h1>
					<p>{pendingRecommendations.length} pending recommendations need a human decision.</p>
				</div>
				<Button variant="ghost" size="sm" onClick={loadRecommendations}>
					<Icon name="refresh" size={14} />
					Refresh
				</Button>
			</div>

			<div className="recommendation-inbox__list">
				{recommendations.map((recommendation) => (
					<article key={recommendation.id} className="recommendation-card">
						<div className="recommendation-card__meta">
							<span className={`recommendation-card__status recommendation-card__status--${recommendationStatusClass(recommendation.status)}`}>
								{recommendationStatusLabel(recommendation.status)}
							</span>
							<span className="recommendation-card__kind">{recommendationKindLabel(recommendation.kind)}</span>
							<span className="recommendation-card__source">{recommendation.sourceTaskId}</span>
						</div>

						<div className="recommendation-card__body">
							<h2>{recommendation.title}</h2>
							<p>{recommendation.summary}</p>
						</div>

						<div className="recommendation-card__detail">
							<strong>Proposed action</strong>
							<p>{recommendation.proposedAction}</p>
						</div>

						<div className="recommendation-card__detail">
							<strong>Evidence</strong>
							<p>{recommendation.evidence}</p>
						</div>

						<label className="recommendation-card__note">
							<span>Decision note</span>
							<textarea
								value={decisionNotes[recommendation.id] ?? ''}
								onChange={(event) => {
									const value = event.target.value;
									setDecisionNotes((current) => ({
										...current,
										[recommendation.id]: value,
									}));
								}}
								disabled={busyId === recommendationStateKey(projectId, recommendation.id)}
								placeholder="Optional rationale for the acceptance, rejection, or discussion request."
								rows={3}
							/>
						</label>

						<div className="recommendation-card__actions">
							<Button
								variant="primary"
								size="sm"
								disabled={busyId === recommendationStateKey(projectId, recommendation.id) || !canAcceptRecommendation(recommendation.status)}
								onClick={() => handleDecision(recommendation, 'accept')}
							>
								Accept
							</Button>
							<Button
								variant="ghost"
								size="sm"
								disabled={busyId === recommendationStateKey(projectId, recommendation.id) || !canRejectRecommendation(recommendation.status)}
								onClick={() => handleDecision(recommendation, 'reject')}
							>
								Reject
							</Button>
							<Button
								variant="secondary"
								size="sm"
								disabled={busyId === recommendationStateKey(projectId, recommendation.id) || !canDiscussRecommendation(recommendation.status)}
								onClick={() => handleDecision(recommendation, 'discuss')}
							>
								Discuss
							</Button>
						</div>

						{recommendation.decisionReason && (
							<div className="recommendation-card__detail">
								<strong>Decision note</strong>
								<p>{recommendation.decisionReason}</p>
							</div>
						)}

						{(recommendation.decidedBy || recommendation.decidedAt) && (
							<div className="recommendation-card__detail">
								<strong>Decision</strong>
								<p>{formatDecisionSummary(recommendation)}</p>
							</div>
						)}

						{recommendation.promotedToType && recommendation.promotedToId && (
							<div className="recommendation-card__detail">
								<strong>Promoted artifact</strong>
								<p>{formatPromotedArtifact(recommendation)}</p>
							</div>
						)}

						<div className="recommendation-card__history-toggle">
							<Button
								variant="ghost"
								size="sm"
								disabled={historyLoadingId === recommendation.id}
								onClick={() => void toggleHistory(recommendation.id)}
							>
								{expandedHistory[recommendation.id] ? 'Hide history' : 'Show history'}
							</Button>
						</div>

						{historyLoadingId === recommendation.id && (
							<div className="recommendation-card__detail">
								<p>Loading history...</p>
							</div>
						)}

						{expandedHistory[recommendation.id] && historyById[recommendation.id] && (
							<div className="recommendation-card__detail">
								<strong>Decision history</strong>
								<ol className="recommendation-card__history">
									{historyById[recommendation.id].map((entry) => (
										<li key={entry.id}>
											{formatRecommendationHistoryEntry(entry)}
										</li>
									))}
								</ol>
							</div>
						)}

						{contextPacks[recommendationStateKey(projectId, recommendation.id)] && (
							<pre className="recommendation-card__context-pack">{contextPacks[recommendationStateKey(projectId, recommendation.id)]}</pre>
						)}
					</article>
				))}
			</div>
		</div>
	);
}

function recommendationStatusClass(status: RecommendationStatus): string {
	switch (status) {
		case RecommendationStatus.ACCEPTED:
			return 'accepted';
		case RecommendationStatus.REJECTED:
			return 'rejected';
		case RecommendationStatus.DISCUSSED:
			return 'discussed';
		default:
			return 'pending';
	}
}

function recommendationStatusLabel(status: RecommendationStatus): string {
	switch (status) {
		case RecommendationStatus.ACCEPTED:
			return 'Accepted';
		case RecommendationStatus.REJECTED:
			return 'Rejected';
		case RecommendationStatus.DISCUSSED:
			return 'Discussed';
		default:
			return 'Pending';
	}
}

function canAcceptRecommendation(status: RecommendationStatus): boolean {
	return status === RecommendationStatus.PENDING || status === RecommendationStatus.DISCUSSED;
}

function canRejectRecommendation(status: RecommendationStatus): boolean {
	return status === RecommendationStatus.PENDING || status === RecommendationStatus.DISCUSSED;
}

function canDiscussRecommendation(status: RecommendationStatus): boolean {
	return status === RecommendationStatus.PENDING;
}

function recommendationStateKey(projectId: string, recommendationId: string): string {
	return `${projectId}:${recommendationId}`;
}

function formatDecisionSummary(recommendation: Recommendation): string {
	const parts: string[] = [];
	if (recommendation.decidedBy) {
		parts.push(`by ${recommendation.decidedBy}`);
	}
	const decidedAt = timestampToDate(recommendation.decidedAt);
	if (decidedAt) {
		parts.push(`on ${decidedAt.toLocaleString()}`);
	}
	return parts.join(' ');
}

function formatPromotedArtifact(recommendation: Recommendation): string {
	if (!recommendation.promotedToType || !recommendation.promotedToId) {
		return '';
	}
	if (recommendation.promotedToType === 'initiative_decision') {
		return `Initiative decision ${recommendation.promotedToId}`;
	}
	return `Task ${recommendation.promotedToId}`;
}

function formatRecommendationHistoryEntry(entry: RecommendationHistoryEntry): string {
	const parts = [recommendationStatusLabel(entry.toStatus)];
	if (entry.fromStatus !== RecommendationStatus.UNSPECIFIED) {
		parts.push(`from ${recommendationStatusLabel(entry.fromStatus).toLowerCase()}`);
	}
	if (entry.decidedBy) {
		parts.push(`by ${entry.decidedBy}`);
	}
	const createdAt = timestampToDate(entry.createdAt);
	if (createdAt) {
		parts.push(`on ${createdAt.toLocaleString()}`);
	}
	if (entry.decisionReason) {
		parts.push(`(${entry.decisionReason})`);
	}
	return parts.join(' ');
}
