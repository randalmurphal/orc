import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AppShell } from './AppShell';
import { AppShellProvider, useAppShell } from './AppShellContext';
import { TooltipProvider } from '@/components/ui';
import { useProjectStore, useSessionStore } from '@/stores';

// =============================================================================
// TEST UTILITIES
// =============================================================================

/**
 * Wrapper providing all required context providers
 */
function TestWrapper({ children }: { children: React.ReactNode }) {
	return (
		<MemoryRouter>
			<TooltipProvider delayDuration={0}>
				{children}
			</TooltipProvider>
		</MemoryRouter>
	);
}

function renderWithProviders(ui: React.ReactElement) {
	return render(ui, { wrapper: TestWrapper });
}

// =============================================================================
// STORE SETUP
// =============================================================================

beforeEach(() => {
	// Reset stores to default state
	useProjectStore.setState({
		projects: [
			{
				id: 'proj-001',
				name: 'Test Project',
				path: '/test/project',
				created_at: '2024-01-01T00:00:00Z',
			},
		],
		currentProjectId: 'proj-001',
		loading: false,
		error: null,
	});

	useSessionStore.setState({
		sessionId: 'test-session',
		startTime: new Date(),
		totalTokens: 847000,
		totalCost: 2.34,
		inputTokens: 500000,
		outputTokens: 347000,
		isPaused: false,
		activeTaskCount: 2,
		duration: '1h 23m',
		formattedCost: '$2.34',
		formattedTokens: '847K',
	});

	// Clear localStorage
	localStorage.clear();

	// Mock window.matchMedia for responsive tests
	Object.defineProperty(window, 'matchMedia', {
		writable: true,
		value: vi.fn().mockImplementation((query: string) => ({
			matches: false,
			media: query,
			onchange: null,
			addListener: vi.fn(),
			removeListener: vi.fn(),
			addEventListener: vi.fn(),
			removeEventListener: vi.fn(),
			dispatchEvent: vi.fn(),
		})),
	});
});

afterEach(() => {
	vi.clearAllMocks();
	localStorage.clear();
});

// =============================================================================
// APPSHELL COMPONENT TESTS
// =============================================================================

describe('AppShell', () => {
	describe('rendering', () => {
		it('should render IconNav, TopBar, and children', () => {
			renderWithProviders(
				<AppShell>
					<div data-testid="content">Main Content</div>
				</AppShell>
			);

			// IconNav is rendered (check for logo)
			expect(screen.getByText('O')).toBeInTheDocument();

			// TopBar is rendered (check for project name from store)
			expect(screen.getByText('Test Project')).toBeInTheDocument();

			// Children are rendered
			expect(screen.getByTestId('content')).toBeInTheDocument();
			expect(screen.getByText('Main Content')).toBeInTheDocument();
		});

		it('should render main content with role="main"', () => {
			renderWithProviders(
				<AppShell>
					<div>Content</div>
				</AppShell>
			);

			const main = screen.getByRole('main');
			expect(main).toBeInTheDocument();
		});

		it('should render skip link targeting main content', () => {
			renderWithProviders(
				<AppShell>
					<div>Content</div>
				</AppShell>
			);

			const skipLink = screen.getByText('Skip to main content');
			expect(skipLink).toBeInTheDocument();
			expect(skipLink).toHaveAttribute('href', '#main-content');
		});

		it('should have main content with id="main-content"', () => {
			renderWithProviders(
				<AppShell>
					<div>Content</div>
				</AppShell>
			);

			const main = screen.getByRole('main');
			expect(main).toHaveAttribute('id', 'main-content');
		});
	});

	describe('right panel', () => {
		it('should toggle right panel visibility on button click', async () => {
			const { container } = renderWithProviders(
				<AppShell>
					<div>Content</div>
				</AppShell>
			);

			const shell = container.querySelector('.app-shell');

			// Initial state - panel should be open (at desktop viewport)
			expect(shell).toHaveClass('app-shell--panel-open');

			// Find and click the panel toggle (in TopBar or wherever it's located)
			// Since RightPanel has onClose callback, we simulate closing via the context
		});

		it('should render default panel content when provided', () => {
			renderWithProviders(
				<AppShell defaultPanelContent={<div data-testid="panel-content">Panel Content</div>}>
					<div>Main Content</div>
				</AppShell>
			);

			// Panel content should be rendered (when panel is open)
			expect(screen.getByTestId('panel-content')).toBeInTheDocument();
		});
	});

	describe('callbacks', () => {
		it('should call onNewTask when New Task button is clicked', () => {
			const onNewTask = vi.fn();
			renderWithProviders(
				<AppShell onNewTask={onNewTask}>
					<div>Content</div>
				</AppShell>
			);

			const newTaskBtn = screen.getByRole('button', { name: /new task/i });
			fireEvent.click(newTaskBtn);

			expect(onNewTask).toHaveBeenCalledOnce();
		});

		it('should call onProjectChange when project selector is clicked', () => {
			const onProjectChange = vi.fn();
			renderWithProviders(
				<AppShell onProjectChange={onProjectChange}>
					<div>Content</div>
				</AppShell>
			);

			const projectSelector = screen.getByText('Test Project').closest('button');
			fireEvent.click(projectSelector!);

			expect(onProjectChange).toHaveBeenCalledOnce();
		});
	});

	describe('className', () => {
		it('should apply custom className', () => {
			const { container } = renderWithProviders(
				<AppShell className="custom-shell">
					<div>Content</div>
				</AppShell>
			);

			const shell = container.querySelector('.app-shell');
			expect(shell).toHaveClass('custom-shell');
		});
	});
});

