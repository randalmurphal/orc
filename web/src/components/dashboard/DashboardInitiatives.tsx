/**
 * DashboardInitiatives component - displays active initiatives with progress bars.
 * Clicking an initiative filters the board by that initiative.
 */

import { useNavigate } from 'react-router-dom';
import type { Initiative } from '@/lib/types';
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

function getProgress(initiative: Initiative): ProgressInfo {
	const tasks = initiative.tasks || [];
	const total = tasks.length;
	if (total === 0) return { completed: 0, total: 0, percent: 0 };

	const completed = tasks.filter(
		(t) => t.status === 'completed' || t.status === 'finished'
	).length;
	const percent = Math.round((completed / total) * 100);
	return { completed, total, percent };
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
					const progress = getProgress(initiative);
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
