<script lang="ts">
	import { onMount } from 'svelte';
	import Icon from '$lib/components/ui/Icon.svelte';
	import {
		listPlugins,
		getPlugin,
		enablePlugin,
		disablePlugin,
		uninstallPlugin,
		listPluginCommands,
		browseMarketplace,
		searchMarketplace,
		installPlugin,
		checkPluginUpdates,
		updatePlugin,
		type PluginInfo,
		type Plugin,
		type PluginCommand,
		type PluginScope,
		type MarketplacePlugin,
		type PluginUpdateInfo
	} from '$lib/api';

	// State (using Svelte 5 runes)
	let activeTab = $state<'installed' | 'marketplace'>('installed');
	let scope = $state<PluginScope | undefined>(undefined); // undefined = merged view
	let loading = $state(true);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let success = $state<string | null>(null);
	let restartNeeded = $state(false);

	// Installed plugins
	let plugins = $state<PluginInfo[]>([]);
	let selectedPlugin = $state<Plugin | null>(null);
	let pluginCommands = $state<PluginCommand[]>([]);
	let updates = $state<PluginUpdateInfo[]>([]);

	// Marketplace
	let marketplacePlugins = $state<MarketplacePlugin[]>([]);
	let marketplaceLoading = $state(false);
	let marketplaceError = $state<string | null>(null);
	let searchQuery = $state('');
	let marketplacePage = $state(1);
	let marketplaceTotal = $state(0);
	let selectedMarketplacePlugin = $state<MarketplacePlugin | null>(null);

	onMount(async () => {
		await loadPlugins();
	});

	async function loadPlugins() {
		loading = true;
		error = null;
		try {
			plugins = await listPlugins(scope);
			// Check for updates in background
			checkPluginUpdates().then((u) => (updates = u)).catch(() => {});
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load plugins';
		} finally {
			loading = false;
		}
	}

	async function selectPlugin(info: PluginInfo) {
		error = null;
		selectedMarketplacePlugin = null;
		try {
			selectedPlugin = await getPlugin(info.name, info.scope);
			pluginCommands = await listPluginCommands(info.name, info.scope);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load plugin details';
		}
	}

	async function togglePlugin(info: PluginInfo) {
		saving = true;
		error = null;
		try {
			const response = info.enabled
				? await disablePlugin(info.name, info.scope)
				: await enablePlugin(info.name, info.scope);

			if (response.requires_restart) {
				restartNeeded = true;
			}
			success = response.message || (info.enabled ? 'Plugin disabled' : 'Plugin enabled');
			await loadPlugins();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to toggle plugin';
		} finally {
			saving = false;
		}
	}

	async function handleUninstall() {
		if (!selectedPlugin) return;
		if (!confirm(`Uninstall plugin "${selectedPlugin.name}"? This cannot be undone.`)) return;

		saving = true;
		error = null;
		try {
			const response = await uninstallPlugin(selectedPlugin.name, selectedPlugin.scope);
			if (response.requires_restart) {
				restartNeeded = true;
			}
			success = response.message || 'Plugin uninstalled';
			selectedPlugin = null;
			await loadPlugins();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to uninstall plugin';
		} finally {
			saving = false;
		}
	}

	async function handleUpdate(info: PluginInfo) {
		saving = true;
		error = null;
		try {
			const response = await updatePlugin(info.name, info.scope);
			if (response.requires_restart) {
				restartNeeded = true;
			}
			success = response.message || 'Plugin updated';
			await loadPlugins();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to update plugin';
		} finally {
			saving = false;
		}
	}

	// Marketplace functions
	async function loadMarketplace() {
		marketplaceLoading = true;
		marketplaceError = null;
		try {
			const response = await browseMarketplace(marketplacePage, 20);
			marketplacePlugins = response.plugins;
			marketplaceTotal = response.total;
		} catch (e) {
			marketplaceError = e instanceof Error ? e.message : 'Marketplace unavailable';
		} finally {
			marketplaceLoading = false;
		}
	}

	async function handleSearch() {
		if (!searchQuery.trim()) {
			await loadMarketplace();
			return;
		}
		marketplaceLoading = true;
		marketplaceError = null;
		try {
			marketplacePlugins = await searchMarketplace(searchQuery);
			marketplaceTotal = marketplacePlugins.length;
		} catch (e) {
			marketplaceError = e instanceof Error ? e.message : 'Search failed';
		} finally {
			marketplaceLoading = false;
		}
	}

	function selectMarketplacePlugin(mp: MarketplacePlugin) {
		selectedPlugin = null;
		selectedMarketplacePlugin = mp;
	}

	async function handleInstall(mp: MarketplacePlugin, installScope: PluginScope = 'project') {
		saving = true;
		error = null;
		try {
			const response = await installPlugin(mp.name, installScope);
			if (response.requires_restart) {
				restartNeeded = true;
			}
			success = response.message || 'Plugin installed';
			selectedMarketplacePlugin = null;
			activeTab = 'installed';
			await loadPlugins();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to install plugin';
		} finally {
			saving = false;
		}
	}

	// Tab switching
	function switchTab(tab: 'installed' | 'marketplace') {
		activeTab = tab;
		error = null;
		success = null;
		if (tab === 'marketplace' && marketplacePlugins.length === 0) {
			loadMarketplace();
		}
	}

	// Scope filtering
	function changeScope(newScope: PluginScope | undefined) {
		scope = newScope;
		selectedPlugin = null;
		loadPlugins();
	}

	// Check if plugin has update available
	function hasUpdate(info: PluginInfo): PluginUpdateInfo | undefined {
		return updates.find((u) => u.name === info.name && u.scope === info.scope);
	}

	// Check if marketplace plugin is already installed
	function isInstalled(mp: MarketplacePlugin): boolean {
		return plugins.some((p) => p.name === mp.name);
	}
