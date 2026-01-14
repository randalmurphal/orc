import { useSearchParams } from 'react-router-dom';
import { useCurrentProjectId, useCurrentInitiativeId } from '@/stores';

/**
 * Kanban board page (/board)
 *
 * URL params:
 * - project: Project filter
 * - initiative: Initiative filter
 * - dependency_status: Dependency status filter
 */
export function Board() {
	const [searchParams] = useSearchParams();
	const currentProjectId = useCurrentProjectId();
	const currentInitiativeId = useCurrentInitiativeId();
	const dependencyStatus = searchParams.get('dependency_status');

	return (
		<div className="page board-page">
			<h2>Board</h2>
			<div className="page-debug">
				<p>
					<strong>Project:</strong> {currentProjectId ?? 'none'}
				</p>
				<p>
					<strong>Initiative:</strong> {currentInitiativeId ?? 'all'}
				</p>
				<p>
					<strong>Dependency Status:</strong> {dependencyStatus ?? 'all'}
				</p>
			</div>
			<p className="page-placeholder">Kanban board will be implemented in Phase 3</p>
		</div>
	);
}
