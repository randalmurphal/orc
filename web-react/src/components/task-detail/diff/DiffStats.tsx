import type { DiffStats as DiffStatsType } from '@/lib/types';
import './DiffStats.css';

interface DiffStatsProps {
	stats: DiffStatsType;
}

export function DiffStats({ stats }: DiffStatsProps) {
	return (
		<div className="diff-stats">
			<span className="stat files">{stats.files_changed} file{stats.files_changed !== 1 ? 's' : ''}</span>
			<span className="stat additions">+{stats.additions}</span>
			<span className="stat deletions">-{stats.deletions}</span>
		</div>
	);
}
