<script lang="ts">
	import { onMount } from 'svelte';
	import Icon from '$lib/components/ui/Icon.svelte';
	import Breadcrumbs from '$lib/components/ui/Breadcrumbs.svelte';
	import {
		getGlobalSettings,
		getSettings,
		updateSettings,
		updateGlobalSettings,
		getClaudeMDHierarchy,
		type Settings,
		type ClaudeMDHierarchy
	} from '$lib/api';

	// State
	let loading = $state(true);
	let saving = $state(false);
	let savingGlobal = $state(false);
	let error = $state<string | null>(null);
	let success = $state<string | null>(null);

	// Data
	let globalSettings = $state<Settings | null>(null);
	let mergedSettings = $state<Settings | null>(null);
	let claudeMD = $state<ClaudeMDHierarchy | null>(null);

	// Form state for project settings (editable)
	let envEntries = $state<{ key: string; value: string }[]>([]);
	let statusLineType = $state('');
	let statusLineCommand = $state('');
	let newEnvKey = $state('');
	let newEnvValue = $state('');

	// Form state for global settings (editable)
	let globalEnvEntries = $state<{ key: string; value: string }[]>([]);
	let globalStatusLineType = $state('');
	let globalStatusLineCommand = $state('');
	let newGlobalEnvKey = $state('');
	let newGlobalEnvValue = $state('');

	// Active section
	let activeSection = $state<'global' | 'project'>('global');

	onMount(async () => {
		try {
			const [globalRes, mergedRes, claudeMDRes] = await Promise.all([
				getGlobalSettings().catch(() => null),
				getSettings().catch(() => null),
				getClaudeMDHierarchy().catch(() => null)
			]);

			globalSettings = globalRes;
			mergedSettings = mergedRes;
			claudeMD = claudeMDRes;

			resetProjectForm();
			resetGlobalForm();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load preferences';
		} finally {
			loading = false;
		}
	});

	function resetProjectForm() {
		if (mergedSettings?.env) {
			envEntries = Object.entries(mergedSettings.env).map(([key, value]) => ({ key, value }));
		}
		if (mergedSettings?.statusLine) {
			statusLineType = mergedSettings.statusLine.type || '';
			statusLineCommand = mergedSettings.statusLine.command || '';
		}
	}

	function resetGlobalForm() {
		if (globalSettings?.env) {
			globalEnvEntries = Object.entries(globalSettings.env).map(([key, value]) => ({ key, value }));
		} else {
			globalEnvEntries = [];
		}
		if (globalSettings?.statusLine) {
			globalStatusLineType = globalSettings.statusLine.type || '';
			globalStatusLineCommand = globalSettings.statusLine.command || '';
		} else {
			globalStatusLineType = '';
			globalStatusLineCommand = '';
		}
	}

	function addEnvVar() {
		if (!newEnvKey.trim()) return;
		envEntries = [...envEntries, { key: newEnvKey.trim(), value: newEnvValue }];
		newEnvKey = '';
		newEnvValue = '';
	}

	function removeEnvVar(index: number) {
		envEntries = envEntries.filter((_, i) => i !== index);
	}

	function addGlobalEnvVar() {
		if (!newGlobalEnvKey.trim()) return;
		globalEnvEntries = [...globalEnvEntries, { key: newGlobalEnvKey.trim(), value: newGlobalEnvValue }];
		newGlobalEnvKey = '';
		newGlobalEnvValue = '';
	}

	function removeGlobalEnvVar(index: number) {
		globalEnvEntries = globalEnvEntries.filter((_, i) => i !== index);
	}

	async function saveProjectSettings() {
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
			mergedSettings = await getSettings();
			success = 'Project settings saved';
			setTimeout(() => (success = null), 3000);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save settings';
		} finally {
			saving = false;
		}
	}

	async function saveGlobalSettings() {
		savingGlobal = true;
		error = null;
		success = null;

		const settings: Settings = {
			env: globalEnvEntries.reduce(
				(acc, { key, value }) => {
					if (key.trim()) acc[key.trim()] = value;
					return acc;
				},
				{} as Record<string, string>
			)
		};

		if (globalStatusLineType || globalStatusLineCommand) {
			settings.statusLine = {};
			if (globalStatusLineType) settings.statusLine.type = globalStatusLineType;
			if (globalStatusLineCommand) settings.statusLine.command = globalStatusLineCommand;
		}

		try {
			await updateGlobalSettings(settings);
			globalSettings = await getGlobalSettings();
			mergedSettings = await getSettings();
			success = 'Global settings saved';
			setTimeout(() => (success = null), 3000);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save global settings';
		} finally {
			savingGlobal = false;
		}
	}
</script>

