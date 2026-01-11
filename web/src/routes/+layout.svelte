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
	import { currentProject, loadProjects } from '$lib/stores/project';
	import { sidebarPinned } from '$lib/stores/sidebar';
	import { setupGlobalShortcuts } from '$lib/shortcuts';
	import { onMount, onDestroy } from 'svelte';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();

	let showProjectSwitcher = $state(false);
	let showCommandPalette = $state(false);
	let showNewTaskForm = $state(false);
	let showShortcutsHelp = $state(false);
	let cleanupShortcuts: (() => void) | null = null;

	onMount(() => {
		loadProjects();

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
			onGoSettings: () => goto('/settings'),
			onGoPrompts: () => goto('/prompts'),
			onGoHooks: () => goto('/hooks'),
			onGoSkills: () => goto('/skills')
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
		};
	});

	onDestroy(() => {
		if (cleanupShortcuts) {
			cleanupShortcuts();
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
		min-height: 100vh;
		transition: margin-left var(--duration-normal) var(--ease-out);
	}

	main {
		flex: 1;
		padding: var(--space-6);
		overflow-y: auto;
		max-width: 1800px;
		width: 100%;
		margin: 0 auto;
	}
</style>
