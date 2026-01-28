/**
 * DecisionsPanel component for pending decisions
 *
 * Displays pending decisions from running tasks that need user input.
 * Features:
 * - Purple-themed section header with decision icon and count
 * - Each decision shows question, task context, and option buttons
 * - Recommended option highlighted (first option or explicitly marked)
 * - Loading state while submitting decisions
 * - Empty state when no decisions
 */

import { useState, useCallback } from 'react';
import { Icon } from '@/components/ui/Icon';
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
 * Always renders (shows empty state when no decisions).
 */
export function DecisionsPanel({ decisions, onDecide }: DecisionsPanelProps) {
	// Track which decisions are currently being submitted
	const [submittingDecisions, setSubmittingDecisions] = useState<Set<string>>(new Set());
	const [collapsed, setCollapsed] = useState(false);

	const handleToggle = useCallback(() => {
		setCollapsed((prev) => !prev);
	}, []);

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

	const decisionCount = decisions.length;

	return (
		<div className={`decisions-panel panel-section ${collapsed ? 'collapsed' : ''}`}>
			<button
				className="panel-header"
				onClick={handleToggle}
				aria-expanded={!collapsed}
				aria-controls="decisions-panel-body"
			>
				<div className="panel-title">
					<div className="panel-title-icon purple">
						<Icon name="help" size={12} />
					</div>
					<span>Decisions</span>
				</div>
				{decisionCount > 0 && (
					<span className="panel-badge purple" aria-label={`${decisionCount} pending decisions`}>
						{decisionCount}
					</span>
				)}
				<Icon
					name={collapsed ? 'chevron-right' : 'chevron-down'}
					size={12}
					className="panel-chevron"
				/>
			</button>

			<div id="decisions-panel-body" className="panel-body" role="region">
				{decisions.length === 0 ? (
					<div className="decisions-empty">No pending decisions</div>
				) : (
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
				)}
			</div>
		</div>
	);
}
