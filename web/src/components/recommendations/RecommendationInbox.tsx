import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Button, Icon } from '@/components/ui';
import { useCurrentProjectId } from '@/stores/projectStore';
import {
	acceptRecommendation,
	discussRecommendation,
	listRecommendations,
	rejectRecommendation,
} from '@/lib/api/recommendation';
import type { Recommendation } from '@/gen/orc/v1/recommendation_pb';
import { RecommendationStatus } from '@/gen/orc/v1/recommendation_pb';
import { onRecommendationSignal } from '@/lib/events/recommendationSignals';
import { recommendationKindLabel } from '@/lib/recommendations';
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
			void loadRecommendations();
		});
	}, [loadRecommendations, projectId]);

	const pendingRecommendations = useMemo(
		() => recommendations.filter((recommendation) => recommendation.status === RecommendationStatus.PENDING),
		[recommendations],
	);

	const handleDecision = useCallback(async (
		recommendation: Recommendation,
		action: 'accept' | 'reject' | 'discuss',
	) => {
		const decidedBy = '';
		const decisionProjectId = projectId;
		const stateKey = recommendationStateKey(decisionProjectId, recommendation.id);
		setBusyId(stateKey);
		setError(null);
		try {
			if (action === 'accept') {
				await acceptRecommendation(decisionProjectId, recommendation.id, decidedBy, '');
			} else if (action === 'reject') {
				await rejectRecommendation(decisionProjectId, recommendation.id, decidedBy, '');
			} else {
				const response = await discussRecommendation(decisionProjectId, recommendation.id, decidedBy, '');
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
	}, [loadRecommendations, projectId]);

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
