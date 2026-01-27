/**
 * NotFoundPage component for 404 routes.
 *
 * Displayed when a user navigates to a route that doesn't exist.
 */

import { Link, useNavigate } from 'react-router-dom';
import { Icon } from '@/components/ui/Icon';
import { useDocumentTitle } from '@/hooks';
import './NotFoundPage.css';

export function NotFoundPage() {
	useDocumentTitle('Page Not Found');
	const navigate = useNavigate();

	const handleGoBack = () => {
		navigate(-1);
	};

	return (
		<div className="not-found-page">
			<div className="not-found-page__content">
				<div className="not-found-page__icon">
					<Icon name="search" size={48} />
				</div>
				<div className="not-found-page__status">404</div>
				<h1 className="not-found-page__title">Page not found</h1>
				<p className="not-found-page__message">
					The page you are looking for doesn't exist or has been moved.
				</p>
				<div className="not-found-page__actions">
					<button
						type="button"
						className="not-found-page__button not-found-page__button--secondary"
						onClick={handleGoBack}
					>
						<Icon name="arrow-left" size={16} />
						Go back
					</button>
					<Link to="/board" className="not-found-page__button not-found-page__button--primary">
						<Icon name="board" size={16} />
						Go to Board
					</Link>
				</div>
			</div>
		</div>
	);
}
