<script lang="ts">
	import { onMount } from 'svelte';
	import {
		listPrompts,
		getPrompt,
		getPromptDefault,
		getPromptVariables,
		savePrompt,
		deletePrompt,
		type PromptInfo,
		type Prompt
	} from '$lib/api';

	let prompts: PromptInfo[] = [];
	let variables: Record<string, string> = {};
	let selectedPhase: string | null = null;
	let currentPrompt: Prompt | null = null;
	let defaultPrompt: Prompt | null = null;
	let editContent = '';
	let loading = true;
	let saving = false;
	let error: string | null = null;
	let success: string | null = null;

	onMount(async () => {
		try {
			[prompts, variables] = await Promise.all([listPrompts(), getPromptVariables()]);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load prompts';
		} finally {
			loading = false;
		}
	});

	async function selectPrompt(phase: string) {
		error = null;
		success = null;
		try {
			selectedPhase = phase;
			[currentPrompt, defaultPrompt] = await Promise.all([
				getPrompt(phase),
				getPromptDefault(phase).catch(() => null)
			]);
			editContent = currentPrompt.content;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load prompt';
		}
	}

	async function handleSave() {
		if (!selectedPhase || !editContent.trim()) return;

		saving = true;
		error = null;
		success = null;

		try {
			currentPrompt = await savePrompt(selectedPhase, editContent);
			// Refresh list to update override status
			prompts = await listPrompts();
			success = 'Prompt saved successfully';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save prompt';
		} finally {
			saving = false;
		}
	}

	async function handleReset() {
		if (!selectedPhase || !defaultPrompt) return;

		saving = true;
		error = null;
		success = null;

		try {
			await deletePrompt(selectedPhase);
			// Reload prompt (will now be default)
			currentPrompt = await getPrompt(selectedPhase);
			editContent = currentPrompt.content;
			// Refresh list to update override status
			prompts = await listPrompts();
			success = 'Reset to default successfully';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to reset prompt';
		} finally {
			saving = false;
		}
	}

	function hasChanges(): boolean {
		return currentPrompt !== null && editContent !== currentPrompt.content;
	}

	function getSourceBadge(prompt: PromptInfo): { text: string; class: string } {
		if (prompt.has_override) {
			return { text: 'override', class: 'badge-override' };
		}
		return { text: prompt.source, class: 'badge-default' };
	}
</script>

<svelte:head>
	<title>Prompts - orc</title>
</svelte:head>

