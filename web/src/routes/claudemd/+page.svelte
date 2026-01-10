<script lang="ts">
	import { onMount } from 'svelte';
	import {
		getClaudeMD,
		updateClaudeMD,
		getClaudeMDHierarchy,
		type ClaudeMD,
		type ClaudeMDHierarchy
	} from '$lib/api';

	let content = '';
	let hierarchy: ClaudeMDHierarchy | null = null;
	let selectedSource: 'project' | 'global' | 'user' = 'project';
	let loading = true;
	let saving = false;
	let error: string | null = null;
	let success: string | null = null;
	let hasProject = false;

	onMount(async () => {
		try {
			hierarchy = await getClaudeMDHierarchy();
			hasProject = !!hierarchy.project;

			// Load project content if it exists
			if (hierarchy.project) {
				content = hierarchy.project.content;
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load CLAUDE.md';
		} finally {
			loading = false;
		}
	});

	function selectSource(source: 'project' | 'global' | 'user') {
		selectedSource = source;
		if (hierarchy) {
			switch (source) {
				case 'global':
					content = hierarchy.global?.content || '';
					break;
				case 'user':
					content = hierarchy.user?.content || '';
					break;
				case 'project':
					content = hierarchy.project?.content || '';
					break;
			}
		}
	}

	async function handleSave() {
		if (selectedSource !== 'project') {
			error = 'Only project CLAUDE.md can be edited';
			return;
		}

		saving = true;
		error = null;
		success = null;

		try {
			await updateClaudeMD(content);
			success = 'CLAUDE.md saved successfully';
			hasProject = true;

			// Refresh hierarchy
			hierarchy = await getClaudeMDHierarchy();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save CLAUDE.md';
		} finally {
			saving = false;
		}
	}

	function getSourceLabel(source: string): string {
		switch (source) {
			case 'global':
				return 'Global (~/.claude/CLAUDE.md)';
			case 'user':
				return 'User (~/CLAUDE.md)';
			case 'project':
				return 'Project (./CLAUDE.md)';
			default:
				return source;
		}
	}
</script>

<svelte:head>
	<title>CLAUDE.md - orc</title>
</svelte:head>

<div class="claudemd-page">
	<header class="page-header">
		<div class="header-content">
			<div>
				<h1>CLAUDE.md</h1>
				<p class="subtitle">View and edit project instructions for Claude</p>
			</div>
			{#if selectedSource === 'project'}
				<button class="btn btn-primary" on:click={handleSave} disabled={saving}>
					{saving ? 'Saving...' : 'Save'}
				</button>
			{/if}
		</div>
	</header>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if success}
		<div class="alert alert-success">{success}</div>
	{/if}

	{#if loading}
		<div class="loading">Loading CLAUDE.md...</div>
	{:else}
		<div class="claudemd-layout">
			<!-- Source Selector -->
			<aside class="source-list">
				<h2>Sources</h2>
				<p class="help-text">CLAUDE.md files are applied in order: global, user, project</p>
				<ul>
					{#if hierarchy?.global}
						<li>
							<button
								class="source-item"
								class:selected={selectedSource === 'global'}
								on:click={() => selectSource('global')}
							>
								<span class="source-name">Global</span>
								<span class="source-path">~/.claude/CLAUDE.md</span>
							</button>
						</li>
					{/if}
					{#if hierarchy?.user}
						<li>
							<button
								class="source-item"
								class:selected={selectedSource === 'user'}
								on:click={() => selectSource('user')}
							>
								<span class="source-name">User</span>
								<span class="source-path">~/CLAUDE.md</span>
							</button>
						</li>
					{/if}
					<li>
						<button
							class="source-item"
							class:selected={selectedSource === 'project'}
							on:click={() => selectSource('project')}
						>
							<span class="source-name">Project</span>
							<span class="source-path">./CLAUDE.md</span>
							{#if !hasProject}
								<span class="badge badge-new">New</span>
							{/if}
						</button>
					</li>
				</ul>
			</aside>

			<!-- Editor Panel -->
			<div class="editor-panel">
				<div class="editor-header">
					<h2>{getSourceLabel(selectedSource)}</h2>
					{#if selectedSource !== 'project'}
						<span class="readonly-badge">Read Only</span>
					{/if}
				</div>

				<div class="editor-content">
					<textarea
						bind:value={content}
						placeholder={selectedSource === 'project'
							? '# Project Instructions\n\nAdd instructions for Claude here...'
							: 'No content'}
						disabled={selectedSource !== 'project'}
						rows="30"
					></textarea>
				</div>
			</div>
		</div>
	{/if}
</div>

<style>
	.claudemd-page {
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
	}

	.page-header h1 {
		margin: 0;
		font-size: 1.5rem;
	}

	.header-content {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
	}

	.subtitle {
		margin: 0.5rem 0 0;
		color: var(--text-secondary);
		font-size: 0.875rem;
	}

	.alert {
		padding: 0.75rem 1rem;
		border-radius: 6px;
		font-size: 0.875rem;
	}

	.alert-error {
		background: var(--error-bg, #fee2e2);
		color: var(--error-text, #dc2626);
		border: 1px solid var(--error-border, #fecaca);
	}

	.alert-success {
		background: var(--success-bg, #dcfce7);
		color: var(--success-text, #16a34a);
		border: 1px solid var(--success-border, #bbf7d0);
	}

	.loading {
		text-align: center;
		padding: 3rem;
		color: var(--text-secondary);
	}

	.claudemd-layout {
		display: grid;
		grid-template-columns: 250px 1fr;
		gap: 1.5rem;
		min-height: 600px;
	}

	/* Source List */
	.source-list {
		background: var(--bg-secondary);
		border-radius: 8px;
		padding: 1rem;
		border: 1px solid var(--border-color);
	}

	.source-list h2 {
		font-size: 0.875rem;
		font-weight: 600;
		margin: 0 0 0.5rem;
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.help-text {
		font-size: 0.75rem;
		color: var(--text-secondary);
		margin: 0 0 1rem;
	}

	.source-list ul {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.source-item {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		width: 100%;
		padding: 0.75rem;
		background: transparent;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		text-align: left;
		color: var(--text-primary);
		font-size: 0.875rem;
		gap: 0.25rem;
	}

	.source-item:hover {
		background: var(--bg-tertiary, rgba(0, 0, 0, 0.05));
	}

	.source-item.selected {
		background: var(--primary-bg, #dbeafe);
		color: var(--primary-text, #1d4ed8);
	}

	.source-name {
		font-weight: 600;
	}

	.source-path {
		font-size: 0.75rem;
		color: var(--text-secondary);
		font-family: 'JetBrains Mono', 'Fira Code', monospace;
	}

	.source-item.selected .source-path {
		color: var(--primary-text, #1d4ed8);
		opacity: 0.7;
	}

	.badge {
		font-size: 0.625rem;
		padding: 0.125rem 0.375rem;
		border-radius: 4px;
		text-transform: uppercase;
		font-weight: 600;
	}

	.badge-new {
		background: var(--info-bg, #dbeafe);
		color: var(--info-text, #1d4ed8);
	}

	/* Editor Panel */
	.editor-panel {
		display: flex;
		flex-direction: column;
		background: var(--bg-secondary);
		border-radius: 8px;
		border: 1px solid var(--border-color);
		overflow: hidden;
	}

	.editor-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 1rem;
		border-bottom: 1px solid var(--border-color);
	}

	.editor-header h2 {
		margin: 0;
		font-size: 1rem;
	}

	.readonly-badge {
		font-size: 0.75rem;
		padding: 0.25rem 0.5rem;
		border-radius: 4px;
		background: var(--bg-tertiary, #e5e7eb);
		color: var(--text-secondary);
	}

	.editor-content {
		flex: 1;
		padding: 1rem;
	}

	.editor-content textarea {
		width: 100%;
		height: 100%;
		min-height: 500px;
		padding: 1rem;
		border: 1px solid var(--border-color);
		border-radius: 6px;
		font-size: 0.875rem;
		background: var(--bg-primary);
		color: var(--text-primary);
		font-family: 'JetBrains Mono', 'Fira Code', monospace;
		resize: vertical;
	}

	.editor-content textarea:focus {
		outline: none;
		border-color: var(--primary, #3b82f6);
	}

	.editor-content textarea:disabled {
		background: var(--bg-tertiary, #f3f4f6);
		cursor: not-allowed;
	}

	.btn {
		padding: 0.5rem 1rem;
		border-radius: 6px;
		font-size: 0.875rem;
		font-weight: 500;
		cursor: pointer;
		border: 1px solid transparent;
	}

	.btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.btn-primary {
		background: var(--primary, #3b82f6);
		color: white;
	}

	.btn-primary:hover:not(:disabled) {
		background: var(--primary-hover, #2563eb);
	}

	@media (max-width: 768px) {
		.claudemd-layout {
			grid-template-columns: 1fr;
		}

		.source-list {
			max-height: 200px;
			overflow-y: auto;
		}
	}
</style>
