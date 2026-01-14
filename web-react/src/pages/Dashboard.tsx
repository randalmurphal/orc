import { useCurrentProjectId } from '@/stores';

/**
 * Dashboard page (/dashboard)
 *
 * URL params:
 * - project: Project filter
 */
export function Dashboard() {
	const currentProjectId = useCurrentProjectId();

	return (
		<div className="page dashboard-page">
			<h2>Dashboard</h2>
			<div className="page-debug">
				<p>
					<strong>Project:</strong> {currentProjectId ?? 'none'}
				</p>
			</div>
			<p className="page-placeholder">Dashboard with stats will be implemented in Phase 3</p>
		</div>
	);
}
