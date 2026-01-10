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

	interface Props {
		open: boolean;
		onClose: () => void;
	}

	let { open, onClose }: Props = $props();

	let searchQuery = $state('');
	let selectedIndex = $state(0);
	let inputRef: HTMLInputElement;

	onMount(async () => {
		await loadProjects();
	});

	const filteredProjects = $derived(() => {
		if (!searchQuery.trim()) return $projects;
		const query = searchQuery.toLowerCase();
		return $projects.filter(
			(p) =>
				p.name.toLowerCase().includes(query) ||
				p.path.toLowerCase().includes(query)
		);
	});

	$effect(() => {
		if (open && inputRef) {
			inputRef.focus();
			searchQuery = '';
			selectedIndex = 0;
		}
	});

	// Reset selected index when search changes
	$effect(() => {
		searchQuery;
		selectedIndex = 0;
	});

	function handleSelect(id: string) {
		selectProject(id);
		onClose();
	}

	function handleKeydown(e: KeyboardEvent) {
		const filtered = filteredProjects();

		switch (e.key) {
			case 'ArrowDown':
				e.preventDefault();
				selectedIndex = Math.min(selectedIndex + 1, filtered.length - 1);
				break;
			case 'ArrowUp':
				e.preventDefault();
				selectedIndex = Math.max(selectedIndex - 1, 0);
				break;
			case 'Enter':
				e.preventDefault();
				if (filtered[selectedIndex]) {
					handleSelect(filtered[selectedIndex].id);
				}
				break;
			case 'Escape':
				onClose();
				break;
		}
	}

	function handleBackdropClick(e: MouseEvent) {
		if (e.target === e.currentTarget) {
			onClose();
		}
	}
</script>

