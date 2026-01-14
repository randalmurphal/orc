import { useParams, useSearchParams } from 'react-router-dom';

/**
 * Task detail page (/tasks/:id)
 *
 * Route params:
 * - id: Task ID
 *
 * URL params:
 * - tab: Active tab (overview, transcript, diff, etc.)
 */
export function TaskDetail() {
	const { id } = useParams<{ id: string }>();
	const [searchParams] = useSearchParams();
	const tab = searchParams.get('tab') ?? 'overview';

	return (
		<div className="page task-detail-page">
			<h2>Task: {id}</h2>
			<div className="page-debug">
				<p>
					<strong>Task ID:</strong> {id}
				</p>
				<p>
					<strong>Active Tab:</strong> {tab}
				</p>
			</div>
			<p className="page-placeholder">Task detail view will be implemented in Phase 3</p>
		</div>
	);
}
