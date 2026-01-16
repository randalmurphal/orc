import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { InitiativeDetail } from './InitiativeDetail';
import * as api from '@/lib/api';
import type { Initiative } from '@/lib/types';

// Mock the API functions
vi.mock('@/lib/api', () => ({
	getInitiative: vi.fn(),
	updateInitiative: vi.fn(),
	addInitiativeTask: vi.fn(),
	removeInitiativeTask: vi.fn(),
	addInitiativeDecision: vi.fn(),
	listTasks: vi.fn(),
	getInitiativeDependencyGraph: vi.fn(),
}));

// Mock useNavigate
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
	const actual = await vi.importActual('react-router-dom');
	return {
		...actual,
		useNavigate: () => mockNavigate,
	};
});

describe('InitiativeDetail', () => {
	const mockInitiative: Initiative = {
		version: 1,
		id: 'INIT-001',
		title: 'Test Initiative',
		status: 'active',
		vision: 'Test vision statement',
		tasks: [
			{ id: 'TASK-001', title: 'First Task', status: 'completed' },
			{ id: 'TASK-002', title: 'Second Task', status: 'running' },
		],
		decisions: [
			{
				id: 'DEC-001',
				date: '2024-01-15',
				decision: 'Use React for frontend',
				rationale: 'Better ecosystem',
				by: 'John',
			},
		],
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-15T00:00:00Z',
	};

	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.getInitiative).mockResolvedValue(mockInitiative);
		vi.mocked(api.listTasks).mockResolvedValue([]);
		vi.mocked(api.getInitiativeDependencyGraph).mockResolvedValue({
			nodes: [],
			edges: [],
		});
	});

	const renderInitiativeDetail = (initiativeId: string = 'INIT-001') => {
		return render(
			<MemoryRouter initialEntries={[`/initiatives/${initiativeId}`]}>
				<Routes>
					<Route path="/initiatives/:id" element={<InitiativeDetail />} />
				</Routes>
			</MemoryRouter>
		);
	};

	describe('loading state', () => {
		it('shows loading spinner initially', async () => {
			// Delay the API response
			vi.mocked(api.getInitiative).mockImplementation(
				() => new Promise((resolve) => setTimeout(() => resolve(mockInitiative), 100))
			);

			renderInitiativeDetail();
			expect(screen.getByText('Loading initiative...')).toBeInTheDocument();
		});
	});

	describe('error state', () => {
		it('shows error message when load fails', async () => {
			vi.mocked(api.getInitiative).mockRejectedValue(new Error('Failed to load'));

			renderInitiativeDetail();

			await waitFor(() => {
				expect(screen.getByText('Failed to load')).toBeInTheDocument();
			});
		});

		it('shows retry button on error', async () => {
			vi.mocked(api.getInitiative).mockRejectedValue(new Error('Failed'));

			renderInitiativeDetail();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
			});
		});
	});

	describe('header section', () => {
		it('displays initiative title', async () => {
			renderInitiativeDetail();

			await waitFor(() => {
				expect(screen.getByText('Test Initiative')).toBeInTheDocument();
			});
		});

		it('displays vision statement', async () => {
			renderInitiativeDetail();

			await waitFor(() => {
				expect(screen.getByText('Test vision statement')).toBeInTheDocument();
			});
		});

		it('displays status badge', async () => {
			renderInitiativeDetail();

			await waitFor(() => {
				expect(screen.getByText('active')).toBeInTheDocument();
			});
		});

		it('displays progress bar', async () => {
			renderInitiativeDetail();

			await waitFor(() => {
				expect(screen.getByText('1/2 tasks (50%)')).toBeInTheDocument();
			});
		});
	});

	describe('status management', () => {
		it('shows Complete button for active initiative', async () => {
			renderInitiativeDetail();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /complete/i })).toBeInTheDocument();
			});
		});

		it('shows Activate button for draft initiative', async () => {
			vi.mocked(api.getInitiative).mockResolvedValue({
				...mockInitiative,
				status: 'draft',
			});

			renderInitiativeDetail();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /activate/i })).toBeInTheDocument();
			});
		});

		it('calls updateInitiative when status button clicked', async () => {
			vi.mocked(api.updateInitiative).mockResolvedValue({
				...mockInitiative,
				status: 'completed',
			});

			renderInitiativeDetail();

			await waitFor(() => {
				fireEvent.click(screen.getByRole('button', { name: /complete/i }));
			});

			await waitFor(() => {
				expect(api.updateInitiative).toHaveBeenCalledWith('INIT-001', {
					status: 'completed',
				});
			});
		});
	});

	describe('tab navigation', () => {
		it('shows all three tabs', async () => {
			renderInitiativeDetail();

			await waitFor(() => {
				expect(screen.getByRole('tab', { name: /tasks/i })).toBeInTheDocument();
				expect(screen.getByRole('tab', { name: /graph/i })).toBeInTheDocument();
				expect(screen.getByRole('tab', { name: /decisions/i })).toBeInTheDocument();
			});
		});

		it('shows task count badge', async () => {
			renderInitiativeDetail();

			await waitFor(() => {
				// Tab has count badge showing 2 tasks
				const tasksTab = screen.getByRole('tab', { name: /tasks/i });
				expect(tasksTab).toHaveTextContent('2');
			});
		});

		it('switches to decisions tab when clicked', async () => {
			renderInitiativeDetail();

			await waitFor(() => {
				fireEvent.click(screen.getByRole('tab', { name: /decisions/i }));
			});

			await waitFor(() => {
				expect(screen.getByText('Use React for frontend')).toBeInTheDocument();
			});
		});
	});

	describe('tasks tab', () => {
		it('displays linked tasks', async () => {
			renderInitiativeDetail();

			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
				expect(screen.getByText('First Task')).toBeInTheDocument();
				expect(screen.getByText('TASK-002')).toBeInTheDocument();
				expect(screen.getByText('Second Task')).toBeInTheDocument();
			});
		});

		it('shows Add Task button', async () => {
			renderInitiativeDetail();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /add task/i })).toBeInTheDocument();
			});
		});

		it('shows Link Existing button', async () => {
			renderInitiativeDetail();

			await waitFor(() => {
				expect(
					screen.getByRole('button', { name: /link existing/i })
				).toBeInTheDocument();
			});
		});
	});

	describe('decisions tab', () => {
		it('displays decisions list', async () => {
			renderInitiativeDetail();

			await waitFor(() => {
				fireEvent.click(screen.getByRole('tab', { name: /decisions/i }));
			});

			await waitFor(() => {
				expect(screen.getByText('Use React for frontend')).toBeInTheDocument();
				expect(screen.getByText(/better ecosystem/i)).toBeInTheDocument();
				expect(screen.getByText(/by john/i)).toBeInTheDocument();
			});
		});

		it('shows Add Decision button', async () => {
			renderInitiativeDetail();

			await waitFor(() => {
				fireEvent.click(screen.getByRole('tab', { name: /decisions/i }));
			});

			await waitFor(() => {
				expect(
					screen.getByRole('button', { name: /add decision/i })
				).toBeInTheDocument();
			});
		});
	});

	describe('graph tab', () => {
		it('loads graph data when tab clicked', async () => {
			renderInitiativeDetail();

			await waitFor(() => {
				fireEvent.click(screen.getByRole('tab', { name: /graph/i }));
			});

			await waitFor(() => {
				expect(api.getInitiativeDependencyGraph).toHaveBeenCalledWith('INIT-001');
			});
		});

		it('shows empty state when no tasks have dependencies', async () => {
			vi.mocked(api.getInitiativeDependencyGraph).mockResolvedValue({
				nodes: [],
				edges: [],
			});

			renderInitiativeDetail();

			await waitFor(() => {
				fireEvent.click(screen.getByRole('tab', { name: /graph/i }));
			});

			await waitFor(() => {
				expect(
					screen.getByText(/no tasks with dependencies to visualize/i)
				).toBeInTheDocument();
			});
		});
	});

	describe('edit modal', () => {
		it('opens edit modal when Edit button clicked', async () => {
			renderInitiativeDetail();

			await waitFor(() => {
				fireEvent.click(screen.getByRole('button', { name: /edit/i }));
			});

			await waitFor(() => {
				expect(screen.getByText('Edit Initiative')).toBeInTheDocument();
			});
		});

		it('pre-fills form with current values', async () => {
			renderInitiativeDetail();

			await waitFor(() => {
				fireEvent.click(screen.getByRole('button', { name: /edit/i }));
			});

			await waitFor(() => {
				expect(screen.getByDisplayValue('Test Initiative')).toBeInTheDocument();
				expect(screen.getByDisplayValue('Test vision statement')).toBeInTheDocument();
			});
		});
	});

	describe('archive confirmation', () => {
		it('opens confirmation modal when Archive clicked', async () => {
			renderInitiativeDetail();

			await waitFor(() => {
				fireEvent.click(screen.getByRole('button', { name: /^archive$/i }));
			});

			await waitFor(() => {
				// Modal title
				expect(screen.getByRole('heading', { name: 'Archive Initiative' })).toBeInTheDocument();
				expect(
					screen.getByText(/are you sure you want to archive/i)
				).toBeInTheDocument();
			});
		});
	});

	describe('back link', () => {
		it('shows back to tasks link', async () => {
			renderInitiativeDetail();

			await waitFor(() => {
				expect(screen.getByText('Back to Tasks')).toBeInTheDocument();
			});
		});

		it('links to board filtered by initiative', async () => {
			renderInitiativeDetail();

			await waitFor(() => {
				const backLink = screen.getByRole('link', { name: /back to tasks/i });
				expect(backLink).toHaveAttribute('href', '/board?initiative=INIT-001');
			});
		});
	});
});