</script>

<svelte:head>
	<title>Plugins - orc</title>
</svelte:head>

<div class="plugins-page">
	<header class="page-header">
		<div class="header-content">
			<div>
				<h1>Claude Code Plugins</h1>
				<p class="subtitle">Manage plugins in .claude/plugins/</p>
			</div>
		</div>

		<!-- Tabs -->
		<div class="tabs">
			<button
				class="tab"
				class:active={activeTab === 'installed'}
				onclick={() => switchTab('installed')}
			>
				Installed
				{#if plugins.length > 0}
					<span class="badge">{plugins.length}</span>
				{/if}
			</button>
			<button
				class="tab"
				class:active={activeTab === 'marketplace'}
				onclick={() => switchTab('marketplace')}
			>
				Marketplace
			</button>
		</div>
	</header>

	{#if restartNeeded}
		<div class="alert alert-warning">
			<Icon name="warning" size={16} />
			Restart Claude Code to apply plugin changes.
		</div>
	{/if}

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if success}
		<div class="alert alert-success">{success}</div>
	{/if}

	{#if activeTab === 'installed'}
		<!-- Scope Filter -->
		<div class="scope-filter">
			<span class="filter-label">Scope:</span>
			<button class="scope-btn" class:active={scope === undefined} onclick={() => changeScope(undefined)}>
				All
			</button>
			<button class="scope-btn" class:active={scope === 'global'} onclick={() => changeScope('global')}>
				Global
			</button>
			<button class="scope-btn" class:active={scope === 'project'} onclick={() => changeScope('project')}>
				Project
			</button>
		</div>

		{#if loading}
			<div class="loading">Loading plugins...</div>
		{:else}
			<div class="plugins-layout">
				<!-- Plugin List -->
				<aside class="plugin-list">
					<h2>Plugins</h2>
					{#if plugins.length === 0}
						<p class="empty-message">No plugins installed</p>
						<button class="btn btn-link" onclick={() => switchTab('marketplace')}>
							Browse marketplace
						</button>
					{:else}
						<ul>
							{#each plugins as plugin}
								{@const update = hasUpdate(plugin)}
								<li>
									<button
										class="plugin-item"
										class:selected={selectedPlugin?.name === plugin.name &&
											selectedPlugin?.scope === plugin.scope}
										onclick={() => selectPlugin(plugin)}
									>
										<div class="plugin-main">
											<span class="plugin-name">{plugin.name}</span>
											<span class="plugin-scope">{plugin.scope}</span>
											{#if update}
												<span class="update-badge" title="Update available">
													{update.latest_version}
												</span>
											{/if}
										</div>
										{#if plugin.description}
											<span class="plugin-desc">{plugin.description}</span>
										{/if}
										<div class="plugin-meta">
											{#if plugin.has_commands}
												<span class="meta-tag">{plugin.command_count} commands</span>
											{/if}
										</div>
									</button>
									<label class="toggle-container" title={plugin.enabled ? 'Disable' : 'Enable'}>
										<input
											type="checkbox"
											checked={plugin.enabled}
											disabled={saving}
											onchange={() => togglePlugin(plugin)}
										/>
										<span class="toggle-slider"></span>
									</label>
								</li>
							{/each}
						</ul>
					{/if}
				</aside>

				<!-- Detail Panel -->
				<div class="detail-panel">
					{#if selectedPlugin}
						{@const pluginUpdate = updates.find(
							(u) => u.name === selectedPlugin?.name && u.scope === selectedPlugin?.scope
						)}
						<div class="detail-header">
							<div class="detail-title">
								<h2>{selectedPlugin.name}</h2>
								<span class="scope-tag">{selectedPlugin.scope}</span>
							</div>
							<div class="detail-actions">
								{#if pluginUpdate}
									<button
										class="btn btn-secondary"
										onclick={() =>
											handleUpdate({
												name: selectedPlugin!.name,
												scope: selectedPlugin!.scope
											} as PluginInfo)}
										disabled={saving}
									>
										Update to {pluginUpdate.latest_version}
									</button>
								{/if}
								<button class="btn btn-danger" onclick={handleUninstall} disabled={saving}>
									Uninstall
								</button>
							</div>
						</div>

						<div class="detail-content">
							<p class="description">{selectedPlugin.description}</p>

							{#if selectedPlugin.author}
								<div class="detail-row">
									<span class="label">Author</span>
									<span class="value">
										{selectedPlugin.author.name}
										{#if selectedPlugin.author.url}
											<a href={selectedPlugin.author.url} target="_blank" rel="noopener">
												(website)
											</a>
										{/if}
									</span>
								</div>
							{/if}

							{#if selectedPlugin.version}
								<div class="detail-row">
									<span class="label">Version</span>
									<span class="value">{selectedPlugin.version}</span>
								</div>
							{/if}

							{#if selectedPlugin.homepage}
								<div class="detail-row">
									<span class="label">Homepage</span>
									<a href={selectedPlugin.homepage} target="_blank" rel="noopener" class="value">
										{selectedPlugin.homepage}
									</a>
								</div>
							{/if}

							<div class="detail-row">
								<span class="label">Path</span>
								<span class="value mono">{selectedPlugin.path}</span>
							</div>

							{#if selectedPlugin.keywords && selectedPlugin.keywords.length > 0}
								<div class="detail-row">
									<span class="label">Keywords</span>
									<div class="keywords">
										{#each selectedPlugin.keywords as keyword}
											<span class="keyword">{keyword}</span>
										{/each}
									</div>
								</div>
							{/if}

							<!-- Commands Section -->
							{#if pluginCommands.length > 0}
								<div class="commands-section">
									<h3>Commands</h3>
									<ul class="commands-list">
										{#each pluginCommands as cmd}
											<li class="command-item">
												<code class="command-name">/{selectedPlugin.name}:{cmd.name}</code>
												<span class="command-desc">{cmd.description}</span>
												{#if cmd.argument_hint}
													<span class="command-hint">{cmd.argument_hint}</span>
												{/if}
											</li>
										{/each}
									</ul>
								</div>
							{/if}

							<div class="capabilities">
								<span class="cap" class:active={selectedPlugin.has_commands}>
									<Icon name="terminal" size={14} /> Commands
								</span>
								<span class="cap" class:active={selectedPlugin.has_hooks}>
									<Icon name="hooks" size={14} /> Hooks
								</span>
								<span class="cap" class:active={selectedPlugin.has_scripts}>
									<Icon name="scripts" size={14} /> Scripts
								</span>
							</div>
						</div>
					{:else}
						<div class="no-selection">
							<Icon name="plugin" size={48} />
							<p>Select a plugin to view details</p>
						</div>
					{/if}
				</div>
			</div>
		{/if}
	{:else}
		<!-- Marketplace Tab -->
		<div class="marketplace-header">
			<form class="search-form" onsubmit={(e) => { e.preventDefault(); handleSearch(); }}>
				<input
					type="text"
					placeholder="Search plugins..."
					bind:value={searchQuery}
					class="search-input"
				/>
				<button type="submit" class="btn btn-primary">Search</button>
			</form>
		</div>

		{#if marketplaceLoading}
			<div class="loading">Loading marketplace...</div>
		{:else if marketplaceError}
			<div class="marketplace-error">
				<Icon name="error" size={24} />
				<p>{marketplaceError}</p>
				<button class="btn btn-secondary" onclick={loadMarketplace}>Retry</button>
			</div>
		{:else}
			<div class="plugins-layout">
				<aside class="plugin-list">
					<h2>Available Plugins ({marketplaceTotal})</h2>
					{#if marketplacePlugins.length === 0}
						<p class="empty-message">No plugins found</p>
					{:else}
						<ul>
							{#each marketplacePlugins as mp}
								<li>
									<button
										class="plugin-item"
										class:selected={selectedMarketplacePlugin?.name === mp.name}
										onclick={() => selectMarketplacePlugin(mp)}
									>
										<div class="plugin-main">
											<span class="plugin-name">{mp.name}</span>
											<span class="plugin-version">v{mp.version}</span>
											{#if isInstalled(mp)}
												<span class="installed-badge">Installed</span>
											{/if}
										</div>
										{#if mp.description}
											<span class="plugin-desc">{mp.description}</span>
										{/if}
										{#if mp.downloads}
											<div class="plugin-meta">
												<span class="meta-tag">{mp.downloads.toLocaleString()} downloads</span>
											</div>
										{/if}
									</button>
								</li>
							{/each}
						</ul>
					{/if}
				</aside>

				<div class="detail-panel">
					{#if selectedMarketplacePlugin}
						<div class="detail-header">
							<div class="detail-title">
								<h2>{selectedMarketplacePlugin.name}</h2>
								<span class="version-tag">v{selectedMarketplacePlugin.version}</span>
							</div>
							<div class="detail-actions">
								{#if isInstalled(selectedMarketplacePlugin)}
									<span class="installed-label">Already installed</span>
								{:else}
									<button
										class="btn btn-primary"
										onclick={() => handleInstall(selectedMarketplacePlugin!, 'project')}
										disabled={saving}
									>
										Install (Project)
									</button>
									<button
										class="btn btn-secondary"
										onclick={() => handleInstall(selectedMarketplacePlugin!, 'global')}
										disabled={saving}
									>
										Install (Global)
									</button>
								{/if}
							</div>
						</div>

						<div class="detail-content">
							<p class="description">{selectedMarketplacePlugin.description}</p>

							{#if selectedMarketplacePlugin.author}
								<div class="detail-row">
									<span class="label">Author</span>
									<span class="value">
										{selectedMarketplacePlugin.author.name}
										{#if selectedMarketplacePlugin.author.url}
											<a
												href={selectedMarketplacePlugin.author.url}
												target="_blank"
												rel="noopener"
											>
												(website)
											</a>
										{/if}
									</span>
								</div>
							{/if}

							{#if selectedMarketplacePlugin.repository}
								<div class="detail-row">
									<span class="label">Repository</span>
									<a
										href={selectedMarketplacePlugin.repository}
										target="_blank"
										rel="noopener"
										class="value"
									>
										{selectedMarketplacePlugin.repository}
									</a>
								</div>
							{/if}

							{#if selectedMarketplacePlugin.keywords && selectedMarketplacePlugin.keywords.length > 0}
								<div class="detail-row">
									<span class="label">Keywords</span>
									<div class="keywords">
										{#each selectedMarketplacePlugin.keywords as keyword}
											<span class="keyword">{keyword}</span>
										{/each}
									</div>
								</div>
							{/if}
						</div>
					{:else}
						<div class="no-selection">
							<Icon name="plugin" size={48} />
							<p>Select a plugin to view details and install</p>
						</div>
					{/if}
				</div>
			</div>
		{/if}
	{/if}
</div>

<style>
	.plugins-page {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.page-header {
		display: flex;
		flex-direction: column;
		gap: 1rem;
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
		margin: 0.25rem 0 0;
		color: var(--text-secondary);
		font-size: 0.875rem;
	}

	/* Tabs */
	.tabs {
		display: flex;
		gap: 0.5rem;
		border-bottom: 1px solid var(--border-color);
		padding-bottom: 0;
	}

	.tab {
		padding: 0.5rem 1rem;
		background: none;
		border: none;
		border-bottom: 2px solid transparent;
		cursor: pointer;
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--text-secondary);
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.tab:hover {
		color: var(--text-primary);
	}

	.tab.active {
		color: var(--primary, #3b82f6);
		border-bottom-color: var(--primary, #3b82f6);
	}

	.badge {
		background: var(--bg-tertiary);
		padding: 0.125rem 0.5rem;
		border-radius: 10px;
		font-size: 0.75rem;
	}

	/* Scope Filter */
	.scope-filter {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.filter-label {
		font-size: 0.875rem;
		color: var(--text-secondary);
	}

	.scope-btn {
		padding: 0.25rem 0.75rem;
		background: var(--bg-secondary);
		border: 1px solid var(--border-color);
		border-radius: 4px;
		font-size: 0.75rem;
		cursor: pointer;
		color: var(--text-secondary);
	}

	.scope-btn:hover {
		background: var(--bg-tertiary);
	}

	.scope-btn.active {
		background: var(--primary, #3b82f6);
		border-color: var(--primary, #3b82f6);
		color: white;
	}

	/* Alerts */
	.alert {
		padding: 0.75rem 1rem;
		border-radius: 6px;
		font-size: 0.875rem;
		display: flex;
		align-items: center;
		gap: 0.5rem;
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

	.alert-warning {
		background: var(--warning-bg, #fef3c7);
		color: var(--warning-text, #d97706);
		border: 1px solid var(--warning-border, #fde68a);
	}

	.loading {
		text-align: center;
		padding: 3rem;
		color: var(--text-secondary);
	}

	/* Layout */
	.plugins-layout {
		display: grid;
		grid-template-columns: 300px 1fr;
		gap: 1.5rem;
		min-height: 500px;
	}

	/* Plugin List */
	.plugin-list {
		background: var(--bg-secondary);
		border-radius: 8px;
		padding: 1rem;
		border: 1px solid var(--border-color);
		overflow-y: auto;
		max-height: 600px;
	}

	.plugin-list h2 {
		font-size: 0.875rem;
		font-weight: 600;
		margin: 0 0 0.75rem;
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.plugin-list ul {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.plugin-list li {
		display: flex;
		align-items: flex-start;
		gap: 0.5rem;
	}

	.empty-message {
		color: var(--text-secondary);
		font-size: 0.875rem;
		font-style: italic;
		margin-bottom: 0.5rem;
	}

	.plugin-item {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		padding: 0.5rem 0.75rem;
		background: transparent;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		text-align: left;
		color: var(--text-primary);
		font-size: 0.875rem;
		gap: 0.25rem;
	}

	.plugin-item:hover {
		background: var(--bg-tertiary, rgba(0, 0, 0, 0.05));
	}

	.plugin-item.selected {
		background: var(--primary-bg, #dbeafe);
		color: var(--primary-text, #1d4ed8);
	}

	.plugin-main {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		flex-wrap: wrap;
	}

	.plugin-name {
		font-weight: 500;
	}

	.plugin-scope,
	.plugin-version {
		font-size: 0.7rem;
		padding: 0.125rem 0.375rem;
		background: var(--bg-tertiary);
		border-radius: 3px;
		color: var(--text-secondary);
	}

	.update-badge {
		font-size: 0.7rem;
		padding: 0.125rem 0.375rem;
		background: var(--warning-bg, #fef3c7);
		color: var(--warning-text, #d97706);
		border-radius: 3px;
	}

	.installed-badge {
		font-size: 0.7rem;
		padding: 0.125rem 0.375rem;
		background: var(--success-bg, #dcfce7);
		color: var(--success-text, #16a34a);
		border-radius: 3px;
	}

	.plugin-desc {
		font-size: 0.75rem;
		color: var(--text-secondary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		max-width: 100%;
	}

	.plugin-item.selected .plugin-desc {
		color: var(--primary-text, #1d4ed8);
		opacity: 0.7;
	}

	.plugin-meta {
		display: flex;
		gap: 0.5rem;
	}

	.meta-tag {
		font-size: 0.7rem;
		color: var(--text-secondary);
	}

	/* Toggle Switch */
	.toggle-container {
		position: relative;
		display: inline-block;
		width: 36px;
		height: 20px;
		flex-shrink: 0;
		margin-top: 0.5rem;
	}

	.toggle-container input {
		opacity: 0;
		width: 0;
		height: 0;
	}

	.toggle-slider {
		position: absolute;
		cursor: pointer;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background-color: var(--bg-tertiary);
		transition: 0.2s;
		border-radius: 20px;
	}

	.toggle-slider:before {
		position: absolute;
		content: '';
		height: 14px;
		width: 14px;
		left: 3px;
		bottom: 3px;
		background-color: white;
		transition: 0.2s;
		border-radius: 50%;
	}

	.toggle-container input:checked + .toggle-slider {
		background-color: var(--primary, #3b82f6);
	}

	.toggle-container input:checked + .toggle-slider:before {
		transform: translateX(16px);
	}

	.toggle-container input:disabled + .toggle-slider {
		opacity: 0.5;
		cursor: not-allowed;
	}

	/* Detail Panel */
	.detail-panel {
		display: flex;
		flex-direction: column;
		background: var(--bg-secondary);
		border-radius: 8px;
		border: 1px solid var(--border-color);
		overflow: hidden;
	}

	.detail-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 1rem;
		border-bottom: 1px solid var(--border-color);
		flex-wrap: wrap;
		gap: 0.5rem;
	}

	.detail-title {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.detail-title h2 {
		margin: 0;
		font-size: 1.125rem;
	}

	.scope-tag,
	.version-tag {
		font-size: 0.75rem;
		padding: 0.125rem 0.5rem;
		background: var(--bg-tertiary);
		border-radius: 4px;
		color: var(--text-secondary);
	}

	.detail-actions {
		display: flex;
		gap: 0.5rem;
	}

	.detail-content {
		padding: 1.5rem;
		display: flex;
		flex-direction: column;
		gap: 1rem;
		flex: 1;
		overflow-y: auto;
	}

	.description {
		margin: 0;
		color: var(--text-primary);
		line-height: 1.5;
	}

	.detail-row {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.detail-row .label {
		font-size: 0.75rem;
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.detail-row .value {
		font-size: 0.875rem;
		color: var(--text-primary);
	}

	.detail-row .value.mono {
		font-family: 'JetBrains Mono', 'Fira Code', monospace;
		font-size: 0.8rem;
	}

	.detail-row a {
		color: var(--primary, #3b82f6);
		text-decoration: none;
	}

	.detail-row a:hover {
		text-decoration: underline;
	}

	.keywords {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}

	.keyword {
		font-size: 0.75rem;
		padding: 0.125rem 0.5rem;
		background: var(--bg-tertiary);
		border-radius: 4px;
		color: var(--text-secondary);
	}

	/* Commands Section */
	.commands-section {
		margin-top: 1rem;
		padding-top: 1rem;
		border-top: 1px solid var(--border-color);
	}

	.commands-section h3 {
		font-size: 0.875rem;
		font-weight: 600;
		margin: 0 0 0.75rem;
		color: var(--text-secondary);
	}

	.commands-list {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	.command-item {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
		padding: 0.5rem;
		background: var(--bg-tertiary);
		border-radius: 6px;
	}

	.command-name {
		font-family: 'JetBrains Mono', 'Fira Code', monospace;
		font-size: 0.8rem;
		color: var(--primary, #3b82f6);
	}

	.command-desc {
		font-size: 0.8rem;
		color: var(--text-primary);
	}

	.command-hint {
		font-size: 0.75rem;
		color: var(--text-secondary);
		font-style: italic;
	}

	/* Capabilities */
	.capabilities {
		display: flex;
		gap: 1rem;
		margin-top: 1rem;
		padding-top: 1rem;
		border-top: 1px solid var(--border-color);
	}

	.cap {
		display: flex;
		align-items: center;
		gap: 0.375rem;
		font-size: 0.75rem;
		color: var(--text-secondary);
		opacity: 0.5;
	}

	.cap.active {
		opacity: 1;
		color: var(--success-text, #16a34a);
	}

	.no-selection {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100%;
		padding: 3rem;
		text-align: center;
		color: var(--text-secondary);
		gap: 1rem;
	}

	/* Marketplace */
	.marketplace-header {
		display: flex;
		gap: 1rem;
	}

	.search-form {
		display: flex;
		gap: 0.5rem;
		flex: 1;
		max-width: 500px;
	}

	.search-input {
		flex: 1;
		padding: 0.5rem 0.75rem;
		border: 1px solid var(--border-color);
		border-radius: 6px;
		font-size: 0.875rem;
		background: var(--bg-primary);
		color: var(--text-primary);
	}

	.search-input:focus {
		outline: none;
		border-color: var(--primary, #3b82f6);
	}

	.marketplace-error {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 3rem;
		color: var(--text-secondary);
		gap: 1rem;
	}

	.installed-label {
		font-size: 0.875rem;
		color: var(--success-text, #16a34a);
		padding: 0.5rem 1rem;
	}

	/* Buttons */
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
		background: var(--bg-tertiary);
		color: var(--text-primary);
		border: 1px solid var(--border-color);
	}

	.btn-secondary:hover:not(:disabled) {
		background: var(--bg-hover);
	}

	.btn-danger {
		background: var(--error-text, #dc2626);
		color: white;
	}

	.btn-danger:hover:not(:disabled) {
		background: #b91c1c;
	}

	.btn-link {
		background: none;
		border: none;
		color: var(--primary, #3b82f6);
		padding: 0;
		font-size: 0.875rem;
		cursor: pointer;
		text-decoration: underline;
	}

	@media (max-width: 768px) {
		.plugins-layout {
			grid-template-columns: 1fr;
		}

		.plugin-list {
			max-height: 250px;
		}

		.detail-header {
			flex-direction: column;
			align-items: flex-start;
		}

		.detail-actions {
			width: 100%;
			justify-content: flex-end;
		}
	}
</style>
