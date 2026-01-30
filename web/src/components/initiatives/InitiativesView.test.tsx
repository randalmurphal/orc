import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { TooltipProvider } from '@/components/ui/Tooltip';
import { InitiativesView } from './InitiativesView';
import { useTaskStore, useProjectStore } from '@/stores';
import type { Initiative } from '@/gen/orc/v1/initiative_pb';
import { InitiativeStatus } from '@/gen/orc/v1/initiative_pb';
import type { Task } from '@/gen/orc/v1/task_pb';
import { TaskStatus, TaskWeight, ExecutionStateSchema } from '@/gen/orc/v1/task_pb';
import { TokenUsageSchema, CostTrackingSchema } from '@/gen/orc/v1/common_pb';
import { create } from '@bufbuild/protobuf';
import { createMockInitiative, createMockTask, createMockTaskRef } from '@/test/factories';

// Mock the Connect RPC client
const mockListInitiatives = vi.fn();
vi.mock('@/lib/client', () => ({
	initiativeClient: {
		listInitiatives: (...args: unknown[]) => mockListInitiatives(...args),
	},
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
		createMockInitiative({
			id: 'INIT-001',
			title: 'Frontend Polish',
			status: InitiativeStatus.ACTIVE,
			vision: 'UI refresh with component library',
			tasks: [
				createMockTaskRef({ id: 'TASK-001', title: 'Task 1', status: TaskStatus.COMPLETED }),
				createMockTaskRef({ id: 'TASK-002', title: 'Task 2', status: TaskStatus.RUNNING }),
			],
		}),
		createMockInitiative({
			id: 'INIT-002',
			title: 'Auth Overhaul',
			status: InitiativeStatus.ACTIVE,
			vision: 'OAuth2 with PKCE',
			tasks: [
				createMockTaskRef({ id: 'TASK-003', title: 'Task 3', status: TaskStatus.COMPLETED }),
				createMockTaskRef({ id: 'TASK-004', title: 'Task 4', status: TaskStatus.COMPLETED }),
				createMockTaskRef({ id: 'TASK-005', title: 'Task 5', status: TaskStatus.CREATED }),
			],
		}),
		createMockInitiative({
			id: 'INIT-003',
			title: 'Analytics Dashboard',
			status: InitiativeStatus.DRAFT,
			vision: 'Real-time metrics',
			tasks: [],
		}),
	];

	const mockTasks: Task[] = [
		createMockTask({
			id: 'TASK-001',
			title: 'Task 1',
			status: TaskStatus.COMPLETED,
			weight: TaskWeight.SMALL,
			branch: 'orc/TASK-001',
			initiativeId: 'INIT-001',
		}),
		createMockTask({
			id: 'TASK-002',
			title: 'Task 2',
			status: TaskStatus.RUNNING,
			weight: TaskWeight.MEDIUM,
			branch: 'orc/TASK-002',
			initiativeId: 'INIT-001',
		}),
		createMockTask({
			id: 'TASK-003',
			title: 'Task 3',
			status: TaskStatus.COMPLETED,
			weight: TaskWeight.SMALL,
			branch: 'orc/TASK-003',
			initiativeId: 'INIT-002',
		}),
		createMockTask({
			id: 'TASK-004',
			title: 'Task 4',
			status: TaskStatus.COMPLETED,
			weight: TaskWeight.MEDIUM,
			branch: 'orc/TASK-004',
			initiativeId: 'INIT-002',
		}),
		createMockTask({
			id: 'TASK-005',
			title: 'Task 5',
			status: TaskStatus.CREATED,
			weight: TaskWeight.LARGE,
			branch: 'orc/TASK-005',
			initiativeId: 'INIT-002',
		}),
	];

	beforeEach(() => {
		vi.clearAllMocks();
		mockListInitiatives.mockResolvedValue({ initiatives: mockInitiatives });
		// Set up task store with mock tasks
		useTaskStore.setState({ tasks: mockTasks, taskStates: new Map() });
		// Set a project ID so the component fetches data
		useProjectStore.setState({ currentProjectId: 'test-project' });
	});

	const renderInitiativesView = () => {
		return render(
			<TooltipProvider>
				<MemoryRouter>
					<InitiativesView />
				</MemoryRouter>
			</TooltipProvider>
		);
	};

	describe('loading state', () => {
		it('shows loading skeleton initially', async () => {
			// Delay the API response
			mockListInitiatives.mockImplementation(
				() =>
					new Promise((resolve) => setTimeout(() => resolve({ initiatives: mockInitiatives }), 100))
			);

			renderInitiativesView();

			// Should show skeleton cards
			expect(document.querySelector('.initiatives-view-card-skeleton')).toBeInTheDocument();
		});

		it('shows stats row in loading state', async () => {
			mockListInitiatives.mockImplementation(
				() =>
					new Promise((resolve) => setTimeout(() => resolve({ initiatives: mockInitiatives }), 100))
			);

			renderInitiativesView();

			// Stats row should be present
			expect(document.querySelector('.stats-row')).toBeInTheDocument();
		});
	});

	describe('error state', () => {
		it('shows error message when load fails', async () => {
			mockListInitiatives.mockRejectedValue(new Error('Failed to load'));

			renderInitiativesView();

			await waitFor(() => {
				expect(screen.getByText('Failed to load initiatives')).toBeInTheDocument();
				expect(screen.getByText('Failed to load')).toBeInTheDocument();
			});
		});

		it('shows retry button on error', async () => {
			mockListInitiatives.mockRejectedValue(new Error('Failed'));

			renderInitiativesView();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
			});
		});

		it('retries loading when retry button is clicked', async () => {
			mockListInitiatives
				.mockRejectedValueOnce(new Error('Failed'))
				.mockResolvedValueOnce({ initiatives: mockInitiatives });

			renderInitiativesView();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
			});

			fireEvent.click(screen.getByRole('button', { name: /retry/i }));

			await waitFor(() => {
				expect(mockListInitiatives).toHaveBeenCalledTimes(2);
			});
		});
	});

	describe('empty state', () => {
		it('shows empty state when no initiatives', async () => {
			mockListInitiatives.mockResolvedValue({ initiatives: [] });

			renderInitiativesView();

			await waitFor(() => {
				expect(screen.getByText('Create your first initiative')).toBeInTheDocument();
			});
		});

		it('shows helpful description in empty state', async () => {
			mockListInitiatives.mockResolvedValue({ initiatives: [] });

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
		it('opens new initiative modal when clicked', async () => {
			renderInitiativesView();

			await waitFor(() => {
				fireEvent.click(screen.getByRole('button', { name: /new initiative/i }));
			});

			// Modal should appear with the Create Initiative button
			await waitFor(() => {
				expect(screen.getByRole('button', { name: /create initiative/i })).toBeInTheDocument();
			});
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
			mockListInitiatives.mockResolvedValue({ initiatives: [] });

			renderInitiativesView();

			await waitFor(() => {
				expect(screen.getByRole('status')).toBeInTheDocument();
			});
		});

		it('error state has alert role', async () => {
			mockListInitiatives.mockRejectedValue(new Error('Failed'));

			renderInitiativesView();

			await waitFor(() => {
				expect(screen.getByRole('alert')).toBeInTheDocument();
			});
		});
	});

	describe('per-initiative meta info', () => {
		it('passes cost and tokens to initiative cards when task state data exists', async () => {
			// Arrange - set up taskStates with token/cost data for tasks in INIT-001
			const taskStates = new Map();
			taskStates.set(
				'TASK-001',
				create(ExecutionStateSchema, {
					tokens: create(TokenUsageSchema, {
						inputTokens: 50000,
						outputTokens: 10000,
						totalTokens: 60000,
					}),
					cost: create(CostTrackingSchema, { totalCostUsd: 2.34 }),
				})
			);
			taskStates.set(
				'TASK-002',
				create(ExecutionStateSchema, {
					tokens: create(TokenUsageSchema, {
						inputTokens: 80000,
						outputTokens: 20000,
						totalTokens: 100000,
					}),
					cost: create(CostTrackingSchema, { totalCostUsd: 5.0 }),
				})
			);

			useTaskStore.setState({ tasks: mockTasks, taskStates });

			renderInitiativesView();

			// Assert - INIT-001 card should show aggregated cost ($2.34 + $5.00 = $7.34)
			// and aggregated tokens (60000 + 100000 = 160000 -> "160K")
			await waitFor(() => {
				expect(screen.getByText('Frontend Polish')).toBeInTheDocument();
			});

			// Cost: $7.34 spent
			expect(screen.getByText('$7.34 spent')).toBeInTheDocument();
			// Tokens: 160K tokens
			expect(screen.getByText('160K tokens')).toBeInTheDocument();
		});

		it('does not render meta row when no task state data exists', async () => {
			// Arrange - empty taskStates (already set in beforeEach)
			useTaskStore.setState({ tasks: mockTasks, taskStates: new Map() });

			renderInitiativesView();

			await waitFor(() => {
				expect(screen.getByText('Frontend Polish')).toBeInTheDocument();
			});

			// Assert - no meta rows should be rendered
			const metaElements = document.querySelectorAll('.initiative-card-meta');
			expect(metaElements).toHaveLength(0);
		});
	});

});
