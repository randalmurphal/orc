<script lang="ts">
	import { exportTask, getExportConfig, updateExportConfig, type ExportConfig } from '$lib/api';
	import { toast } from '$lib/stores/toast.svelte';
	import type { TaskStatus } from '$lib/types';

	let {
		taskId,
		taskStatus
	}: {
		taskId: string;
		taskStatus: TaskStatus;
	} = $props();

	let config = $state<ExportConfig | null>(null);
	let loading = $state(false);
	let exporting = $state(false);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let expanded = $state(false);

	// Export options (local state)
	let taskDefinition = $state(true);
	let finalState = $state(true);
	let contextSummary = $state(true);
	let transcripts = $state(false);
	let toBranch = $state(false);

	// Only show for completed tasks
	const shouldShow = $derived(taskStatus === 'completed');

	// Load config when shouldShow becomes true (reactive, handles mount and status changes)
	$effect(() => {
		if (shouldShow && !config && !loading) {
			loadConfig();
		}
	});

	async function loadConfig() {
		loading = true;
		error = null;

		try {
			config = await getExportConfig();
			// Initialize local state from config
			taskDefinition = config.task_definition;
			finalState = config.final_state;
			contextSummary = config.context_summary;
			transcripts = config.transcripts;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load export config';
		} finally {
			loading = false;
		}
	}

	async function handleExport() {
		exporting = true;
		error = null;

		try {
			const result = await exportTask(taskId, {
				task_definition: taskDefinition,
				final_state: finalState,
				context_summary: contextSummary,
				transcripts: transcripts,
				to_branch: toBranch
			});

			toast.success(`Exported to ${result.exported_to}`, { duration: 4000 });
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to export';
			toast.error(error, { duration: 5000 });
		} finally {
			exporting = false;
		}
	}

	async function handleSaveDefaults() {
		saving = true;
		error = null;

		try {
			await updateExportConfig({
				task_definition: taskDefinition,
				final_state: finalState,
				context_summary: contextSummary,
				transcripts: transcripts
			});

			toast.success('Export defaults saved', { duration: 3000 });
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save defaults';
			toast.error(error, { duration: 5000 });
		} finally {
			saving = false;
		}
	}

	function toggleExpanded() {
		expanded = !expanded;
	}
</script>

