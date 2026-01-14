import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import App from './App';

// Mock WebSocket to prevent actual connections
vi.mock('@/lib/websocket', () => ({
	OrcWebSocket: vi.fn().mockImplementation(() => ({
		connect: vi.fn(),
		disconnect: vi.fn(),
		subscribe: vi.fn(),
		unsubscribe: vi.fn(),
		subscribeGlobal: vi.fn(),
		setPrimarySubscription: vi.fn(),
		on: vi.fn().mockReturnValue(() => {}),
		onStatusChange: vi.fn().mockReturnValue(() => {}),
		isConnected: vi.fn().mockReturnValue(false),
		getTaskId: vi.fn().mockReturnValue(null),
		command: vi.fn(),
	})),
	GLOBAL_TASK_ID: '*',
}));

function renderApp(initialPath: string = '/') {
	return render(
		<MemoryRouter initialEntries={[initialPath]}>
			<App />
		</MemoryRouter>
	);
}

describe('App', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('renders App with router and WebSocketProvider', async () => {
		renderApp('/');
		// App renders routes wrapped in WebSocketProvider
		await waitFor(() => {
			// Should render the layout with sidebar and content
			expect(screen.getByRole('navigation', { name: 'Main navigation' })).toBeInTheDocument();
		});
	});

	it('renders TaskList page at root route', async () => {
		renderApp('/');
		await waitFor(() => {
			expect(screen.getByText('Task List')).toBeInTheDocument();
		});
	});

	it('renders Board page at /board route (shows empty state without project)', async () => {
		renderApp('/board');
		await waitFor(() => {
			// Without project param in URL, Board shows "No Project Selected" empty state
			expect(screen.getByText('No Project Selected')).toBeInTheDocument();
		});
	});

	it('renders Dashboard page at /dashboard route', async () => {
		// Mock API responses for dashboard
		vi.mocked(fetch).mockImplementation((url) => {
			const urlStr = typeof url === 'string' ? url : url.toString();
			if (urlStr.includes('/api/dashboard/stats')) {
				return Promise.resolve({
					ok: true,
					json: () =>
						Promise.resolve({
							running: 1,
							paused: 0,
							blocked: 2,
							completed: 5,
							failed: 0,
							today: 3,
							total: 10,
							tokens: 50000,
							cost: 0.5,
						}),
				} as Response);
			}
			if (urlStr.includes('/api/initiatives')) {
				return Promise.resolve({
					ok: true,
					json: () => Promise.resolve([]),
				} as Response);
			}
			return Promise.resolve({
				ok: true,
				json: () => Promise.resolve({}),
			} as Response);
		});

		renderApp('/dashboard');
		await waitFor(() => {
			// Dashboard renders Quick Stats section
			expect(screen.getByText('Quick Stats')).toBeInTheDocument();
		});
	});
});
