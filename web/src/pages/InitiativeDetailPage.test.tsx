import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { create } from '@bufbuild/protobuf';
import { InitiativeDetailPage } from './InitiativeDetailPage';
import { useInitiativeStore, useProjectStore } from '@/stores';
import type { Initiative } from '@/gen/orc/v1/initiative_pb';
import {
	InitiativeStatus,
	InitiativeDecisionSchema,
} from '@/gen/orc/v1/initiative_pb';
import { TaskStatus } from '@/gen/orc/v1/task_pb';
import {
	createMockInitiative,
	createMockTaskRef,
	createTimestamp,
} from '@/test/factories';

// Mock the Connect RPC clients
const mockGetInitiative = vi.fn();
const mockUpdateInitiative = vi.fn();
const mockGetDependencyGraph = vi.fn();
const mockLinkTasks = vi.fn();
const mockUnlinkTask = vi.fn();
const mockAddDecision = vi.fn();
const mockListTasks = vi.fn();

vi.mock('@/lib/client', () => ({
	initiativeClient: {
		getInitiative: (...args: unknown[]) => mockGetInitiative(...args),
		updateInitiative: (...args: unknown[]) => mockUpdateInitiative(...args),
		getDependencyGraph: (...args: unknown[]) => mockGetDependencyGraph(...args),
		linkTasks: (...args: unknown[]) => mockLinkTasks(...args),
		unlinkTask: (...args: unknown[]) => mockUnlinkTask(...args),
		addDecision: (...args: unknown[]) => mockAddDecision(...args),
	},
	taskClient: {
		listTasks: (...args: unknown[]) => mockListTasks(...args),
	},
}));

// Helper to create a mock InitiativeDecision
function createMockDecision(overrides: {
	id?: string;
	date?: string;
	by?: string;
	decision?: string;
	rationale?: string;
} = {}) {
	return create(InitiativeDecisionSchema, {
		id: overrides.id ?? 'DEC-001',
		date: createTimestamp(overrides.date ?? '2024-01-15T00:00:00Z'),
		by: overrides.by ?? '',
		decision: overrides.decision ?? 'Test decision',
		rationale: overrides.rationale,
	});
}

describe('InitiativeDetailPage', () => {
	const mockInitiative: Initiative = createMockInitiative({
		id: 'INIT-001',
		title: 'Test Initiative',
		status: InitiativeStatus.ACTIVE,
		vision: 'Test vision statement',
		tasks: [
			createMockTaskRef({ id: 'TASK-001', title: 'First Task', status: TaskStatus.COMPLETED }),
			createMockTaskRef({ id: 'TASK-002', title: 'Second Task', status: TaskStatus.RUNNING }),
			createMockTaskRef({ id: 'TASK-003', title: 'Third Task', status: TaskStatus.CREATED }),
		],
		decisions: [
			createMockDecision({
				id: 'DEC-001',
				date: '2024-01-15T00:00:00Z',
				decision: 'Use React for frontend',
				rationale: 'Better ecosystem',
				by: 'John',
			}),
		],
	});

	beforeEach(() => {
		vi.clearAllMocks();
		mockGetInitiative.mockResolvedValue({ initiative: mockInitiative });
		mockListTasks.mockResolvedValue({ tasks: [] });
		mockGetDependencyGraph.mockResolvedValue({
			graph: { nodes: [], edges: [] },
		});
		// Reset the initiative store
		useInitiativeStore.setState({
			initiatives: new Map(),
			currentInitiativeId: null,
			loading: false,
			error: null,
			hasLoaded: false,
		});
		// Set a project ID so the component fetches data
		useProjectStore.setState({ currentProjectId: 'test-project' });
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
			mockGetInitiative.mockImplementation(
				() => new Promise((resolve) => setTimeout(() => resolve({ initiative: mockInitiative }), 100))
			);

			renderInitiativeDetailPage();
			expect(screen.getByText('Loading initiative...')).toBeInTheDocument();
		});
	});

	describe('error state', () => {
		it('shows error message when load fails', async () => {
			mockGetInitiative.mockRejectedValue(new Error('Failed to load'));

			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(screen.getByText('Failed to load')).toBeInTheDocument();
			});
		});

		it('shows retry button on error', async () => {
			mockGetInitiative.mockRejectedValue(new Error('Failed'));

			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
			});
		});

		it('shows 404 when initiative not found', async () => {
			mockGetInitiative.mockResolvedValue({ initiative: undefined });

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
			const draftInitiative = createMockInitiative({
				...mockInitiative,
				status: InitiativeStatus.DRAFT,
			});
			mockGetInitiative.mockResolvedValue({ initiative: draftInitiative });

			renderInitiativeDetailPage();

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /activate/i })).toBeInTheDocument();
			});
		});

		it('calls updateInitiative when status button clicked', async () => {
			const updatedInitiative = createMockInitiative({
				...mockInitiative,
				status: InitiativeStatus.COMPLETED,
			});
			mockUpdateInitiative.mockResolvedValue({ initiative: updatedInitiative });

			renderInitiativeDetailPage();

			await waitFor(() => {
				fireEvent.click(screen.getByRole('button', { name: /complete/i }));
			});

			await waitFor(() => {
				expect(mockUpdateInitiative).toHaveBeenCalledWith(expect.objectContaining({
					projectId: 'test-project',
					initiativeId: 'INIT-001',
					status: InitiativeStatus.COMPLETED,
				}));
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
				expect(mockGetDependencyGraph).toHaveBeenCalledWith({ projectId: 'test-project', initiativeId: 'INIT-001' });
			});
		});

		it('shows empty state when expanded with no dependencies', async () => {
			mockGetDependencyGraph.mockResolvedValue({
				graph: { nodes: [], edges: [] },
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
