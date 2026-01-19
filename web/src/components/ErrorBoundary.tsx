/**
 * ErrorBoundary component for route-level error handling.
 *
 * Used as the errorElement in React Router to display a user-friendly
 * error page when route loading or rendering fails.
 *
 * Features:
 * - Displays error message
 * - Provides navigation options (go home, go back)
 * - Reports error details for debugging
 */

import { useRouteError, isRouteErrorResponse, Link, useNavigate } from 'react-router-dom';
import { Icon } from '@/components/ui/Icon';
import './ErrorBoundary.css';

export function ErrorBoundary() {
	const error = useRouteError();
	const navigate = useNavigate();

	// Extract error message
	let title = 'Something went wrong';
	let message = 'An unexpected error occurred while loading this page.';
	let status: number | undefined;

	if (isRouteErrorResponse(error)) {
		status = error.status;
		if (error.status === 404) {
			title = 'Page not found';
			message = 'The page you are looking for does not exist.';
		} else if (error.status === 500) {
			title = 'Server error';
			message = 'An internal server error occurred.';
		} else {
			message = error.statusText || message;
		}
	} else if (error instanceof Error) {
		message = error.message;
	}

	const handleGoBack = () => {
		navigate(-1);
	};

	return (
		<div className="error-boundary">
			<div className="error-boundary__content">
				<div className="error-boundary__icon">
					<Icon name="error" size={48} />
				</div>
				{status && <div className="error-boundary__status">{status}</div>}
				<h1 className="error-boundary__title">{title}</h1>
				<p className="error-boundary__message">{message}</p>
				<div className="error-boundary__actions">
					<button
						type="button"
						className="error-boundary__button error-boundary__button--secondary"
						onClick={handleGoBack}
					>
						<Icon name="arrow-left" size={16} />
						Go back
					</button>
					<Link to="/board" className="error-boundary__button error-boundary__button--primary">
						<Icon name="board" size={16} />
						Go to Board
					</Link>
				</div>
			</div>
		</div>
	);
}
