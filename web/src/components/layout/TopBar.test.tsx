import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent, waitFor, act, type RenderOptions } from '@testing-library/react';
import type { ReactElement } from 'react';
import { TopBar } from './TopBar';
import { AppShellProvider } from './AppShellContext';
import { useProjectStore, useSessionStore } from '@/stores';

// Wrapper component that includes AppShellProvider
function renderWithProviders(
	ui: ReactElement,
	options?: Omit<RenderOptions, 'wrapper'>
) {
	return render(ui, {
		wrapper: ({ children }) => <AppShellProvider>{children}</AppShellProvider>,
		...options,
	});
}

describe('TopBar', () => {
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
	});

	describe('rendering', () => {
		it('should render with role="banner"', () => {
			renderWithProviders(<TopBar />);
			expect(screen.getByRole('banner')).toBeInTheDocument();
		});

		it('should render with default props (no project selected)', () => {
			useProjectStore.setState({
				projects: [],
				currentProjectId: null,
			});
			renderWithProviders(<TopBar />);
			expect(screen.getByText('Select project')).toBeInTheDocument();
		});

		it('should display project name when project is selected', () => {
			renderWithProviders(<TopBar />);
			expect(screen.getByText('Test Project')).toBeInTheDocument();
		});

		it('should allow projectName prop to override store value', () => {
			renderWithProviders(<TopBar projectName="Override Project" />);
			expect(screen.getByText('Override Project')).toBeInTheDocument();
			expect(screen.queryByText('Test Project')).not.toBeInTheDocument();
		});
	});

	describe('session stats', () => {
		it('should display duration from session store', () => {
			renderWithProviders(<TopBar />);
			expect(screen.getByText('Session')).toBeInTheDocument();
			expect(screen.getByText('1h 23m')).toBeInTheDocument();
		});

		it('should display formatted token count', () => {
			renderWithProviders(<TopBar />);
			expect(screen.getByText('Tokens')).toBeInTheDocument();
			expect(screen.getByText('847K')).toBeInTheDocument();
		});

		it('should display formatted cost', () => {
			renderWithProviders(<TopBar />);
			expect(screen.getByText('Cost')).toBeInTheDocument();
			expect(screen.getByText('$2.34')).toBeInTheDocument();
		});

		it('should display updated values when store changes', () => {
			const { rerender } = renderWithProviders(<TopBar />);
			expect(screen.getByText('$2.34')).toBeInTheDocument();

			act(() => {
				useSessionStore.setState({
					formattedCost: '$5.00',
					formattedTokens: '1.2M',
					duration: '2h 45m',
				});
			});

			rerender(<AppShellProvider><TopBar /></AppShellProvider>);
			expect(screen.getByText('$5.00')).toBeInTheDocument();
			expect(screen.getByText('1.2M')).toBeInTheDocument();
			expect(screen.getByText('2h 45m')).toBeInTheDocument();
		});
	});

	describe('pause/resume button', () => {
		it('should show "Pause" when not paused', () => {
			useSessionStore.setState({ isPaused: false });
			renderWithProviders(<TopBar />);
			expect(screen.getByRole('button', { name: /pause/i })).toBeInTheDocument();
		});

		it('should show "Resume" when paused', () => {
			useSessionStore.setState({ isPaused: true });
			renderWithProviders(<TopBar />);
			expect(screen.getByRole('button', { name: /resume/i })).toBeInTheDocument();
		});

		it('should call pauseAll() when Pause is clicked', async () => {
			const pauseAll = vi.fn().mockResolvedValue(undefined);
			useSessionStore.setState({ isPaused: false, pauseAll });

			renderWithProviders(<TopBar />);
			const pauseBtn = screen.getByRole('button', { name: /pause/i });
			fireEvent.click(pauseBtn);

			await waitFor(() => {
				expect(pauseAll).toHaveBeenCalledOnce();
			});
		});

		it('should call resumeAll() when Resume is clicked', async () => {
			const resumeAll = vi.fn().mockResolvedValue(undefined);
			useSessionStore.setState({ isPaused: true, resumeAll });

			renderWithProviders(<TopBar />);
			const resumeBtn = screen.getByRole('button', { name: /resume/i });
			fireEvent.click(resumeBtn);

			await waitFor(() => {
				expect(resumeAll).toHaveBeenCalledOnce();
			});
		});
	});

	describe('new task button', () => {
		it('should render New Task button when onNewTask is provided', () => {
			const onNewTask = vi.fn();
			renderWithProviders(<TopBar onNewTask={onNewTask} />);
			expect(screen.getByRole('button', { name: /new task/i })).toBeInTheDocument();
		});

		it('should not render New Task button when onNewTask is not provided', () => {
			renderWithProviders(<TopBar />);
			expect(screen.queryByRole('button', { name: /new task/i })).not.toBeInTheDocument();
		});

		it('should call onNewTask when clicked', () => {
			const onNewTask = vi.fn();
			renderWithProviders(<TopBar onNewTask={onNewTask} />);

			const newTaskBtn = screen.getByRole('button', { name: /new task/i });
			fireEvent.click(newTaskBtn);

			expect(onNewTask).toHaveBeenCalledOnce();
		});
	});

	describe('accessibility', () => {
		it('should have role="banner" on header', () => {
			renderWithProviders(<TopBar />);
			expect(screen.getByRole('banner')).toBeInTheDocument();
		});

		it('should have aria-label on search input', () => {
			renderWithProviders(<TopBar />);
			expect(screen.getByLabelText('Search tasks')).toBeInTheDocument();
		});

		it('should have aria-haspopup="listbox" on project selector', () => {
			renderWithProviders(<TopBar />);
			const projectSelector = screen.getByText('Test Project').closest('button');
			expect(projectSelector).toHaveAttribute('aria-haspopup', 'listbox');
		});
	});

	describe('project selector', () => {
		it('should call onProjectChange when clicked', () => {
			const onProjectChange = vi.fn();
			renderWithProviders(<TopBar onProjectChange={onProjectChange} />);

			const projectSelector = screen.getByText('Test Project').closest('button');
			fireEvent.click(projectSelector!);

			expect(onProjectChange).toHaveBeenCalledOnce();
		});

		it('should have folder icon', () => {
			renderWithProviders(<TopBar />);
			const projectSelector = screen.getByText('Test Project').closest('button');
			// Check for SVG icons (folder and chevron-down)
			const svgs = projectSelector?.querySelectorAll('svg');
			expect(svgs?.length).toBe(2);
		});
	});

	describe('keyboard shortcuts', () => {
		it('should focus search input on Cmd+K', async () => {
			renderWithProviders(<TopBar />);
			const searchInput = screen.getByLabelText('Search tasks');

			// Simulate Cmd+K
			act(() => {
				fireEvent.keyDown(document, { key: 'k', metaKey: true });
			});

			await waitFor(() => {
				expect(document.activeElement).toBe(searchInput);
			});
		});

		it('should focus search input on Ctrl+K', async () => {
			renderWithProviders(<TopBar />);
			const searchInput = screen.getByLabelText('Search tasks');

			// Simulate Ctrl+K
			act(() => {
				fireEvent.keyDown(document, { key: 'k', ctrlKey: true });
			});

			await waitFor(() => {
				expect(document.activeElement).toBe(searchInput);
			});
		});

		it('should show keyboard hint in search box', () => {
			renderWithProviders(<TopBar />);
			expect(screen.getByText('⌘')).toBeInTheDocument();
			expect(screen.getByText('K')).toBeInTheDocument();
		});
	});

	describe('mobile search toggle', () => {
		it('should render search toggle button', () => {
			renderWithProviders(<TopBar />);
			expect(screen.getByLabelText('Toggle search')).toBeInTheDocument();
		});

		it('should have aria-expanded on search toggle', () => {
			renderWithProviders(<TopBar />);
			const toggleBtn = screen.getByLabelText('Toggle search');
			expect(toggleBtn).toHaveAttribute('aria-expanded', 'false');
		});

		it('should toggle search expanded state when clicked', () => {
			renderWithProviders(<TopBar />);
			const toggleBtn = screen.getByLabelText('Toggle search');

			fireEvent.click(toggleBtn);
			expect(toggleBtn).toHaveAttribute('aria-expanded', 'true');

			fireEvent.click(toggleBtn);
			expect(toggleBtn).toHaveAttribute('aria-expanded', 'false');
		});
	});
});
