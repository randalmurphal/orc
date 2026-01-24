/**
 * TranscriptNav - Sidebar navigation for transcript phases and iterations.
 *
 * Displays a list of phases with expand/collapse functionality for iterations.
 * Supports navigation callbacks and highlights the current position.
 */

import { useState, useCallback, type KeyboardEvent } from 'react';
import { Icon } from '@/components/ui/Icon';
import './TranscriptNav.css';

export interface TranscriptNavPhase {
	phase: string;
	iterations: number;
	transcript_count: number;
	status: 'completed' | 'failed' | 'running' | 'pending';
}

export interface TranscriptNavProps {
	phases: TranscriptNavPhase[];
	currentPhase?: string;
	currentIteration?: number;
	onNavigate: (phase: string, iteration?: number) => void;
	testId?: string;
}

/**
 * TranscriptNav component for phase/iteration navigation.
 *
 * @example
 * // Basic usage
 * <TranscriptNav
 *   phases={phases}
 *   onNavigate={(phase, iteration) => console.log(phase, iteration)}
 * />
 *
 * @example
 * // With current position
 * <TranscriptNav
 *   phases={phases}
 *   currentPhase="implement"
 *   currentIteration={2}
 *   onNavigate={handleNavigate}
 * />
 */
export function TranscriptNav({
	phases,
	currentPhase,
	currentIteration,
	onNavigate,
	testId,
}: TranscriptNavProps) {
	// Track which phases are manually expanded (user toggled)
	const [manuallyExpanded, setManuallyExpanded] = useState<Set<string>>(new Set());
	// Track which phases have been manually collapsed
	const [manuallyCollapsed, setManuallyCollapsed] = useState<Set<string>>(new Set());

	// A phase is expanded if:
	// 1. It's the current phase and not manually collapsed, OR
	// 2. It was manually expanded
	const isPhaseExpanded = useCallback(
		(phase: string): boolean => {
			if (manuallyCollapsed.has(phase)) return false;
			if (manuallyExpanded.has(phase)) return true;
			return phase === currentPhase;
		},
		[currentPhase, manuallyExpanded, manuallyCollapsed]
	);

	const handlePhaseClick = useCallback(
		(phase: string) => {
			const expanded = isPhaseExpanded(phase);

			if (expanded) {
				// Collapse: remove from manually expanded, add to manually collapsed
				setManuallyExpanded((prev) => {
					const next = new Set(prev);
					next.delete(phase);
					return next;
				});
				setManuallyCollapsed((prev) => new Set(prev).add(phase));
			} else {
				// Expand: add to manually expanded, remove from manually collapsed
				setManuallyExpanded((prev) => new Set(prev).add(phase));
				setManuallyCollapsed((prev) => {
					const next = new Set(prev);
					next.delete(phase);
					return next;
				});
			}

			onNavigate(phase, undefined);
		},
		[isPhaseExpanded, onNavigate]
	);

	const handlePhaseKeyDown = useCallback(
		(e: KeyboardEvent<HTMLButtonElement>, phase: string) => {
			if (e.key === 'Enter') {
				handlePhaseClick(phase);
			}
		},
		[handlePhaseClick]
	);

	const handleIterationClick = useCallback(
		(phase: string, iteration: number) => {
			onNavigate(phase, iteration);
		},
		[onNavigate]
	);

	return (
		<nav className="transcript-nav" data-testid={testId}>
			{phases.map((phaseData) => {
				const isActive = phaseData.phase === currentPhase;
				const isExpanded = isPhaseExpanded(phaseData.phase);
				const hasIterations = phaseData.iterations > 0;

				return (
					<div
						key={phaseData.phase}
						className={`nav-phase ${isActive ? 'nav-phase--active' : ''}`}
						data-phase={phaseData.phase}
						data-status={phaseData.status}
					>
						<button
							type="button"
							className="nav-phase-button"
							onClick={() => handlePhaseClick(phaseData.phase)}
							onKeyDown={(e) => handlePhaseKeyDown(e, phaseData.phase)}
							aria-expanded={isExpanded}
							tabIndex={0}
						>
							{/* Expand/collapse chevron */}
							{hasIterations && (
								<span className="nav-phase-chevron">
									<Icon name={isExpanded ? 'chevron-down' : 'chevron-right'} size={12} />
								</span>
							)}
							{!hasIterations && <span className="nav-phase-chevron-placeholder" />}

							{/* Status indicator */}
							<span className={`nav-phase-status nav-phase-status--${phaseData.status}`}>
								<StatusIndicator status={phaseData.status} />
							</span>

							{/* Phase name */}
							<span className="nav-phase-name">{phaseData.phase}</span>
						</button>

						{/* Iterations list */}
						{isExpanded && hasIterations && (
							<div className="nav-iterations">
								{Array.from({ length: phaseData.iterations }, (_, i) => i + 1).map(
									(iteration) => {
										const isIterationActive =
											isActive && currentIteration === iteration;

										return (
											<button
												key={iteration}
												type="button"
												className={`nav-iteration ${isIterationActive ? 'nav-iteration--active' : ''}`}
												data-iteration={iteration}
												onClick={() =>
													handleIterationClick(phaseData.phase, iteration)
												}
												tabIndex={0}
											>
												Iteration {iteration}
											</button>
										);
									}
								)}
							</div>
						)}
					</div>
				);
			})}
		</nav>
	);
}

/**
 * Renders the appropriate status indicator icon.
 */
function StatusIndicator({ status }: { status: TranscriptNavPhase['status'] }) {
	switch (status) {
		case 'completed':
			return <Icon name="check" size={10} />;
		case 'failed':
			return <Icon name="x" size={10} />;
		case 'running':
			return <span className="nav-status-dot" />;
		case 'pending':
			return <span className="nav-status-circle" />;
	}
}
