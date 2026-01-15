<script lang="ts">
	import { onMount } from 'svelte';
	import {
		listToolsByCategory,
		getToolPermissions,
		updateToolPermissions,
		type ToolsByCategory,
		type ToolPermissions
	} from '$lib/api';

	let toolsByCategory: ToolsByCategory = {};
	let permissions: ToolPermissions = { allow: [], deny: [] };
	let loading = true;
	let saving = false;
	let error: string | null = null;
	let success: string | null = null;

	onMount(async () => {
		try {
			[toolsByCategory, permissions] = await Promise.all([
				listToolsByCategory(),
				getToolPermissions()
			]);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load tools';
		} finally {
			loading = false;
		}
	});

	function getToolStatus(toolName: string): 'allow' | 'deny' | 'neutral' {
		if (permissions.allow?.includes(toolName)) return 'allow';
		if (permissions.deny?.includes(toolName)) return 'deny';
		return 'neutral';
	}

	function toggleTool(toolName: string) {
		const current = getToolStatus(toolName);

		// Remove from both lists first
		permissions.allow = permissions.allow?.filter((t) => t !== toolName) || [];
		permissions.deny = permissions.deny?.filter((t) => t !== toolName) || [];

		// Cycle: neutral -> allow -> deny -> neutral
		if (current === 'neutral') {
			permissions.allow = [...(permissions.allow || []), toolName];
		} else if (current === 'allow') {
			permissions.deny = [...(permissions.deny || []), toolName];
		}
		// deny -> neutral: already removed above

		// Trigger reactivity
		permissions = { ...permissions };
	}

	async function handleSave() {
		saving = true;
		error = null;
		success = null;

		try {
			await updateToolPermissions(permissions);
			success = 'Tool permissions saved successfully';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save permissions';
		} finally {
			saving = false;
		}
	}

	function clearAll() {
		permissions = { allow: [], deny: [] };
	}

	$: categories = Object.keys(toolsByCategory).sort();
	$: hasChanges =
		(permissions.allow?.length || 0) > 0 || (permissions.deny?.length || 0) > 0;
</script>

<svelte:head>
	<title>Tools - orc</title>
</svelte:head>

<div class="tools-page">
	<header class="page-header">
		<div class="header-content">
			<div>
				<h1>Tool Permissions</h1>
				<p class="subtitle">Configure allowed and denied tools for Claude Code</p>
			</div>
			<div class="header-actions">
				{#if hasChanges}
					<button class="btn btn-secondary" on:click={clearAll} disabled={saving}>
						Clear All
					</button>
				{/if}
				<button class="btn btn-primary" on:click={handleSave} disabled={saving}>
					{saving ? 'Saving...' : 'Save'}
				</button>
			</div>
		</div>
	</header>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if success}
		<div class="alert alert-success">{success}</div>
	{/if}

	<div class="permissions-summary">
		<div class="summary-item summary-allow">
			<span class="summary-count">{permissions.allow?.length || 0}</span>
			<span class="summary-label">Allowed</span>
		</div>
		<div class="summary-item summary-deny">
			<span class="summary-count">{permissions.deny?.length || 0}</span>
			<span class="summary-label">Denied</span>
		</div>
	</div>

	{#if loading}
		<div class="loading">Loading tools...</div>
	{:else}
		<div class="tools-grid">
			{#each categories as category}
				<div class="category-section">
					<h2 class="category-title">{category}</h2>
					<div class="tools-list">
						{#each toolsByCategory[category] as tool}
							{@const status = getToolStatus(tool.name)}
							<button
								class="tool-item"
								class:allow={status === 'allow'}
								class:deny={status === 'deny'}
								on:click={() => toggleTool(tool.name)}
							>
								<div class="tool-header">
									<span class="tool-name">{tool.name}</span>
									<span class="status-badge" class:allow={status === 'allow'} class:deny={status === 'deny'}>
										{#if status === 'allow'}
											Allowed
										{:else if status === 'deny'}
											Denied
										{:else}
											Default
										{/if}
									</span>
								</div>
								<p class="tool-description">{tool.description}</p>
							</button>
						{/each}
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<style>
	.tools-page {
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

	.header-actions {
		display: flex;
		gap: 0.5rem;
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

	.permissions-summary {
		display: flex;
		gap: 1rem;
	}

	.summary-item {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.75rem 1rem;
		border-radius: 8px;
		background: var(--bg-secondary);
		border: 1px solid var(--border-color);
	}

	.summary-count {
		font-size: 1.25rem;
		font-weight: 600;
	}

	.summary-label {
		font-size: 0.875rem;
		color: var(--text-secondary);
	}

	.summary-allow .summary-count {
		color: var(--success-text, #16a34a);
	}

	.summary-deny .summary-count {
		color: var(--error-text, #dc2626);
	}

	.loading {
		text-align: center;
		padding: 3rem;
		color: var(--text-secondary);
	}

	.tools-grid {
		display: flex;
		flex-direction: column;
		gap: 2rem;
	}

	.category-section {
		background: var(--bg-secondary);
		border-radius: 8px;
		padding: 1.5rem;
		border: 1px solid var(--border-color);
	}

	.category-title {
		font-size: 1rem;
		font-weight: 600;
		margin: 0 0 1rem;
		color: var(--text-primary);
		text-transform: capitalize;
	}

	.tools-list {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
		gap: 0.75rem;
	}

	.tool-item {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		width: 100%;
		padding: 0.75rem 1rem;
		background: var(--bg-primary);
		border: 2px solid var(--border-color);
		border-radius: 6px;
		cursor: pointer;
		text-align: left;
		color: var(--text-primary);
		font-size: 0.875rem;
		transition: border-color 0.15s, background-color 0.15s;
	}

	.tool-item:hover {
		background: var(--bg-tertiary, rgba(0, 0, 0, 0.02));
	}

	.tool-item.allow {
		border-color: var(--success-border, #bbf7d0);
		background: var(--success-bg, #dcfce7);
	}

	.tool-item.deny {
		border-color: var(--error-border, #fecaca);
		background: var(--error-bg, #fee2e2);
	}

	.tool-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		width: 100%;
		margin-bottom: 0.25rem;
	}

	.tool-name {
		font-weight: 600;
		font-family: 'JetBrains Mono', 'Fira Code', monospace;
	}

	.status-badge {
		font-size: 0.625rem;
		padding: 0.125rem 0.375rem;
		border-radius: 4px;
		text-transform: uppercase;
		font-weight: 600;
		background: var(--bg-tertiary, #e5e7eb);
		color: var(--text-secondary);
	}

	.status-badge.allow {
		background: var(--success-text, #16a34a);
		color: white;
	}

	.status-badge.deny {
		background: var(--error-text, #dc2626);
		color: white;
	}

	.tool-description {
		margin: 0;
		color: var(--text-secondary);
		font-size: 0.75rem;
		line-height: 1.4;
	}

	.tool-item.allow .tool-description,
	.tool-item.deny .tool-description {
		color: var(--text-primary);
		opacity: 0.8;
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

	.btn-secondary {
		background: var(--bg-secondary);
		color: var(--text-primary);
		border-color: var(--border-color);
	}

	.btn-secondary:hover:not(:disabled) {
		background: var(--bg-tertiary, #e5e7eb);
	}

	@media (max-width: 768px) {
		.header-content {
			flex-direction: column;
			gap: 1rem;
		}

		.permissions-summary {
			flex-direction: column;
		}

		.tools-list {
			grid-template-columns: 1fr;
		}
	}
</style>
