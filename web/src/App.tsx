import { useRoutes } from 'react-router-dom';
import { routes } from '@/router';
import { ShortcutProvider } from '@/hooks';
import { EventProvider } from '@/hooks/EventProvider';
import { DataProvider } from '@/components/layout';
import { TooltipProvider } from '@/components/ui';

/**
 * Root application component.
 *
 * Wraps the app with:
 * - TooltipProvider for hover tooltips
 * - ShortcutProvider for keyboard shortcuts
 * - EventProvider for real-time updates via Connect RPC streaming
 * - DataProvider for centralized data loading
 * - Router for navigation
 */
function App() {
	const routeElements = useRoutes(routes);

	return (
		<TooltipProvider delayDuration={300}>
			<ShortcutProvider>
				<EventProvider>
					<DataProvider>{routeElements}</DataProvider>
				</EventProvider>
			</ShortcutProvider>
		</TooltipProvider>
	);
}

export default App;
