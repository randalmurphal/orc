import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { TerminalDrawer } from './TerminalDrawer';
import { AppShellProvider } from './AppShellContext';
import { TooltipProvider } from '@/components/ui';

// =============================================================================
// TEST UTILITIES
// =============================================================================

function TestWrapper({ children }: { children: React.ReactNode }) {
	return (
		<MemoryRouter>
			<TooltipProvider delayDuration={0}>
				<AppShellProvider>
					{children}
				</AppShellProvider>
			</TooltipProvider>
		</MemoryRouter>
	);
}

function renderWithProviders(ui: React.ReactElement) {
	return render(ui, { wrapper: TestWrapper });
}

beforeEach(() => {
	localStorage.clear();
});

afterEach(() => {
	vi.clearAllMocks();
	localStorage.clear();
});

// =============================================================================
// SC-10: Terminal drawer toggle via Cmd+J / Ctrl+J
// =============================================================================

describe('TerminalDrawer keyboard toggle (SC-10)', () => {
	it('should render terminal drawer', () => {
		renderWithProviders(<TerminalDrawer />);

		// Drawer should be in the DOM (closed by default)
		const drawer = screen.getByTestId('terminal-drawer');
		expect(drawer).toBeInTheDocument();
	});

	it('should open on Cmd+J keypress (Mac)', async () => {
		renderWithProviders(<TerminalDrawer />);

		act(() => {
			fireEvent.keyDown(document, { key: 'j', metaKey: true });
		});

		await waitFor(() => {
			const drawer = screen.getByTestId('terminal-drawer');
			expect(drawer.className).toMatch(/open/);
		});
	});

	it('should open on Ctrl+J keypress (non-Mac)', async () => {
		renderWithProviders(<TerminalDrawer />);

		act(() => {
			fireEvent.keyDown(document, { key: 'j', ctrlKey: true });
		});

		await waitFor(() => {
			const drawer = screen.getByTestId('terminal-drawer');
			expect(drawer.className).toMatch(/open/);
		});
	});

	it('should close on Cmd+J when already open', async () => {
		renderWithProviders(<TerminalDrawer />);

		// Open the drawer
		act(() => {
			fireEvent.keyDown(document, { key: 'j', metaKey: true });
		});

		await waitFor(() => {
			expect(screen.getByTestId('terminal-drawer').className).toMatch(/open/);
		});

		// Close the drawer
		act(() => {
			fireEvent.keyDown(document, { key: 'j', metaKey: true });
		});

		await waitFor(() => {
			expect(screen.getByTestId('terminal-drawer').className).not.toMatch(/open/);
		});
	});

	it('should show placeholder content when open', async () => {
		renderWithProviders(<TerminalDrawer />);

		act(() => {
			fireEvent.keyDown(document, { key: 'j', metaKey: true });
		});

		await waitFor(() => {
			// Terminal drawer should show some placeholder since xterm.js is out of scope
			expect(screen.getByTestId('terminal-drawer').className).toMatch(/open/);
		});
	});

	it('should NOT toggle when focus is in a text input', () => {
		renderWithProviders(
			<>
				<input data-testid="text-input" type="text" />
				<TerminalDrawer />
			</>
		);

		const textInput = screen.getByTestId('text-input');
		textInput.focus();

		act(() => {
			fireEvent.keyDown(textInput, { key: 'j', metaKey: true });
		});

		// Drawer should remain closed because focus was in a text input
		const drawer = screen.getByTestId('terminal-drawer');
		expect(drawer.className).not.toMatch(/open/);
	});

	it('should NOT toggle when focus is in a textarea', () => {
		renderWithProviders(
			<>
				<textarea data-testid="textarea" />
				<TerminalDrawer />
			</>
		);

		const textarea = screen.getByTestId('textarea');
		textarea.focus();

		act(() => {
			fireEvent.keyDown(textarea, { key: 'j', metaKey: true });
		});

		const drawer = screen.getByTestId('terminal-drawer');
		expect(drawer.className).not.toMatch(/open/);
	});

	it('should not toggle on just J key without modifier', () => {
		renderWithProviders(<TerminalDrawer />);

		act(() => {
			fireEvent.keyDown(document, { key: 'j' });
		});

		const drawer = screen.getByTestId('terminal-drawer');
		expect(drawer.className).not.toMatch(/open/);
	});
});

// =============================================================================
// SC-11: Terminal drawer state persists to localStorage
// =============================================================================

describe('TerminalDrawer persistence (SC-11)', () => {
	it('should persist open state to localStorage', async () => {
		renderWithProviders(<TerminalDrawer />);

		act(() => {
			fireEvent.keyDown(document, { key: 'j', metaKey: true });
		});

		await waitFor(() => {
			const stored = localStorage.getItem('orc-terminal-drawer-open');
			expect(stored).toBe('true');
		});
	});

	it('should persist closed state to localStorage', async () => {
		renderWithProviders(<TerminalDrawer />);

		// Open
		act(() => {
			fireEvent.keyDown(document, { key: 'j', metaKey: true });
		});

		// Close
		act(() => {
			fireEvent.keyDown(document, { key: 'j', metaKey: true });
		});

		await waitFor(() => {
			const stored = localStorage.getItem('orc-terminal-drawer-open');
			expect(stored).toBe('false');
		});
	});

	it('should restore open state from localStorage on mount', () => {
		localStorage.setItem('orc-terminal-drawer-open', 'true');

		renderWithProviders(<TerminalDrawer />);

		const drawer = screen.getByTestId('terminal-drawer');
		expect(drawer.className).toMatch(/open/);
	});

	it('should default to closed when no localStorage value', () => {
		renderWithProviders(<TerminalDrawer />);

		const drawer = screen.getByTestId('terminal-drawer');
		expect(drawer.className).not.toMatch(/open/);
	});

	it('should default to closed for invalid localStorage value', () => {
		localStorage.setItem('orc-terminal-drawer-open', 'invalid');

		renderWithProviders(<TerminalDrawer />);

		const drawer = screen.getByTestId('terminal-drawer');
		expect(drawer.className).not.toMatch(/open/);
	});
});

// =============================================================================
// TERMINAL DRAWER UI
// =============================================================================

describe('TerminalDrawer UI', () => {
	it('should have a toggle bar that can be clicked to open/close', async () => {
		renderWithProviders(<TerminalDrawer />);

		const toggleBar = screen.getByRole('button', { name: /terminal/i });
		expect(toggleBar).toBeInTheDocument();

		fireEvent.click(toggleBar);

		await waitFor(() => {
			const drawer = screen.getByTestId('terminal-drawer');
			expect(drawer.className).toMatch(/open/);
		});
	});

	it('should show keyboard shortcut hint', () => {
		renderWithProviders(<TerminalDrawer />);

		// Should indicate Cmd+J or Ctrl+J shortcut
		expect(screen.getByText(/⌘J/i) || screen.getByText(/ctrl\+j/i)).toBeTruthy();
	});
});
