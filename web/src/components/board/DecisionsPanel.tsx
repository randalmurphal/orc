/**
 * DecisionsPanel component for pending decisions
 *
 * Displays pending decisions from running tasks that need user input.
 * Features:
 * - Purple-themed section header with decision icon and count
 * - Each decision shows question, task context, and option buttons
 * - Recommended option highlighted (first option or explicitly marked)
 * - Loading state while submitting decisions
 * - Empty state when no decisions (component hidden)
 */

import { useState, useCallback } from 'react';
import { RightPanel } from '@/components/layout/RightPanel';
import { Button } from '@/components/ui';
import type { PendingDecision } from '@/gen/orc/v1/decision_pb';
import './DecisionsPanel.css';

export interface DecisionsPanelProps {
	/** Array of pending decisions awaiting user input */
	decisions: PendingDecision[];
	/** Callback when user selects an option for a decision */
	onDecide: (decisionId: string, optionId: string) => void;
}

/**
 * DecisionsPanel displays pending decisions from running tasks.
 * Hidden when there are no pending decisions.
 */
export function DecisionsPanel({ decisions, onDecide }: DecisionsPanelProps) {
	// Track which decisions are currently being submitted
	const [submittingDecisions, setSubmittingDecisions] = useState<Set<string>>(new Set());

	const handleOptionClick = useCallback(
		async (decisionId: string, optionId: string) => {
			// Mark as submitting
			setSubmittingDecisions((prev) => new Set(prev).add(decisionId));

			try {
				await onDecide(decisionId, optionId);
			} finally {
				// Remove from submitting set after completion
				setSubmittingDecisions((prev) => {
					const next = new Set(prev);
					next.delete(decisionId);
					return next;
				});
			}
		},
		[onDecide]
	);

	// Don't render if no decisions
	if (decisions.length === 0) {
		return null;
	}

	return (
		<RightPanel.Section id="decisions">
			<RightPanel.Header
				title="Decisions"
				icon="help"
				iconColor="purple"
				count={decisions.length}
				badgeColor="purple"
			/>
			<RightPanel.Body>
				<div className="decisions-panel-list">
					{decisions.map((decision) => {
						const isSubmitting = submittingDecisions.has(decision.id);

						return (
							<div
								key={decision.id}
								className={`decision-item ${isSubmitting ? 'submitting' : ''}`}
								aria-busy={isSubmitting}
							>
								<div className="decision-header">
									<div className="decision-icon">
										<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
											<path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z" />
										</svg>
									</div>
									<div className="decision-content">
										<span className="decision-task">{decision.taskId}</span>
										<span className="decision-question">{decision.question}</span>
									</div>
								</div>
								<div className="decision-options">
									{decision.options.map((option, index) => {
										// First option is recommended by default, unless explicitly marked
										const isRecommended =
											option.recommended || (index === 0 && !decision.options.some((o) => o.recommended));

										return (
											<Button
												key={option.id}
												variant={isRecommended ? 'primary' : 'ghost'}
												size="sm"
												className={`decision-option ${isRecommended ? 'recommended' : ''}`}
												onClick={() => handleOptionClick(decision.id, option.id)}
												disabled={isSubmitting}
												title={option.description}
												aria-label={`${option.label}${isRecommended ? ' (recommended)' : ''}`}
											>
												{option.label}
											</Button>
										);
									})}
								</div>
							</div>
						);
					})}
				</div>
			</RightPanel.Body>
		</RightPanel.Section>
	);
}
