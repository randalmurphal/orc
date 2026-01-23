import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { InitiativeDetailPage } from './InitiativeDetailPage';
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

describe('InitiativeDetailPage', () => {
	const mockInitiative: Initiative = {
		version: 1,
		id: 'INIT-001',
		title: 'Test Initiative',
		status: 'active',
		vision: 'Test vision statement',
		tasks: [
			{ id: 'TASK-001', title: 'First Task', status: 'completed' },
			{ id: 'TASK-002', title: 'Second Task', status: 'running' },
			{ id: 'TASK-003', title: 'Third Task', status: 'pending' },
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

	const renderInitiativeDetailPage = (initiativeId: string = 'INIT-001') => {
		return render(
			<MemoryRouter initialEntries={[`/initiatives/${initiativeId}`]}>
				<Routes>
					<Route path="/initiatives/:id" element={<InitiativeDetailPage />} />
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

			renderInitiativeDetailPage();
			expect(screen.getByText('Loading initiative...')).toBeInTheDocument();
		});
	});

	describe('error state', () => {
		it('shows error message when load fails', async () => {
			vi.mocked(api.getInitiative).mockRejectedValue(new Error('Failed to load'));

			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(screen.getByText('Failed to load')).toBeInTheDocument();
			});
		});

		it('shows retry button on error', async () => {
			vi.mocked(api.getInitiative).mockRejectedValue(new Error('Failed'));

			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
			});
		});

		it('shows 404 when initiative not found', async () => {
			vi.mocked(api.getInitiative).mockResolvedValue(null as unknown as Awaited<ReturnType<typeof api.getInitiative>>);

			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(screen.getByText('Initiative not found')).toBeInTheDocument();
			});
		});
	});

	describe('header section', () => {
		it('displays initiative title', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(screen.getByText('Test Initiative')).toBeInTheDocument();
			});
		});

		it('displays vision statement', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(screen.getByText('Test vision statement')).toBeInTheDocument();
			});
		});

		it('displays status badge', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(screen.getByText('active')).toBeInTheDocument();
			});
		});

		it('displays progress bar with correct values', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(screen.getByText('1/3 tasks (33%)')).toBeInTheDocument();
			});
		});
	});

	describe('stats row', () => {
		it('displays Total Tasks stat', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(screen.getByText('Total Tasks')).toBeInTheDocument();
				expect(screen.getByText('3')).toBeInTheDocument();
			});
		});

		it('displays Completed stat', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				// Check for the stat label inside stat-card
				const statCards = document.querySelectorAll('.stat-card');
				const completedCard = Array.from(statCards).find(
					(card) => card.textContent?.includes('Completed')
				);
				expect(completedCard).toBeTruthy();
				// 1 completed task - the value should be in the same card
				expect(completedCard?.textContent).toContain('1');
			});
		});

		it('displays Total Cost stat', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(screen.getByText('Total Cost')).toBeInTheDocument();
				expect(screen.getByText('$0.00')).toBeInTheDocument();
			});
		});
	});

	describe('status management', () => {
		it('shows Complete button for active initiative', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /complete/i })).toBeInTheDocument();
			});
		});

		it('shows Activate button for draft initiative', async () => {
			vi.mocked(api.getInitiative).mockResolvedValue({
				...mockInitiative,
				status: 'draft',
			});

			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /activate/i })).toBeInTheDocument();
			});
		});

		it('calls updateInitiative when status button clicked', async () => {
			vi.mocked(api.updateInitiative).mockResolvedValue({
				...mockInitiative,
				status: 'completed',
			});

			renderInitiativeDetailPage();

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

	describe('task list', () => {
		it('displays all linked tasks', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
				expect(screen.getByText('First Task')).toBeInTheDocument();
				expect(screen.getByText('TASK-002')).toBeInTheDocument();
				expect(screen.getByText('Second Task')).toBeInTheDocument();
				expect(screen.getByText('TASK-003')).toBeInTheDocument();
				expect(screen.getByText('Third Task')).toBeInTheDocument();
			});
		});

		it('shows Link Existing button', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(
					screen.getByRole('button', { name: /link existing/i })
				).toBeInTheDocument();
			});
		});
	});

	describe('task filter', () => {
		it('shows filter dropdown', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(screen.getByRole('combobox', { name: /filter tasks/i })).toBeInTheDocument();
			});
		});

		it('filters to completed tasks', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				const filterSelect = screen.getByRole('combobox', { name: /filter tasks/i });
				fireEvent.change(filterSelect, { target: { value: 'completed' } });
			});

			await waitFor(() => {
				expect(screen.getByText('TASK-001')).toBeInTheDocument();
				expect(screen.queryByText('TASK-002')).not.toBeInTheDocument();
				expect(screen.queryByText('TASK-003')).not.toBeInTheDocument();
			});
		});

		it('filters to running tasks', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				const filterSelect = screen.getByRole('combobox', { name: /filter tasks/i });
				fireEvent.change(filterSelect, { target: { value: 'running' } });
			});

			await waitFor(() => {
				expect(screen.queryByText('TASK-001')).not.toBeInTheDocument();
				expect(screen.getByText('TASK-002')).toBeInTheDocument();
				expect(screen.queryByText('TASK-003')).not.toBeInTheDocument();
			});
		});

		it('filters to planned tasks', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				const filterSelect = screen.getByRole('combobox', { name: /filter tasks/i });
				fireEvent.change(filterSelect, { target: { value: 'planned' } });
			});

			await waitFor(() => {
				expect(screen.queryByText('TASK-001')).not.toBeInTheDocument();
				expect(screen.queryByText('TASK-002')).not.toBeInTheDocument();
				expect(screen.getByText('TASK-003')).toBeInTheDocument();
			});
		});
	});

	describe('decisions section', () => {
		it('displays decisions inline (not in tab)', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				// Should be visible immediately without clicking a tab
				expect(screen.getByText('Use React for frontend')).toBeInTheDocument();
				expect(screen.getByText(/better ecosystem/i)).toBeInTheDocument();
				expect(screen.getByText(/by john/i)).toBeInTheDocument();
			});
		});

		it('shows Add Decision button', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(
					screen.getByRole('button', { name: /add decision/i })
				).toBeInTheDocument();
			});
		});
	});

	describe('dependency graph section', () => {
		it('shows expand button for collapsed graph', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /expand/i })).toBeInTheDocument();
			});
		});

		it('is collapsed by default', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				// The expand button should be visible when collapsed
				const expandButton = screen.getByRole('button', { name: /expand/i });
				expect(expandButton).toHaveAttribute('aria-expanded', 'false');
			});
		});

		it('loads graph data when expanded', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				const expandButton = screen.getByRole('button', { name: /expand/i });
				fireEvent.click(expandButton);
			});

			await waitFor(() => {
				expect(api.getInitiativeDependencyGraph).toHaveBeenCalledWith('INIT-001');
			});
		});

		it('shows empty state when expanded with no dependencies', async () => {
			vi.mocked(api.getInitiativeDependencyGraph).mockResolvedValue({
				nodes: [],
				edges: [],
			});

			renderInitiativeDetailPage();

			await waitFor(() => {
				const expandButton = screen.getByRole('button', { name: /expand/i });
				fireEvent.click(expandButton);
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
			renderInitiativeDetailPage();

			await waitFor(() => {
				fireEvent.click(screen.getByRole('button', { name: /edit/i }));
			});

			await waitFor(() => {
				expect(screen.getByText('Edit Initiative')).toBeInTheDocument();
			});
		});

		it('pre-fills form with current values', async () => {
			renderInitiativeDetailPage();

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
			renderInitiativeDetailPage();

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
		it('shows back to initiatives link', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(screen.getByText('Back to Initiatives')).toBeInTheDocument();
			});
		});

		it('links to /initiatives', async () => {
			renderInitiativeDetailPage();

			await waitFor(() => {
				const backLink = screen.getByRole('link', { name: /back to initiatives/i });
				expect(backLink).toHaveAttribute('href', '/initiatives');
			});
		});
	});
});
