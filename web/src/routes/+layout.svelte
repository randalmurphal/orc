<script lang="ts">
	import '../app.css';
	import type { Snippet } from 'svelte';
	import { goto } from '$app/navigation';
	import Sidebar from '$lib/components/layout/Sidebar.svelte';
	import Header from '$lib/components/layout/Header.svelte';
	import CommandPalette from '$lib/components/overlays/CommandPalette.svelte';
	import ProjectSwitcher from '$lib/components/ProjectSwitcher.svelte';
	import KeyboardShortcutsHelp from '$lib/components/overlays/KeyboardShortcutsHelp.svelte';
	import ToastContainer from '$lib/components/ui/ToastContainer.svelte';
	import { currentProject, loadProjects, currentProjectId } from '$lib/stores/project';
	import { sidebarPinned } from '$lib/stores/sidebar';
	import { loadTasks, updateTaskStatus, updateTaskState, refreshTask, addTask, removeTask, updateTask } from '$lib/stores/tasks';
	import { initGlobalWebSocket, type WSEventType, type ConnectionStatus } from '$lib/websocket';
	import { toast } from '$lib/stores/toast.svelte';
	import { setupGlobalShortcuts } from '$lib/shortcuts';
	import { onMount, onDestroy } from 'svelte';
	import type { Task, TaskState } from '$lib/types';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();

	let showProjectSwitcher = $state(false);
	let showCommandPalette = $state(false);
	let showNewTaskForm = $state(false);
	let showShortcutsHelp = $state(false);
	let cleanupShortcuts: (() => void) | null = null;
	let cleanupWebSocket: (() => void) | null = null;
	let wsStatus = $state<ConnectionStatus>('disconnected');

	// Handle WebSocket events globally
	function handleGlobalWSEvent(taskId: string, eventType: WSEventType, data: unknown) {
		switch (eventType) {
			case 'state': {
				const stateData = data as TaskState;
				updateTaskState(taskId, stateData);

				// Show toasts for important state changes
				if (stateData.status === 'completed') {
					toast.success(`Task ${taskId} completed`, { title: 'Task Complete' });
				} else if (stateData.status === 'failed') {
					toast.error(`Task ${taskId} failed`, { title: 'Task Failed' });
				} else if (stateData.status === 'blocked') {
					toast.warning(`Task ${taskId} is blocked`, { title: 'Task Blocked' });
				}
				break;
			}
			case 'phase': {
				const phaseData = data as { phase?: string; status?: string };
				if (phaseData.status === 'started' || phaseData.status === 'completed' || phaseData.status === 'failed') {
					// Refresh the task to get updated current_phase
					refreshTask(taskId);
				}
				if (phaseData.status === 'completed') {
					toast.info(`Phase ${phaseData.phase} completed for ${taskId}`, { duration: 3000 });
				}
				break;
			}
			case 'complete': {
				const completeData = data as { status?: string };
				if (completeData.status) {
					updateTaskStatus(taskId, completeData.status as 'completed' | 'failed');
				}
				// Refresh to get final state
				refreshTask(taskId);
				break;
			}
			case 'error': {
				const errorData = data as { message?: string; fatal?: boolean };
				if (errorData.fatal) {
					toast.error(errorData.message || `Error in task ${taskId}`, { title: 'Task Error' });
				}
				break;
			}
			// File watcher events (triggered by external file changes)
			case 'task_created': {
				const taskData = data as { task: Task };
				if (taskData.task) {
					addTask(taskData.task);
					toast.info(`Task ${taskId} created`, { duration: 3000 });
				}
				break;
			}
			case 'task_updated': {
				const taskData = data as { task: Task };
				if (taskData.task) {
					updateTask(taskId, taskData.task);
				}
				break;
			}
			case 'task_deleted': {
				removeTask(taskId);
				toast.info(`Task ${taskId} deleted`, { duration: 3000 });
				break;
			}
		}
	}

	onMount(() => {
		loadProjects();
		loadTasks();

		// Initialize global WebSocket for real-time updates
		cleanupWebSocket = initGlobalWebSocket(
			handleGlobalWSEvent,
			(status) => { wsStatus = status; }
		);

		// Reload tasks when project changes
		const unsubProject = currentProjectId.subscribe(() => {
			loadTasks();
		});

		// Setup global shortcuts using ShortcutManager
		cleanupShortcuts = setupGlobalShortcuts({
			onCommandPalette: () => {
				showCommandPalette = true;
				showProjectSwitcher = false;
			},
			onNewTask: () => {
				showNewTaskForm = true;
			},
			onToggleSidebar: () => {
				sidebarPinned.toggle();
			},
			onHelp: () => {
				showShortcutsHelp = true;
			},
			onEscape: () => {
				showProjectSwitcher = false;
				showCommandPalette = false;
				showNewTaskForm = false;
				showShortcutsHelp = false;
			},
			// Navigation sequences
			onGoDashboard: () => goto('/dashboard'),
			onGoTasks: () => goto('/'),
			onGoEnvironment: () => goto('/environment'),
			onGoPreferences: () => goto('/preferences'),
			onGoPrompts: () => goto('/environment/orchestrator/prompts'),
			onGoHooks: () => goto('/environment/claude/hooks'),
			onGoSkills: () => goto('/environment/claude/skills')
		});

		// Additional shortcuts not in the manager (Cmd+P for project switcher)
		function handleKeydown(e: KeyboardEvent) {
			const target = e.target as HTMLElement;
			if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable) {
				if (e.key === 'Escape') {
					showProjectSwitcher = false;
					showCommandPalette = false;
					showNewTaskForm = false;
					showShortcutsHelp = false;
				}
				return;
			}

			// Cmd/Ctrl + P = Project switcher (not handled by ShortcutManager to avoid conflict)
			if ((e.metaKey || e.ctrlKey) && e.key === 'p') {
				e.preventDefault();
				showProjectSwitcher = true;
				showCommandPalette = false;
			}
		}

		// Listen for custom events from command palette
		function handleSwitchProject() {
			showProjectSwitcher = true;
		}

		function handleToggleSidebar() {
			sidebarPinned.toggle();
		}

		function handleNewTask() {
			showNewTaskForm = true;
		}

		function handleShowShortcuts() {
			showShortcutsHelp = true;
		}

		window.addEventListener('keydown', handleKeydown);
		window.addEventListener('orc:switch-project', handleSwitchProject);
		window.addEventListener('orc:toggle-sidebar', handleToggleSidebar);
		window.addEventListener('orc:new-task', handleNewTask);
		window.addEventListener('orc:show-shortcuts', handleShowShortcuts);

		return () => {
			window.removeEventListener('keydown', handleKeydown);
			window.removeEventListener('orc:switch-project', handleSwitchProject);
			window.removeEventListener('orc:toggle-sidebar', handleToggleSidebar);
			window.removeEventListener('orc:new-task', handleNewTask);
			window.removeEventListener('orc:show-shortcuts', handleShowShortcuts);
			unsubProject();
		};
	});

	onDestroy(() => {
		if (cleanupShortcuts) {
			cleanupShortcuts();
		}
		if (cleanupWebSocket) {
			cleanupWebSocket();
		}
	});

	function handleProjectClick() {
		showProjectSwitcher = true;
	}

	function handleNewTaskClick() {
		showNewTaskForm = true;
	}

	function handleCommandPaletteClick() {
		showCommandPalette = true;
	}

	// Export state for child pages
	export function getNewTaskFormState() {
		return {
			show: showNewTaskForm,
			setShow: (value: boolean) => {
				showNewTaskForm = value;
			}
		};
	}
