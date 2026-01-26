import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { create } from '@bufbuild/protobuf';
import { ProjectSwitcher } from './ProjectSwitcher';
import { useProjectStore } from '@/stores';
import { createTimestamp } from '@/test/factories';
import { ProjectSchema } from '@/gen/orc/v1/project_pb';

// Mock localStorage
const localStorageMock = (() => {
	let store: Record<string, string> = {};
	return {
		getItem: vi.fn((key: string) => store[key] ?? null),
		setItem: vi.fn((key: string, value: string) => {
			store[key] = value;
		}),
		removeItem: vi.fn((key: string) => {
			delete store[key];
		}),
		clear: vi.fn(() => {
			store = {};
		}),
	};
})();
Object.defineProperty(window, 'localStorage', { value: localStorageMock });

// Mock window.history
const historyMock = {
	pushState: vi.fn(),
	replaceState: vi.fn(),
};
Object.defineProperty(window, 'history', { value: historyMock, writable: true });

describe('ProjectSwitcher', () => {
	const mockProjects = [
		create(ProjectSchema, { id: 'proj-001', name: 'Test Project', path: '/test/project', createdAt: createTimestamp('2024-01-01T00:00:00Z') }),
		create(ProjectSchema, { id: 'proj-002', name: 'Another Project', path: '/another/project', createdAt: createTimestamp('2024-01-02T00:00:00Z') }),
		create(ProjectSchema, { id: 'proj-003', name: 'Third Project', path: '/third/project', createdAt: createTimestamp('2024-01-03T00:00:00Z') }),
	];

	beforeEach(() => {
		// Reset stores
		useProjectStore.setState({
			projects: mockProjects,
			currentProjectId: 'proj-001',
			loading: false,
			error: null,
			_isHandlingPopState: false,
		});
		localStorageMock.clear();
		vi.clearAllMocks();
	});

	describe('rendering', () => {
		it('should not render when closed', () => {
			render(<ProjectSwitcher open={false} onClose={() => {}} />);
			expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
		});

		it('should render dialog when open', () => {
			render(<ProjectSwitcher open={true} onClose={() => {}} />);
			expect(screen.getByRole('dialog')).toBeInTheDocument();
		});

		it('should show "Switch Project" title', () => {
			render(<ProjectSwitcher open={true} onClose={() => {}} />);
			expect(screen.getByText('Switch Project')).toBeInTheDocument();
		});

		it('should show current project indicator', () => {
			render(<ProjectSwitcher open={true} onClose={() => {}} />);
			expect(screen.getByText('Current')).toBeInTheDocument();
			// Test Project appears in both current section and project list
			const currentSection = screen.getByText('Current').closest('.current-project');
			expect(currentSection).toBeInTheDocument();
			expect(currentSection?.querySelector('.current-name')?.textContent).toBe('Test Project');
		});

		it('should show search input', () => {
			render(<ProjectSwitcher open={true} onClose={() => {}} />);
			expect(screen.getByPlaceholderText('Search projects...')).toBeInTheDocument();
		});

		it('should show project list', () => {
			render(<ProjectSwitcher open={true} onClose={() => {}} />);
			// All projects should appear in the list
			const projectList = screen.getByRole('dialog').querySelector('.project-list');
			expect(projectList).toBeInTheDocument();
			expect(screen.getByText('Another Project')).toBeInTheDocument();
			expect(screen.getByText('Third Project')).toBeInTheDocument();
		});

		it('should show "Active" badge for current project', () => {
			render(<ProjectSwitcher open={true} onClose={() => {}} />);
			expect(screen.getByText('Active')).toBeInTheDocument();
		});

		it('should show footer hints', () => {
			render(<ProjectSwitcher open={true} onClose={() => {}} />);
			expect(screen.getByText('navigate')).toBeInTheDocument();
			expect(screen.getByText('select')).toBeInTheDocument();
			expect(screen.getByText('close')).toBeInTheDocument();
		});
	});

	describe('search functionality', () => {
		it('should filter projects by name', () => {
			render(<ProjectSwitcher open={true} onClose={() => {}} />);

			const searchInput = screen.getByPlaceholderText('Search projects...');
			fireEvent.change(searchInput, { target: { value: 'Another' } });

			expect(screen.getByText('Another Project')).toBeInTheDocument();
			expect(screen.queryByText('Third Project')).not.toBeInTheDocument();
		});

		it('should filter projects by path', () => {
			render(<ProjectSwitcher open={true} onClose={() => {}} />);

			const searchInput = screen.getByPlaceholderText('Search projects...');
			fireEvent.change(searchInput, { target: { value: '/third' } });

			expect(screen.getByText('Third Project')).toBeInTheDocument();
			expect(screen.queryByText('Another Project')).not.toBeInTheDocument();
		});

		it('should show empty message when no matches', () => {
			render(<ProjectSwitcher open={true} onClose={() => {}} />);

			const searchInput = screen.getByPlaceholderText('Search projects...');
			fireEvent.change(searchInput, { target: { value: 'nonexistent' } });

			expect(screen.getByText(/No projects match "nonexistent"/)).toBeInTheDocument();
		});
	});

	describe('keyboard navigation', () => {
		it('should close on Escape', () => {
			const onClose = vi.fn();
			render(<ProjectSwitcher open={true} onClose={onClose} />);

			const dialog = screen.getByRole('dialog');
			fireEvent.keyDown(dialog, { key: 'Escape' });

			expect(onClose).toHaveBeenCalledOnce();
		});

		it('should select project on Enter', () => {
			const onClose = vi.fn();
			render(<ProjectSwitcher open={true} onClose={onClose} />);

			const dialog = screen.getByRole('dialog');
			fireEvent.keyDown(dialog, { key: 'Enter' });

			// First project is selected by default
			expect(onClose).toHaveBeenCalledOnce();
		});

		it('should navigate down on ArrowDown', () => {
			render(<ProjectSwitcher open={true} onClose={() => {}} />);

			const dialog = screen.getByRole('dialog');
			const projectList = dialog.querySelector('.project-list');

			// First item should be selected initially
			const projectButtons = projectList?.querySelectorAll('button.project-item');
			expect(projectButtons?.[0]).toHaveClass('selected');

			// Navigate down
			fireEvent.keyDown(dialog, { key: 'ArrowDown' });

			// Second item should now be selected
			expect(projectButtons?.[1]).toHaveClass('selected');
		});

		it('should navigate up on ArrowUp', () => {
			render(<ProjectSwitcher open={true} onClose={() => {}} />);

			const dialog = screen.getByRole('dialog');
			const projectList = dialog.querySelector('.project-list');

			// Navigate down first, then up
			fireEvent.keyDown(dialog, { key: 'ArrowDown' });
			fireEvent.keyDown(dialog, { key: 'ArrowUp' });

			// First item should be selected again
			const projectButtons = projectList?.querySelectorAll('button.project-item');
			expect(projectButtons?.[0]).toHaveClass('selected');
		});
	});

	describe('project selection', () => {
		it('should select project when clicked', () => {
			const onClose = vi.fn();
			render(<ProjectSwitcher open={true} onClose={onClose} />);

			const projectItem = screen.getByText('Another Project').closest('button');
			fireEvent.click(projectItem!);

			expect(useProjectStore.getState().currentProjectId).toBe('proj-002');
			expect(onClose).toHaveBeenCalledOnce();
		});

		it('should highlight project on mouse enter', () => {
			render(<ProjectSwitcher open={true} onClose={() => {}} />);

			const secondItem = screen.getByText('Another Project').closest('button');
			fireEvent.mouseEnter(secondItem!);

			expect(secondItem).toHaveClass('selected');
		});
	});

	describe('close button', () => {
		it('should close when close button clicked', () => {
			const onClose = vi.fn();
			render(<ProjectSwitcher open={true} onClose={onClose} />);

			const closeBtn = screen.getByRole('button', { name: 'Close' });
			fireEvent.click(closeBtn);

			expect(onClose).toHaveBeenCalledOnce();
		});
	});

	describe('backdrop click', () => {
		it('should close when backdrop clicked', () => {
			const onClose = vi.fn();
			render(<ProjectSwitcher open={true} onClose={onClose} />);

			const backdrop = screen.getByRole('dialog');
			// Click directly on the backdrop (not on content)
			fireEvent.click(backdrop);

			expect(onClose).toHaveBeenCalledOnce();
		});

		it('should not close when content clicked', () => {
			const onClose = vi.fn();
			render(<ProjectSwitcher open={true} onClose={onClose} />);

			const content = screen.getByText('Switch Project').closest('.switcher-content');
			fireEvent.click(content!);

			expect(onClose).not.toHaveBeenCalled();
		});
	});

	describe('loading state', () => {
		it('should show loading indicator when loading', () => {
			useProjectStore.setState({ loading: true, projects: [] });
			render(<ProjectSwitcher open={true} onClose={() => {}} />);

			expect(screen.getByText('Loading projects...')).toBeInTheDocument();
		});
	});

	describe('error state', () => {
		it('should show error message when error exists', () => {
			useProjectStore.setState({ error: 'Failed to load', projects: [] });
			render(<ProjectSwitcher open={true} onClose={() => {}} />);

			expect(screen.getByText('Failed to load')).toBeInTheDocument();
		});
	});

	describe('empty state', () => {
		it('should show empty message when no projects', () => {
			useProjectStore.setState({ projects: [] });
			render(<ProjectSwitcher open={true} onClose={() => {}} />);

			expect(screen.getByText('No projects registered')).toBeInTheDocument();
			expect(screen.getByText(/orc init/)).toBeInTheDocument();
		});
	});

	describe('focus management', () => {
		it('should focus search input when opened', async () => {
			render(<ProjectSwitcher open={true} onClose={() => {}} />);

			await waitFor(() => {
				const searchInput = screen.getByPlaceholderText('Search projects...');
				expect(searchInput).toHaveFocus();
			});
		});
	});
});