{#if shouldShow}
	<div class="export-panel">
		<button
			type="button"
			class="export-header"
			onclick={toggleExpanded}
			aria-expanded={expanded}
			aria-controls="export-panel-content"
		>
			<div class="header-content">
				<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
					<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
					<polyline points="17 8 12 3 7 8"/>
					<line x1="12" y1="3" x2="12" y2="15"/>
				</svg>
				<span>Export</span>
			</div>
			<svg class="chevron" class:expanded xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
				<polyline points="6 9 12 15 18 9"/>
			</svg>
		</button>

		{#if expanded}
			<div id="export-panel-content" class="export-content" role="region" aria-label="Export options">
				{#if loading}
					<div class="export-loading" role="status" aria-live="polite">
						<div class="spinner" aria-hidden="true"></div>
						<span>Loading export options...</span>
					</div>
				{:else if error}
					<div class="export-error" role="alert">
						<span class="error-icon" aria-hidden="true">!</span>
						<span>{error}</span>
						<button type="button" class="btn-retry" onclick={loadConfig}>Retry</button>
					</div>
				{:else}
					<div class="export-options">
						<label class="checkbox-label">
							<input type="checkbox" bind:checked={taskDefinition} />
							<span class="checkbox-text">
								<span class="option-name">Task definition</span>
								<span class="option-desc">task.yaml, plan.yaml</span>
							</span>
						</label>

						<label class="checkbox-label">
							<input type="checkbox" bind:checked={finalState} />
							<span class="checkbox-text">
								<span class="option-name">Final state</span>
								<span class="option-desc">state.yaml with execution results</span>
							</span>
						</label>

						<label class="checkbox-label">
							<input type="checkbox" bind:checked={contextSummary} />
							<span class="checkbox-text">
								<span class="option-name">Context summary</span>
								<span class="option-desc">context.md for PR reviewers</span>
							</span>
						</label>

						<label class="checkbox-label">
							<input type="checkbox" bind:checked={transcripts} />
							<span class="checkbox-text">
								<span class="option-name">Transcripts</span>
								<span class="option-desc">Full Claude conversation logs</span>
							</span>
						</label>

						<div class="divider"></div>

						<label class="checkbox-label">
							<input type="checkbox" bind:checked={toBranch} />
							<span class="checkbox-text">
								<span class="option-name">Export to branch</span>
								<span class="option-desc">Commit exports to task branch</span>
							</span>
						</label>
					</div>

					<div class="export-actions">
						<button type="button" class="btn-export" onclick={handleExport} disabled={exporting}>
							{#if exporting}
								<div class="spinner small" aria-hidden="true"></div>
								Exporting...
							{:else}
								<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
									<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
									<polyline points="17 8 12 3 7 8"/>
									<line x1="12" y1="3" x2="12" y2="15"/>
								</svg>
								Export Now
							{/if}
						</button>

						<button type="button" class="btn-save-defaults" onclick={handleSaveDefaults} disabled={saving}>
							{#if saving}
								<div class="spinner small" aria-hidden="true"></div>
							{:else}
								<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
									<path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/>
									<polyline points="17 21 17 13 7 13 7 21"/>
									<polyline points="7 3 7 8 15 8"/>
								</svg>
								Save as Default
							{/if}
						</button>
					</div>
				{/if}
			</div>
		{/if}
	</div>
{/if}

<style>
	.export-panel {
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		background: var(--bg-secondary);
		overflow: hidden;
	}

	.export-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		width: 100%;
		padding: var(--space-3) var(--space-4);
		background: transparent;
		border: none;
		cursor: pointer;
		transition: background 0.15s ease;
	}

	.export-header:hover {
		background: var(--bg-tertiary);
	}

	.header-content {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
	}

	.chevron {
		color: var(--text-muted);
		transition: transform 0.2s ease;
	}

	.chevron.expanded {
		transform: rotate(180deg);
	}

	.export-content {
		padding: var(--space-4);
		padding-top: 0;
	}

	.export-loading {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		font-size: var(--text-sm);
		color: var(--text-muted);
		padding: var(--space-2) 0;
	}

	.export-error {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		font-size: var(--text-sm);
		color: var(--status-danger);
		padding: var(--space-2) 0;
	}

	.error-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 16px;
		height: 16px;
		background: var(--status-danger);
		color: var(--text-on-danger, white);
		border-radius: 50%;
		font-size: 10px;
		font-weight: bold;
	}

	.btn-retry {
		padding: var(--space-1) var(--space-2);
		font-size: var(--text-xs);
		background: transparent;
		border: 1px solid currentColor;
		border-radius: var(--radius-sm);
		color: inherit;
		cursor: pointer;
	}

	.export-options {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.checkbox-label {
		display: flex;
		align-items: flex-start;
		gap: var(--space-2);
		cursor: pointer;
		padding: var(--space-1) 0;
	}

	.checkbox-label input[type="checkbox"] {
		margin-top: 2px;
		accent-color: var(--accent-primary);
	}

	.checkbox-text {
		display: flex;
		flex-direction: column;
		gap: 2px;
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

	.divider {
		height: 1px;
		background: var(--border-default);
		margin: var(--space-2) 0;
	}

	.export-actions {
		display: flex;
		gap: var(--space-2);
		margin-top: var(--space-4);
	}

	.btn-export {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		background: var(--accent-primary);
		border: none;
		border-radius: var(--radius-md);
		color: var(--text-on-accent, white);
		cursor: pointer;
		transition: background 0.15s ease;
	}

	.btn-export:hover:not(:disabled) {
		background: var(--accent-primary-hover);
	}

	.btn-export:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.btn-save-defaults {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		background: var(--bg-primary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-primary);
		cursor: pointer;
		transition: background 0.15s ease, border-color 0.15s ease;
	}

	.btn-save-defaults:hover:not(:disabled) {
		background: var(--bg-tertiary);
		border-color: var(--border-hover);
	}

	.btn-save-defaults:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.spinner {
		width: 16px;
		height: 16px;
		border: 2px solid var(--border-default);
		border-top-color: var(--accent-primary);
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	.spinner.small {
		width: 12px;
		height: 12px;
		border-width: 1.5px;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
