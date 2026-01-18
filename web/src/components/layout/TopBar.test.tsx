import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { TopBar } from './TopBar';
import { useProjectStore, useSessionStore } from '@/stores';

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
			render(<TopBar />);
			expect(screen.getByRole('banner')).toBeInTheDocument();
		});

		it('should render with default props (no project selected)', () => {
			useProjectStore.setState({
				projects: [],
				currentProjectId: null,
			});
			render(<TopBar />);
			expect(screen.getByText('Select project')).toBeInTheDocument();
		});

		it('should display project name when project is selected', () => {
			render(<TopBar />);
			expect(screen.getByText('Test Project')).toBeInTheDocument();
		});

		it('should allow projectName prop to override store value', () => {
			render(<TopBar projectName="Override Project" />);
			expect(screen.getByText('Override Project')).toBeInTheDocument();
			expect(screen.queryByText('Test Project')).not.toBeInTheDocument();
		});
	});

	describe('session stats', () => {
		it('should display duration from session store', () => {
			render(<TopBar />);
			expect(screen.getByText('Session')).toBeInTheDocument();
			expect(screen.getByText('1h 23m')).toBeInTheDocument();
		});

		it('should display formatted token count', () => {
			render(<TopBar />);
			expect(screen.getByText('Tokens')).toBeInTheDocument();
			expect(screen.getByText('847K')).toBeInTheDocument();
		});

		it('should display formatted cost', () => {
			render(<TopBar />);
			expect(screen.getByText('Cost')).toBeInTheDocument();
			expect(screen.getByText('$2.34')).toBeInTheDocument();
		});

		it('should display updated values when store changes', () => {
			const { rerender } = render(<TopBar />);
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
			render(<TopBar />);
			expect(screen.getByRole('button', { name: /pause/i })).toBeInTheDocument();
		});

		it('should show "Resume" when paused', () => {
			useSessionStore.setState({ isPaused: true });
			render(<TopBar />);
			expect(screen.getByRole('button', { name: /resume/i })).toBeInTheDocument();
		});

		it('should call pauseAll() when Pause is clicked', async () => {
			const pauseAll = vi.fn().mockResolvedValue(undefined);
			useSessionStore.setState({ isPaused: false, pauseAll });

			render(<TopBar />);
			const pauseBtn = screen.getByRole('button', { name: /pause/i });
			fireEvent.click(pauseBtn);

			await waitFor(() => {
				expect(pauseAll).toHaveBeenCalledOnce();
			});
		});

		it('should call resumeAll() when Resume is clicked', async () => {
			const resumeAll = vi.fn().mockResolvedValue(undefined);
			useSessionStore.setState({ isPaused: true, resumeAll });

			render(<TopBar />);
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
			render(<TopBar onNewTask={onNewTask} />);
			expect(screen.getByRole('button', { name: /new task/i })).toBeInTheDocument();
		});

		it('should not render New Task button when onNewTask is not provided', () => {
			render(<TopBar />);
			expect(screen.queryByRole('button', { name: /new task/i })).not.toBeInTheDocument();
		});

		it('should call onNewTask when clicked', () => {
			const onNewTask = vi.fn();
			render(<TopBar onNewTask={onNewTask} />);

			const newTaskBtn = screen.getByRole('button', { name: /new task/i });
			fireEvent.click(newTaskBtn);

			expect(onNewTask).toHaveBeenCalledOnce();
		});
	});

	describe('accessibility', () => {
		it('should have role="banner" on header', () => {
			render(<TopBar />);
			expect(screen.getByRole('banner')).toBeInTheDocument();
		});

		it('should have aria-label on search input', () => {
			render(<TopBar />);
			expect(screen.getByLabelText('Search tasks')).toBeInTheDocument();
		});

		it('should have aria-haspopup="listbox" on project selector', () => {
			render(<TopBar />);
			const projectSelector = screen.getByText('Test Project').closest('button');
			expect(projectSelector).toHaveAttribute('aria-haspopup', 'listbox');
		});
	});

	describe('project selector', () => {
		it('should call onProjectChange when clicked', () => {
			const onProjectChange = vi.fn();
			render(<TopBar onProjectChange={onProjectChange} />);

			const projectSelector = screen.getByText('Test Project').closest('button');
			fireEvent.click(projectSelector!);

			expect(onProjectChange).toHaveBeenCalledOnce();
		});

		it('should have folder icon', () => {
			render(<TopBar />);
			const projectSelector = screen.getByText('Test Project').closest('button');
			// Check for SVG icons (folder and chevron-down)
			const svgs = projectSelector?.querySelectorAll('svg');
			expect(svgs?.length).toBe(2);
		});
	});
});
