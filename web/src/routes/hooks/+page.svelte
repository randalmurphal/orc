<script lang="ts">
	import { onMount } from 'svelte';
	import {
		listHooks,
		getHookTypes,
		createHook,
		deleteHook,
		type HooksMap,
		type HookEvent,
		type Hook,
		type HookEntry
	} from '$lib/api';

	let hooksMap: HooksMap = {};
	let hookEvents: HookEvent[] = [];
	let selectedEvent: HookEvent | null = null;
	let selectedHookIndex: number | null = null;
	let isCreating = false;
	let loading = true;
	let saving = false;
	let error: string | null = null;
	let success: string | null = null;

	// Form fields for a single hook
	let formMatcher = '';
	let formCommand = '';
	let formEvent: HookEvent = 'PreToolUse';

	onMount(async () => {
		try {
			[hooksMap, hookEvents] = await Promise.all([listHooks(), getHookTypes()]);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load hooks';
		} finally {
			loading = false;
		}
	});

	function selectHook(event: HookEvent, index: number) {
		error = null;
		success = null;
		isCreating = false;
		selectedEvent = event;
		selectedHookIndex = index;

		const hook = hooksMap[event]?.[index];
		if (hook) {
			formMatcher = hook.matcher;
			formCommand = hook.hooks[0]?.command || '';
			formEvent = event;
		}
	}

	function startCreate() {
		error = null;
		success = null;
		selectedEvent = null;
		selectedHookIndex = null;
		isCreating = true;

		formMatcher = '';
		formCommand = '';
		formEvent = hookEvents[0] || 'PreToolUse';
	}

	async function handleSave() {
		if (!formMatcher.trim() || !formCommand.trim()) {
			error = 'Matcher and command are required';
			return;
		}

		saving = true;
		error = null;
		success = null;

		const hook: Hook = {
			matcher: formMatcher.trim(),
			hooks: [{ type: 'command', command: formCommand.trim() }]
		};

		try {
			await createHook(formEvent, hook);
			success = 'Hook saved successfully';
			hooksMap = await listHooks();

			if (isCreating) {
				isCreating = false;
				selectedEvent = formEvent;
				selectedHookIndex = (hooksMap[formEvent]?.length ?? 1) - 1;
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save hook';
		} finally {
			saving = false;
		}
	}

	async function handleDelete() {
		if (!selectedEvent) return;

		if (!confirm(`Delete hooks for "${selectedEvent}"?`)) return;

		saving = true;
		error = null;
		success = null;

		try {
			await deleteHook(selectedEvent);
			hooksMap = await listHooks();
			selectedEvent = null;
			selectedHookIndex = null;
			isCreating = false;
			success = 'Hook deleted successfully';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete hook';
		} finally {
			saving = false;
		}
	}

	function getEventBadgeClass(event: string): string {
		if (event.startsWith('Pre')) return 'badge-pre';
		if (event.startsWith('Post')) return 'badge-post';
		return 'badge-other';
	}

	// Flatten hooks for display
	$: flatHooks = Object.entries(hooksMap).flatMap(([event, hooks]) =>
		hooks.map((hook, index) => ({ event: event as HookEvent, hook, index }))
	);
</script>

<svelte:head>
	<title>Hooks - orc</title>
</svelte:head>

<div class="hooks-page">
	<header class="page-header">
		<div class="header-content">
			<div>
				<h1>Claude Code Hooks</h1>
				<p class="subtitle">Manage hooks in .claude/settings.json</p>
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
				{#if flatHooks.length === 0}
					<p class="empty-message">No hooks configured</p>
				{:else}
					<ul>
						{#each flatHooks as { event, hook, index }}
							<li>
								<button
									class="hook-item"
									class:selected={selectedEvent === event && selectedHookIndex === index}
									on:click={() => selectHook(event, index)}
								>
									<span class="hook-name">{hook.matcher}</span>
									<span class="badge {getEventBadgeClass(event)}">{event}</span>
								</button>
							</li>
						{/each}
					</ul>
				{/if}
			</aside>

			<!-- Editor Panel -->
			<div class="editor-panel">
				{#if selectedEvent !== null || isCreating}
					<div class="editor-header">
						<h2>{isCreating ? 'New Hook' : formMatcher || 'Edit Hook'}</h2>
						{#if selectedEvent && !isCreating}
							<button class="btn btn-danger" on:click={handleDelete} disabled={saving}>
								Delete Event
							</button>
						{/if}
					</div>

					<form class="hook-form" on:submit|preventDefault={handleSave}>
						<div class="form-group">
							<label for="event">Event</label>
							<select id="event" bind:value={formEvent} disabled={!isCreating}>
								{#each hookEvents as event}
									<option value={event}>{event}</option>
								{/each}
							</select>
						</div>

						<div class="form-group">
							<label for="matcher">Matcher Pattern</label>
							<input
								id="matcher"
								type="text"
								bind:value={formMatcher}
								placeholder="Bash, Edit, *"
							/>
							<span class="help-text">Tool name pattern (use * for all tools)</span>
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

	.hook-name {
		font-weight: 500;
		font-family: 'JetBrains Mono', 'Fira Code', monospace;
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

	.help-text {
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
	}
</style>
