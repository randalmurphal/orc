<script lang="ts">
	import { onMount } from 'svelte';
	import {
		projects,
		currentProject,
		currentProjectId,
		projectsLoading,
		projectsError,
		loadProjects,
		selectProject
	} from '$lib/stores/project';

	let showDropdown = $state(false);

	onMount(async () => {
		await loadProjects();
	});

	function handleSelect(id: string) {
		selectProject(id);
		showDropdown = false;
	}

	function handleClickOutside(event: MouseEvent) {
		const target = event.target as HTMLElement;
		if (!target.closest('.project-switcher')) {
			showDropdown = false;
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && showDropdown) {
			showDropdown = false;
		}
	}
</script>

<svelte:window onclick={handleClickOutside} onkeydown={handleKeydown} />

<div class="project-switcher">
	<button
		class="switcher-button"
		onclick={() => showDropdown = !showDropdown}
		disabled={$projectsLoading || ($projects.length === 0 && !$projectsError)}
	>
		{#if $projectsLoading}
			<span class="loading">Loading...</span>
		{:else if $projectsError}
			<span class="error" title={$projectsError}>Error</span>
			<span class="chevron">{showDropdown ? '▲' : '▼'}</span>
		{:else if $currentProject}
			<span class="project-name">{$currentProject.name}</span>
			<span class="chevron">{showDropdown ? '▲' : '▼'}</span>
		{:else}
			<span class="no-project">No projects</span>
		{/if}
	</button>

	{#if showDropdown}
		<div class="dropdown">
			{#if $projectsError}
				<div class="dropdown-error">
					<span class="error-message">{$projectsError}</span>
					<button class="retry-button" onclick={() => loadProjects()}>Retry</button>
				</div>
			{:else if $projects.length > 0}
				{#each $projects as project (project.id)}
					<button
						class="dropdown-item"
						class:active={project.id === $currentProjectId}
						onclick={() => handleSelect(project.id)}
					>
						<span class="project-name">{project.name}</span>
						<span class="project-path">{project.path}</span>
					</button>
				{/each}
			{:else}
				<div class="dropdown-empty">No projects registered</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.project-switcher {
		position: relative;
	}

	.switcher-button {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		background: var(--bg-tertiary);
		border: 1px solid var(--border-color);
		border-radius: 6px;
		padding: 0.5rem 0.75rem;
		color: var(--text-primary);
		font-size: 0.875rem;
		cursor: pointer;
		min-width: 150px;
	}

	.switcher-button:hover:not(:disabled) {
		background: var(--bg-secondary);
	}

	.switcher-button:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.project-name {
		flex: 1;
		text-align: left;
		font-weight: 500;
	}

	.no-project {
		color: var(--text-secondary);
		font-style: italic;
	}

	.chevron {
		font-size: 0.625rem;
		color: var(--text-secondary);
	}

	.dropdown {
		position: absolute;
		top: 100%;
		left: 0;
		right: 0;
		margin-top: 0.25rem;
		background: var(--bg-secondary);
		border: 1px solid var(--border-color);
		border-radius: 6px;
		overflow: hidden;
		z-index: 100;
		min-width: 250px;
	}

	.dropdown-item {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		width: 100%;
		padding: 0.75rem;
		border: none;
		background: transparent;
		color: var(--text-primary);
		cursor: pointer;
		text-align: left;
	}

	.dropdown-item:hover {
		background: var(--bg-tertiary);
	}

	.dropdown-item.active {
		background: var(--accent-primary);
		color: white;
	}

	.dropdown-item .project-name {
		font-weight: 500;
	}

	.dropdown-item .project-path {
		font-size: 0.75rem;
		color: var(--text-secondary);
		margin-top: 0.25rem;
	}

	.dropdown-item.active .project-path {
		color: rgba(255, 255, 255, 0.8);
	}

	.loading {
		color: var(--text-secondary);
		font-style: italic;
	}

	.error {
		color: var(--error-color, #ef4444);
		font-weight: 500;
	}

	.dropdown-error {
		padding: 0.75rem;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.error-message {
		color: var(--error-color, #ef4444);
		font-size: 0.875rem;
	}

	.retry-button {
		background: var(--accent-primary);
		color: white;
		border: none;
		border-radius: 4px;
		padding: 0.5rem 0.75rem;
		cursor: pointer;
		font-size: 0.75rem;
		align-self: flex-start;
	}

	.retry-button:hover {
		opacity: 0.9;
	}

	.dropdown-empty {
		padding: 0.75rem;
		color: var(--text-secondary);
		font-style: italic;
		font-size: 0.875rem;
	}
</style>
