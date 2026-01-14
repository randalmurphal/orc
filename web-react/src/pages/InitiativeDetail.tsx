import { useParams } from 'react-router-dom';

/**
 * Initiative detail page (/initiatives/:id)
 *
 * Route params:
 * - id: Initiative ID
 */
export function InitiativeDetail() {
	const { id } = useParams<{ id: string }>();

	return (
		<div className="page initiative-detail-page">
			<h2>Initiative: {id}</h2>
			<div className="page-debug">
				<p>
					<strong>Initiative ID:</strong> {id}
				</p>
			</div>
			<p className="page-placeholder">
				Initiative detail view will be implemented in Phase 3
			</p>
		</div>
	);
}
