/**
 * FeedbackIndicator component - shows a 💬 icon for lines with existing feedback.
 *
 * Features:
 * - Shows 💬 icon when feedback exists for a line
 * - Shows count badge when multiple feedbacks exist
 * - Opens popover on click showing feedback details
 * - Truncates long feedback text at 200 chars
 * - Keyboard accessible (Enter/Space to open, Escape to close)
 */

import { useState, useRef, useEffect, useCallback } from 'react';
import type { Feedback } from '@/gen/orc/v1/feedback_pb';
import { FeedbackTiming } from '@/gen/orc/v1/feedback_pb';
import './FeedbackIndicator.css';

interface FeedbackIndicatorProps {
	feedback: Feedback[];
}

const TIMING_LABELS: Record<FeedbackTiming, string> = {
	[FeedbackTiming.UNSPECIFIED]: 'Unspecified',
	[FeedbackTiming.NOW]: 'Send Now',
	[FeedbackTiming.WHEN_DONE]: 'When Done',
	[FeedbackTiming.MANUAL]: 'Manual',
};

const MAX_TEXT_LENGTH = 200;

function truncateText(text: string): string {
	if (text.length <= MAX_TEXT_LENGTH) {
		return text;
	}
	return text.slice(0, MAX_TEXT_LENGTH) + '...';
}

export function FeedbackIndicator({ feedback }: FeedbackIndicatorProps) {
	const [isOpen, setIsOpen] = useState(false);
	const buttonRef = useRef<HTMLButtonElement>(null);
	const popoverRef = useRef<HTMLDivElement>(null);

	// Determine if any feedback is pending (not received)
	const hasPending = feedback.some((f) => !f.received);
	const statusClass = hasPending ? 'pending' : 'received';

	const handleClick = useCallback(() => {
		setIsOpen((prev) => !prev);
	}, []);

	const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
		if (e.key === 'Escape' && isOpen) {
			setIsOpen(false);
			buttonRef.current?.focus();
		}
	}, [isOpen]);

	// Close on click outside
	useEffect(() => {
		if (!isOpen) return;

		const handleClickOutside = (e: MouseEvent) => {
			const target = e.target as Node;
			if (
				popoverRef.current &&
				!popoverRef.current.contains(target) &&
				buttonRef.current &&
				!buttonRef.current.contains(target)
			) {
				setIsOpen(false);
			}
		};

		document.addEventListener('mousedown', handleClickOutside);
		return () => document.removeEventListener('mousedown', handleClickOutside);
	}, [isOpen]);

	// Close on Escape key (global handler)
	useEffect(() => {
		if (!isOpen) return;

		const handleEscape = (e: KeyboardEvent) => {
			if (e.key === 'Escape') {
				setIsOpen(false);
				buttonRef.current?.focus();
			}
		};

		document.addEventListener('keydown', handleEscape);
		return () => document.removeEventListener('keydown', handleEscape);
	}, [isOpen]);

	// Don't render if no feedback (after all hooks)
	if (feedback.length === 0) {
		return null;
	}

	return (
		<div className="feedback-indicator-wrapper" data-testid="feedback-indicator">
			<button
				ref={buttonRef}
				type="button"
				className={`feedback-indicator ${statusClass}`}
				onClick={handleClick}
				onKeyDown={handleKeyDown}
				aria-label={`${feedback.length} feedback comment${feedback.length > 1 ? 's' : ''}`}
				aria-expanded={isOpen}
				aria-haspopup="true"
			>
				<span className="feedback-indicator__icon">💬</span>
				{feedback.length > 1 && (
					<span className="feedback-indicator__count">{feedback.length}</span>
				)}
			</button>

			{isOpen && (
				<div
					ref={popoverRef}
					className="feedback-indicator__popover"
					role="dialog"
					aria-label="Feedback details"
				>
					<div className="feedback-indicator__list">
						{feedback.map((item) => (
							<div key={item.id} className="feedback-indicator__item">
								<div className="feedback-indicator__item-header">
									<span className={`feedback-indicator__status ${item.received ? 'received' : 'pending'}`}>
										{item.received ? 'Received' : 'Pending'}
									</span>
									<span className="feedback-indicator__timing">
										{TIMING_LABELS[item.timing]}
									</span>
								</div>
								<div className="feedback-indicator__item-text" data-testid="feedback-text">
									{truncateText(item.text)}
								</div>
							</div>
						))}
					</div>
				</div>
			)}
		</div>
	);
}