</script>

<svelte:head>
	<title>{$currentProject?.name || 'orc'}</title>
</svelte:head>

<div class="app-layout">
	<Sidebar />

	<div class="main-area">
		<Header
			currentProject={$currentProject}
			onProjectClick={handleProjectClick}
			onNewTask={handleNewTaskClick}
			onCommandPalette={handleCommandPaletteClick}
		/>

		<main>
			{@render children()}
		</main>
	</div>
</div>

<!-- Command Palette -->
<CommandPalette
	open={showCommandPalette}
	onClose={() => (showCommandPalette = false)}
/>

<!-- Project Switcher Modal -->
<ProjectSwitcher
	open={showProjectSwitcher}
	onClose={() => (showProjectSwitcher = false)}
/>

<!-- Keyboard Shortcuts Help -->
<KeyboardShortcutsHelp
	open={showShortcutsHelp}
	onClose={() => (showShortcutsHelp = false)}
/>

<!-- Toast Notifications -->
<ToastContainer />

<style>
	.app-layout {
		display: flex;
		min-height: 100vh;
		background: var(--bg-primary);
	}

	.main-area {
		flex: 1;
		margin-left: var(--sidebar-width-collapsed);
		display: flex;
		flex-direction: column;
		height: 100vh;
		overflow: hidden;
		transition: margin-left var(--duration-normal) var(--ease-out);
	}

	main {
		flex: 1;
		padding: var(--space-6);
		overflow-y: auto;
		overflow-x: hidden;
		/* Center children within the now-balanced main-area */
		display: flex;
		flex-direction: column;
		align-items: center;
	}

	main > :global(*) {
		width: 100%;
		max-width: 1200px;
	}

	/* Full-width pages like the board need to break out of centering */
	main > :global(.full-width) {
		max-width: calc(100vw - var(--sidebar-width-collapsed) - var(--space-6) * 2);
		align-self: stretch;
	}
</style>
