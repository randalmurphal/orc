<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import {
		getClaudeMD,
		updateClaudeMD,
		getClaudeMDHierarchy,
		type ClaudeMD,
		type ClaudeMDHierarchy
	} from '$lib/api';

	let content = $state('');
	let hierarchy = $state<ClaudeMDHierarchy | null>(null);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let success = $state<string | null>(null);

	// Get scope from URL params (default to project)
	const urlScope = $derived($page.url.searchParams.get('scope') as 'global' | 'user' | 'project' | null);
	const selectedSource = $derived(urlScope || 'project');

	onMount(async () => {
		// Read scope directly from URL on mount
		const urlScopeParam = new URL(window.location.href).searchParams.get('scope');
		const initialScope = (urlScopeParam as 'global' | 'user' | 'project') || 'project';
		await loadContent(initialScope);
	});

	async function loadContent(scope: 'global' | 'user' | 'project' = 'project') {
		loading = true;
		error = null;

		try {
			hierarchy = await getClaudeMDHierarchy();

			// Load content for the selected scope
			const claudeMD = await getClaudeMD(scope);
			content = claudeMD.content || '';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load CLAUDE.md';
		} finally {
			loading = false;
		}
	}

	// Reload when scope changes
	$effect(() => {
		if (!loading && hierarchy) {
			loadContentForScope();
		}
	});

	async function loadContentForScope() {
		try {
			const claudeMD = await getClaudeMD(selectedSource);
			content = claudeMD.content || '';
		} catch (e) {
			// Ignore errors when switching - will show empty
			content = '';
		}
	}

	function selectSource(source: 'project' | 'global' | 'user') {
		const params = source === 'project' ? '' : `?scope=${source}`;
		goto(`/environment/docs${params}`);
	}

	async function handleSave() {
		saving = true;
		error = null;
		success = null;

		try {
			await updateClaudeMD(content, selectedSource);
			success = 'CLAUDE.md saved successfully';

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

	const hasContent = $derived((source: string) => {
		if (!hierarchy) return false;
		switch (source) {
			case 'global': return !!hierarchy.global?.content;
			case 'user': return !!hierarchy.user?.content;
			case 'project': return !!hierarchy.project?.content;
			default: return false;
		}
	});
</script>

<svelte:head>
	<title>{selectedSource === 'project' ? '' : selectedSource.charAt(0).toUpperCase() + selectedSource.slice(1) + ' '}CLAUDE.md - orc</title>
</svelte:head>

<div class="claudemd-page">
	<header class="page-header">
		<div class="header-content">
			<div>
				<h1>{selectedSource === 'project' ? '' : selectedSource.charAt(0).toUpperCase() + selectedSource.slice(1) + ' '}CLAUDE.md</h1>
				<p class="subtitle">
					{#if selectedSource === 'global'}
						Global instructions at ~/.claude/CLAUDE.md
					{:else if selectedSource === 'user'}
						User instructions at ~/CLAUDE.md
					{:else}
						Project instructions for Claude
					{/if}
				</p>
			</div>
			<button class="btn btn-primary" onclick={handleSave} disabled={saving}>
				{saving ? 'Saving...' : 'Save'}
			</button>
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
					<li>
						<button
							class="source-item"
							class:selected={selectedSource === 'global'}
							onclick={() => selectSource('global')}
						>
							<span class="source-name">Global</span>
							<span class="source-path">~/.claude/CLAUDE.md</span>
							{#if !hierarchy?.global?.content}
								<span class="badge badge-new">New</span>
							{/if}
						</button>
					</li>
					<li>
						<button
							class="source-item"
							class:selected={selectedSource === 'user'}
							onclick={() => selectSource('user')}
						>
							<span class="source-name">User</span>
							<span class="source-path">~/CLAUDE.md</span>
							{#if !hierarchy?.user?.content}
								<span class="badge badge-new">New</span>
							{/if}
						</button>
					</li>
					<li>
						<button
							class="source-item"
							class:selected={selectedSource === 'project'}
							onclick={() => selectSource('project')}
						>
							<span class="source-name">Project</span>
							<span class="source-path">./CLAUDE.md</span>
							{#if !hierarchy?.project?.content}
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
				</div>

				<div class="editor-content">
					<textarea
						bind:value={content}
						placeholder="# Instructions\n\nAdd instructions for Claude here..."
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
