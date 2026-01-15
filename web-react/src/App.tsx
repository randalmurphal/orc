import { useRoutes } from 'react-router-dom';
import { routes } from '@/router';
import { WebSocketProvider, ShortcutProvider } from '@/hooks';
import { DataProvider } from '@/components/layout';

/**
 * Root application component.
 *
 * Wraps the app with:
 * - ShortcutProvider for keyboard shortcuts
 * - WebSocketProvider for real-time updates
 * - DataProvider for centralized data loading
 * - Router for navigation
 */
function App() {
	const routeElements = useRoutes(routes);

	return (
		<ShortcutProvider>
			<WebSocketProvider>
				<DataProvider>{routeElements}</DataProvider>
			</WebSocketProvider>
		</ShortcutProvider>
	);
}

export default App;
