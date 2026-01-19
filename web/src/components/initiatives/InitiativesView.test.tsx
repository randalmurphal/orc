import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { InitiativesView } from './InitiativesView';
import * as api from '@/lib/api';
import { useTaskStore } from '@/stores';
import type { Initiative, Task } from '@/lib/types';

// Mock the API functions
vi.mock('@/lib/api', () => ({
	listInitiatives: vi.fn(),
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

describe('InitiativesView', () => {
	const mockInitiatives: Initiative[] = [
		{
			version: 1,
			id: 'INIT-001',
			title: 'Frontend Polish',
			status: 'active',
			vision: 'UI refresh with component library',
			tasks: [
				{ id: 'TASK-001', title: 'Task 1', status: 'completed' },
				{ id: 'TASK-002', title: 'Task 2', status: 'running' },
			],
			created_at: '2024-01-01T00:00:00Z',
			updated_at: '2024-01-15T00:00:00Z',
		},
		{
			version: 1,
			id: 'INIT-002',
			title: 'Auth Overhaul',
			status: 'active',
			vision: 'OAuth2 with PKCE',
			tasks: [
				{ id: 'TASK-003', title: 'Task 3', status: 'completed' },
				{ id: 'TASK-004', title: 'Task 4', status: 'completed' },
				{ id: 'TASK-005', title: 'Task 5', status: 'created' },
			],
			created_at: '2024-01-01T00:00:00Z',
			updated_at: '2024-01-15T00:00:00Z',
		},
		{
			version: 1,
			id: 'INIT-003',
			title: 'Analytics Dashboard',
			status: 'draft',
			vision: 'Real-time metrics',
			tasks: [],
			created_at: '2024-01-01T00:00:00Z',
			updated_at: '2024-01-15T00:00:00Z',
		},
	];

	const mockTasks: Task[] = [
		{
			id: 'TASK-001',
			title: 'Task 1',
			status: 'completed',
			weight: 'small',
			branch: 'orc/TASK-001',
			initiative_id: 'INIT-001',
			created_at: '2024-01-01T00:00:00Z',
			updated_at: '2024-01-15T00:00:00Z',
		},
		{
			id: 'TASK-002',
			title: 'Task 2',
			status: 'running',
			weight: 'medium',
			branch: 'orc/TASK-002',
			initiative_id: 'INIT-001',
			created_at: '2024-01-01T00:00:00Z',
			updated_at: '2024-01-15T00:00:00Z',
		},
		{
			id: 'TASK-003',
			title: 'Task 3',
			status: 'completed',
			weight: 'small',
			branch: 'orc/TASK-003',
			initiative_id: 'INIT-002',
			created_at: '2024-01-01T00:00:00Z',
			updated_at: '2024-01-15T00:00:00Z',
		},
		{
			id: 'TASK-004',
			title: 'Task 4',
			status: 'completed',
			weight: 'medium',
			branch: 'orc/TASK-004',
			initiative_id: 'INIT-002',
			created_at: '2024-01-01T00:00:00Z',
			updated_at: '2024-01-15T00:00:00Z',
		},
		{
			id: 'TASK-005',
			title: 'Task 5',
			status: 'created',
			weight: 'large',
			branch: 'orc/TASK-005',
			initiative_id: 'INIT-002',
			created_at: '2024-01-01T00:00:00Z',
			updated_at: '2024-01-15T00:00:00Z',
		},
	];

	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(api.listInitiatives).mockResolvedValue(mockInitiatives);
		// Set up task store with mock tasks
		useTaskStore.setState({ tasks: mockTasks, taskStates: new Map() });
	});

	const renderInitiativesView = () => {
		return render(
			<MemoryRouter>
				<InitiativesView />
			</MemoryRouter>
		);
	};

	describe('loading state', () => {
		it('shows loading skeleton initially', async () => {
			// Delay the API response
			vi.mocked(api.listInitiatives).mockImplementation(
				() =>
					new Promise((resolve) => setTimeout(() => resolve(mockInitiatives), 100))
			);

			renderInitiativesView();

			// Should show skeleton cards
			expect(document.querySelector('.initiatives-view-card-skeleton')).toBeInTheDocument();
		});

		it('shows stats row in loading state', async () => {
			vi.mocked(api.listInitiatives).mockImplementation(
				() =>
					new Promise((resolve) => setTimeout(() => resolve(mockInitiatives), 100))
			);

			renderInitiativesView();

			// Stats row should be present
			expect(document.querySelector('.stats-row')).toBeInTheDocument();
		});
	});

	describe('error state', () => {
		it('shows error message when load fails', async () => {
			vi.mocked(api.listInitiatives).mockRejectedValue(new Error('Failed to load'));

			renderInitiativesView();

			await waitFor(() => {
				expect(screen.getByText('Failed to load initiatives')).toBeInTheDocument();
				expect(screen.getByText('Failed to load')).toBeInTheDocument();
			});
		});

		it('shows retry button on error', async () => {
			vi.mocked(api.listInitiatives).mockRejectedValue(new Error('Failed'));

			renderInitiativesView();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
			});
		});

		it('retries loading when retry button is clicked', async () => {
			vi.mocked(api.listInitiatives)
				.mockRejectedValueOnce(new Error('Failed'))
				.mockResolvedValueOnce(mockInitiatives);

			renderInitiativesView();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /retry/i }));

			await waitFor(() => {
				expect(api.listInitiatives).toHaveBeenCalledTimes(2);
			});
		});
	});

	describe('empty state', () => {
		it('shows empty state when no initiatives', async () => {
			vi.mocked(api.listInitiatives).mockResolvedValue([]);

			renderInitiativesView();

			await waitFor(() => {
				expect(screen.getByText('Create your first initiative')).toBeInTheDocument();
			});
		});

		it('shows helpful description in empty state', async () => {
			vi.mocked(api.listInitiatives).mockResolvedValue([]);

			renderInitiativesView();

			await waitFor(() => {
				expect(
					screen.getByText(/initiatives help you organize related tasks/i)
				).toBeInTheDocument();
			});
		});
	});

	describe('populated state', () => {
		it('renders initiative cards in grid', async () => {
			renderInitiativesView();

			await waitFor(() => {
				expect(screen.getByText('Frontend Polish')).toBeInTheDocument();
				expect(screen.getByText('Auth Overhaul')).toBeInTheDocument();
				expect(screen.getByText('Analytics Dashboard')).toBeInTheDocument();
			});
		});

		it('displays initiative visions', async () => {
			renderInitiativesView();

			await waitFor(() => {
				expect(screen.getByText('UI refresh with component library')).toBeInTheDocument();
				expect(screen.getByText('OAuth2 with PKCE')).toBeInTheDocument();
			});
		});

		it('displays correct task progress', async () => {
			renderInitiativesView();

			await waitFor(() => {
				// INIT-001: 1 completed / 2 total (from tasks with initiative_id)
				expect(screen.getByText('1 / 2 tasks')).toBeInTheDocument();
				// INIT-002: 2 completed / 3 total
				expect(screen.getByText('2 / 3 tasks')).toBeInTheDocument();
			});
		});
	});

	describe('aggregate stats calculation', () => {
		it('calculates active initiatives count', async () => {
			renderInitiativesView();

			await waitFor(() => {
				// 2 active initiatives (INIT-001, INIT-002), 1 draft
				expect(screen.getByText('2')).toBeInTheDocument();
			});
		});

		it('calculates total tasks count', async () => {
			renderInitiativesView();

			await waitFor(() => {
				// 5 total tasks linked to initiatives
				expect(screen.getByText('5')).toBeInTheDocument();
			});
		});

		it('calculates completion rate', async () => {
			renderInitiativesView();

			await waitFor(() => {
				// 3 completed out of 5 = 60%
				expect(screen.getByText('60%')).toBeInTheDocument();
			});
		});
	});

	describe('header', () => {
		it('displays page title', async () => {
			renderInitiativesView();

			await waitFor(() => {
				expect(screen.getByText('Initiatives')).toBeInTheDocument();
			});
		});

		it('displays page subtitle', async () => {
			renderInitiativesView();

			await waitFor(() => {
				expect(
					screen.getByText('Manage your project epics and milestones')
				).toBeInTheDocument();
			});
		});

		it('displays New Initiative button', async () => {
			renderInitiativesView();

			await waitFor(() => {
				expect(
					screen.getByRole('button', { name: /new initiative/i })
				).toBeInTheDocument();
			});
		});
	});

	describe('new initiative button', () => {
		it('dispatches orc:new-initiative event when clicked', async () => {
			const dispatchSpy = vi.spyOn(window, 'dispatchEvent');

			renderInitiativesView();

			await waitFor(() => {
				fireEvent.click(screen.getByRole('button', { name: /new initiative/i }));
			});

			expect(dispatchSpy).toHaveBeenCalledWith(
				expect.objectContaining({
					type: 'orc:new-initiative',
				})
			);

			dispatchSpy.mockRestore();
		});
	});

	describe('card click navigation', () => {
		it('navigates to initiative detail when card is clicked', async () => {
			renderInitiativesView();

			await waitFor(() => {
				expect(screen.getByText('Frontend Polish')).toBeInTheDocument();
			});

			// Find and click the initiative card
			const card = screen.getByText('Frontend Polish').closest('article');
			expect(card).toBeInTheDocument();
			fireEvent.click(card!);

			expect(mockNavigate).toHaveBeenCalledWith('/initiatives/INIT-001');
		});

		it('navigates to correct initiative on different card click', async () => {
			renderInitiativesView();

			await waitFor(() => {
				expect(screen.getByText('Auth Overhaul')).toBeInTheDocument();
			});

			const card = screen.getByText('Auth Overhaul').closest('article');
			fireEvent.click(card!);

			expect(mockNavigate).toHaveBeenCalledWith('/initiatives/INIT-002');
		});
	});

	describe('accessibility', () => {
		it('has proper heading hierarchy', async () => {
			renderInitiativesView();

			await waitFor(() => {
				const heading = screen.getByRole('heading', { level: 1 });
				expect(heading).toHaveTextContent('Initiatives');
			});
		});

		it('empty state has status role', async () => {
			vi.mocked(api.listInitiatives).mockResolvedValue([]);

			renderInitiativesView();

			await waitFor(() => {
				expect(screen.getByRole('status')).toBeInTheDocument();
			});
		});

		it('error state has alert role', async () => {
			vi.mocked(api.listInitiatives).mockRejectedValue(new Error('Failed'));

			renderInitiativesView();

			await waitFor(() => {
				expect(screen.getByRole('alert')).toBeInTheDocument();
			});
		});
	});
});
