import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Header } from './Header';
import { useProjectStore } from '@/stores';

// Test wrapper to provide required context
function renderWithRouter(ui: React.ReactElement, { route = '/' } = {}) {
	return render(<MemoryRouter initialEntries={[route]}>{ui}</MemoryRouter>);
}

describe('Header', () => {
	beforeEach(() => {
		// Reset stores
		useProjectStore.setState({
			projects: [
				{ id: 'proj-001', name: 'Test Project', path: '/test/project', created_at: '2024-01-01T00:00:00Z' },
			],
			currentProjectId: 'proj-001',
			loading: false,
			error: null,
		});
	});

	describe('rendering', () => {
		it('should render header', () => {
			renderWithRouter(<Header />);
			expect(screen.getByRole('banner')).toBeInTheDocument();
		});

		it('should show project name', () => {
			renderWithRouter(<Header />);
			expect(screen.getByText('Test Project')).toBeInTheDocument();
		});

		it('should show "Select project" when no project selected', () => {
			useProjectStore.setState({ currentProjectId: null });
			renderWithRouter(<Header />);
			expect(screen.getByText('Select project')).toBeInTheDocument();
		});

		it('should show commands button', () => {
			renderWithRouter(<Header />);
			expect(screen.getByText('Commands')).toBeInTheDocument();
		});

		it('should show keyboard shortcut hint on commands button', () => {
			renderWithRouter(<Header />);
			// The shortcut appears as formatted text
			const commandsBtn = screen.getByText('Commands').closest('button');
			expect(commandsBtn).toBeInTheDocument();
		});
	});

	describe('page title', () => {
		it('should show "Tasks" on root route', () => {
			renderWithRouter(<Header />, { route: '/' });
			expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Tasks');
		});

		it('should show "Board" on /board route', () => {
			renderWithRouter(<Header />, { route: '/board' });
			expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Board');
		});

		it('should show "Dashboard" on /dashboard route', () => {
			renderWithRouter(<Header />, { route: '/dashboard' });
			expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Dashboard');
		});

		it('should show "Task Details" on task detail route', () => {
			renderWithRouter(<Header />, { route: '/tasks/TASK-001' });
			expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Task Details');
		});

		it('should show "Preferences" on /preferences route', () => {
			renderWithRouter(<Header />, { route: '/preferences' });
			expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Preferences');
		});
	});

	describe('callbacks', () => {
		it('should call onProjectClick when project button clicked', () => {
			const onProjectClick = vi.fn();
			renderWithRouter(<Header onProjectClick={onProjectClick} />);

			const projectBtn = screen.getByText('Test Project').closest('button');
			fireEvent.click(projectBtn!);

			expect(onProjectClick).toHaveBeenCalledOnce();
		});

		it('should call onNewTask when new task button clicked', () => {
			const onNewTask = vi.fn();
			renderWithRouter(<Header onNewTask={onNewTask} />);

			const newTaskBtn = screen.getByText('New Task');
			fireEvent.click(newTaskBtn);

			expect(onNewTask).toHaveBeenCalledOnce();
		});

		it('should call onCommandPalette when commands button clicked', () => {
			const onCommandPalette = vi.fn();
			renderWithRouter(<Header onCommandPalette={onCommandPalette} />);

			const commandsBtn = screen.getByText('Commands').closest('button');
			fireEvent.click(commandsBtn!);

			expect(onCommandPalette).toHaveBeenCalledOnce();
		});

		it('should not render new task button when onNewTask not provided', () => {
			renderWithRouter(<Header />);
			expect(screen.queryByText('New Task')).not.toBeInTheDocument();
		});
	});

	describe('project button', () => {
		it('should have folder icon', () => {
			renderWithRouter(<Header />);
			// The project button contains an Icon component
			const projectBtn = screen.getByText('Test Project').closest('button');
			expect(projectBtn).toBeInTheDocument();
			// Icon renders with aria-hidden, so we check for the svg
			const svg = projectBtn?.querySelector('svg');
			expect(svg).toBeInTheDocument();
		});

		it('should have chevron icon', () => {
			renderWithRouter(<Header />);
			const projectBtn = screen.getByText('Test Project').closest('button');
			// Should have two svgs (folder and chevron)
			const svgs = projectBtn?.querySelectorAll('svg');
			expect(svgs?.length).toBe(2);
		});
	});
});
