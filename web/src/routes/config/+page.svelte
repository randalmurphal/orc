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

	const profileDescriptions: Record<string, string> = {
		auto: 'Fully automated, no human intervention',
		fast: 'Minimal gates, speed over safety',
		safe: 'AI reviews, human only for merge',
		strict: 'Human gates on spec/review/merge'
	};

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
			setTimeout(() => (success = null), 3000);
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
	{#if success}
		<div class="toast success">
			<svg
				xmlns="http://www.w3.org/2000/svg"
				width="16"
				height="16"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				stroke-width="2"
				stroke-linecap="round"
				stroke-linejoin="round"
			>
				<path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
				<polyline points="22 4 12 14.01 9 11.01" />
			</svg>
			{success}
		</div>
	{/if}

	{#if error}
		<div class="toast error">
			<svg
				xmlns="http://www.w3.org/2000/svg"
				width="16"
				height="16"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				stroke-width="2"
				stroke-linecap="round"
				stroke-linejoin="round"
			>
				<circle cx="12" cy="12" r="10" />
				<line x1="15" y1="9" x2="9" y2="15" />
				<line x1="9" y1="9" x2="15" y2="15" />
			</svg>
			{error}
			<button class="toast-dismiss" onclick={() => (error = null)}>
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="14"
					height="14"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
				>
					<line x1="18" y1="6" x2="6" y2="18" />
					<line x1="6" y1="6" x2="18" y2="18" />
				</svg>
			</button>
		</div>
	{/if}

	{#if loading}
		<div class="loading-state">
			<div class="spinner"></div>
			<span>Loading configuration...</span>
		</div>
	{:else if config}
		{#if editing}
			<!-- Edit Mode -->
			<form
				class="config-form"
				onsubmit={(e) => {
					e.preventDefault();
					saveConfig();
				}}
			>
				<!-- Automation Card -->
				<div class="config-card">
					<div class="card-header">
						<div class="card-icon automation">
							<svg
								xmlns="http://www.w3.org/2000/svg"
								width="18"
								height="18"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
							>
								<circle cx="12" cy="12" r="3" />
								<path
									d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"
								/>
							</svg>
						</div>
						<h2>Automation</h2>
					</div>

					<div class="form-grid">
						<div class="form-group">
							<label for="profile">Profile</label>
							<select id="profile" bind:value={editProfile}>
								{#each profiles as p}
									<option value={p}>{p}</option>
								{/each}
							</select>
							<span class="hint">{profileDescriptions[editProfile]}</span>
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

						<div class="form-group toggle-group">
							<div class="toggle-label">
								<label for="retry_enabled">Enable Retry</label>
								<span class="hint">Automatically retry failed phases</span>
							</div>
							<label class="toggle">
								<input type="checkbox" id="retry_enabled" bind:checked={editRetryEnabled} />
								<span class="toggle-slider"></span>
							</label>
						</div>

						<div class="form-group">
							<label for="retry_max">Max Retries</label>
							<input type="number" id="retry_max" bind:value={editRetryMax} min="0" max="10" />
							<span class="hint">Maximum retry attempts (0-10)</span>
						</div>
					</div>
				</div>

				<!-- Execution Card -->
				<div class="config-card">
					<div class="card-header">
						<div class="card-icon execution">
							<svg
								xmlns="http://www.w3.org/2000/svg"
								width="18"
								height="18"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
							>
								<polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2" />
							</svg>
						</div>
						<h2>Execution</h2>
					</div>

					<div class="form-grid">
						<div class="form-group full-width">
							<label for="model">Model</label>
							<input
								type="text"
								id="model"
								bind:value={editModel}
								placeholder="claude-sonnet-4-20250514"
							/>
							<span class="hint">Claude model for task execution</span>
						</div>

						<div class="form-group">
							<label for="max_iterations">Max Iterations</label>
							<input
								type="number"
								id="max_iterations"
								bind:value={editMaxIterations}
								min="1"
								max="100"
							/>
							<span class="hint">Max iterations per phase</span>
						</div>

						<div class="form-group">
							<label for="timeout">Timeout</label>
							<input type="text" id="timeout" bind:value={editTimeout} placeholder="30m" />
							<span class="hint">e.g., 30m, 1h, 2h30m</span>
						</div>
					</div>
				</div>

				<!-- Git Card -->
				<div class="config-card">
					<div class="card-header">
						<div class="card-icon git">
							<svg
								xmlns="http://www.w3.org/2000/svg"
								width="18"
								height="18"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
							>
								<circle cx="18" cy="18" r="3" />
								<circle cx="6" cy="6" r="3" />
								<path d="M6 21V9a9 9 0 0 0 9 9" />
							</svg>
						</div>
						<h2>Git</h2>
					</div>

					<div class="form-grid">
						<div class="form-group">
							<label for="branch_prefix">Branch Prefix</label>
							<input
								type="text"
								id="branch_prefix"
								bind:value={editBranchPrefix}
								placeholder="orc/"
							/>
							<span class="hint">Prefix for task branches</span>
						</div>

						<div class="form-group">
							<label for="commit_prefix">Commit Prefix</label>
							<input
								type="text"
								id="commit_prefix"
								bind:value={editCommitPrefix}
								placeholder="[orc]"
							/>
							<span class="hint">Prefix for commit messages</span>
						</div>
					</div>
				</div>

				<div class="form-actions">
					<button type="button" class="btn-secondary" onclick={cancelEdit} disabled={saving}>
						Cancel
					</button>
					<button type="submit" class="btn-primary" disabled={saving}>
						{#if saving}
							<span class="spinner-small"></span>
							Saving...
						{:else}
							Save Changes
						{/if}
					</button>
				</div>
			</form>
		{:else}
			<!-- View Mode -->
			<div class="config-grid">
				<!-- Automation Card -->
				<div class="config-card">
					<div class="card-header">
						<div class="card-icon automation">
							<svg
								xmlns="http://www.w3.org/2000/svg"
								width="18"
								height="18"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
							>
								<circle cx="12" cy="12" r="3" />
								<path
									d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"
								/>
							</svg>
						</div>
						<h2>Automation</h2>
						<button class="edit-btn" onclick={startEdit} title="Edit configuration">
							<svg
								xmlns="http://www.w3.org/2000/svg"
								width="14"
								height="14"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
							>
								<path d="M17 3a2.828 2.828 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5L17 3z" />
							</svg>
						</button>
					</div>

					<div class="config-items">
						<div class="config-item">
							<span class="item-label">Profile</span>
							<span class="item-value badge">{config.automation.profile}</span>
						</div>
						<div class="config-item">
							<span class="item-label">Default Gate</span>
							<span class="item-value">{config.automation.gates_default}</span>
						</div>
						<div class="config-item">
							<span class="item-label">Retry Enabled</span>
							<span class="item-value">
								{#if config.automation.retry_enabled}
									<span class="status-pill enabled">Enabled</span>
								{:else}
									<span class="status-pill disabled">Disabled</span>
								{/if}
							</span>
						</div>
						<div class="config-item">
							<span class="item-label">Max Retries</span>
							<span class="item-value mono">{config.automation.retry_max}</span>
						</div>
					</div>
				</div>

				<!-- Execution Card -->
				<div class="config-card">
					<div class="card-header">
						<div class="card-icon execution">
							<svg
								xmlns="http://www.w3.org/2000/svg"
								width="18"
								height="18"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
							>
								<polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2" />
							</svg>
						</div>
						<h2>Execution</h2>
					</div>

					<div class="config-items">
						<div class="config-item full-width">
							<span class="item-label">Model</span>
							<span class="item-value mono model-value">{config.execution.model}</span>
						</div>
						<div class="config-item">
							<span class="item-label">Max Iterations</span>
							<span class="item-value mono">{config.execution.max_iterations}</span>
						</div>
						<div class="config-item">
							<span class="item-label">Timeout</span>
							<span class="item-value mono">{config.execution.timeout}</span>
						</div>
					</div>
				</div>

				<!-- Git Card -->
				<div class="config-card">
					<div class="card-header">
						<div class="card-icon git">
							<svg
								xmlns="http://www.w3.org/2000/svg"
								width="18"
								height="18"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
							>
								<circle cx="18" cy="18" r="3" />
								<circle cx="6" cy="6" r="3" />
								<path d="M6 21V9a9 9 0 0 0 9 9" />
							</svg>
						</div>
						<h2>Git</h2>
					</div>

					<div class="config-items">
						<div class="config-item">
							<span class="item-label">Branch Prefix</span>
							<span class="item-value mono">{config.git.branch_prefix}</span>
						</div>
						<div class="config-item">
							<span class="item-label">Commit Prefix</span>
							<span class="item-value mono">{config.git.commit_prefix}</span>
						</div>
					</div>
				</div>

				<!-- Version Card -->
				<div class="config-card version-card">
					<div class="card-header">
						<div class="card-icon version">
							<svg
								xmlns="http://www.w3.org/2000/svg"
								width="18"
								height="18"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
							>
								<path d="M12 20h9" />
								<path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z" />
							</svg>
						</div>
						<h2>Version</h2>
					</div>
					<div class="version-display">
						<span class="version-number">{config.version}</span>
						<span class="version-label">orc orchestrator</span>
					</div>
				</div>
			</div>
		{/if}
	{/if}
</div>

<style>
	.page {
		max-width: 900px;
	}

	/* Toast notifications */
	.toast {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-3) var(--space-4);
		border-radius: var(--radius-md);
		margin-bottom: var(--space-6);
		font-size: var(--text-sm);
		animation: slideIn var(--duration-normal) var(--ease-out);
	}

	@keyframes slideIn {
		from {
			opacity: 0;
			transform: translateY(-8px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
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

	.toast-dismiss {
		margin-left: auto;
		background: transparent;
		border: none;
		color: inherit;
		cursor: pointer;
		padding: var(--space-1);
		border-radius: var(--radius-sm);
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.toast-dismiss:hover {
		background: rgba(255, 255, 255, 0.1);
	}

	/* Loading state */
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

	.spinner-small {
		width: 14px;
		height: 14px;
		border: 2px solid rgba(255, 255, 255, 0.3);
		border-top-color: white;
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	/* Config grid */
	.config-grid {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: var(--space-5);
	}

	@media (max-width: 768px) {
		.config-grid {
			grid-template-columns: 1fr;
		}
	}

	/* Config cards */
	.config-card {
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		padding: var(--space-5);
		transition: border-color var(--duration-fast) var(--ease-out);
	}

	.config-card:hover {
		border-color: var(--border-default);
	}

	.version-card {
		grid-column: span 2;
	}

	@media (max-width: 768px) {
		.version-card {
			grid-column: span 1;
		}
	}

	.card-header {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		margin-bottom: var(--space-5);
	}

	.card-header h2 {
		font-size: var(--text-sm);
		font-weight: var(--font-semibold);
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		margin: 0;
		flex: 1;
	}

	.card-icon {
		width: 36px;
		height: 36px;
		display: flex;
		align-items: center;
		justify-content: center;
		border-radius: var(--radius-md);
	}

	.card-icon.automation {
		background: rgba(139, 92, 246, 0.15);
		color: var(--accent-primary);
	}

	.card-icon.execution {
		background: rgba(245, 158, 11, 0.15);
		color: var(--status-warning);
	}

	.card-icon.git {
		background: rgba(239, 68, 68, 0.15);
		color: #f87171;
	}

	.card-icon.version {
		background: rgba(16, 185, 129, 0.15);
		color: var(--status-success);
	}

	.edit-btn {
		background: var(--bg-tertiary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-sm);
		color: var(--text-muted);
		padding: var(--space-2);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.edit-btn:hover {
		background: var(--bg-surface);
		border-color: var(--border-default);
		color: var(--text-primary);
	}

	/* Config items */
	.config-items {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: var(--space-4);
	}

	.config-item {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
	}

	.config-item.full-width {
		grid-column: span 2;
	}

	.item-label {
		font-size: var(--text-xs);
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.item-value {
		font-size: var(--text-sm);
		color: var(--text-primary);
		font-weight: var(--font-medium);
	}

	.item-value.mono {
		font-family: var(--font-mono);
	}

	.item-value.model-value {
		font-size: var(--text-xs);
		word-break: break-all;
	}

	.item-value.badge {
		display: inline-flex;
		background: var(--accent-glow);
		color: var(--accent-primary);
		padding: var(--space-1) var(--space-2);
		border-radius: var(--radius-sm);
		font-size: var(--text-xs);
		font-weight: var(--font-semibold);
		width: fit-content;
	}

	.status-pill {
		display: inline-flex;
		padding: var(--space-0-5) var(--space-2);
		border-radius: var(--radius-full);
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
	}

	.status-pill.enabled {
		background: rgba(16, 185, 129, 0.15);
		color: var(--status-success);
	}

	.status-pill.disabled {
		background: rgba(107, 114, 128, 0.15);
		color: var(--text-muted);
	}

	/* Version display */
	.version-display {
		display: flex;
		align-items: baseline;
		gap: var(--space-3);
	}

	.version-number {
		font-family: var(--font-mono);
		font-size: var(--text-2xl);
		font-weight: var(--font-bold);
		color: var(--text-primary);
	}

	.version-label {
		font-size: var(--text-sm);
		color: var(--text-muted);
	}

	/* Form styles */
	.config-form {
		display: flex;
		flex-direction: column;
		gap: var(--space-5);
	}

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

	.form-group.full-width {
		grid-column: span 2;
	}

	.form-group label {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
	}

	.form-group input[type='text'],
	.form-group input[type='number'],
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

	.form-group .hint {
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	/* Toggle switch */
	.toggle-group {
		flex-direction: row;
		align-items: center;
		justify-content: space-between;
		grid-column: span 2;
		padding: var(--space-3);
		background: var(--bg-tertiary);
		border-radius: var(--radius-md);
	}

	.toggle-label {
		display: flex;
		flex-direction: column;
		gap: var(--space-0-5);
	}

	.toggle-label label {
		margin: 0;
	}

	.toggle-label .hint {
		margin: 0;
	}

	.toggle {
		position: relative;
		display: inline-block;
		width: 44px;
		height: 24px;
		cursor: pointer;
	}

	.toggle input {
		opacity: 0;
		width: 0;
		height: 0;
	}

	.toggle-slider {
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background: var(--bg-surface);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-full);
		transition: all var(--duration-fast) var(--ease-out);
	}

	.toggle-slider::before {
		content: '';
		position: absolute;
		width: 18px;
		height: 18px;
		left: 2px;
		bottom: 2px;
		background: var(--text-muted);
		border-radius: 50%;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.toggle input:checked + .toggle-slider {
		background: var(--accent-primary);
		border-color: var(--accent-primary);
	}

	.toggle input:checked + .toggle-slider::before {
		transform: translateX(20px);
		background: white;
	}

	.toggle input:focus + .toggle-slider {
		box-shadow: 0 0 0 3px var(--accent-glow);
	}

	/* Form actions */
	.form-actions {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-3);
		padding-top: var(--space-4);
		border-top: 1px solid var(--border-subtle);
	}

	.btn-primary,
	.btn-secondary {
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2-5) var(--space-5);
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.btn-primary {
		background: var(--accent-primary);
		border: 1px solid var(--accent-primary);
		color: white;
	}

	.btn-primary:hover:not(:disabled) {
		background: var(--accent-hover);
		border-color: var(--accent-hover);
	}

	.btn-primary:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.btn-secondary {
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		color: var(--text-primary);
	}

	.btn-secondary:hover:not(:disabled) {
		background: var(--bg-surface);
		border-color: var(--border-strong);
	}

	.btn-secondary:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}
</style>
