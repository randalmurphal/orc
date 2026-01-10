<script lang="ts">
	import { onMount } from 'svelte';
	import { getConfig, updateConfig, type Config, type ConfigUpdateRequest } from '$lib/api';

	let config = $state<Config | null>(null);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let success = $state<string | null>(null);
	let editing = $state(false);

	// Edit form state
	let editProfile = $state('');
	let editGatesDefault = $state('');
	let editRetryEnabled = $state(false);
	let editRetryMax = $state(3);
	let editModel = $state('');
	let editMaxIterations = $state(10);
	let editTimeout = $state('');
	let editBranchPrefix = $state('');
	let editCommitPrefix = $state('');

	const profiles = ['auto', 'fast', 'safe', 'strict'];
	const gateTypes = ['auto', 'human', 'ai'];

	onMount(async () => {
		await loadConfig();
	});

	async function loadConfig() {
		loading = true;
		error = null;
		try {
			config = await getConfig();
			resetEditForm();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load config';
		} finally {
			loading = false;
		}
	}

	function resetEditForm() {
		if (!config) return;
		editProfile = config.profile;
		editGatesDefault = config.automation.gates_default;
		editRetryEnabled = config.automation.retry_enabled;
		editRetryMax = config.automation.retry_max;
		editModel = config.execution.model;
		editMaxIterations = config.execution.max_iterations;
		editTimeout = config.execution.timeout;
		editBranchPrefix = config.git.branch_prefix;
		editCommitPrefix = config.git.commit_prefix;
	}

	function startEdit() {
		resetEditForm();
		editing = true;
		success = null;
	}

	function cancelEdit() {
		editing = false;
		resetEditForm();
	}

	async function saveConfig() {
		saving = true;
		error = null;
		success = null;

		try {
			const req: ConfigUpdateRequest = {
				profile: editProfile,
				automation: {
					gates_default: editGatesDefault,
					retry_enabled: editRetryEnabled,
					retry_max: editRetryMax
				},
				execution: {
					model: editModel,
					max_iterations: editMaxIterations,
					timeout: editTimeout
				},
				git: {
					branch_prefix: editBranchPrefix,
					commit_prefix: editCommitPrefix
				}
			};

			config = await updateConfig(req);
			editing = false;
			success = 'Configuration saved successfully';
			setTimeout(() => success = null, 3000);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save config';
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head>
	<title>Config - orc</title>
</svelte:head>

<div class="page">
	<header class="page-header">
		<h1>Configuration</h1>
		{#if !editing && config}
			<button class="primary" onclick={startEdit}>Edit</button>
		{/if}
	</header>

	{#if success}
		<div class="success-banner">{success}</div>
	{/if}

	{#if error}
		<div class="error-banner">
			{error}
			<button onclick={() => error = null}>Dismiss</button>
		</div>
	{/if}

	{#if loading}
		<div class="loading">Loading configuration...</div>
	{:else if config}
		{#if editing}
			<!-- Edit Mode -->
			<form class="config-form" onsubmit={(e) => { e.preventDefault(); saveConfig(); }}>
				<section class="config-section">
					<h2>Automation</h2>
					<div class="form-group">
						<label for="profile">Profile</label>
						<select id="profile" bind:value={editProfile}>
							{#each profiles as p}
								<option value={p}>{p}</option>
							{/each}
						</select>
						<span class="hint">Controls overall automation level</span>
					</div>
					<div class="form-group">
						<label for="gates_default">Default Gate</label>
						<select id="gates_default" bind:value={editGatesDefault}>
							{#each gateTypes as g}
								<option value={g}>{g}</option>
							{/each}
						</select>
						<span class="hint">Gate type for phase transitions</span>
					</div>
					<div class="form-group checkbox">
						<input type="checkbox" id="retry_enabled" bind:checked={editRetryEnabled} />
						<label for="retry_enabled">Enable Retry</label>
						<span class="hint">Automatically retry failed phases</span>
					</div>
					<div class="form-group">
						<label for="retry_max">Max Retries</label>
						<input type="number" id="retry_max" bind:value={editRetryMax} min="0" max="10" />
						<span class="hint">Maximum retry attempts (0-10)</span>
					</div>
				</section>

				<section class="config-section">
					<h2>Execution</h2>
					<div class="form-group">
						<label for="model">Model</label>
						<input type="text" id="model" bind:value={editModel} placeholder="claude-sonnet-4-20250514" />
						<span class="hint">Claude model for execution</span>
					</div>
					<div class="form-group">
						<label for="max_iterations">Max Iterations</label>
						<input type="number" id="max_iterations" bind:value={editMaxIterations} min="1" max="100" />
						<span class="hint">Max iterations per phase (1-100)</span>
					</div>
					<div class="form-group">
						<label for="timeout">Timeout</label>
						<input type="text" id="timeout" bind:value={editTimeout} placeholder="30m" />
						<span class="hint">Task timeout (e.g., 30m, 1h)</span>
					</div>
				</section>

				<section class="config-section">
					<h2>Git</h2>
					<div class="form-group">
						<label for="branch_prefix">Branch Prefix</label>
						<input type="text" id="branch_prefix" bind:value={editBranchPrefix} placeholder="orc/" />
						<span class="hint">Prefix for task branches</span>
					</div>
					<div class="form-group">
						<label for="commit_prefix">Commit Prefix</label>
						<input type="text" id="commit_prefix" bind:value={editCommitPrefix} placeholder="[orc]" />
						<span class="hint">Prefix for commit messages</span>
					</div>
				</section>

				<div class="form-actions">
					<button type="button" onclick={cancelEdit} disabled={saving}>Cancel</button>
					<button type="submit" class="primary" disabled={saving}>
						{saving ? 'Saving...' : 'Save Changes'}
					</button>
				</div>
			</form>
		{:else}
			<!-- View Mode -->
			<div class="config-sections">
				<section class="config-section">
					<h2>Automation</h2>
					<table class="config-table">
						<tbody>
							<tr>
								<td class="label">Profile</td>
								<td class="value">{config.automation.profile}</td>
							</tr>
							<tr>
								<td class="label">Default Gate</td>
								<td class="value">{config.automation.gates_default}</td>
							</tr>
							<tr>
								<td class="label">Retry Enabled</td>
								<td class="value">{config.automation.retry_enabled ? 'Yes' : 'No'}</td>
							</tr>
							<tr>
								<td class="label">Max Retries</td>
								<td class="value">{config.automation.retry_max}</td>
							</tr>
						</tbody>
					</table>
				</section>

				<section class="config-section">
					<h2>Execution</h2>
					<table class="config-table">
						<tbody>
							<tr>
								<td class="label">Model</td>
								<td class="value mono">{config.execution.model}</td>
							</tr>
							<tr>
								<td class="label">Max Iterations</td>
								<td class="value">{config.execution.max_iterations}</td>
							</tr>
							<tr>
								<td class="label">Timeout</td>
								<td class="value">{config.execution.timeout}</td>
							</tr>
						</tbody>
					</table>
				</section>

				<section class="config-section">
					<h2>Git</h2>
					<table class="config-table">
						<tbody>
							<tr>
								<td class="label">Branch Prefix</td>
								<td class="value mono">{config.git.branch_prefix}</td>
							</tr>
							<tr>
								<td class="label">Commit Prefix</td>
								<td class="value mono">{config.git.commit_prefix}</td>
							</tr>
						</tbody>
					</table>
				</section>

				<section class="config-section">
					<h2>Version</h2>
					<p class="version">{config.version}</p>
				</section>
			</div>
		{/if}
	{/if}
</div>

<style>
	.page {
		max-width: 700px;
	}

	.page-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 2rem;
	}

	h1 {
		font-size: 1.5rem;
		font-weight: 600;
	}

	.config-sections, .config-form {
		display: flex;
		flex-direction: column;
		gap: 2rem;
	}

	.config-section {
		background: var(--bg-secondary);
		border: 1px solid var(--border-color);
		border-radius: 8px;
		padding: 1.25rem;
	}

	.config-section h2 {
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		margin-bottom: 1rem;
	}

	.config-table {
		width: 100%;
		border-collapse: collapse;
	}

	.config-table tr {
		border-bottom: 1px solid var(--border-color);
	}

	.config-table tr:last-child {
		border-bottom: none;
	}

	.config-table td {
		padding: 0.75rem 0;
	}

	.config-table .label {
		color: var(--text-secondary);
		font-size: 0.875rem;
		width: 40%;
	}

	.config-table .value {
		color: var(--text-primary);
		font-weight: 500;
	}

	.config-table .value.mono {
		font-family: var(--font-mono);
		font-size: 0.875rem;
	}

	.version {
		font-family: var(--font-mono);
		font-size: 0.875rem;
		color: var(--text-secondary);
	}

	/* Form styles */
	.form-group {
		margin-bottom: 1rem;
	}

	.form-group:last-child {
		margin-bottom: 0;
	}

	.form-group label {
		display: block;
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--text-primary);
		margin-bottom: 0.375rem;
	}

	.form-group.checkbox {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.form-group.checkbox label {
		display: inline;
		margin-bottom: 0;
	}

	.form-group.checkbox .hint {
		flex: 1;
	}

	.form-group input[type="text"],
	.form-group input[type="number"],
	.form-group select {
		width: 100%;
		padding: 0.625rem 0.75rem;
		background: var(--bg-tertiary);
		border: 1px solid var(--border-color);
		border-radius: 6px;
		color: var(--text-primary);
		font-size: 0.875rem;
	}

	.form-group input:focus,
	.form-group select:focus {
		outline: none;
		border-color: var(--accent-primary);
	}

	.form-group input[type="checkbox"] {
		width: 1rem;
		height: 1rem;
		accent-color: var(--accent-primary);
	}

	.form-group .hint {
		display: block;
		font-size: 0.75rem;
		color: var(--text-muted);
		margin-top: 0.25rem;
	}

	.form-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.75rem;
	}

	.success-banner {
		background: rgba(63, 185, 80, 0.1);
		border: 1px solid var(--accent-success);
		border-radius: 6px;
		padding: 0.75rem 1rem;
		margin-bottom: 1rem;
		color: var(--accent-success);
	}

	.error-banner {
		background: rgba(248, 81, 73, 0.1);
		border: 1px solid var(--accent-danger);
		border-radius: 6px;
		padding: 0.75rem 1rem;
		margin-bottom: 1rem;
		display: flex;
		justify-content: space-between;
		align-items: center;
		color: var(--accent-danger);
	}

	.error-banner button {
		background: transparent;
		border: none;
		color: var(--accent-danger);
		padding: 0.25rem 0.5rem;
	}

	.loading {
		text-align: center;
		padding: 3rem;
		color: var(--text-secondary);
	}
</style>
