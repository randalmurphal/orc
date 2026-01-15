<script lang="ts">
	import { onMount } from 'svelte';
	import Icon from '$lib/components/ui/Icon.svelte';
	import { getExportConfig, updateExportConfig, type ExportConfig } from '$lib/api';

	let config = $state<ExportConfig | null>(null);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let success = $state<string | null>(null);

	// Form state
	let enabled = $state(false);
	let preset = $state('');
	let taskDefinition = $state(true);
	let finalState = $state(true);
	let transcripts = $state(false);
	let contextSummary = $state(true);

	const presets = [
		{ value: '', label: 'Custom', description: 'Configure individual options' },
		{ value: 'minimal', label: 'Minimal', description: 'Task definition only' },
		{ value: 'standard', label: 'Standard', description: 'Definition + state + context' },
		{ value: 'full', label: 'Full', description: 'Everything including transcripts' }
	];

	onMount(async () => {
		try {
			config = await getExportConfig();
			resetForm();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load export config';
		} finally {
			loading = false;
		}
	});

	function resetForm() {
		if (!config) return;
		enabled = config.enabled ?? false;
		preset = config.preset ?? '';
		taskDefinition = config.task_definition ?? true;
		finalState = config.final_state ?? true;
		transcripts = config.transcripts ?? false;
		contextSummary = config.context_summary ?? true;
	}

	function applyPreset(presetValue: string) {
		preset = presetValue;
		switch (presetValue) {
			case 'minimal':
				taskDefinition = true;
				finalState = false;
				transcripts = false;
				contextSummary = false;
				break;
			case 'standard':
				taskDefinition = true;
				finalState = true;
				transcripts = false;
				contextSummary = true;
				break;
			case 'full':
				taskDefinition = true;
				finalState = true;
				transcripts = true;
				contextSummary = true;
				break;
			default:
				// Custom - keep current settings
				break;
		}
	}

	async function saveConfig() {
		saving = true;
		error = null;
		success = null;

		try {
			const newConfig: ExportConfig = {
				enabled,
				preset,
				task_definition: taskDefinition,
				final_state: finalState,
				transcripts,
				context_summary: contextSummary
			};

			config = await updateExportConfig(newConfig);
			success = 'Export configuration saved';
			setTimeout(() => (success = null), 3000);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save config';
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head>
	<title>Export - orc</title>
</svelte:head>

<div class="export-page">
	<header class="page-header">
		<div class="header-content">
			<div>
				<h1>Export Configuration</h1>
				<p class="subtitle">Configure task artifact export settings</p>
			</div>
			<button class="btn-primary" onclick={saveConfig} disabled={saving || loading}>
				{saving ? 'Saving...' : 'Save'}
			</button>
		</div>
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
			<span>Loading configuration...</span>
		</div>
	{:else}
		<div class="config-sections">
			<!-- Master Toggle -->
			<div class="config-section">
				<div class="section-header">
					<div class="section-icon">
						<Icon name="export" size={18} />
					</div>
					<h2>Auto-Export</h2>
				</div>

				<div class="toggle-row">
					<div class="toggle-info">
						<label for="enabled">Enable Auto-Export</label>
						<span class="hint">Automatically export task artifacts on completion</span>
					</div>
					<label class="toggle">
						<input type="checkbox" id="enabled" bind:checked={enabled} />
						<span class="toggle-slider"></span>
					</label>
				</div>
			</div>

			<!-- Presets -->
			<div class="config-section">
				<div class="section-header">
					<div class="section-icon">
						<Icon name="config" size={18} />
					</div>
					<h2>Presets</h2>
				</div>

				<div class="preset-grid">
					{#each presets as p}
						<button
							class="preset-card"
							class:active={preset === p.value}
							onclick={() => applyPreset(p.value)}
						>
							<span class="preset-name">{p.label}</span>
							<span class="preset-desc">{p.description}</span>
						</button>
					{/each}
				</div>
			</div>

			<!-- Individual Options -->
			<div class="config-section">
				<div class="section-header">
					<div class="section-icon">
						<Icon name="file" size={18} />
					</div>
					<h2>Export Options</h2>
				</div>

				<div class="options-list">
					<div class="option-row">
						<label class="checkbox-label">
							<input type="checkbox" bind:checked={taskDefinition} />
							<span class="checkbox-box"></span>
							<span class="option-info">
								<span class="option-name">Task Definition</span>
								<span class="option-desc">Export task.yaml and plan.yaml</span>
							</span>
						</label>
					</div>

					<div class="option-row">
						<label class="checkbox-label">
							<input type="checkbox" bind:checked={finalState} />
							<span class="checkbox-box"></span>
							<span class="option-info">
								<span class="option-name">Final State</span>
								<span class="option-desc">Export state.yaml with execution results</span>
							</span>
						</label>
					</div>

					<div class="option-row">
						<label class="checkbox-label">
							<input type="checkbox" bind:checked={contextSummary} />
							<span class="checkbox-box"></span>
							<span class="option-info">
								<span class="option-name">Context Summary</span>
								<span class="option-desc">Export context.md with task summary</span>
							</span>
						</label>
					</div>

					<div class="option-row">
						<label class="checkbox-label">
							<input type="checkbox" bind:checked={transcripts} />
							<span class="checkbox-box"></span>
							<span class="option-info">
								<span class="option-name">Transcripts</span>
								<span class="option-desc">Export full Claude conversation logs (large files)</span>
							</span>
						</label>
						{#if transcripts}
							<div class="option-warning">
								<Icon name="warning" size={12} />
								<span>Transcripts can be very large files</span>
							</div>
						{/if}
					</div>
				</div>
			</div>

			<!-- CLI Tip -->
			<div class="cli-tip">
				<Icon name="terminal" size={14} />
				<span>
					Manual export: <code>orc export TASK-001</code> or <code>orc export TASK-001 --all</code>
				</span>
			</div>
		</div>
	{/if}
</div>

<style>
	.export-page {
		max-width: 700px;
	}

	.page-header {
		margin-bottom: var(--space-6);
	}

	.header-content {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
	}

	.header-content h1 {
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

	/* Config Sections */
	.config-sections {
		display: flex;
		flex-direction: column;
		gap: var(--space-5);
	}

	.config-section {
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		padding: var(--space-5);
	}

	.section-header {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		margin-bottom: var(--space-4);
	}

	.section-header h2 {
		font-size: var(--text-sm);
		font-weight: var(--font-semibold);
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		margin: 0;
	}

	.section-icon {
		width: 36px;
		height: 36px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--accent-subtle);
		color: var(--accent-primary);
		border-radius: var(--radius-md);
	}

	/* Toggle Row */
	.toggle-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--space-3);
		background: var(--bg-tertiary);
		border-radius: var(--radius-md);
	}

	.toggle-info {
		display: flex;
		flex-direction: column;
		gap: var(--space-0-5);
	}

	.toggle-info label {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
	}

	.hint {
		font-size: var(--text-xs);
		color: var(--text-muted);
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

	/* Preset Grid */
	.preset-grid {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: var(--space-3);
	}

	@media (max-width: 640px) {
		.preset-grid {
			grid-template-columns: repeat(2, 1fr);
		}
	}

	.preset-card {
		display: flex;
		flex-direction: column;
		align-items: center;
		padding: var(--space-3);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-md);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.preset-card:hover {
		border-color: var(--border-default);
		background: var(--bg-surface);
	}

	.preset-card.active {
		border-color: var(--accent-primary);
		background: var(--accent-subtle);
	}

	.preset-name {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
	}

	.preset-card.active .preset-name {
		color: var(--accent-primary);
	}

	.preset-desc {
		font-size: var(--text-xs);
		color: var(--text-muted);
		text-align: center;
		margin-top: var(--space-1);
	}

	/* Options List */
	.options-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
	}

	.option-row {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.checkbox-label {
		display: flex;
		align-items: flex-start;
		gap: var(--space-3);
		cursor: pointer;
		padding: var(--space-3);
		background: var(--bg-tertiary);
		border-radius: var(--radius-md);
		transition: background var(--duration-fast) var(--ease-out);
	}

	.checkbox-label:hover {
		background: var(--bg-surface);
	}

	.checkbox-label input {
		display: none;
	}

	.checkbox-box {
		width: 18px;
		height: 18px;
		border: 2px solid var(--border-default);
		border-radius: var(--radius-sm);
		flex-shrink: 0;
		margin-top: 2px;
		transition: all var(--duration-fast) var(--ease-out);
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.checkbox-label input:checked + .checkbox-box {
		background: var(--accent-primary);
		border-color: var(--accent-primary);
	}

	.checkbox-label input:checked + .checkbox-box::after {
		content: '';
		width: 5px;
		height: 9px;
		border: 2px solid white;
		border-top: none;
		border-left: none;
		transform: rotate(45deg) translateY(-1px);
	}

	.option-info {
		display: flex;
		flex-direction: column;
		gap: var(--space-0-5);
	}

	.option-name {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
	}

	.option-desc {
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.option-warning {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		margin-left: calc(18px + var(--space-3));
		padding: var(--space-2);
		background: rgba(245, 158, 11, 0.1);
		border-radius: var(--radius-sm);
		color: var(--status-warning);
		font-size: var(--text-xs);
	}

	/* CLI Tip */
	.cli-tip {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-3) var(--space-4);
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		color: var(--text-secondary);
	}

	.cli-tip code {
		font-family: var(--font-mono);
		background: var(--bg-tertiary);
		padding: 2px 6px;
		border-radius: var(--radius-sm);
		color: var(--text-primary);
	}
</style>
