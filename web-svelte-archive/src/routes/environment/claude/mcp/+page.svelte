<script lang="ts">
	import { onMount } from 'svelte';
	import {
		listMCPServers,
		getMCPServer,
		createMCPServer,
		updateMCPServer,
		deleteMCPServer,
		getPluginResources,
		type MCPServerInfo,
		type MCPServer,
		type PluginMCPServerWithSource
	} from '$lib/api';

	let servers: MCPServerInfo[] = [];
	let pluginServers: PluginMCPServerWithSource[] = [];
	let selectedServer: MCPServer | null = null;
	let selectedPluginServer: PluginMCPServerWithSource | null = null;
	let isCreating = false;
	let loading = true;
	let saving = false;
	let error: string | null = null;
	let success: string | null = null;

	// Form fields
	let formName = '';
	let formType = 'stdio';
	let formCommand = '';
	let formArgs = '';
	let formUrl = '';
	let formHeaders = '';
	let formEnv: { key: string; value: string }[] = [];
	let formDisabled = false;

	onMount(async () => {
		try {
			const [serverList, resources] = await Promise.all([
				listMCPServers(),
				getPluginResources().catch(() => ({ mcp_servers: [], hooks: [], commands: [] }))
			]);
			servers = serverList;
			pluginServers = resources.mcp_servers;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load MCP servers';
		} finally {
			loading = false;
		}
	});

	function selectPluginServer(server: PluginMCPServerWithSource) {
		error = null;
		success = null;
		isCreating = false;
		selectedServer = null;
		selectedPluginServer = server;
	}

	async function selectServerByName(name: string) {
		error = null;
		success = null;
		isCreating = false;
		selectedPluginServer = null;

		try {
			selectedServer = await getMCPServer(name);
			formName = selectedServer.name;
			formType = selectedServer.type || 'stdio';
			formCommand = selectedServer.command || '';
			formArgs = selectedServer.args?.join('\n') || '';
			formUrl = selectedServer.url || '';
			formHeaders = selectedServer.headers?.join('\n') || '';
			formDisabled = selectedServer.disabled;

			// Convert env map to array
			if (selectedServer.env) {
				formEnv = Object.entries(selectedServer.env).map(([key, value]) => ({
					key,
					value
				}));
			} else {
				formEnv = [];
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load server';
		}
	}

	function startCreate() {
		error = null;
		success = null;
		selectedServer = null;
		selectedPluginServer = null;
		isCreating = true;

		formName = '';
		formType = 'stdio';
		formCommand = '';
		formArgs = '';
		formUrl = '';
		formHeaders = '';
		formEnv = [];
		formDisabled = false;
	}

	function addEnvVar() {
		formEnv = [...formEnv, { key: '', value: '' }];
	}

	function removeEnvVar(index: number) {
		formEnv = formEnv.filter((_, i) => i !== index);
	}

	async function handleSave() {
		if (!formName.trim()) {
			error = 'Server name is required';
			return;
		}

		if (formType === 'stdio' && !formCommand.trim()) {
			error = 'Command is required for stdio transport';
			return;
		}

		if ((formType === 'http' || formType === 'sse') && !formUrl.trim()) {
			error = 'URL is required for http/sse transport';
			return;
		}

		saving = true;
		error = null;
		success = null;

		// Parse args from newline-separated text
		const args = formArgs
			.split('\n')
			.map((a) => a.trim())
			.filter((a) => a);

		// Parse headers from newline-separated text
		const headers = formHeaders
			.split('\n')
			.map((h) => h.trim())
			.filter((h) => h);

		// Convert env array to map
		const env: Record<string, string> = {};
		for (const { key, value } of formEnv) {
			if (key.trim()) {
				env[key.trim()] = value;
			}
		}

		const serverData = {
			name: formName.trim(),
			type: formType,
			command: formType === 'stdio' ? formCommand.trim() : undefined,
			args: formType === 'stdio' && args.length > 0 ? args : undefined,
			url: formType !== 'stdio' ? formUrl.trim() : undefined,
			headers: formType !== 'stdio' && headers.length > 0 ? headers : undefined,
			env: Object.keys(env).length > 0 ? env : undefined,
			disabled: formDisabled
		};

		try {
			if (isCreating) {
				await createMCPServer(serverData);
				success = 'MCP server created successfully';
			} else if (selectedServer) {
				await updateMCPServer(selectedServer.name, serverData);
				success = 'MCP server updated successfully';
			}

			servers = await listMCPServers();
			isCreating = false;

			// Refresh selected server
			if (formName) {
				selectedServer = await getMCPServer(formName);
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save server';
		} finally {
			saving = false;
		}
	}

	async function handleDelete() {
		if (!selectedServer) return;

		if (!confirm(`Delete MCP server "${selectedServer.name}"?`)) return;

		saving = true;
		error = null;
		success = null;

		try {
			await deleteMCPServer(selectedServer.name);
			servers = await listMCPServers();
			selectedServer = null;
			isCreating = false;
			success = 'MCP server deleted successfully';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete server';
		} finally {
			saving = false;
		}
	}

	function getTypeIcon(type: string): string {
		switch (type) {
			case 'stdio':
				return '>';
			case 'http':
				return 'H';
			case 'sse':
				return 'S';
			default:
				return '?';
		}
	}
</script>

<svelte:head>
	<title>MCP Servers - orc</title>
</svelte:head>

<div class="mcp-page">
	<header class="page-header">
		<div class="header-content">
			<div>
				<h1>MCP Servers</h1>
				<p class="subtitle">Configure Model Context Protocol servers for Claude Code</p>
			</div>
			<button class="btn btn-primary" onclick={startCreate}>New Server</button>
		</div>
	</header>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if success}
		<div class="alert alert-success">{success}</div>
	{/if}

	{#if loading}
		<div class="loading">Loading MCP servers...</div>
	{:else}
		<div class="mcp-layout">
			<!-- Server List -->
			<aside class="server-list">
				<h2>Servers</h2>
				{#if servers.length === 0 && pluginServers.length === 0}
					<p class="empty-message">No MCP servers configured</p>
				{:else}
					{#if servers.length > 0}
						<ul>
							{#each servers as server}
								<li>
									<button
										class="server-item"
										class:selected={selectedServer?.name === server.name}
										class:disabled={server.disabled}
										onclick={() => selectServerByName(server.name)}
									>
										<div class="server-header">
											<span class="type-badge" title={server.type}>
												{getTypeIcon(server.type)}
											</span>
											<span class="server-name">{server.name}</span>
										</div>
										<span class="server-detail">
											{#if server.type === 'stdio'}
												{server.command}
											{:else}
												{server.url}
											{/if}
										</span>
									</button>
								</li>
							{/each}
						</ul>
					{/if}

					{#if pluginServers.length > 0}
						<div class="plugin-section">
							<h3>From Plugins</h3>
							<ul>
								{#each pluginServers as server}
									<li>
										<button
											class="server-item plugin-item"
											class:selected={selectedPluginServer?.name === server.name && selectedPluginServer?.plugin_name === server.plugin_name}
											onclick={() => selectPluginServer(server)}
										>
											<div class="server-header">
												<span class="type-badge" title={server.type || 'stdio'}>
													{getTypeIcon(server.type || 'stdio')}
												</span>
												<span class="server-name">{server.name}</span>
											</div>
											<span class="server-detail plugin-source">
												via {server.plugin_name}
											</span>
										</button>
									</li>
								{/each}
							</ul>
						</div>
					{/if}
				{/if}
			</aside>

			<!-- Editor Panel -->
			<div class="editor-panel">
				{#if selectedPluginServer}
					<!-- Read-only view for plugin servers -->
					<div class="editor-header">
						<h2>{selectedPluginServer.name}</h2>
						<span class="plugin-badge">From plugin: {selectedPluginServer.plugin_name}</span>
					</div>

					<div class="plugin-server-details">
						<div class="detail-group">
							<span class="detail-label">Transport Type</span>
							<span class="detail-value">{selectedPluginServer.type || 'stdio'}</span>
						</div>

						{#if selectedPluginServer.command}
							<div class="detail-group">
								<span class="detail-label">Command</span>
								<code class="detail-value">{selectedPluginServer.command}</code>
							</div>
						{/if}

						{#if selectedPluginServer.args && selectedPluginServer.args.length > 0}
							<div class="detail-group">
								<span class="detail-label">Arguments</span>
								<code class="detail-value">{selectedPluginServer.args.join(' ')}</code>
							</div>
						{/if}

						{#if selectedPluginServer.url}
							<div class="detail-group">
								<span class="detail-label">URL</span>
								<code class="detail-value">{selectedPluginServer.url}</code>
							</div>
						{/if}

						{#if selectedPluginServer.env && Object.keys(selectedPluginServer.env).length > 0}
							<div class="detail-group">
								<span class="detail-label">Environment Variables</span>
								<div class="env-display">
									{#each Object.entries(selectedPluginServer.env) as [key, value]}
										<div class="env-item">
											<code>{key}</code> = <code>{value}</code>
										</div>
									{/each}
								</div>
							</div>
						{/if}

						<div class="plugin-notice">
							<p>This MCP server is provided by the <strong>{selectedPluginServer.plugin_name}</strong> plugin.</p>
							<p>To modify or remove it, manage the plugin in the <a href="/environment/claude/plugins">Plugins</a> section.</p>
						</div>
					</div>
				{:else if selectedServer || isCreating}
					<div class="editor-header">
						<h2>{isCreating ? 'New MCP Server' : selectedServer?.name}</h2>
						{#if selectedServer && !isCreating}
							<button class="btn btn-danger" onclick={handleDelete} disabled={saving}>
								Delete
							</button>
						{/if}
					</div>

					<form class="server-form" onsubmit={(e) => { e.preventDefault(); handleSave(); }}>
						<div class="form-row">
							<div class="form-group">
								<label for="name">Server Name</label>
								<input
									id="name"
									type="text"
									bind:value={formName}
									placeholder="my-mcp-server"
									disabled={!isCreating}
								/>
							</div>

							<div class="form-group">
								<label for="type">Transport Type</label>
								<select id="type" bind:value={formType}>
									<option value="stdio">stdio (local command)</option>
									<option value="http">http (REST endpoint)</option>
									<option value="sse">sse (Server-Sent Events)</option>
								</select>
							</div>
						</div>

						{#if formType === 'stdio'}
							<div class="form-group">
								<label for="command">Command</label>
								<input
									id="command"
									type="text"
									bind:value={formCommand}
									placeholder="npx"
								/>
								<span class="form-hint">Executable to run (e.g., npx, python, node)</span>
							</div>

							<div class="form-group">
								<label for="args">Arguments</label>
								<textarea
									id="args"
									bind:value={formArgs}
									placeholder="-y&#10;@modelcontextprotocol/server-github"
									rows="4"
								></textarea>
								<span class="form-hint">One argument per line</span>
							</div>
						{:else}
							<div class="form-group">
								<label for="url">URL</label>
								<input
									id="url"
									type="text"
									bind:value={formUrl}
									placeholder="https://example.com/mcp"
								/>
								<span class="form-hint">
									{formType === 'http' ? 'HTTP endpoint for MCP requests' : 'SSE endpoint URL'}
								</span>
							</div>

							<div class="form-group">
								<label for="headers">HTTP Headers</label>
								<textarea
									id="headers"
									bind:value={formHeaders}
									placeholder="Authorization: Bearer $&#123;TOKEN&#125;"
									rows="3"
								></textarea>
								<span class="form-hint">One header per line. Use $&#123;VAR&#125; for env vars</span>
							</div>
						{/if}

						<div class="form-group">
							<div class="env-header">
								<span class="form-label">Environment Variables</span>
								<button type="button" class="btn btn-sm" onclick={addEnvVar}>
									+ Add Variable
								</button>
							</div>
							{#if formEnv.length === 0}
								<p class="empty-env">No environment variables configured</p>
							{:else}
								<div class="env-list">
									{#each formEnv as envVar, i}
										<div class="env-row">
											<input
												type="text"
												bind:value={envVar.key}
												placeholder="KEY"
												class="env-key"
											/>
											<input
												type="text"
												bind:value={envVar.value}
												placeholder={'value or ${VAR}'}
												class="env-value"
											/>
											<button
												type="button"
												class="btn btn-icon"
												onclick={() => removeEnvVar(i)}
												title="Remove"
											>
												x
											</button>
										</div>
									{/each}
								</div>
							{/if}
							<span class="form-hint">
								Use $&#123;VAR&#125; or $&#123;VAR:-default&#125; to reference system env vars
							</span>
						</div>

						<div class="form-group">
							<label class="checkbox-label">
								<input type="checkbox" bind:checked={formDisabled} />
								<span>Disabled</span>
							</label>
							<span class="form-hint">
								Disabled servers are not loaded by Claude Code
							</span>
						</div>

						<div class="form-actions">
							<button type="submit" class="btn btn-primary" disabled={saving}>
								{saving ? 'Saving...' : isCreating ? 'Create' : 'Update'}
							</button>
						</div>
					</form>
				{:else}
					<div class="no-selection">
						<p>Select an MCP server from the list or create a new one.</p>
						<div class="hint">
							<p>MCP servers extend Claude Code with additional tools:</p>
							<ul>
								<li><strong>stdio</strong> - Local servers via command execution</li>
								<li><strong>http</strong> - Remote servers via HTTP requests</li>
								<li><strong>sse</strong> - Remote servers via Server-Sent Events</li>
							</ul>
						</div>
					</div>
				{/if}
			</div>
		</div>
	{/if}
</div>

<style>
	.mcp-page {
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

	.mcp-layout {
		display: grid;
		grid-template-columns: 280px 1fr;
		gap: 1.5rem;
		min-height: 600px;
	}

	/* Server List */
	.server-list {
		background: var(--bg-secondary);
		border-radius: 8px;
		padding: 1rem;
		border: 1px solid var(--border-color);
	}

	.server-list h2 {
		font-size: 0.875rem;
		font-weight: 600;
		margin: 0 0 0.75rem;
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.server-list ul {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.empty-message {
		color: var(--text-secondary);
		font-size: 0.875rem;
		font-style: italic;
	}

	.server-item {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		width: 100%;
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

	.server-item:hover {
		background: var(--bg-tertiary, rgba(0, 0, 0, 0.05));
	}

	.server-item.selected {
		background: var(--primary-bg, #dbeafe);
		color: var(--primary-text, #1d4ed8);
	}

	.server-item.disabled {
		opacity: 0.5;
	}

	.server-header {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.type-badge {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 20px;
		height: 20px;
		font-size: 0.75rem;
		font-weight: 600;
		background: var(--bg-tertiary, rgba(0, 0, 0, 0.1));
		border-radius: 4px;
		font-family: monospace;
	}

	.server-name {
		font-weight: 500;
	}

	.server-detail {
		font-size: 0.75rem;
		color: var(--text-secondary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		max-width: 100%;
	}

	.server-item.selected .server-detail {
		color: var(--primary-text, #1d4ed8);
		opacity: 0.7;
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

	.server-form {
		padding: 1.5rem;
		display: flex;
		flex-direction: column;
		gap: 1rem;
		overflow-y: auto;
	}

	.form-row {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 1rem;
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.form-group label,
	.form-label {
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--text-primary);
	}

	.form-group input,
	.form-group select,
	.form-group textarea {
		padding: 0.5rem 0.75rem;
		border: 1px solid var(--border-color);
		border-radius: 6px;
		font-size: 0.875rem;
		background: var(--bg-primary);
		color: var(--text-primary);
	}

	.form-group textarea {
		font-family: 'JetBrains Mono', 'Fira Code', monospace;
		resize: vertical;
	}

	.form-group input:focus,
	.form-group select:focus,
	.form-group textarea:focus {
		outline: none;
		border-color: var(--primary, #3b82f6);
	}

	.form-hint {
		font-size: 0.75rem;
		color: var(--text-secondary);
	}

	.env-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.env-list {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.env-row {
		display: grid;
		grid-template-columns: 1fr 2fr auto;
		gap: 0.5rem;
		align-items: center;
	}

	.env-key,
	.env-value {
		padding: 0.5rem 0.75rem;
		border: 1px solid var(--border-color);
		border-radius: 6px;
		font-size: 0.875rem;
		background: var(--bg-primary);
		color: var(--text-primary);
		font-family: 'JetBrains Mono', 'Fira Code', monospace;
	}

	.empty-env {
		color: var(--text-secondary);
		font-size: 0.875rem;
		font-style: italic;
		margin: 0;
	}

	.checkbox-label {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		cursor: pointer;
	}

	.checkbox-label input {
		width: 16px;
		height: 16px;
	}

	.form-actions {
		padding-top: 0.5rem;
	}

	.btn {
		padding: 0.5rem 1rem;
		border-radius: 6px;
		font-size: 0.875rem;
		font-weight: 500;
		cursor: pointer;
		border: 1px solid transparent;
	}

	.btn-sm {
		padding: 0.375rem 0.75rem;
		font-size: 0.75rem;
	}

	.btn-icon {
		padding: 0.375rem 0.5rem;
		background: transparent;
		border: 1px solid var(--border-color);
		color: var(--text-secondary);
	}

	.btn-icon:hover {
		background: var(--error-bg, #fee2e2);
		color: var(--error-text, #dc2626);
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

	.btn-danger {
		background: var(--error-text, #dc2626);
		color: white;
	}

	.btn-danger:hover:not(:disabled) {
		background: #b91c1c;
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
		gap: 1.5rem;
	}

	.no-selection .hint {
		font-size: 0.875rem;
		text-align: left;
	}

	.no-selection .hint ul {
		margin: 0.5rem 0 0;
		padding-left: 1.25rem;
	}

	.no-selection .hint li {
		margin: 0.25rem 0;
	}

	/* Plugin section styles */
	.plugin-section {
		margin-top: 1rem;
		padding-top: 1rem;
		border-top: 1px solid var(--border-color);
	}

	.plugin-section h3 {
		font-size: 0.75rem;
		font-weight: 600;
		margin: 0 0 0.5rem;
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.plugin-item {
		opacity: 0.85;
	}

	.plugin-source {
		font-style: italic;
		color: var(--text-tertiary, #9ca3af);
	}

	.plugin-badge {
		font-size: 0.75rem;
		padding: 0.25rem 0.5rem;
		background: var(--primary-bg, #dbeafe);
		color: var(--primary-text, #1d4ed8);
		border-radius: 4px;
	}

	.plugin-server-details {
		padding: 1.5rem;
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.detail-group {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.detail-label {
		font-size: 0.75rem;
		font-weight: 600;
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.detail-value {
		font-size: 0.875rem;
		color: var(--text-primary);
	}

	code.detail-value {
		font-family: 'JetBrains Mono', 'Fira Code', monospace;
		background: var(--bg-tertiary, rgba(0, 0, 0, 0.05));
		padding: 0.25rem 0.5rem;
		border-radius: 4px;
	}

	.env-display {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.env-item code {
		font-family: 'JetBrains Mono', 'Fira Code', monospace;
		font-size: 0.75rem;
		background: var(--bg-tertiary, rgba(0, 0, 0, 0.05));
		padding: 0.125rem 0.375rem;
		border-radius: 3px;
	}

	.plugin-notice {
		margin-top: 1rem;
		padding: 1rem;
		background: var(--bg-tertiary, rgba(0, 0, 0, 0.03));
		border-radius: 6px;
		font-size: 0.875rem;
		color: var(--text-secondary);
	}

	.plugin-notice p {
		margin: 0 0 0.5rem;
	}

	.plugin-notice p:last-child {
		margin-bottom: 0;
	}

	.plugin-notice a {
		color: var(--primary, #3b82f6);
		text-decoration: none;
	}

	.plugin-notice a:hover {
		text-decoration: underline;
	}

	@media (max-width: 768px) {
		.mcp-layout {
			grid-template-columns: 1fr;
		}

		.server-list {
			max-height: 200px;
			overflow-y: auto;
		}

		.form-row {
			grid-template-columns: 1fr;
		}

		.env-row {
			grid-template-columns: 1fr 1fr auto;
		}
	}
</style>
