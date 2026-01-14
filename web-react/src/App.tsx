import { useRoutes } from 'react-router-dom';
import { routes } from '@/router';
import { WebSocketProvider } from '@/hooks';

/**
 * Root application component.
 *
 * Wraps the app with:
 * - WebSocketProvider for real-time updates
 * - Router for navigation
 */
function App() {
	const routeElements = useRoutes(routes);

	return <WebSocketProvider>{routeElements}</WebSocketProvider>;
}

export default App;
