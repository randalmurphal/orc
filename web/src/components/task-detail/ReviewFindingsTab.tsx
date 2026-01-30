/**
 * ReviewFindingsTab - Displays structured review phase findings.
 * Shows issues, positives, and questions from each review round.
 */

import { useState, useEffect, useCallback } from 'react';
import { taskClient } from '@/lib/client';
import type { ReviewFinding, ReviewRoundFindings } from '@/gen/orc/v1/task_pb';
import { timestampToDate } from '@/lib/time';
import { Icon, type IconName } from '@/components/ui/Icon';
import { Button } from '@/components/ui/Button';
import { useCurrentProjectId } from '@/stores';
import './ReviewFindingsTab.css';

interface ReviewFindingsTabProps {
	taskId: string;
}

// Map severity to color scheme
const severityConfig: Record<string, { color: string; bg: string; icon: IconName }> = {
	critical: { color: 'var(--red)', bg: 'rgba(var(--red-rgb), 0.1)', icon: 'alert-circle' },
	high: { color: 'var(--amber)', bg: 'rgba(var(--amber-rgb), 0.1)', icon: 'alert-triangle' },
	medium: { color: 'var(--blue)', bg: 'rgba(var(--blue-rgb), 0.1)', icon: 'info' },
	low: { color: 'var(--text-muted)', bg: 'var(--bg-tertiary)', icon: 'circle' },
};

function formatRelativeTime(date: Date | null | undefined): string {
	if (!date) return '';
	const now = new Date();
	const diffMs = now.getTime() - date.getTime();
	const diffSec = Math.floor(diffMs / 1000);
	const diffMin = Math.floor(diffSec / 60);
	const diffHour = Math.floor(diffMin / 60);
	const diffDay = Math.floor(diffHour / 24);

	if (diffSec < 60) return 'just now';
	if (diffMin < 60) return `${diffMin}m ago`;
	if (diffHour < 24) return `${diffHour}h ago`;
	if (diffDay < 7) return `${diffDay}d ago`;

	return date.toLocaleDateString(undefined, {
		month: 'short',
		day: 'numeric',
	});
}

interface FindingCardProps {
	finding: ReviewFinding;
}

function FindingCard({ finding }: FindingCardProps) {
	const severity = severityConfig[finding.severity] || severityConfig.medium;
	const hasConstitutionViolation = finding.constitutionViolation;

	return (
		<div className="finding-card" style={{ borderLeftColor: severity.color }}>
			<div className="finding-header">
				<div className="finding-severity" style={{ background: severity.bg, color: severity.color }}>
					<Icon name={severity.icon} size={14} />
					<span>{finding.severity}</span>
				</div>
				{hasConstitutionViolation && (
					<div className={`constitution-badge ${finding.constitutionViolation}`}>
						<Icon name="shield" size={12} />
						<span>{finding.constitutionViolation === 'invariant' ? 'Invariant Violation' : 'Default Deviation'}</span>
					</div>
				)}
				{finding.file && (
					<div className="finding-location">
						<Icon name="file" size={12} />
						<code>{finding.file}{finding.line ? `:${finding.line}` : ''}</code>
					</div>
				)}
			</div>

			<div className="finding-description">{finding.description}</div>

			{finding.suggestion && (
				<div className="finding-suggestion">
					<Icon name="chevron-right" size={12} />
					<span>{finding.suggestion}</span>
				</div>
			)}

			{finding.agentId && (
				<div className="finding-agent">
					<span className="agent-label">Reviewer:</span>
					<span>{finding.agentId}</span>
				</div>
			)}
		</div>
	);
}

interface RoundSectionProps {
	findings: ReviewRoundFindings;
	isExpanded: boolean;
	onToggle: () => void;
}