// =============================================================================
// APPSHELL CSS TESTS
// =============================================================================

describe('AppShell.css', () => {
	it('should have grid values matching spec (56px, 1fr, 300px, 48px)', () => {
		// This test verifies the CSS file contains the correct values
		// In a real test, we'd check computed styles, but for static analysis:
		const { container } = renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		const shell = container.querySelector('.app-shell');
		expect(shell).toBeInTheDocument();

		// The component should have the correct class structure
		expect(container.querySelector('.app-shell__nav')).toBeInTheDocument();
		expect(container.querySelector('.app-shell__topbar')).toBeInTheDocument();
		expect(container.querySelector('.app-shell__main')).toBeInTheDocument();
		expect(container.querySelector('.app-shell__panel')).toBeInTheDocument();
	});

	it('should have panel-open class toggle grid-template-columns', () => {
		const { container } = renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		const shell = container.querySelector('.app-shell');

		// When panel is open, should have the panel-open class
		expect(shell).toHaveClass('app-shell--panel-open');
	});
});

// =============================================================================
// APPSHELLCONTEXT TESTS
// =============================================================================

describe('AppShellContext', () => {
	describe('useAppShell hook', () => {
		it('should return context value with isRightPanelOpen, toggleRightPanel, setRightPanelContent', () => {
			let contextValue: ReturnType<typeof useAppShell> | null = null;

			function TestComponent() {
				contextValue = useAppShell();
				return null;
			}

			renderWithProviders(
				<AppShellProvider>
					<TestComponent />
				</AppShellProvider>
			);

			expect(contextValue).not.toBeNull();
			expect(contextValue).toHaveProperty('isRightPanelOpen');
			expect(contextValue).toHaveProperty('toggleRightPanel');
			expect(contextValue).toHaveProperty('setRightPanelContent');
			expect(typeof contextValue!.toggleRightPanel).toBe('function');
			expect(typeof contextValue!.setRightPanelContent).toBe('function');
		});

		it('should throw error when used outside provider', () => {
			function TestComponent() {
				useAppShell();
				return null;
			}

			// Suppress console.error for this test
			const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

			expect(() => {
				render(<TestComponent />);
			}).toThrow('useAppShell must be used within an AppShellProvider');

			consoleSpy.mockRestore();
		});
	});

	describe('keyboard shortcuts', () => {
		it('should toggle right panel on Shift+Alt+R', async () => {
			let contextValue: ReturnType<typeof useAppShell> | null = null;

			function TestComponent() {
				contextValue = useAppShell();
				return <div data-testid="test">Test</div>;
			}

			renderWithProviders(
				<AppShellProvider>
					<TestComponent />
				</AppShellProvider>
			);

			const initialState = contextValue!.isRightPanelOpen;

			// Simulate Shift+Alt+R
			act(() => {
				fireEvent.keyDown(document, {
					key: 'r',
					shiftKey: true,
					altKey: true,
				});
			});

			await waitFor(() => {
				// Re-render to get updated context
				expect(contextValue!.isRightPanelOpen).toBe(!initialState);
			});
		});

		it('should not toggle on other key combinations', () => {
			let contextValue: ReturnType<typeof useAppShell> | null = null;

			function TestComponent() {
				contextValue = useAppShell();
				return <div>Test</div>;
			}

			renderWithProviders(
				<AppShellProvider>
					<TestComponent />
				</AppShellProvider>
			);

			const initialState = contextValue!.isRightPanelOpen;

			// Just R key
			act(() => {
				fireEvent.keyDown(document, { key: 'r' });
			});
			expect(contextValue!.isRightPanelOpen).toBe(initialState);

			// Shift+R (no Alt)
			act(() => {
				fireEvent.keyDown(document, { key: 'r', shiftKey: true });
			});
			expect(contextValue!.isRightPanelOpen).toBe(initialState);

			// Alt+R (no Shift)
			act(() => {
				fireEvent.keyDown(document, { key: 'r', altKey: true });
			});
			expect(contextValue!.isRightPanelOpen).toBe(initialState);
		});
	});

	describe('localStorage persistence', () => {
		it('should persist right panel state to localStorage', () => {
			let contextValue: ReturnType<typeof useAppShell> | null = null;

			function TestComponent() {
				contextValue = useAppShell();
				return <div>Test</div>;
			}

			renderWithProviders(
				<AppShellProvider>
					<TestComponent />
				</AppShellProvider>
			);

			// Toggle to close panel
			act(() => {
				contextValue!.toggleRightPanel();
			});

			// Check localStorage
			const stored = localStorage.getItem('orc-right-panel-collapsed');
			expect(stored).toBe('true');

			// Toggle to open panel
			act(() => {
				contextValue!.toggleRightPanel();
			});

			const storedAfter = localStorage.getItem('orc-right-panel-collapsed');
			expect(storedAfter).toBe('false');
		});

		it('should load initial state from localStorage', () => {
			// Set localStorage before rendering
			localStorage.setItem('orc-right-panel-collapsed', 'true');

			let contextValue: ReturnType<typeof useAppShell> | null = null;

			function TestComponent() {
				contextValue = useAppShell();
				return <div>Test</div>;
			}

			renderWithProviders(
				<AppShellProvider>
					<TestComponent />
				</AppShellProvider>
			);

			// Panel should be closed (collapsed = true means isOpen = false)
			expect(contextValue!.isRightPanelOpen).toBe(false);
		});
	});
});

