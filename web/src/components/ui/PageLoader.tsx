/**
 * PageLoader - Full-page loading indicator for lazy-loaded routes
 */

import './PageLoader.css';

export function PageLoader() {
	return (
		<div className="page-loader" role="status" aria-label="Loading page">
			<div className="page-loader__spinner" />
		</div>
	);
}
