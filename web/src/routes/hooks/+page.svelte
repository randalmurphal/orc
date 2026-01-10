<script lang="ts">
	import { onMount } from 'svelte';
	import {
		listHooks,
		getHook,
		getHookTypes,
		createHook,
		updateHook,
		deleteHook,
		type HookInfo,
		type Hook,
		type HookType
	} from '$lib/api';

	let hooks: HookInfo[] = [];
	let hookTypes: HookType[] = [];
	let selectedHook: Hook | null = null;
	let isCreating = false;
	let loading = true;
	let saving = false;
	let error: string | null = null;
	let success: string | null = null;

	// Form fields
	let formName = '';
	let formType: HookType = 'pre:tool';
	let formPattern = '';
	let formCommand = '';
	let formTimeout = 30;
	let formDisabled = false;

	onMount(async () => {
		try {
			[hooks, hookTypes] = await Promise.all([listHooks(), getHookTypes()]);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load hooks';
		} finally {
			loading = false;
		}
	});

	async function selectHook(name: string) {
		error = null;
		success = null;
		isCreating = false;

		try {
			selectedHook = await getHook(name);
			formName = selectedHook.name;
			formType = selectedHook.type;
			formPattern = selectedHook.pattern || '';
			formCommand = selectedHook.command;
			formTimeout = selectedHook.timeout || 30;
			formDisabled = selectedHook.disabled || false;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load hook';
		}
	}

	function startCreate() {
		error = null;
		success = null;
		selectedHook = null;
		isCreating = true;

		formName = '';
		formType = 'pre:tool';
		formPattern = '';
		formCommand = '';
		formTimeout = 30;
		formDisabled = false;
	}

	async function handleSave() {
		if (!formName.trim() || !formCommand.trim()) {
			error = 'Name and command are required';
			return;
		}

		saving = true;
		error = null;
		success = null;

		const hook: Hook = {
			name: formName.trim(),
			type: formType,
			pattern: formPattern.trim() || undefined,
			command: formCommand.trim(),
			timeout: formTimeout > 0 ? formTimeout : undefined,
			disabled: formDisabled
		};

		try {
			if (isCreating) {
				await createHook(hook);
				success = 'Hook created successfully';
			} else if (selectedHook) {
				await updateHook(selectedHook.name, hook);
				success = 'Hook updated successfully';
			}

			hooks = await listHooks();
			selectedHook = hook;
			isCreating = false;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save hook';
		} finally {
			saving = false;
		}
	}

	async function handleDelete() {
		if (!selectedHook) return;

		if (!confirm(`Delete hook "${selectedHook.name}"?`)) return;

		saving = true;
		error = null;
		success = null;

		try {
			await deleteHook(selectedHook.name);
			hooks = await listHooks();
			selectedHook = null;
			isCreating = false;
			success = 'Hook deleted successfully';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete hook';
		} finally {
			saving = false;
		}
	}

	function getTypeBadgeClass(type: HookType): string {
		if (type.startsWith('pre:')) return 'badge-pre';
		if (type.startsWith('post:')) return 'badge-post';
		return 'badge-other';
	}
</script>

<svelte:head>
	<title>Hooks - orc</title>
</svelte:head>

