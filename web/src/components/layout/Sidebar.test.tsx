import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { useUIStore, useInitiativeStore, useTaskStore } from '@/stores';

// Mock localStorage
const localStorageMock = (() => {
	let store: Record<string, string> = {};
	return {
		getItem: vi.fn((key: string) => store[key] ?? null),
		setItem: vi.fn((key: string, value: string) => {
			store[key] = value;
		}),
		removeItem: vi.fn((key: string) => {
			delete store[key];
		}),
		clear: vi.fn(() => {
			store = {};
		}),
	};
})();
Object.defineProperty(window, 'localStorage', { value: localStorageMock });

// Test wrapper to provide required context
function renderWithRouter(ui: React.ReactElement, { route = '/' } = {}) {
	return render(<MemoryRouter initialEntries={[route]}>{ui}</MemoryRouter>);
}

// Test helper to verify navigation (can be used if needed for debugging)
// @ts-expect-error - Intentionally unused, kept for test debugging
// eslint-disable-next-line @typescript-eslint/no-unused-vars
function LocationDisplay() {
	const location = useLocation();
	return <div data-testid="location">{location.pathname}</div>;
}

describe('Sidebar', () => {
	beforeEach(() => {
		// Reset stores with act to batch updates
		act(() => {
			useUIStore.setState({ sidebarExpanded: true });
			useInitiativeStore.setState({
				initiatives: new Map(),
				currentInitiativeId: null,
				loading: false,
				error: null,
				hasLoaded: false,
			});
			useTaskStore.setState({
				tasks: [],
				loading: false,
				error: null,
			});
		});
		localStorageMock.clear();
		vi.clearAllMocks();
	});

	describe('rendering', () => {
		it('should render sidebar with navigation', () => {
			renderWithRouter(<Sidebar />);

			// Check Work section items
			expect(screen.getByText('Dashboard')).toBeInTheDocument();
			expect(screen.getByText('Tasks')).toBeInTheDocument();
			expect(screen.getByText('Board')).toBeInTheDocument();
		});

		it('should render collapsed sidebar when expanded is false', () => {
			act(() => {
				useUIStore.setState({ sidebarExpanded: false });
			});
			renderWithRouter(<Sidebar />);

			// Sidebar should have collapsed state
			const sidebar = screen.getByRole('navigation', { name: 'Main navigation' });
			expect(sidebar).not.toHaveClass('expanded');

			// Labels should be hidden
			expect(screen.queryByText('Dashboard')).not.toBeInTheDocument();
		});

		it('should render ORC logo when expanded', () => {
			renderWithRouter(<Sidebar />);
			expect(screen.getByText('ORC')).toBeInTheDocument();
		});

		it('should hide logo when collapsed', () => {
			act(() => {
				useUIStore.setState({ sidebarExpanded: false });
			});
			renderWithRouter(<Sidebar />);
			expect(screen.queryByText('ORC')).not.toBeInTheDocument();
		});

		it('should render toggle button with correct aria-label', () => {
			renderWithRouter(<Sidebar />);
			expect(screen.getByRole('button', { name: 'Collapse sidebar' })).toBeInTheDocument();
		});

		it('should render Preferences link', () => {
			renderWithRouter(<Sidebar />);
			expect(screen.getByText('Preferences')).toBeInTheDocument();
		});

		it('should render keyboard hint when expanded', () => {
			renderWithRouter(<Sidebar />);
			expect(screen.getByText(/to toggle/)).toBeInTheDocument();
		});
	});

	describe('toggle functionality', () => {
		it('should toggle sidebar when toggle button clicked', () => {
			renderWithRouter(<Sidebar />);

			const toggleBtn = screen.getByRole('button', { name: 'Collapse sidebar' });
			fireEvent.click(toggleBtn);

			expect(useUIStore.getState().sidebarExpanded).toBe(false);
		});
	});

	describe('initiatives section', () => {
		beforeEach(() => {
			act(() => {
				useInitiativeStore.setState({
					initiatives: new Map([
						['INIT-001', { version: 1, id: 'INIT-001', title: 'Test Initiative', status: 'active', created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z' }],
						['INIT-002', { version: 1, id: 'INIT-002', title: 'Completed Init', status: 'completed', created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z' }],
					]),
				});
			});
		});

		it('should show initiatives section header when expanded', () => {
			renderWithRouter(<Sidebar />);
			expect(screen.getByText('Initiatives')).toBeInTheDocument();
		});

		it('should show "All Tasks" option', () => {
			renderWithRouter(<Sidebar />);
			expect(screen.getByText('All Tasks')).toBeInTheDocument();
		});

		it('should show initiative list', () => {
			renderWithRouter(<Sidebar />);
			expect(screen.getByText('Test Initiative')).toBeInTheDocument();
		});

		it('should show status badge for non-active initiatives', () => {
			renderWithRouter(<Sidebar />);
			expect(screen.getByText('completed')).toBeInTheDocument();
		});

		it('should select initiative when clicked', () => {
			renderWithRouter(<Sidebar />);

			const initiativeLink = screen.getByText('Test Initiative');
			fireEvent.click(initiativeLink);

			expect(useInitiativeStore.getState().currentInitiativeId).toBe('INIT-001');
		});

		it('should clear initiative selection when "All Tasks" clicked', () => {
			act(() => {
				useInitiativeStore.setState({ currentInitiativeId: 'INIT-001' });
			});
			renderWithRouter(<Sidebar />);

			const allTasksLink = screen.getByText('All Tasks');
			fireEvent.click(allTasksLink);

			expect(useInitiativeStore.getState().currentInitiativeId).toBeNull();
		});

		it('should display long initiative names with proper styling', () => {
			const longTitle = 'This is a very long initiative name that would be severely truncated in the old layout';
			act(() => {
				useInitiativeStore.setState({
					initiatives: new Map([
						['INIT-LONG', { version: 1, id: 'INIT-LONG', title: longTitle, status: 'active', created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z' }],
					]),
				});
			});
			renderWithRouter(<Sidebar />);

			// The initiative should be visible
			const initiativeElement = screen.getByText(longTitle);
			expect(initiativeElement).toBeInTheDocument();

			// Check that it has the initiative-title class for multi-line support
			expect(initiativeElement).toHaveClass('initiative-title');
		});

		it('should show tooltip with full title for initiatives', () => {
			const longTitle = 'Long Initiative Name For Testing Tooltip Attribute';
			act(() => {
				useInitiativeStore.setState({
					initiatives: new Map([
						['INIT-TIP', { version: 1, id: 'INIT-TIP', title: longTitle, status: 'active', created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z' }],
					]),
				});
			});
			renderWithRouter(<Sidebar />);

			// Find the initiative link element
			const initiativeLink = screen.getByText(longTitle).closest('a');
			expect(initiativeLink).toHaveAttribute('title', longTitle);
		});
	});

	describe('collapsible sections', () => {
		it('should toggle initiatives section when clicked', () => {
			renderWithRouter(<Sidebar />);

			const initiativesHeader = screen.getByRole('button', { name: /Initiatives/i });

			// Initially expanded
			expect(screen.getByText('All Tasks')).toBeInTheDocument();

			// Click to collapse
			fireEvent.click(initiativesHeader);
			expect(screen.queryByText('All Tasks')).not.toBeInTheDocument();

			// Click to expand
			fireEvent.click(initiativesHeader);
			expect(screen.getByText('All Tasks')).toBeInTheDocument();
		});

		it('should toggle environment section when clicked', () => {
			renderWithRouter(<Sidebar />);

			const environmentHeader = screen.getByRole('button', { name: /Environment/i });

			// Initially collapsed (no Overview visible)
			expect(screen.queryByText('Overview')).not.toBeInTheDocument();

			// Click to expand
			fireEvent.click(environmentHeader);
			expect(screen.getByText('Overview')).toBeInTheDocument();
		});
	});

	describe('navigation active states', () => {
		it('should mark Tasks as active on root route', () => {
			renderWithRouter(<Sidebar />, { route: '/' });

			const tasksLink = screen.getByText('Tasks').closest('a');
			expect(tasksLink).toHaveClass('active');
		});

		it('should mark Board as active on /board route', () => {
			renderWithRouter(<Sidebar />, { route: '/board' });

			const boardLink = screen.getByText('Board').closest('a');
			expect(boardLink).toHaveClass('active');
		});

		it('should mark Dashboard as active on /dashboard route', () => {
			renderWithRouter(<Sidebar />, { route: '/dashboard' });

			const dashboardLink = screen.getByText('Dashboard').closest('a');
			expect(dashboardLink).toHaveClass('active');
		});
	});
});
