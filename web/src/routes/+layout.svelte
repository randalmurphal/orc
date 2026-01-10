<script lang="ts">
	import '../app.css';
	import type { Snippet } from 'svelte';
	import Sidebar from '$lib/components/layout/Sidebar.svelte';
	import Header from '$lib/components/layout/Header.svelte';
	import CommandPalette from '$lib/components/overlays/CommandPalette.svelte';
	import ProjectSwitcher from '$lib/components/ProjectSwitcher.svelte';
	import { currentProject, loadProjects } from '$lib/stores/project';
	import { sidebarPinned } from '$lib/stores/sidebar';
	import { onMount } from 'svelte';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();

	let showProjectSwitcher = $state(false);
	let showCommandPalette = $state(false);
	let showNewTaskForm = $state(false);

	onMount(() => {
		loadProjects();

		// Global keyboard shortcuts
		function handleKeydown(e: KeyboardEvent) {
			// Don't trigger shortcuts if user is typing in an input
			const target = e.target as HTMLElement;
			if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable) {
				// Allow Escape to close modals even when in input
				if (e.key === 'Escape') {
					showProjectSwitcher = false;
					showCommandPalette = false;
					showNewTaskForm = false;
				}
				return;
			}

			// Cmd/Ctrl + B = Toggle sidebar
			if ((e.metaKey || e.ctrlKey) && e.key === 'b') {
				e.preventDefault();
				sidebarPinned.toggle();
			}

			// Cmd/Ctrl + P = Project switcher
			if ((e.metaKey || e.ctrlKey) && e.key === 'p') {
				e.preventDefault();
				showProjectSwitcher = true;
				showCommandPalette = false;
			}

			// Cmd/Ctrl + K = Command palette
			if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
				e.preventDefault();
				showCommandPalette = true;
				showProjectSwitcher = false;
			}

			// Cmd/Ctrl + N = New task
			if ((e.metaKey || e.ctrlKey) && e.key === 'n') {
				e.preventDefault();
				showNewTaskForm = true;
			}

			// Escape = Close overlays
			if (e.key === 'Escape') {
				showProjectSwitcher = false;
				showCommandPalette = false;
				showNewTaskForm = false;
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

		window.addEventListener('keydown', handleKeydown);
		window.addEventListener('orc:switch-project', handleSwitchProject);
		window.addEventListener('orc:toggle-sidebar', handleToggleSidebar);
		window.addEventListener('orc:new-task', handleNewTask);

		return () => {
			window.removeEventListener('keydown', handleKeydown);
			window.removeEventListener('orc:switch-project', handleSwitchProject);
			window.removeEventListener('orc:toggle-sidebar', handleToggleSidebar);
			window.removeEventListener('orc:new-task', handleNewTask);
		};
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
		max-width: 1400px;
	}
</style>