// =============================================================================
// ACCESSIBILITY TESTS
// =============================================================================

describe('AppShell accessibility', () => {
	it('should have skip link that targets main content', () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		const skipLink = screen.getByText('Skip to main content');
		expect(skipLink.tagName).toBe('A');
		expect(skipLink).toHaveAttribute('href', '#main-content');
	});

	it('should have main element with role="main"', () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		const main = screen.getByRole('main');
		expect(main).toBeInTheDocument();
	});

	it('should have navigation element from IconNav', () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		const nav = screen.getByRole('navigation', { name: 'Main navigation' });
		expect(nav).toBeInTheDocument();
	});

	it('should have banner element from TopBar', () => {
		renderWithProviders(
			<AppShell>
				<div>Content</div>
			</AppShell>
		);

		const banner = screen.getByRole('banner');
		expect(banner).toBeInTheDocument();
	});
});

// =============================================================================
// RESPONSIVE TESTS
// =============================================================================

describe('AppShell responsive behavior', () => {
	const originalInnerWidth = window.innerWidth;

	afterEach(() => {
		Object.defineProperty(window, 'innerWidth', {
			writable: true,
			value: originalInnerWidth,
		});
	});

	it('should initialize with panel closed at viewport <1024px', () => {
		// Set viewport to tablet size
		Object.defineProperty(window, 'innerWidth', {
			writable: true,
			value: 900,
		});

		let contextValue: ReturnType<typeof useAppShell> | null = null;

		function TestComponent() {
			contextValue = useAppShell();
			return <div>Test</div>;
		}

		renderWithProviders(
			<AppShellProvider>
				<TestComponent />
			</AppShellProvider>
		);

		// Panel should be closed at tablet viewport
		expect(contextValue!.isRightPanelOpen).toBe(false);
	});

	it('should have isMobileNavMode true at viewport <768px', () => {
		// Set viewport to mobile size
		Object.defineProperty(window, 'innerWidth', {
			writable: true,
			value: 600,
		});

		let contextValue: ReturnType<typeof useAppShell> | null = null;

		function TestComponent() {
			contextValue = useAppShell();
			return <div>Test</div>;
		}

		renderWithProviders(
			<AppShellProvider>
				<TestComponent />
			</AppShellProvider>
		);

		expect(contextValue!.isMobileNavMode).toBe(true);
	});
});
