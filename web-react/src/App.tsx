import { useRoutes } from 'react-router-dom';
import { routes } from '@/router';
import { WebSocketProvider, ShortcutProvider } from '@/hooks';

/**
 * Root application component.
 *
 * Wraps the app with:
 * - ShortcutProvider for keyboard shortcuts
 * - WebSocketProvider for real-time updates
 * - Router for navigation
 */
function App() {
	const routeElements = useRoutes(routes);

	return (
		<ShortcutProvider>
			<WebSocketProvider>{routeElements}</WebSocketProvider>
		</ShortcutProvider>
	);
}

export default App;
