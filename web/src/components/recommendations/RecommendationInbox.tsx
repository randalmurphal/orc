import { useCallback, useEffect, useMemo, useState } from 'react';
import { Button, Icon } from '@/components/ui';
import { useCurrentProjectId } from '@/stores/projectStore';
import {
	acceptRecommendation,
	discussRecommendation,
	listRecommendations,
	rejectRecommendation,
} from '@/lib/api/recommendation';
import type { Recommendation } from '@/gen/orc/v1/recommendation_pb';
import { RecommendationKind, RecommendationStatus } from '@/gen/orc/v1/recommendation_pb';
import './RecommendationInbox.css';

export function RecommendationInbox() {
	const projectId = useCurrentProjectId() || '';
	const [recommendations, setRecommendations] = useState<Recommendation[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [busyId, setBusyId] = useState<string | null>(null);
	const [contextPacks, setContextPacks] = useState<Record<string, string>>({});

	const loadRecommendations = useCallback(async () => {
		setLoading(true);
		setError(null);
		try {
			const response = await listRecommendations(projectId);
			setRecommendations(response.recommendations);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load recommendations');
		} finally {
			setLoading(false);
		}
	}, [projectId]);

	useEffect(() => {
		loadRecommendations();
	}, [loadRecommendations]);

	const pendingRecommendations = useMemo(
		() => recommendations.filter((recommendation) => recommendation.status === RecommendationStatus.PENDING),
		[recommendations],
	);

	const handleDecision = useCallback(async (
		recommendation: Recommendation,
		action: 'accept' | 'reject' | 'discuss',
	) => {
		const decidedBy = 'operator';
		setBusyId(recommendation.id);
		setError(null);
		try {
			if (action === 'accept') {
				await acceptRecommendation(projectId, recommendation.id, decidedBy, '');
			} else if (action === 'reject') {
				await rejectRecommendation(projectId, recommendation.id, decidedBy, '');
			} else {
				const response = await discussRecommendation(projectId, recommendation.id, decidedBy, '');
				setContextPacks((current) => ({
					...current,
					[recommendation.id]: response.contextPack,
				}));
			}
			await loadRecommendations();
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to update recommendation');
		} finally {
			setBusyId(null);
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
								disabled={busyId === recommendation.id || !canAcceptRecommendation(recommendation.status)}
								onClick={() => handleDecision(recommendation, 'accept')}
							>
								Accept
							</Button>
							<Button
								variant="ghost"
								size="sm"
								disabled={busyId === recommendation.id || !canRejectRecommendation(recommendation.status)}
								onClick={() => handleDecision(recommendation, 'reject')}
							>
								Reject
							</Button>
							<Button
								variant="secondary"
								size="sm"
								disabled={busyId === recommendation.id || !canDiscussRecommendation(recommendation.status)}
								onClick={() => handleDecision(recommendation, 'discuss')}
							>
								Discuss
							</Button>
						</div>

						{contextPacks[recommendation.id] && (
							<pre className="recommendation-card__context-pack">{contextPacks[recommendation.id]}</pre>
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

function recommendationKindLabel(kind: RecommendationKind): string {
	switch (kind) {
		case RecommendationKind.RISK:
			return 'Risk';
		case RecommendationKind.FOLLOW_UP:
			return 'Follow-up';
		case RecommendationKind.DECISION_REQUEST:
			return 'Decision request';
		default:
			return 'Cleanup';
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
