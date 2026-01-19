/**
 * LeaderboardTable - Ranked list display for showing top items.
 * Used for most active initiatives, most modified files, etc.
 */

import './LeaderboardTable.css';

export interface LeaderboardItem {
	/** Display rank (1-99) */
	rank: number;
	/** Name or path to display */
	name: string;
	/** Metric value (e.g., "89 tasks", "34x") */
	value: string;
}

export interface LeaderboardTableProps {
	/** Table title displayed in header */
	title: string;
	/** Items to display (limited to first 4) */
	items: LeaderboardItem[];
	/** Callback for "View all" button click */
	onViewAll?: () => void;
	/** Whether names are file paths (uses monospace, left-truncation) */
	isFilePath?: boolean;
}

const MAX_ITEMS = 4;

/**
 * A table showing a ranked list of items with optional "View all" link.
 * Supports both regular names and file paths (with left-truncation).
 *
 * @example
 * <LeaderboardTable
 *   title="Most Active Initiatives"
 *   items={[
 *     { rank: 1, name: "Systems Reliability", value: "89 tasks" },
 *     { rank: 2, name: "Auth & Permissions", value: "67 tasks" },
 *   ]}
 *   onViewAll={() => navigate('/initiatives')}
 * />
 */
export function LeaderboardTable({
	title,
	items,
	onViewAll,
	isFilePath = false,
}: LeaderboardTableProps) {
	// Limit to first 4 items
	const displayItems = items.slice(0, MAX_ITEMS);

	return (
		<div className="leaderboard-table">
			<div className="leaderboard-table-header">
				<span className="leaderboard-table-title">{title}</span>
				{onViewAll && (
					<button
						type="button"
						className="leaderboard-table-view-all"
						onClick={onViewAll}
					>
						View all
					</button>
				)}
			</div>
			<div className="leaderboard-table-body">
				{displayItems.length === 0 ? (
					<div className="leaderboard-table-empty">No data</div>
				) : (
					displayItems.map((item) => (
						<div key={`${item.rank}-${item.name}`} className="leaderboard-table-row">
							<span className="leaderboard-table-rank">{item.rank}</span>
							<span
								className={`leaderboard-table-name${isFilePath ? ' leaderboard-table-name--path' : ''}`}
								title={item.name}
							>
								{item.name}
							</span>
							<span className="leaderboard-table-value">{item.value}</span>
						</div>
					))
				)}
			</div>
		</div>
	);
}
