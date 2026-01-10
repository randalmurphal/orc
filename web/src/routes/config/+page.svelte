<script lang="ts">
	import { onMount } from 'svelte';

	interface Config {
		version: string;
		profile: string;
		automation: {
			profile: string;
			gates_default: string;
			retry_enabled: boolean;
			retry_max: number;
		};
		execution: {
			model: string;
			max_iterations: number;
			timeout: string;
		};
		git: {
			branch_prefix: string;
			commit_prefix: string;
		};
	}

	let config = $state<Config | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	onMount(async () => {
		try {
			const res = await fetch('/api/config');
			if (!res.ok) throw new Error('Failed to load config');
			config = await res.json();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load config';
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>Config - orc</title>
</svelte:head>

<div class="page">
	<header class="page-header">
		<h1>Configuration</h1>
	</header>

	{#if loading}
		<div class="loading">Loading configuration...</div>
	{:else if error}
		<div class="error">{error}</div>
	{:else if config}
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
</div>

<style>
	.page {
		max-width: 700px;
	}

	.page-header {
		margin-bottom: 2rem;
	}

	h1 {
		font-size: 1.5rem;
		font-weight: 600;
	}

	.config-sections {
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

	.loading, .error {
		text-align: center;
		padding: 3rem;
		color: var(--text-secondary);
	}

	.error {
		color: var(--accent-danger);
	}
</style>