<svelte:head>
	<title>Preferences - orc</title>
</svelte:head>

<div class="preferences-page">
	<Breadcrumbs />
	<header class="page-header">
		<h1>Preferences</h1>
		<p class="subtitle">Personal and global Claude Code settings</p>
	</header>

	{#if success}
		<div class="toast success">
			<Icon name="check" size={16} />
			{success}
		</div>
	{/if}

	{#if error}
		<div class="toast error">
			<Icon name="error" size={16} />
			{error}
		</div>
	{/if}

	{#if loading}
		<div class="loading-state">
			<div class="spinner"></div>
			<span>Loading preferences...</span>
		</div>
	{:else}
		<!-- Section Tabs -->
		<div class="section-tabs">
			<button
				class="section-tab"
				class:active={activeSection === 'global'}
				onclick={() => (activeSection = 'global')}
			>
				<Icon name="user" size={16} />
				<span>Global Settings</span>
			</button>
			<button
				class="section-tab"
				class:active={activeSection === 'project'}
				onclick={() => (activeSection = 'project')}
			>
				<Icon name="folder" size={16} />
				<span>Project Settings</span>
			</button>
		</div>

		<!-- Global Settings (Editable) -->
		{#if activeSection === 'global'}
			<div class="content-section">
				<div class="section-header-actions">
					<h2>Global Claude Settings</h2>
					<button class="btn-primary" onclick={saveGlobalSettings} disabled={savingGlobal}>
						{savingGlobal ? 'Saving...' : 'Save Changes'}
					</button>
				</div>

				<!-- Global Environment Variables -->
				<div class="config-card editable">
					<div class="card-header">
						<div class="card-icon">
							<Icon name="config" size={18} />
						</div>
						<div class="card-title">
							<h3>Environment Variables</h3>
							<p>~/.claude/settings.json → env</p>
						</div>
					</div>
					<div class="card-content">
						<div class="env-list">
							{#each globalEnvEntries as entry, index}
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
										onclick={() => removeGlobalEnvVar(index)}
										title="Remove"
									>
										<Icon name="close" size={14} />
									</button>
								</div>
							{/each}

							<div class="env-row env-new">
								<input
									type="text"
									bind:value={newGlobalEnvKey}
									placeholder="NEW_KEY"
									class="env-key"
									onkeydown={(e) => e.key === 'Enter' && addGlobalEnvVar()}
								/>
								<span class="env-equals">=</span>
								<input
									type="text"
									bind:value={newGlobalEnvValue}
									placeholder="value"
									class="env-value"
									onkeydown={(e) => e.key === 'Enter' && addGlobalEnvVar()}
								/>
								<button class="btn-icon btn-add" onclick={addGlobalEnvVar} title="Add">
									<Icon name="plus" size={14} />
								</button>
							</div>
						</div>
					</div>
				</div>

				<!-- Global Status Line -->
				<div class="config-card editable">
					<div class="card-header">
						<div class="card-icon">
							<Icon name="info" size={18} />
						</div>
						<div class="card-title">
							<h3>Status Line</h3>
							<p>Global status line display</p>
						</div>
					</div>
					<div class="card-content">
						<div class="form-grid">
							<div class="form-group">
								<label for="global-status-type">Type</label>
								<select id="global-status-type" bind:value={globalStatusLineType}>
									<option value="">Default</option>
									<option value="text">Text</option>
									<option value="command">Command</option>
								</select>
							</div>

							{#if globalStatusLineType === 'command'}
								<div class="form-group">
									<label for="global-status-command">Command</label>
									<input
										id="global-status-command"
										type="text"
										bind:value={globalStatusLineCommand}
										placeholder="echo 'Status: ready'"
									/>
								</div>
							{/if}
						</div>
					</div>
				</div>

				<!-- Global CLAUDE.md (Read-only preview with link) -->
				{#if claudeMD?.global?.content}
					<div class="config-card">
						<div class="card-header">
							<div class="card-icon">
								<Icon name="file-text" size={18} />
							</div>
							<div class="card-title">
								<h3>Global CLAUDE.md</h3>
								<p>~/.claude/CLAUDE.md</p>
							</div>
							<a href="/environment/docs?scope=global" class="card-link">
								<span>Edit</span>
								<Icon name="chevron-right" size={14} />
							</a>
						</div>
						<div class="card-content">
							<div class="preview-box">
								<pre>{claudeMD.global.content.slice(0, 500)}{claudeMD.global.content.length > 500 ? '...' : ''}</pre>
							</div>
							<p class="preview-info">{claudeMD.global.content.length.toLocaleString()} characters</p>
						</div>
					</div>
				{:else}
					<div class="empty-card">
						<Icon name="file-text" size={24} />
						<p>No global CLAUDE.md configured</p>
						<span class="empty-hint">Create ~/.claude/CLAUDE.md to add global instructions</span>
					</div>
				{/if}

				<!-- User CLAUDE.md -->
				{#if claudeMD?.user?.content}
					<div class="config-card">
						<div class="card-header">
							<div class="card-icon">
								<Icon name="file-text" size={18} />
							</div>
							<div class="card-title">
								<h3>User CLAUDE.md</h3>
								<p>~/CLAUDE.md</p>
							</div>
						</div>
						<div class="card-content">
							<div class="preview-box">
								<pre>{claudeMD.user.content.slice(0, 500)}{claudeMD.user.content.length > 500 ? '...' : ''}</pre>
							</div>
							<p class="preview-info">{claudeMD.user.content.length.toLocaleString()} characters</p>
						</div>
					</div>
				{/if}
			</div>
		{/if}

		<!-- Project Settings (Editable) -->
		{#if activeSection === 'project'}
			<div class="content-section">
				<div class="section-header-actions">
					<h2>Project Claude Settings</h2>
					<button class="btn-primary" onclick={saveProjectSettings} disabled={saving}>
						{saving ? 'Saving...' : 'Save Changes'}
					</button>
				</div>

				<!-- Environment Variables -->
				<div class="config-card editable">
					<div class="card-header">
						<div class="card-icon">
							<Icon name="config" size={18} />
						</div>
						<div class="card-title">
							<h3>Environment Variables</h3>
							<p>.claude/settings.json → env</p>
						</div>
					</div>
					<div class="card-content">
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
										onclick={() => removeEnvVar(index)}
										title="Remove"
									>
										<Icon name="close" size={14} />
									</button>
								</div>
							{/each}

							<div class="env-row env-new">
								<input
									type="text"
									bind:value={newEnvKey}
									placeholder="NEW_KEY"
									class="env-key"
									onkeydown={(e) => e.key === 'Enter' && addEnvVar()}
								/>
								<span class="env-equals">=</span>
								<input
									type="text"
									bind:value={newEnvValue}
									placeholder="value"
									class="env-value"
									onkeydown={(e) => e.key === 'Enter' && addEnvVar()}
								/>
								<button class="btn-icon btn-add" onclick={addEnvVar} title="Add">
									<Icon name="plus" size={14} />
								</button>
							</div>
						</div>
					</div>
				</div>

				<!-- Status Line -->
				<div class="config-card editable">
					<div class="card-header">
						<div class="card-icon">
							<Icon name="info" size={18} />
						</div>
						<div class="card-title">
							<h3>Status Line</h3>
							<p>Custom status line display</p>
						</div>
					</div>
					<div class="card-content">
						<div class="form-grid">
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
								</div>
							{/if}
						</div>
					</div>
				</div>

				<!-- Effective Settings -->
				<div class="config-card">
					<div class="card-header">
						<div class="card-icon">
							<Icon name="file" size={18} />
						</div>
						<div class="card-title">
							<h3>Effective Settings</h3>
							<p>Merged global + project settings</p>
						</div>
					</div>
					<div class="card-content">
						<div class="preview-box">
							<pre>{JSON.stringify(mergedSettings, null, 2)}</pre>
						</div>
					</div>
				</div>
			</div>
		{/if}
	{/if}
</div>

<style>
	.preferences-page {
		max-width: 800px;
	}

	.page-header {
		margin-bottom: var(--space-6);
	}

	.page-header h1 {
		font-size: var(--text-xl);
		font-weight: var(--font-bold);
		color: var(--text-primary);
		margin: 0;
	}

	.subtitle {
		font-size: var(--text-sm);
		color: var(--text-muted);
		margin: var(--space-1) 0 0;
	}

	/* Toast */
	.toast {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-3) var(--space-4);
		border-radius: var(--radius-md);
		margin-bottom: var(--space-4);
		font-size: var(--text-sm);
	}

	.toast.success {
		background: rgba(16, 185, 129, 0.1);
		border: 1px solid var(--status-success);
		color: var(--status-success);
	}

	.toast.error {
		background: rgba(239, 68, 68, 0.1);
		border: 1px solid var(--status-danger);
		color: var(--status-danger);
	}

	/* Loading */
	.loading-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: var(--space-16);
		gap: var(--space-4);
		color: var(--text-secondary);
	}

	.spinner {
		width: 32px;
		height: 32px;
		border: 3px solid var(--border-subtle);
		border-top-color: var(--accent-primary);
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	/* Section Tabs */
	.section-tabs {
		display: flex;
		gap: var(--space-1);
		background: var(--bg-secondary);
		padding: var(--space-1);
		border-radius: var(--radius-lg);
		margin-bottom: var(--space-5);
	}

	.section-tab {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-2);
		padding: var(--space-3) var(--space-4);
		background: transparent;
		border: none;
		border-radius: var(--radius-md);
		color: var(--text-secondary);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.section-tab:hover {
		color: var(--text-primary);
		background: var(--bg-tertiary);
	}

	.section-tab.active {
		background: var(--bg-primary);
		color: var(--accent-primary);
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
	}

	/* Content Section */
	.content-section {
		display: flex;
		flex-direction: column;
		gap: var(--space-5);
	}

	.section-header-actions {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	.section-header-actions h2 {
		font-size: var(--text-lg);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		margin: 0;
	}

	.btn-primary {
		padding: var(--space-2-5) var(--space-5);
		background: var(--accent-primary);
		border: 1px solid var(--accent-primary);
		border-radius: var(--radius-md);
		color: white;
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.btn-primary:hover:not(:disabled) {
		background: var(--accent-hover);
	}

	.btn-primary:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	/* Config Card */
	.config-card {
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		overflow: hidden;
	}

	.config-card.editable {
		border-color: var(--accent-primary);
		border-style: dashed;
	}

	.card-header {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-4);
		background: var(--bg-tertiary);
		border-bottom: 1px solid var(--border-subtle);
	}

	.card-icon {
		width: 36px;
		height: 36px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--accent-subtle);
		color: var(--accent-primary);
		border-radius: var(--radius-md);
		flex-shrink: 0;
	}

	.card-title h3 {
		font-size: var(--text-sm);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		margin: 0;
	}

	.card-title p {
		font-size: var(--text-xs);
		color: var(--text-muted);
		margin: var(--space-0-5) 0 0;
		font-family: var(--font-mono);
	}

	.card-title {
		flex: 1;
	}

	.card-link {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		padding: var(--space-1-5) var(--space-3);
		background: var(--bg-primary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-secondary);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		text-decoration: none;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.card-link:hover {
		border-color: var(--accent-primary);
		color: var(--accent-primary);
	}

	.card-content {
		padding: var(--space-4);
	}

	/* Preview Box */
	.preview-box {
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		padding: var(--space-3);
		max-height: 200px;
		overflow: auto;
	}

	.preview-box pre {
		margin: 0;
		font-family: var(--font-mono);
		font-size: var(--text-xs);
		color: var(--text-primary);
		white-space: pre-wrap;
	}

	.preview-info {
		font-size: var(--text-xs);
		color: var(--text-muted);
		margin: var(--space-2) 0 0;
	}

	/* Empty Card */
	.empty-card {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: var(--space-8);
		background: var(--bg-secondary);
		border: 1px dashed var(--border-default);
		border-radius: var(--radius-lg);
		color: var(--text-muted);
		text-align: center;
	}

	.empty-card p {
		font-size: var(--text-sm);
		margin: var(--space-3) 0 var(--space-1);
	}

	.empty-hint {
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	/* Environment Variables */
	.env-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.env-row {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.env-key {
		flex: 0 0 180px;
		padding: var(--space-2) var(--space-3);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-primary);
		font-family: var(--font-mono);
		font-size: var(--text-sm);
	}

	.env-equals {
		color: var(--text-muted);
		font-family: var(--font-mono);
	}

	.env-value {
		flex: 1;
		padding: var(--space-2) var(--space-3);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-primary);
		font-size: var(--text-sm);
	}

	.env-new {
		opacity: 0.6;
	}

	.env-new:focus-within {
		opacity: 1;
	}

	.btn-icon {
		width: 32px;
		height: 32px;
		display: flex;
		align-items: center;
		justify-content: center;
		border: none;
		border-radius: var(--radius-md);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.btn-remove {
		background: rgba(239, 68, 68, 0.1);
		color: var(--status-danger);
	}

	.btn-remove:hover {
		background: rgba(239, 68, 68, 0.2);
	}

	.btn-add {
		background: rgba(16, 185, 129, 0.1);
		color: var(--status-success);
	}

	.btn-add:hover {
		background: rgba(16, 185, 129, 0.2);
	}

	/* Form Grid */
	.form-grid {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: var(--space-4);
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: var(--space-1-5);
	}

	.form-group label {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
	}

	.form-group input,
	.form-group select {
		width: 100%;
		padding: var(--space-2-5) var(--space-3);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-primary);
		font-size: var(--text-sm);
		transition: all var(--duration-fast) var(--ease-out);
	}

	.form-group input:focus,
	.form-group select:focus {
		outline: none;
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 3px var(--accent-glow);
	}
</style>
