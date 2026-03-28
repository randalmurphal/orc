import { describe, it, expect, beforeEach } from 'vitest';
import { useProjectStore } from './projectStore';
import { useThreadStore } from './threadStore';
import { resetUrlMocks, setMockSearch } from '../test-setup';
import type { Project } from '@/gen/orc/v1/project_pb';

// Factory for creating test projects
function createProject(overrides: Partial<Project> = {}): Project {
	return {
		$typeName: 'orc.v1.Project',
		id: `proj-${Math.random().toString(36).slice(2, 7)}`,
		name: 'Test Project',
		path: '/path/to/project',
		...overrides,
	} as Project;
}

describe('ProjectStore', () => {
	beforeEach(() => {
		// Reset store and mocks before each test
		useProjectStore.getState().reset();
		useThreadStore.getState().reset();
		resetUrlMocks();
		localStorage.clear();
	});

	describe('setProjects', () => {
		it('should set projects array', () => {
			const projects = [
				createProject({ id: 'proj-001', name: 'Project 1' }),
				createProject({ id: 'proj-002', name: 'Project 2' }),
			];

			useProjectStore.getState().setProjects(projects);

			expect(useProjectStore.getState().projects).toHaveLength(2);
			expect(useProjectStore.getState().projects[0].name).toBe('Project 1');
		});

		it('should fall back to first project if current selection is invalid', () => {
			useProjectStore.setState({ currentProjectId: 'invalid-id' });
			const projects = [
				createProject({ id: 'proj-001' }),
				createProject({ id: 'proj-002' }),
			];

			useProjectStore.getState().setProjects(projects);

			expect(useProjectStore.getState().currentProjectId).toBe('proj-001');
		});

		it('should keep current selection if valid', () => {
			useProjectStore.setState({ currentProjectId: 'proj-002' });
			const projects = [
				createProject({ id: 'proj-001' }),
				createProject({ id: 'proj-002' }),
			];

			useProjectStore.getState().setProjects(projects);

			expect(useProjectStore.getState().currentProjectId).toBe('proj-002');
		});

		it('should set currentProjectId to null if projects array is empty', () => {
			useProjectStore.setState({ currentProjectId: 'some-id' });

			useProjectStore.getState().setProjects([]);

			expect(useProjectStore.getState().currentProjectId).toBeNull();
		});
	});

	describe('selectProject', () => {
		it('should update currentProjectId', () => {
			useProjectStore.getState().selectProject('proj-001');

			expect(useProjectStore.getState().currentProjectId).toBe('proj-001');
		});

		it('should sync to localStorage', () => {
			useProjectStore.getState().selectProject('proj-001');

			expect(localStorage.getItem('orc_current_project_id')).toBe('proj-001');
		});

		it('should push to browser history', () => {
			useProjectStore.getState().selectProject('proj-001');

			expect(window.history.pushState).toHaveBeenCalled();
		});

		it('should not push to history during popstate handling', () => {
			useProjectStore.setState({ _isHandlingPopState: true });

			useProjectStore.getState().selectProject('proj-001');

			expect(window.history.pushState).not.toHaveBeenCalled();
		});

		it('should reset thread state when switching projects', () => {
			useThreadStore.setState({
				threads: [{ id: 'thread-001' } as never],
				selectedThreadId: 'thread-001',
				error: 'stale error',
			});

			useProjectStore.setState({ currentProjectId: 'proj-001' });
			useProjectStore.getState().selectProject('proj-002');

			expect(useThreadStore.getState().threads).toEqual([]);
			expect(useThreadStore.getState().selectedThreadId).toBeNull();
			expect(useThreadStore.getState().error).toBeNull();
		});
	});

	describe('handlePopState', () => {
		it('should update selection from event state', () => {
			const event = new PopStateEvent('popstate', {
				state: { projectId: 'proj-002' },
			});

			useProjectStore.getState().handlePopState(event);

			expect(useProjectStore.getState().currentProjectId).toBe('proj-002');
		});

		it('should fall back to URL param if event state is empty', () => {
			setMockSearch('?project=proj-003');
			const event = new PopStateEvent('popstate', { state: null });

			useProjectStore.getState().handlePopState(event);

			expect(useProjectStore.getState().currentProjectId).toBe('proj-003');
		});

		it('should not push to history during popstate handling', () => {
			const event = new PopStateEvent('popstate', {
				state: { projectId: 'proj-002' },
			});

			useProjectStore.getState().handlePopState(event);

			// pushState should not be called (replaceState might be for localStorage sync)
			expect(window.history.pushState).not.toHaveBeenCalled();
		});

		it('should reset thread state when project changes from browser navigation', () => {
			useThreadStore.setState({
				threads: [{ id: 'thread-001' } as never],
				selectedThreadId: 'thread-001',
			});
			useProjectStore.setState({ currentProjectId: 'proj-001' });

			const event = new PopStateEvent('popstate', {
				state: { projectId: 'proj-002' },
			});

			useProjectStore.getState().handlePopState(event);

			expect(useThreadStore.getState().threads).toEqual([]);
			expect(useThreadStore.getState().selectedThreadId).toBeNull();
		});
	});

	describe('initializeFromUrl', () => {
		it('should initialize from URL param', () => {
			setMockSearch('?project=proj-url');

			useProjectStore.getState().initializeFromUrl();

			expect(useProjectStore.getState().currentProjectId).toBe('proj-url');
		});

		it('should fall back to localStorage if URL param is missing', () => {
			localStorage.setItem('orc_current_project_id', 'proj-stored');

			useProjectStore.getState().initializeFromUrl();

			expect(useProjectStore.getState().currentProjectId).toBe('proj-stored');
		});

		it('should use URL param over localStorage (priority)', () => {
			setMockSearch('?project=proj-url');
			localStorage.setItem('orc_current_project_id', 'proj-stored');

			useProjectStore.getState().initializeFromUrl();

			expect(useProjectStore.getState().currentProjectId).toBe('proj-url');
		});

		it('should sync URL with replaceState', () => {
			setMockSearch('?project=proj-url');

			useProjectStore.getState().initializeFromUrl();

			expect(window.history.replaceState).toHaveBeenCalled();
		});
	});

	describe('getCurrentProject', () => {
		it('should return current project by ID', () => {
			const projects = [
				createProject({ id: 'proj-001', name: 'Project 1' }),
				createProject({ id: 'proj-002', name: 'Project 2' }),
			];
			useProjectStore.getState().setProjects(projects);
			useProjectStore.setState({ currentProjectId: 'proj-002' });

			const current = useProjectStore.getState().getCurrentProject();

			expect(current?.name).toBe('Project 2');
		});

		it('should return undefined if no project selected', () => {
			const projects = [createProject({ id: 'proj-001' })];
			useProjectStore.getState().setProjects(projects);
			useProjectStore.setState({ currentProjectId: null });

			const current = useProjectStore.getState().getCurrentProject();

			expect(current).toBeUndefined();
		});

		it('should return undefined if selected project not in list', () => {
			const projects = [createProject({ id: 'proj-001' })];
			useProjectStore.getState().setProjects(projects);
			useProjectStore.setState({ currentProjectId: 'proj-999' });

			const current = useProjectStore.getState().getCurrentProject();

			expect(current).toBeUndefined();
		});
	});

	describe('loading and error states', () => {
		it('should set loading state', () => {
			useProjectStore.getState().setLoading(true);
			expect(useProjectStore.getState().loading).toBe(true);

			useProjectStore.getState().setLoading(false);
			expect(useProjectStore.getState().loading).toBe(false);
		});

		it('should set error state', () => {
			useProjectStore.getState().setError('Failed to load projects');
			expect(useProjectStore.getState().error).toBe('Failed to load projects');

			useProjectStore.getState().setError(null);
			expect(useProjectStore.getState().error).toBeNull();
		});
	});

	describe('localStorage subscription', () => {
		it('should sync currentProjectId changes to localStorage', () => {
			// Direct state update should still sync via subscription
			useProjectStore.setState({ currentProjectId: 'proj-sync-test' });

			// Wait for subscription to fire
			expect(localStorage.getItem('orc_current_project_id')).toBe('proj-sync-test');
		});
	});

	describe('reset', () => {
		it('should reset store to initial state', () => {
			const projects = [createProject({ id: 'proj-001' })];
			useProjectStore.getState().setProjects(projects);
			useProjectStore.getState().selectProject('proj-001');
			useProjectStore.getState().setLoading(true);
			useProjectStore.getState().setError('error');

			useProjectStore.getState().reset();

			expect(useProjectStore.getState().projects).toHaveLength(0);
			expect(useProjectStore.getState().currentProjectId).toBeNull();
			expect(useProjectStore.getState().loading).toBe(false);
			expect(useProjectStore.getState().error).toBeNull();
		});
	});
});
