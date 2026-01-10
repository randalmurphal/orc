<script lang="ts">
	import { onMount } from 'svelte';
	import { getSettings, getProjectSettings, updateSettings, type Settings } from '$lib/api';

	let mergedSettings: Settings | null = null;
	let projectSettings: Settings | null = null;
	let loading = true;
	let saving = false;
	let error: string | null = null;
	let success: string | null = null;

	// Form state for project settings
	let envEntries: { key: string; value: string }[] = [];
	let statusLineType = '';
	let statusLineCommand = '';
	let newEnvKey = '';
	let newEnvValue = '';

	onMount(async () => {
		try {
			[mergedSettings, projectSettings] = await Promise.all([getSettings(), getProjectSettings()]);

			// Initialize form state from project settings
			if (projectSettings?.env) {
				envEntries = Object.entries(projectSettings.env).map(([key, value]) => ({ key, value }));
			}
			if (projectSettings?.statusLine) {
				statusLineType = projectSettings.statusLine.type || '';
				statusLineCommand = projectSettings.statusLine.command || '';
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load settings';
		} finally {
			loading = false;
		}
	});

	function addEnvVar() {
		if (!newEnvKey.trim()) return;
		envEntries = [...envEntries, { key: newEnvKey.trim(), value: newEnvValue }];
		newEnvKey = '';
		newEnvValue = '';
	}

	function removeEnvVar(index: number) {
		envEntries = envEntries.filter((_, i) => i !== index);
	}

	async function handleSave() {
		saving = true;
		error = null;
		success = null;

		const settings: Settings = {
			env: envEntries.reduce(
				(acc, { key, value }) => {
					if (key.trim()) acc[key.trim()] = value;
					return acc;
				},
				{} as Record<string, string>
			)
		};

		if (statusLineType || statusLineCommand) {
			settings.statusLine = {};
			if (statusLineType) settings.statusLine.type = statusLineType;
			if (statusLineCommand) settings.statusLine.command = statusLineCommand;
		}

		try {
			await updateSettings(settings);
			projectSettings = settings;
			success = 'Settings saved successfully';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save settings';
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head>
	<title>Settings - orc</title>
</svelte:head>

<div class="settings-page">
	<header class="page-header">
		<div class="header-content">
			<div>
				<h1>Project Settings</h1>
				<p class="subtitle">Configure .claude/settings.json for this project</p>
			</div>
			<button class="btn btn-primary" on:click={handleSave} disabled={saving}>
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
		<div class="loading">Loading settings...</div>
	{:else}
		<div class="settings-grid">
			<!-- Environment Variables -->
			<section class="settings-section">
				<h2>Environment Variables</h2>
				<p class="section-help">Variables passed to Claude Code during execution</p>

				<div class="env-list">
					{#each envEntries as entry, index}
						<div class="env-row">
							<input
								type="text"
								bind:value={entry.key}
								placeholder="KEY"
								class="env-key"
							/>
							<span class="env-equals">=</span>
							<input
								type="text"
								bind:value={entry.value}
								placeholder="value"
								class="env-value"
							/>
							<button
								class="btn-icon btn-remove"
								on:click={() => removeEnvVar(index)}
								title="Remove"
							>
								&times;
							</button>
						</div>
					{/each}

					<div class="env-row env-new">
						<input
							type="text"
							bind:value={newEnvKey}
							placeholder="NEW_KEY"
							class="env-key"
							on:keydown={(e) => e.key === 'Enter' && addEnvVar()}
						/>
						<span class="env-equals">=</span>
						<input
							type="text"
							bind:value={newEnvValue}
							placeholder="value"
							class="env-value"
							on:keydown={(e) => e.key === 'Enter' && addEnvVar()}
						/>
						<button class="btn-icon btn-add" on:click={addEnvVar} title="Add">+</button>
					</div>
				</div>
			</section>

			<!-- Status Line -->
			<section class="settings-section">
				<h2>Status Line</h2>
				<p class="section-help">Custom status line shown in Claude Code</p>

				<div class="form-group">
					<label for="status-type">Type</label>
					<select id="status-type" bind:value={statusLineType}>
						<option value="">Default</option>
						<option value="text">Text</option>
						<option value="command">Command</option>
					</select>
				</div>

				{#if statusLineType === 'command'}
					<div class="form-group">
						<label for="status-command">Command</label>
						<input
							id="status-command"
							type="text"
							bind:value={statusLineCommand}
							placeholder="echo 'Status: ready'"
						/>
						<span class="form-hint">Shell command to generate status text</span>
					</div>
				{/if}
			</section>

			<!-- Merged Settings Preview (Read-only) -->
			<section class="settings-section preview-section">
				<h2>Effective Settings</h2>
				<p class="section-help">Merged global + project settings (read-only)</p>

				<div class="preview-content">
					<pre>{JSON.stringify(mergedSettings, null, 2)}</pre>
				</div>
			</section>
		</div>
	{/if}
</div>

<style>
	.settings-page {
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

	.settings-grid {
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
	}

	.settings-section {
		background: var(--bg-secondary);
		border-radius: 8px;
		padding: 1.5rem;
		border: 1px solid var(--border-color);
	}

	.settings-section h2 {
		font-size: 1rem;
		font-weight: 600;
		margin: 0 0 0.25rem;
		color: var(--text-primary);
	}

	.section-help {
		font-size: 0.75rem;
		color: var(--text-secondary);
		margin: 0 0 1rem;
	}

	/* Environment Variables */
	.env-list {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.env-row {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.env-key {
		flex: 0 0 200px;
		padding: 0.5rem 0.75rem;
		border: 1px solid var(--border-color);
		border-radius: 6px;
		font-size: 0.875rem;
		font-family: 'JetBrains Mono', 'Fira Code', monospace;
		background: var(--bg-primary);
		color: var(--text-primary);
	}

	.env-equals {
		color: var(--text-secondary);
		font-family: 'JetBrains Mono', 'Fira Code', monospace;
	}

	.env-value {
		flex: 1;
		padding: 0.5rem 0.75rem;
		border: 1px solid var(--border-color);
		border-radius: 6px;
		font-size: 0.875rem;
		background: var(--bg-primary);
		color: var(--text-primary);
	}

	.env-new {
		opacity: 0.7;
	}

	.env-new:focus-within {
		opacity: 1;
	}

	.btn-icon {
		width: 32px;
		height: 32px;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		font-size: 1.25rem;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.btn-remove {
		background: var(--error-bg, #fee2e2);
		color: var(--error-text, #dc2626);
	}

	.btn-remove:hover {
		background: var(--error-border, #fecaca);
	}

	.btn-add {
		background: var(--success-bg, #dcfce7);
		color: var(--success-text, #16a34a);
	}

	.btn-add:hover {
		background: var(--success-border, #bbf7d0);
	}

	/* Form Groups */
	.form-group {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		margin-bottom: 1rem;
	}

	.form-group:last-child {
		margin-bottom: 0;
	}

	.form-group label {
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--text-primary);
	}

	.form-group input,
	.form-group select {
		padding: 0.5rem 0.75rem;
		border: 1px solid var(--border-color);
		border-radius: 6px;
		font-size: 0.875rem;
		background: var(--bg-primary);
		color: var(--text-primary);
	}

	.form-group input:focus,
	.form-group select:focus {
		outline: none;
		border-color: var(--primary, #3b82f6);
	}

	.form-hint {
		font-size: 0.75rem;
		color: var(--text-secondary);
	}

	/* Preview Section */
	.preview-section {
		background: var(--bg-tertiary, #f3f4f6);
	}

	.preview-content {
		background: var(--bg-primary);
		border: 1px solid var(--border-color);
		border-radius: 6px;
		padding: 1rem;
		overflow-x: auto;
	}

	.preview-content pre {
		margin: 0;
		font-size: 0.75rem;
		font-family: 'JetBrains Mono', 'Fira Code', monospace;
		color: var(--text-primary);
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
		.header-content {
			flex-direction: column;
			gap: 1rem;
		}

		.env-row {
			flex-wrap: wrap;
		}

		.env-key {
			flex: 1 1 100%;
		}

		.env-equals {
			display: none;
		}

		.env-value {
			flex: 1 1 calc(100% - 40px);
		}
	}
</style>
