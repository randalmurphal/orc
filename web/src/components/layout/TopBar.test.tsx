import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { create } from '@bufbuild/protobuf';
import { TopBar } from './TopBar';
import { useProjectStore, useSessionStore } from '@/stores';
import { createTimestamp } from '@/test/factories';
import { ProjectSchema } from '@/gen/orc/v1/project_pb';

const renderWithRouter = (ui: React.ReactElement) =>
	render(ui, { wrapper: ({ children }) => <MemoryRouter>{children}</MemoryRouter> });

describe('TopBar', () => {
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
	});

	describe('rendering', () => {
		it('should render with role="banner"', () => {
			renderWithRouter(<TopBar />);
			expect(screen.getByRole('banner')).toBeInTheDocument();
		});

		it('should render navigation tabs', () => {
			renderWithRouter(<TopBar />);
			expect(screen.getByText('Home')).toBeInTheDocument();
			expect(screen.getByText('Board')).toBeInTheDocument();
			expect(screen.getByText('Settings')).toBeInTheDocument();
		});

		it('should render search box', () => {
			renderWithRouter(<TopBar />);
			expect(screen.getByLabelText('Search tasks')).toBeInTheDocument();
		});
	});

	describe('session stats', () => {
		it('should display duration from session store', () => {
			renderWithRouter(<TopBar />);
			expect(screen.getByText('Session')).toBeInTheDocument();
			expect(screen.getByText('1h 23m')).toBeInTheDocument();
		});

		it('should display formatted token count', () => {
			renderWithRouter(<TopBar />);
			expect(screen.getByText('Tokens')).toBeInTheDocument();
			expect(screen.getByText('847K')).toBeInTheDocument();
		});

		it('should display formatted cost', () => {
			renderWithRouter(<TopBar />);
			expect(screen.getByText('Cost')).toBeInTheDocument();
			expect(screen.getByText('$2.34')).toBeInTheDocument();
		});

		it('should display updated values when store changes', () => {
			const { rerender } = renderWithRouter(<TopBar />);
			expect(screen.getByText('$2.34')).toBeInTheDocument();

			act(() => {
				useSessionStore.setState({
					formattedCost: '$5.00',
					formattedTokens: '1.2M',
					duration: '2h 45m',
				});
			});

			rerender(<TopBar />);
			expect(screen.getByText('$5.00')).toBeInTheDocument();
			expect(screen.getByText('1.2M')).toBeInTheDocument();
			expect(screen.getByText('2h 45m')).toBeInTheDocument();
		});
	});

	describe('pause/resume button', () => {
		it('should show "Pause" when not paused', () => {
			useSessionStore.setState({ isPaused: false });
			renderWithRouter(<TopBar />);
			expect(screen.getByRole('button', { name: /pause/i })).toBeInTheDocument();
		});

		it('should show "Resume" when paused', () => {
			useSessionStore.setState({ isPaused: true });
			renderWithRouter(<TopBar />);
			expect(screen.getByRole('button', { name: /resume/i })).toBeInTheDocument();
		});

		it('should call pauseAll() when Pause is clicked', async () => {
			const pauseAll = vi.fn().mockResolvedValue(undefined);
			useSessionStore.setState({ isPaused: false, pauseAll });

			renderWithRouter(<TopBar />);
			const pauseBtn = screen.getByRole('button', { name: /pause/i });
			fireEvent.click(pauseBtn);

			await waitFor(() => {
				expect(pauseAll).toHaveBeenCalledOnce();
			});
		});

		it('should call resumeAll() when Resume is clicked', async () => {
			const resumeAll = vi.fn().mockResolvedValue(undefined);
			useSessionStore.setState({ isPaused: true, resumeAll });

			renderWithRouter(<TopBar />);
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
			renderWithRouter(<TopBar onNewTask={onNewTask} />);
			expect(screen.getByRole('button', { name: /new task/i })).toBeInTheDocument();
		});

		it('should not render New Task button when onNewTask is not provided', () => {
			renderWithRouter(<TopBar />);
			expect(screen.queryByRole('button', { name: /new task/i })).not.toBeInTheDocument();
		});

		it('should call onNewTask when clicked', () => {
			const onNewTask = vi.fn();
			renderWithRouter(<TopBar onNewTask={onNewTask} />);

			const newTaskBtn = screen.getByRole('button', { name: /new task/i });
			fireEvent.click(newTaskBtn);

			expect(onNewTask).toHaveBeenCalledOnce();
		});
	});

	describe('accessibility', () => {
		it('should have role="banner" on header', () => {
			renderWithRouter(<TopBar />);
			expect(screen.getByRole('banner')).toBeInTheDocument();
		});

		it('should have aria-label on search input', () => {
			renderWithRouter(<TopBar />);
			expect(screen.getByLabelText('Search tasks')).toBeInTheDocument();
		});

	});

	describe('keyboard shortcuts', () => {
		it('should focus search input on Cmd+K', async () => {
			renderWithRouter(<TopBar />);
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
			renderWithRouter(<TopBar />);
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
			renderWithRouter(<TopBar />);
			expect(screen.getByText('⌘')).toBeInTheDocument();
			expect(screen.getByText('K')).toBeInTheDocument();
		});
	});

	describe('mobile search toggle', () => {
		it('should render search toggle button', () => {
			renderWithRouter(<TopBar />);
			expect(screen.getByLabelText('Toggle search')).toBeInTheDocument();
		});

		it('should have aria-expanded on search toggle', () => {
			renderWithRouter(<TopBar />);
			const toggleBtn = screen.getByLabelText('Toggle search');
			expect(toggleBtn).toHaveAttribute('aria-expanded', 'false');
		});

		it('should toggle search expanded state when clicked', () => {
			renderWithRouter(<TopBar />);
			const toggleBtn = screen.getByLabelText('Toggle search');

			fireEvent.click(toggleBtn);
			expect(toggleBtn).toHaveAttribute('aria-expanded', 'true');

			fireEvent.click(toggleBtn);
			expect(toggleBtn).toHaveAttribute('aria-expanded', 'false');
		});
	});
});

