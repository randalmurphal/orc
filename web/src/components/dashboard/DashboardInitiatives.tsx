/**
 * DashboardInitiatives component - displays active initiatives with progress bars.
 * Clicking an initiative filters the board by that initiative.
 *
 * Progress is calculated from the task store (same as Sidebar) rather than
 * relying on initiative.tasks which may not be populated by the API.
 */

import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import type { Initiative } from '@/lib/types';
import { useTaskStore, useInitiativeStore } from '@/stores';
import { Button } from '@/components/ui/Button';
import './DashboardInitiatives.css';

interface DashboardInitiativesProps {
	initiatives: Initiative[];
}

interface ProgressInfo {
	completed: number;
	total: number;
	percent: number;
}

function getProgressColor(percent: number): string {
	if (percent >= 75) return 'progress-high';
	if (percent >= 25) return 'progress-medium';
	return 'progress-low';
}

function truncateTitle(title: string, maxLength: number = 30): string {
	if (title.length <= maxLength) return title;
	return title.slice(0, maxLength - 1) + '…';
}

export function DashboardInitiatives({ initiatives }: DashboardInitiativesProps) {
	const navigate = useNavigate();
	const tasks = useTaskStore((state) => state.tasks);
	const getInitiativeProgress = useInitiativeStore((state) => state.getInitiativeProgress);

	// Calculate progress from task store (same approach as Sidebar)
	// This ensures consistent progress counts across the UI
	const progressMap = useMemo(() => {
		const progress = getInitiativeProgress(tasks);
		// Convert to ProgressInfo format with percent
		const result = new Map<string, ProgressInfo>();
		for (const [id, p] of progress) {
			const percent = p.total > 0 ? Math.round((p.completed / p.total) * 100) : 0;
			result.set(id, { completed: p.completed, total: p.total, percent });
		}
		return result;
	}, [getInitiativeProgress, tasks]);

	const getProgress = (initiativeId: string): ProgressInfo => {
		return progressMap.get(initiativeId) || { completed: 0, total: 0, percent: 0 };
	};

	if (initiatives.length === 0) {
		return null;
	}

	// Sort by updated_at descending, take top 5
	const sortedInitiatives = [...initiatives]
		.sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())
		.slice(0, 5);

	const hasMore = initiatives.length > 5;

	const handleInitiativeClick = (initiativeId: string) => {
		navigate(`/board?initiative=${initiativeId}`);
	};

	const handleViewAll = () => {
		navigate('/board');
	};

	return (
		<section className="initiatives-section">
			<div className="section-header">
				<h2 className="section-title">Active Initiatives</h2>
				<span className="section-count">{initiatives.length}</span>
			</div>

			<div className="initiatives-list">
				{sortedInitiatives.map((initiative) => {
					const progress = getProgress(initiative.id);
					const tooltip = initiative.vision
						? `${initiative.title}\n\n${initiative.vision}`
						: initiative.title;

					return (
						<button
							key={initiative.id}
							className="initiative-row"
							onClick={() => handleInitiativeClick(initiative.id)}
							title={tooltip}
						>
							<span className="initiative-title">{truncateTitle(initiative.title)}</span>
							{initiative.status !== 'active' ? (
								<span className={`initiative-status status-${initiative.status}`}>
									{initiative.status}
								</span>
							) : (
								<div className="progress-container">
									<div className="progress-bar">
										<div
											className={`progress-fill ${getProgressColor(progress.percent)}`}
											style={{ width: `${progress.percent}%` }}
										/>
									</div>
									<span className="progress-count">
										{progress.completed}/{progress.total}
									</span>
								</div>
							)}
						</button>
					);
				})}
			</div>

			{hasMore && (
				<Button
					variant="ghost"
					size="sm"
					className="view-all-link"
					onClick={handleViewAll}
				>
					View All →
				</Button>
			)}
		</section>
	);
}