<div class="hooks-page">
	<header class="page-header">
		<div class="header-content">
			<div>
				<h1>Claude Code Hooks</h1>
				<p class="subtitle">Manage hooks in .claude/hooks/</p>
			</div>
			<button class="btn btn-primary" on:click={startCreate}>New Hook</button>
		</div>
	</header>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if success}
		<div class="alert alert-success">{success}</div>
	{/if}

	{#if loading}
		<div class="loading">Loading hooks...</div>
	{:else}
		<div class="hooks-layout">
			<!-- Hook List -->
			<aside class="hook-list">
				<h2>Hooks</h2>
				{#if hooks.length === 0}
					<p class="empty-message">No hooks configured</p>
				{:else}
					<ul>
						{#each hooks as hook}
							<li>
								<button
									class="hook-item"
									class:selected={selectedHook?.name === hook.name}
									class:disabled={hook.disabled}
									on:click={() => selectHook(hook.name)}
								>
									<span class="hook-name">{hook.name}</span>
									<span class="badge {getTypeBadgeClass(hook.type)}">{hook.type}</span>
								</button>
							</li>
						{/each}
					</ul>
				{/if}
			</aside>

			<!-- Editor Panel -->
			<div class="editor-panel">
				{#if selectedHook || isCreating}
					<div class="editor-header">
						<h2>{isCreating ? 'New Hook' : selectedHook?.name}</h2>
						{#if selectedHook && !isCreating}
							<button class="btn btn-danger" on:click={handleDelete} disabled={saving}>
								Delete
							</button>
						{/if}
					</div>

					<form class="hook-form" on:submit|preventDefault={handleSave}>
						<div class="form-group">
							<label for="name">Name</label>
							<input
								id="name"
								type="text"
								bind:value={formName}
								placeholder="my-hook"
								disabled={!isCreating}
							/>
						</div>

						<div class="form-row">
							<div class="form-group">
								<label for="type">Type</label>
								<select id="type" bind:value={formType}>
									{#each hookTypes as type}
										<option value={type}>{type}</option>
									{/each}
								</select>
							</div>

							<div class="form-group">
								<label for="pattern">Pattern (optional)</label>
								<input
									id="pattern"
									type="text"
									bind:value={formPattern}
									placeholder="Bash, Edit, etc."
								/>
							</div>
						</div>

						<div class="form-group">
							<label for="command">Command</label>
							<textarea
								id="command"
								bind:value={formCommand}
								placeholder="echo 'Hook executed'"
								rows="3"
							></textarea>
						</div>

						<div class="form-row">
							<div class="form-group">
								<label for="timeout">Timeout (seconds)</label>
								<input id="timeout" type="number" bind:value={formTimeout} min="0" max="300" />
							</div>

							<div class="form-group checkbox-group">
								<label>
									<input type="checkbox" bind:checked={formDisabled} />
									Disabled
								</label>
							</div>
						</div>

						<div class="form-actions">
							<button type="submit" class="btn btn-primary" disabled={saving}>
								{saving ? 'Saving...' : isCreating ? 'Create' : 'Update'}
							</button>
						</div>
					</form>
				{:else}
					<div class="no-selection">
						<p>Select a hook from the list or create a new one.</p>
					</div>
				{/if}
			</div>
		</div>
	{/if}
</div>

<style>
	.hooks-page {
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

	.hooks-layout {
		display: grid;
		grid-template-columns: 250px 1fr;
		gap: 1.5rem;
		min-height: 500px;
	}

	/* Hook List */
	.hook-list {
		background: var(--bg-secondary);
		border-radius: 8px;
		padding: 1rem;
		border: 1px solid var(--border-color);
	}

	.hook-list h2 {
		font-size: 0.875rem;
		font-weight: 600;
		margin: 0 0 0.75rem;
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.hook-list ul {
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
	}

	.hook-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		width: 100%;
		padding: 0.5rem 0.75rem;
		background: transparent;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		text-align: left;
		color: var(--text-primary);
		font-size: 0.875rem;
	}

	.hook-item:hover {
		background: var(--bg-tertiary, rgba(0, 0, 0, 0.05));
	}

	.hook-item.selected {
		background: var(--primary-bg, #dbeafe);
		color: var(--primary-text, #1d4ed8);
	}

	.hook-item.disabled {
		opacity: 0.5;
	}

	.hook-name {
		font-weight: 500;
	}

	.badge {
		font-size: 0.625rem;
		padding: 0.125rem 0.375rem;
		border-radius: 4px;
		text-transform: uppercase;
		font-weight: 600;
	}

	.badge-pre {
		background: var(--info-bg, #dbeafe);
		color: var(--info-text, #1d4ed8);
	}

	.badge-post {
		background: var(--success-bg, #dcfce7);
		color: var(--success-text, #16a34a);
	}

	.badge-other {
		background: var(--bg-tertiary, #e5e7eb);
		color: var(--text-secondary);
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

	.hook-form {
		padding: 1.5rem;
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.form-row {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 1rem;
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
		font-family: 'JetBrains Mono', 'Fira Code', monospace;
		resize: vertical;
	}

	.form-group input:focus,
	.form-group select:focus,
	.form-group textarea:focus {
		outline: none;
		border-color: var(--primary, #3b82f6);
	}

	.checkbox-group {
		justify-content: flex-end;
	}

	.checkbox-group label {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		cursor: pointer;
	}

	.checkbox-group input[type='checkbox'] {
		width: auto;
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

	.btn-danger {
		background: var(--error-text, #dc2626);
		color: white;
	}

	.btn-danger:hover:not(:disabled) {
		background: #b91c1c;
	}

	.no-selection {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 100%;
		padding: 3rem;
		text-align: center;
		color: var(--text-secondary);
	}

	@media (max-width: 768px) {
		.hooks-layout {
			grid-template-columns: 1fr;
		}

		.hook-list {
			max-height: 200px;
			overflow-y: auto;
		}

		.form-row {
			grid-template-columns: 1fr;
		}
	}
</style>
