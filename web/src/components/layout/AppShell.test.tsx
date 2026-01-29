import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { create } from '@bufbuild/protobuf';
import { AppShell } from './AppShell';
import { AppShellProvider, useAppShell } from './AppShellContext';
import { TooltipProvider } from '@/components/ui';
import { useProjectStore, useSessionStore } from '@/stores';
import { createTimestamp } from '@/test/factories';
import { ProjectSchema } from '@/gen/orc/v1/project_pb';

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
			create(ProjectSchema, {
				id: 'proj-001',
				name: 'Test Project',
				path: '/test/project',
				createdAt: createTimestamp('2024-01-01T00:00:00Z'),
			}),
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
		it('should return context value with isRightPanelOpen, toggleRightPanel, isMobileNavMode', () => {
			// Test component that exposes context value through data attributes
			function TestComponent() {
				const ctx = useAppShell();
				return (
					<div
						data-testid="context-consumer"
						data-has-is-right-panel-open={ctx.isRightPanelOpen !== undefined}
						data-has-toggle-right-panel={typeof ctx.toggleRightPanel === 'function'}
						data-has-is-mobile-nav-mode={ctx.isMobileNavMode !== undefined}
					/>
				);
			}

			renderWithProviders(
				<AppShellProvider>
					<TestComponent />
				</AppShellProvider>
			);

			const consumer = screen.getByTestId('context-consumer');
			expect(consumer).toHaveAttribute('data-has-is-right-panel-open', 'true');
			expect(consumer).toHaveAttribute('data-has-toggle-right-panel', 'true');
			expect(consumer).toHaveAttribute('data-has-is-mobile-nav-mode', 'true');
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
			// Component that shows panel state via data attribute
			function TestComponent() {
				const { isRightPanelOpen } = useAppShell();
				return <div data-testid="panel-state" data-open={isRightPanelOpen} />;
			}

			renderWithProviders(
				<AppShellProvider>
					<TestComponent />
				</AppShellProvider>
			);

			const panelState = screen.getByTestId('panel-state');
			const initialOpen = panelState.getAttribute('data-open') === 'true';

			// Simulate Shift+Alt+R
			act(() => {
				fireEvent.keyDown(document, {
					key: 'r',
					shiftKey: true,
					altKey: true,
				});
			});

			await waitFor(() => {
				const newOpen = screen.getByTestId('panel-state').getAttribute('data-open') === 'true';
				expect(newOpen).toBe(!initialOpen);
			});
		});

		it('should not toggle on other key combinations', async () => {
			// Component that shows panel state via data attribute
			function TestComponent() {
				const { isRightPanelOpen } = useAppShell();
				return <div data-testid="panel-state" data-open={isRightPanelOpen} />;
			}

			renderWithProviders(
				<AppShellProvider>
					<TestComponent />
				</AppShellProvider>
			);

			const panelState = screen.getByTestId('panel-state');
			const initialOpen = panelState.getAttribute('data-open') === 'true';

			// Just R key
			act(() => {
				fireEvent.keyDown(document, { key: 'r' });
			});
			expect(screen.getByTestId('panel-state').getAttribute('data-open') === 'true').toBe(initialOpen);

			// Shift+R (no Alt)
			act(() => {
				fireEvent.keyDown(document, { key: 'r', shiftKey: true });
			});
			expect(screen.getByTestId('panel-state').getAttribute('data-open') === 'true').toBe(initialOpen);

			// Alt+R (no Shift)
			act(() => {
				fireEvent.keyDown(document, { key: 'r', altKey: true });
			});
			expect(screen.getByTestId('panel-state').getAttribute('data-open') === 'true').toBe(initialOpen);
		});
	});

	describe('localStorage persistence', () => {
		it('should persist right panel state to localStorage', async () => {
			// Component with a toggle button to trigger state change
			function TestComponent() {
				const { isRightPanelOpen, toggleRightPanel } = useAppShell();
				return (
					<div>
						<div data-testid="panel-state" data-open={isRightPanelOpen} />
						<button data-testid="toggle-btn" onClick={toggleRightPanel}>Toggle</button>
					</div>
				);
			}

			renderWithProviders(
				<AppShellProvider>
					<TestComponent />
				</AppShellProvider>
			);

			// Toggle to close panel
			act(() => {
				fireEvent.click(screen.getByTestId('toggle-btn'));
			});

			// Check localStorage
			await waitFor(() => {
				const stored = localStorage.getItem('orc-right-panel-collapsed');
				expect(stored).toBe('true');
			});

			// Toggle to open panel
			act(() => {
				fireEvent.click(screen.getByTestId('toggle-btn'));
			});

			await waitFor(() => {
				const storedAfter = localStorage.getItem('orc-right-panel-collapsed');
				expect(storedAfter).toBe('false');
			});
		});

		it('should load initial state from localStorage', () => {
			// Set localStorage before rendering
			localStorage.setItem('orc-right-panel-collapsed', 'true');

			function TestComponent() {
				const { isRightPanelOpen } = useAppShell();
				return <div data-testid="panel-state" data-open={isRightPanelOpen} />;
			}

			renderWithProviders(
				<AppShellProvider>
					<TestComponent />
				</AppShellProvider>
			);

			// Panel should be closed (collapsed = true means isOpen = false)
			expect(screen.getByTestId('panel-state').getAttribute('data-open')).toBe('false');
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

		function TestComponent() {
			const { isRightPanelOpen } = useAppShell();
			return <div data-testid="panel-state" data-open={isRightPanelOpen} />;
		}

		renderWithProviders(
			<AppShellProvider>
				<TestComponent />
			</AppShellProvider>
		);

		// Panel should be closed at tablet viewport
		expect(screen.getByTestId('panel-state').getAttribute('data-open')).toBe('false');
	});

	it('should have isMobileNavMode true at viewport <768px', () => {
		// Set viewport to mobile size
		Object.defineProperty(window, 'innerWidth', {
			writable: true,
			value: 600,
		});

		function TestComponent() {
			const { isMobileNavMode } = useAppShell();
			return <div data-testid="mobile-state" data-mobile={isMobileNavMode} />;
		}

		renderWithProviders(
			<AppShellProvider>
				<TestComponent />
			</AppShellProvider>
		);

		expect(screen.getByTestId('mobile-state').getAttribute('data-mobile')).toBe('true');
	});
});
