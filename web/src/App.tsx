import { useRoutes } from 'react-router-dom';
import { routes } from '@/router';
import { WebSocketProvider, ShortcutProvider } from '@/hooks';
import { DataProvider } from '@/components/layout';
import { TooltipProvider } from '@/components/ui';

/**
 * Root application component.
 *
 * Wraps the app with:
 * - TooltipProvider for hover tooltips
 * - ShortcutProvider for keyboard shortcuts
 * - WebSocketProvider for real-time updates
 * - DataProvider for centralized data loading
 * - Router for navigation
 */
function App() {
	const routeElements = useRoutes(routes);

	return (
		<TooltipProvider delayDuration={300}>
			<ShortcutProvider>
				<WebSocketProvider>
					<DataProvider>{routeElements}</DataProvider>
				</WebSocketProvider>
			</ShortcutProvider>
		</TooltipProvider>
	);
}

export default App;
