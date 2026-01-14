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
			expect(screen.getByRole('navigation')).toBeInTheDocument();
		});
	});

	it('renders TaskList page at root route', async () => {
		renderApp('/');
		await waitFor(() => {
			expect(screen.getByText('Task List')).toBeInTheDocument();
		});
	});

	it('renders Board page at /board route', async () => {
		renderApp('/board');
		await waitFor(() => {
			expect(screen.getByRole('heading', { level: 2, name: 'Board' })).toBeInTheDocument();
		});
	});

	it('renders Dashboard page at /dashboard route', async () => {
		renderApp('/dashboard');
		await waitFor(() => {
			expect(screen.getByRole('heading', { level: 2, name: 'Dashboard' })).toBeInTheDocument();
		});
	});
});