<div class="prompts-page">
	<header class="page-header">
		<h1>Prompt Templates</h1>
		<p class="subtitle">Manage phase prompts with project-level overrides</p>
	</header>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if success}
		<div class="alert alert-success">{success}</div>
	{/if}

	{#if loading}
		<div class="loading">Loading prompts...</div>
	{:else}
		<div class="prompts-layout">
			<!-- Prompt List -->
			<aside class="prompt-list">
				<h2>Phases</h2>
				<ul>
					{#each prompts as prompt}
						<li>
							<button
								class="prompt-item"
								class:selected={selectedPhase === prompt.phase}
								on:click={() => selectPrompt(prompt.phase)}
							>
								<span class="phase-name">{prompt.phase}</span>
								<span class="badge {getSourceBadge(prompt).class}">
									{getSourceBadge(prompt).text}
								</span>
							</button>
						</li>
					{/each}
				</ul>
			</aside>

			<!-- Editor Panel -->
			<div class="editor-panel">
				{#if selectedPhase && currentPrompt}
					<div class="editor-header">
						<h2>{selectedPhase}</h2>
						<div class="editor-actions">
							{#if currentPrompt.source === 'project' || hasChanges()}
								<button
									class="btn btn-secondary"
									on:click={handleReset}
									disabled={saving || !defaultPrompt}
								>
									Reset to Default
								</button>
							{/if}
							<button
								class="btn btn-primary"
								on:click={handleSave}
								disabled={saving || !hasChanges()}
							>
								{saving ? 'Saving...' : 'Save Override'}
							</button>
						</div>
					</div>

					<div class="editor-content">
						<textarea
							class="editor-textarea"
							bind:value={editContent}
							placeholder="Enter prompt template..."
							spellcheck="false"
						></textarea>
					</div>

					<div class="editor-footer">
						<div class="source-info">
							Source: <span class="source-value">{currentPrompt.source}</span>
							{#if currentPrompt.variables.length > 0}
								<span class="separator">|</span>
								Variables: {currentPrompt.variables.join(', ')}
							{/if}
						</div>
					</div>
				{:else}
					<div class="no-selection">
						<p>Select a phase from the list to view and edit its prompt template.</p>
					</div>
				{/if}
			</div>

			<!-- Variables Reference -->
			<aside class="variables-panel">
				<h2>Variable Reference</h2>
				<div class="variables-list">
					{#each Object.entries(variables) as [name, description]}
						<div class="variable-item">
							<code class="variable-name">{name}</code>
							<span class="variable-desc">{description}</span>
						</div>
					{/each}
				</div>
			</aside>
		</div>
	{/if}
</div>

<style>
	.prompts-page {
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
	}

	.page-header h1 {
		margin: 0;
		font-size: 1.5rem;
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

	.prompts-layout {
		display: grid;
		grid-template-columns: 200px 1fr 280px;
		gap: 1.5rem;
		min-height: 600px;
	}

	/* Prompt List */
	.prompt-list {
		background: var(--bg-secondary);
		border-radius: 8px;
		padding: 1rem;
		border: 1px solid var(--border-color);
	}

	.prompt-list h2 {
		font-size: 0.875rem;
		font-weight: 600;
		margin: 0 0 0.75rem;
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.prompt-list ul {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.prompt-item {
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

	.prompt-item:hover {
		background: var(--bg-tertiary, rgba(0, 0, 0, 0.05));
	}

	.prompt-item.selected {
		background: var(--primary-bg, #dbeafe);
		color: var(--primary-text, #1d4ed8);
	}

	.phase-name {
		font-weight: 500;
	}

	.badge {
		font-size: 0.625rem;
		padding: 0.125rem 0.375rem;
		border-radius: 4px;
		text-transform: uppercase;
		font-weight: 600;
		letter-spacing: 0.025em;
	}

	.badge-default {
		background: var(--bg-tertiary, #e5e7eb);
		color: var(--text-secondary);
	}

	.badge-override {
		background: var(--warning-bg, #fef3c7);
		color: var(--warning-text, #d97706);
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

	.editor-actions {
		display: flex;
		gap: 0.5rem;
	}

	.btn {
		padding: 0.5rem 1rem;
		border-radius: 6px;
		font-size: 0.875rem;
		font-weight: 500;
		cursor: pointer;
		border: 1px solid transparent;
		transition:
			background 0.15s,
			opacity 0.15s;
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
		background: transparent;
		border-color: var(--border-color);
		color: var(--text-primary);
	}

	.btn-secondary:hover:not(:disabled) {
		background: var(--bg-tertiary, rgba(0, 0, 0, 0.05));
	}

	.editor-content {
		flex: 1;
		display: flex;
	}

	.editor-textarea {
		width: 100%;
		height: 100%;
		min-height: 400px;
		padding: 1rem;
		border: none;
		resize: none;
		font-family: 'JetBrains Mono', 'Fira Code', monospace;
		font-size: 0.875rem;
		line-height: 1.6;
		background: var(--bg-primary);
		color: var(--text-primary);
	}

	.editor-textarea:focus {
		outline: none;
	}

	.editor-footer {
		padding: 0.75rem 1rem;
		border-top: 1px solid var(--border-color);
		background: var(--bg-tertiary, rgba(0, 0, 0, 0.02));
	}

	.source-info {
		font-size: 0.75rem;
		color: var(--text-secondary);
	}

	.source-value {
		font-weight: 500;
		color: var(--text-primary);
	}

	.separator {
		margin: 0 0.5rem;
		opacity: 0.5;
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

	/* Variables Panel */
	.variables-panel {
		background: var(--bg-secondary);
		border-radius: 8px;
		padding: 1rem;
		border: 1px solid var(--border-color);
		overflow-y: auto;
	}

	.variables-panel h2 {
		font-size: 0.875rem;
		font-weight: 600;
		margin: 0 0 0.75rem;
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.variables-list {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	.variable-item {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.variable-name {
		font-size: 0.75rem;
		padding: 0.25rem 0.5rem;
		background: var(--bg-tertiary, rgba(0, 0, 0, 0.05));
		border-radius: 4px;
		font-family: 'JetBrains Mono', 'Fira Code', monospace;
		color: var(--primary, #3b82f6);
	}

	.variable-desc {
		font-size: 0.75rem;
		color: var(--text-secondary);
		line-height: 1.4;
	}

	/* Responsive */
	@media (max-width: 1024px) {
		.prompts-layout {
			grid-template-columns: 180px 1fr;
		}

		.variables-panel {
			display: none;
		}
	}

	@media (max-width: 768px) {
		.prompts-layout {
			grid-template-columns: 1fr;
		}

		.prompt-list {
			max-height: 200px;
			overflow-y: auto;
		}
	}
</style>