function RoundSection({ findings, isExpanded, onToggle }: RoundSectionProps) {
	const issueCount = findings.issues?.length || 0;
	const positiveCount = findings.positives?.length || 0;
	const questionCount = findings.questions?.length || 0;

	const hasInvariantViolation = findings.issues?.some(i => i.constitutionViolation === 'invariant');

	return (
		<div className={`round-section ${isExpanded ? 'expanded' : ''} ${hasInvariantViolation ? 'has-blocker' : ''}`}>
			<Button variant="ghost" className="round-header" onClick={onToggle}>
				<div className="round-title">
					<Icon name={isExpanded ? 'chevron-down' : 'chevron-right'} size={16} />
					<span>Round {findings.round}</span>
					{findings.agentId && (
						<span className="round-agent">{findings.agentId}</span>
					)}
				</div>
				<div className="round-stats">
					{issueCount > 0 && (
						<span className={`stat issues ${hasInvariantViolation ? 'blocker' : ''}`}>
							<Icon name="alert-circle" size={12} />
							{issueCount} {issueCount === 1 ? 'issue' : 'issues'}
						</span>
					)}
					{positiveCount > 0 && (
						<span className="stat positives">
							<Icon name="check-circle" size={12} />
							{positiveCount}
						</span>
					)}
					{questionCount > 0 && (
						<span className="stat questions">
							<Icon name="help" size={12} />
							{questionCount}
						</span>
					)}
					<span className="timestamp">{formatRelativeTime(timestampToDate(findings.createdAt))}</span>
				</div>
			</Button>

			{isExpanded && (
				<div className="round-content">
					{findings.summary && (
						<div className="round-summary">
							<p>{findings.summary}</p>
						</div>
					)}

					{issueCount > 0 && (
						<div className="findings-section issues">
							<h4>
								<Icon name="alert-circle" size={14} />
								Issues ({issueCount})
							</h4>
							<div className="findings-list">
								{findings.issues.map((finding, idx) => (
									<FindingCard key={idx} finding={finding} />
								))}
							</div>
						</div>
					)}

					{positiveCount > 0 && (
						<div className="findings-section positives">
							<h4>
								<Icon name="check-circle" size={14} />
								What's Good ({positiveCount})
							</h4>
							<ul className="positives-list">
								{findings.positives.map((positive, idx) => (
									<li key={idx}>{positive}</li>
								))}
							</ul>
						</div>
					)}

					{questionCount > 0 && (
						<div className="findings-section questions">
							<h4>
								<Icon name="help" size={14} />
								Questions ({questionCount})
							</h4>
							<ul className="questions-list">
								{findings.questions.map((question, idx) => (
									<li key={idx}>{question}</li>
								))}
							</ul>
						</div>
					)}
				</div>
			)}
		</div>
	);
}

export function ReviewFindingsTab({ taskId }: ReviewFindingsTabProps) {
	const projectId = useCurrentProjectId();
	const [findings, setFindings] = useState<ReviewRoundFindings[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [expandedRounds, setExpandedRounds] = useState<Set<number>>(new Set());

	const loadFindings = useCallback(async () => {
		if (!projectId) return;
		setLoading(true);
		setError(null);
		try {
			const response = await taskClient.getReviewFindings({ projectId, taskId });
			setFindings(response.rounds);
			// Expand the latest round by default
			if (response.rounds.length > 0) {
				const latestRound = Math.max(...response.rounds.map(f => f.round));
				setExpandedRounds(new Set([latestRound]));
			}
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load review findings');
		} finally {
			setLoading(false);
		}
	}, [projectId, taskId]);

	useEffect(() => {
		loadFindings();
	}, [loadFindings]);

	const toggleRound = useCallback((round: number) => {
		setExpandedRounds(prev => {
			const next = new Set(prev);
			if (next.has(round)) {
				next.delete(round);
			} else {
				next.add(round);
			}
			return next;
		});
	}, []);

	// Aggregate stats
	const totalIssues = findings.reduce((sum, f) => sum + (f.issues?.length || 0), 0);
	const hasBlockers = findings.some(f => f.issues?.some(i => i.constitutionViolation === 'invariant'));

	if (loading) {
		return (
			<div className="review-findings-panel">
				<div className="loading-state">
					<div className="spinner" />
					<span>Loading review findings...</span>
				</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className="review-findings-panel">
				<div className="error-message">
					<Icon name="alert-circle" size={14} />
					{error}
					<Button variant="secondary" size="sm" onClick={loadFindings}>Retry</Button>
				</div>
			</div>
		);
	}

	if (findings.length === 0) {
		return (
			<div className="review-findings-panel">
				<div className="panel-header">
					<div className="header-left">
						<h3>
							<Icon name="search" size={16} />
							Review Findings
						</h3>
					</div>
				</div>
				<div className="empty-state">
					<Icon name="search" size={32} />
					<p>No review findings</p>
					<span>Review findings will appear here after a review phase completes.</span>
				</div>
			</div>
		);
	}

	return (
		<div className="review-findings-panel">
			<div className="panel-header">
				<div className="header-left">
					<h3>
						<Icon name="search" size={16} />
						Review Findings
						<span className="round-count">{findings.length} {findings.length === 1 ? 'round' : 'rounds'}</span>
					</h3>
				</div>
				<div className="header-right">
					{totalIssues > 0 && (
						<span className={`total-issues ${hasBlockers ? 'has-blockers' : ''}`}>
							{hasBlockers && <Icon name="shield" size={12} />}
							{totalIssues} total {totalIssues === 1 ? 'issue' : 'issues'}
						</span>
					)}
				</div>
			</div>

			<div className="rounds-list">
				{findings.map((f) => (
					<RoundSection
						key={f.round}
						findings={f}
						isExpanded={expandedRounds.has(f.round)}
						onToggle={() => toggleRound(f.round)}
					/>
				))}
			</div>
		</div>
	);
}
