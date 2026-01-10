<script lang="ts">
	import { onMount } from 'svelte';
	import {
		listScripts,
		getScript,
		createScript,
		updateScript,
		deleteScript,
		discoverScripts,
		type ProjectScript
	} from '$lib/api';

	let scripts: ProjectScript[] = [];
	let selectedScript: ProjectScript | null = null;
	let isCreating = false;
	let loading = true;
	let saving = false;
	let discovering = false;
	let error: string | null = null;
	let success: string | null = null;

	// Form fields
	let formName = '';
	let formPath = '';
	let formDescription = '';
	let formLanguage = '';

	onMount(async () => {
		try {
			scripts = await listScripts();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load scripts';
		} finally {
			loading = false;
		}
	});

	async function selectScriptByName(name: string) {
		error = null;
		success = null;
		isCreating = false;

		try {
			selectedScript = await getScript(name);
			formName = selectedScript.name;
			formPath = selectedScript.path;
			formDescription = selectedScript.description;
			formLanguage = selectedScript.language || '';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load script';
		}
	}

	function startCreate() {
		error = null;
		success = null;
		selectedScript = null;
		isCreating = true;

		formName = '';
		formPath = '';
		formDescription = '';
		formLanguage = '';
	}

	async function handleDiscover() {
		discovering = true;
		error = null;
		success = null;

		try {
			const discovered = await discoverScripts();
			scripts = await listScripts();
			if (discovered.length > 0) {
				success = `Discovered ${discovered.length} script(s)`;
			} else {
				success = 'No new scripts found in .claude/scripts/';
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to discover scripts';
		} finally {
			discovering = false;
		}
	}

	async function handleSave() {
		if (!formName.trim() || !formPath.trim()) {
			error = 'Name and path are required';
			return;
		}

		saving = true;
		error = null;
		success = null;

		const script: ProjectScript = {
			name: formName.trim(),
			path: formPath.trim(),
			description: formDescription.trim()
		};

		if (formLanguage) script.language = formLanguage;

		try {
			if (isCreating) {
				await createScript(script);
				success = 'Script registered successfully';
			} else if (selectedScript) {
				await updateScript(selectedScript.name, script);
				success = 'Script updated successfully';
			}

			scripts = await listScripts();
			selectedScript = script;
			isCreating = false;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save script';
		} finally {
			saving = false;
		}
	}

	async function handleDelete() {
		if (!selectedScript) return;

		if (!confirm(`Remove script "${selectedScript.name}" from registry?`)) return;

		saving = true;
		error = null;
		success = null;

		try {
			await deleteScript(selectedScript.name);
			scripts = await listScripts();
			selectedScript = null;
			isCreating = false;
			success = 'Script removed from registry';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete script';
		} finally {
			saving = false;
		}
	}

	function getLanguageIcon(lang?: string): string {
		switch (lang?.toLowerCase()) {
			case 'python':
			case 'py':
				return 'py';
			case 'javascript':
			case 'js':
				return 'js';
			case 'typescript':
			case 'ts':
				return 'ts';
			case 'bash':
			case 'sh':
				return 'sh';
			case 'go':
				return 'go';
			default:
				return '';
		}
	}
</script>

<svelte:head>
	<title>Scripts - orc</title>
</svelte:head>

<div class="scripts-page">
	<header class="page-header">
		<div class="header-content">
			<div>
				<h1>Project Scripts</h1>
				<p class="subtitle">Register scripts for agent use</p>
			</div>
			<div class="header-actions">
				<button
					class="btn btn-secondary"
					on:click={handleDiscover}
					disabled={discovering}
				>
					{discovering ? 'Discovering...' : 'Discover'}
				</button>
				<button class="btn btn-primary" on:click={startCreate}>New Script</button>
			</div>
		</div>
	</header>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if success}
		<div class="alert alert-success">{success}</div>
	{/if}

	{#if loading}
		<div class="loading">Loading scripts...</div>
	{:else}
		<div class="scripts-layout">
			<!-- Script List -->
			<aside class="script-list">
				<h2>Scripts</h2>
				{#if scripts.length === 0}
					<p class="empty-message">No scripts registered</p>
					<p class="empty-hint">
						Click "Discover" to find scripts in .claude/scripts/
					</p>
				{:else}
					<ul>
						{#each scripts as script}
							<li>
								<button
									class="script-item"
									class:selected={selectedScript?.name === script.name}
									on:click={() => selectScriptByName(script.name)}
								>
									<div class="script-header">
										<span class="script-name">{script.name}</span>
										{#if script.language}
											<span class="lang-badge">{getLanguageIcon(script.language) || script.language}</span>
										{/if}
									</div>
									{#if script.description}
										<span class="script-desc">{script.description}</span>
									{/if}
								</button>
							</li>
						{/each}
					</ul>
				{/if}
			</aside>

			<!-- Editor Panel -->
			<div class="editor-panel">
				{#if selectedScript || isCreating}
					<div class="editor-header">
						<h2>{isCreating ? 'New Script' : selectedScript?.name}</h2>
						{#if selectedScript && !isCreating}
							<button class="btn btn-danger" on:click={handleDelete} disabled={saving}>
								Remove
							</button>
						{/if}
					</div>

					<form class="script-form" on:submit|preventDefault={handleSave}>
						<div class="form-row">
							<div class="form-group">
								<label for="name">Name</label>
								<input
									id="name"
									type="text"
									bind:value={formName}
									placeholder="my-script"
									disabled={!isCreating}
								/>
							</div>

							<div class="form-group">
								<label for="language">Language (optional)</label>
								<select id="language" bind:value={formLanguage}>
									<option value="">Auto-detect</option>
									<option value="python">Python</option>
									<option value="bash">Bash</option>
									<option value="javascript">JavaScript</option>
									<option value="typescript">TypeScript</option>
									<option value="go">Go</option>
								</select>
							</div>
						</div>

						<div class="form-group">
							<label for="path">Path</label>
							<input
								id="path"
								type="text"
								bind:value={formPath}
								placeholder=".claude/scripts/my-script.py"
							/>
							<span class="form-hint">Relative path from project root</span>
						</div>

						<div class="form-group">
							<label for="description">Description</label>
							<textarea
								id="description"
								bind:value={formDescription}
								placeholder="What this script does and when to use it"
								rows="4"
							></textarea>
						</div>

						<div class="form-actions">
							<button type="submit" class="btn btn-primary" disabled={saving}>
								{saving ? 'Saving...' : isCreating ? 'Register' : 'Update'}
							</button>
						</div>
					</form>
				{:else}
					<div class="no-selection">
						<p>Select a script from the list or register a new one.</p>
						<p class="hint">
							Scripts can be invoked by agents during task execution
						</p>
					</div>
				{/if}
			</div>
		</div>
	{/if}
</div>

<style>
	.scripts-page {
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

	.header-actions {
		display: flex;
		gap: 0.5rem;
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

	.scripts-layout {
		display: grid;
		grid-template-columns: 250px 1fr;
		gap: 1.5rem;
		min-height: 500px;
	}

	/* Script List */
	.script-list {
		background: var(--bg-secondary);
		border-radius: 8px;
		padding: 1rem;
		border: 1px solid var(--border-color);
	}

	.script-list h2 {
		font-size: 0.875rem;
		font-weight: 600;
		margin: 0 0 0.75rem;
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.script-list ul {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.empty-message {
		color: var(--text-secondary);
		font-size: 0.875rem;
		font-style: italic;
		margin: 0;
	}

	.empty-hint {
		color: var(--text-secondary);
		font-size: 0.75rem;
		margin: 0.5rem 0 0;
	}

	.script-item {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		width: 100%;
		padding: 0.5rem 0.75rem;
		background: transparent;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		text-align: left;
		color: var(--text-primary);
		font-size: 0.875rem;
		gap: 0.25rem;
	}

	.script-item:hover {
		background: var(--bg-tertiary, rgba(0, 0, 0, 0.05));
	}

	.script-item.selected {
		background: var(--primary-bg, #dbeafe);
		color: var(--primary-text, #1d4ed8);
	}

	.script-header {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		width: 100%;
	}

	.script-name {
		font-weight: 500;
	}

	.lang-badge {
		font-size: 0.625rem;
		padding: 0.125rem 0.375rem;
		border-radius: 4px;
		background: var(--bg-tertiary, #e5e7eb);
		color: var(--text-secondary);
		text-transform: uppercase;
		font-weight: 600;
	}

	.script-desc {
		font-size: 0.75rem;
		color: var(--text-secondary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		max-width: 100%;
	}

	.script-item.selected .script-desc {
		color: var(--primary-text, #1d4ed8);
		opacity: 0.7;
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

	.script-form {
		padding: 1.5rem;
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.form-row {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 1rem;
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.form-group label {
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--text-primary);
	}

	.form-group input,
	.form-group select,
	.form-group textarea {
		padding: 0.5rem 0.75rem;
		border: 1px solid var(--border-color);
		border-radius: 6px;
		font-size: 0.875rem;
		background: var(--bg-primary);
		color: var(--text-primary);
	}

	.form-group textarea {
		resize: vertical;
		min-height: 100px;
	}

	.form-group input:focus,
	.form-group select:focus,
	.form-group textarea:focus {
		outline: none;
		border-color: var(--primary, #3b82f6);
	}

	.form-hint {
		font-size: 0.75rem;
		color: var(--text-secondary);
	}

	.form-actions {
		padding-top: 0.5rem;
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

	.btn-secondary {
		background: var(--bg-secondary);
		color: var(--text-primary);
		border-color: var(--border-color);
	}

	.btn-secondary:hover:not(:disabled) {
		background: var(--bg-tertiary, #e5e7eb);
	}

	.btn-danger {
		background: var(--error-text, #dc2626);
		color: white;
	}

	.btn-danger:hover:not(:disabled) {
		background: #b91c1c;
	}

	.no-selection {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100%;
		padding: 3rem;
		text-align: center;
		color: var(--text-secondary);
		gap: 1rem;
	}

	.no-selection .hint {
		font-size: 0.875rem;
	}

	@media (max-width: 768px) {
		.header-content {
			flex-direction: column;
			gap: 1rem;
		}

		.scripts-layout {
			grid-template-columns: 1fr;
		}

		.script-list {
			max-height: 200px;
			overflow-y: auto;
		}

		.form-row {
			grid-template-columns: 1fr;
		}
	}
</style>