{#if open}
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="switcher-backdrop"
		role="dialog"
		aria-modal="true"
		aria-label="Switch project"
		onclick={handleBackdropClick}
		onkeydown={handleKeydown}
	>
		<div class="switcher-content">
			<!-- Header -->
			<div class="switcher-header">
				<h2>Switch Project</h2>
				<button class="close-btn" onclick={onClose} aria-label="Close" title="Close (Esc)">
					<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<line x1="18" y1="6" x2="6" y2="18" />
						<line x1="6" y1="6" x2="18" y2="18" />
					</svg>
				</button>
			</div>

			<!-- Current Project -->
			{#if $currentProject}
				<div class="current-project">
					<span class="current-label">Current</span>
					<div class="current-info">
						<span class="current-name">{$currentProject.name}</span>
						<span class="current-path">{$currentProject.path}</span>
					</div>
				</div>
			{/if}

			<!-- Search Input -->
			<div class="switcher-search">
				<svg class="search-icon" xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<circle cx="11" cy="11" r="8" />
					<path d="m21 21-4.35-4.35" />
				</svg>
				<input
					bind:this={inputRef}
					bind:value={searchQuery}
					type="text"
					placeholder="Search projects..."
					class="search-input"
					aria-label="Search projects"
				/>
			</div>

			<!-- Project List -->
			<div class="project-list">
				{#if $projectsLoading}
					<div class="loading-state">
						<div class="spinner"></div>
						<span>Loading projects...</span>
					</div>
				{:else if $projectsError}
					<div class="error-state">
						<span class="error-icon">!</span>
						<span class="error-message">{$projectsError}</span>
						<button class="retry-btn" onclick={() => loadProjects()}>
							Retry
						</button>
					</div>
				{:else if filteredProjects().length === 0}
					<div class="empty-state">
						{#if searchQuery.trim()}
							<p>No projects match "{searchQuery}"</p>
						{:else}
							<p>No projects registered</p>
							<span class="empty-hint">Run `orc init` in a project directory</span>
						{/if}
					</div>
				{:else}
					{#each filteredProjects() as project, i (project.id)}
						<button
							class="project-item"
							class:selected={i === selectedIndex}
							class:active={project.id === $currentProjectId}
							onclick={() => handleSelect(project.id)}
							onmouseenter={() => (selectedIndex = i)}
						>
							<div class="project-icon">
								{#if project.id === $currentProjectId}
									<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
										<polyline points="20 6 9 17 4 12" />
									</svg>
								{:else}
									<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
										<path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
									</svg>
								{/if}
							</div>
							<div class="project-info">
								<span class="project-name">{project.name}</span>
								<span class="project-path">{project.path}</span>
							</div>
							{#if project.id === $currentProjectId}
								<span class="active-badge">Active</span>
							{/if}
						</button>
					{/each}
				{/if}
			</div>

			<!-- Footer -->
			<div class="switcher-footer">
				<div class="footer-hint">
					<kbd>&uarr;</kbd><kbd>&darr;</kbd> navigate
				</div>
				<div class="footer-hint">
					<kbd>&crarr;</kbd> select
				</div>
				<div class="footer-hint">
					<kbd>esc</kbd> close
				</div>
			</div>
		</div>
	</div>
{/if}

<style>
	.switcher-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.6);
		backdrop-filter: blur(4px);
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding: var(--space-16) var(--space-4);
		z-index: 1100;
		animation: fade-in var(--duration-fast) var(--ease-out);
	}

	.switcher-content {
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-xl);
		box-shadow: var(--shadow-2xl);
		width: 100%;
		max-width: 480px;
		max-height: 70vh;
		overflow: hidden;
		display: flex;
		flex-direction: column;
		animation: modal-content-in var(--duration-fast) var(--ease-out);
	}

	/* Header */
	.switcher-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--space-4) var(--space-5);
		border-bottom: 1px solid var(--border-subtle);
	}

	.switcher-header h2 {
		font-size: var(--text-base);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		margin: 0;
	}

	.close-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		background: transparent;
		border: none;
		border-radius: var(--radius-md);
		color: var(--text-muted);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.close-btn:hover {
		background: var(--bg-tertiary);
		color: var(--text-primary);
	}

	/* Current Project */
	.current-project {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-3) var(--space-5);
		background: var(--accent-subtle);
		border-bottom: 1px solid var(--border-subtle);
	}

	.current-label {
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		color: var(--accent-primary);
		padding: var(--space-0-5) var(--space-2);
		background: var(--accent-primary);
		color: var(--text-inverse);
		border-radius: var(--radius-sm);
	}

	.current-info {
		flex: 1;
		min-width: 0;
	}

	.current-name {
		display: block;
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
	}

	.current-path {
		display: block;
		font-size: var(--text-xs);
		color: var(--text-muted);
		font-family: var(--font-mono);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	/* Search */
	.switcher-search {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-3) var(--space-5);
		border-bottom: 1px solid var(--border-subtle);
	}

	.search-icon {
		flex-shrink: 0;
		color: var(--text-muted);
	}

	.search-input {
		flex: 1;
		background: transparent;
		border: none;
		font-size: var(--text-sm);
		color: var(--text-primary);
		outline: none;
	}

	.search-input::placeholder {
		color: var(--text-muted);
	}

	/* Project List */
	.project-list {
		flex: 1;
		overflow-y: auto;
		padding: var(--space-2);
	}

	.project-item {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		width: 100%;
		padding: var(--space-3);
		background: transparent;
		border: none;
		border-radius: var(--radius-md);
		text-align: left;
		cursor: pointer;
		transition: background var(--duration-fast) var(--ease-out);
	}

	.project-item:hover,
	.project-item.selected {
		background: var(--bg-tertiary);
	}

	.project-item.selected {
		outline: 1px solid var(--accent-muted);
	}

	.project-item.active {
		background: var(--accent-subtle);
	}

	.project-icon {
		flex-shrink: 0;
		width: 28px;
		height: 28px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--bg-tertiary);
		border-radius: var(--radius-md);
		color: var(--text-secondary);
	}

	.project-item.active .project-icon {
		background: var(--accent-primary);
		color: var(--text-inverse);
	}

	.project-info {
		flex: 1;
		min-width: 0;
	}

	.project-name {
		display: block;
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
	}

	.project-path {
		display: block;
		font-size: var(--text-xs);
		color: var(--text-muted);
		font-family: var(--font-mono);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.active-badge {
		flex-shrink: 0;
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		color: var(--accent-primary);
		padding: var(--space-0-5) var(--space-1-5);
		background: var(--accent-subtle);
		border-radius: var(--radius-sm);
	}

	/* States */
	.loading-state,
	.error-state,
	.empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: var(--space-8);
		text-align: center;
		gap: var(--space-3);
	}

	.spinner {
		width: 24px;
		height: 24px;
		border: 2px solid var(--border-default);
		border-top-color: var(--accent-primary);
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.loading-state span,
	.empty-state p {
		font-size: var(--text-sm);
		color: var(--text-muted);
	}

	.empty-hint {
		font-size: var(--text-xs);
		font-family: var(--font-mono);
		color: var(--text-muted);
		background: var(--bg-tertiary);
		padding: var(--space-1) var(--space-2);
		border-radius: var(--radius-sm);
	}

	.error-icon {
		width: 32px;
		height: 32px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--status-danger-bg);
		border-radius: 50%;
		font-weight: var(--font-bold);
		color: var(--status-danger);
	}

	.error-message {
		font-size: var(--text-sm);
		color: var(--status-danger);
	}

	.retry-btn {
		padding: var(--space-2) var(--space-4);
		background: var(--accent-primary);
		border: none;
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-inverse);
		cursor: pointer;
		transition: background var(--duration-fast) var(--ease-out);
	}

	.retry-btn:hover {
		background: var(--accent-hover);
	}

	/* Footer */
	.switcher-footer {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-5);
		padding: var(--space-3);
		border-top: 1px solid var(--border-subtle);
		background: var(--bg-tertiary);
	}

	.footer-hint {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.footer-hint kbd {
		padding: var(--space-0-5) var(--space-1);
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-sm);
		font-family: var(--font-mono);
		font-size: var(--text-2xs);
	}
</style>
