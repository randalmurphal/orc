import type { DiffStats as DiffStatsType } from '@/gen/orc/v1/common_pb';
import './DiffStats.css';

interface DiffStatsProps {
	stats: DiffStatsType;
}

export function DiffStats({ stats }: DiffStatsProps) {
	return (
		<div className="diff-stats">
			<span className="stat files">{stats.filesChanged} file{stats.filesChanged !== 1 ? 's' : ''}</span>
			<span className="stat additions">+{stats.additions}</span>
			<span className="stat deletions">-{stats.deletions}</span>
		</div>
	);
}
