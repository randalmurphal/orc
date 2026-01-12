<script lang="ts">
	import { onMount } from 'svelte';
	import {
		getSettings,
		getGlobalSettings,
		getProjectSettings,
		updateSettings,
		getConfig,
		updateConfig,
		type Settings,
		type Config,
		type ConfigUpdateRequest
	} from '$lib/api';
	import Icon from '$lib/components/ui/Icon.svelte';

	// Settings state
	let mergedSettings: Settings | null = $state(null);
	let globalSettings: Settings | null = $state(null);
	let projectSettings: Settings | null = $state(null);
	let config: Config | null = $state(null);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state<string | null>(null);
	let success = $state<string | null>(null);

	// Active tab - default to quick for better discoverability
	let activeTab = $state<'config' | 'claude' | 'quick'>('quick');

	// Config search
	let configSearch = $state('');

	// Config form state
	let editProfile = $state('');
	let editGatesDefault = $state('');
	let editRetryEnabled = $state(false);
	let editRetryMax = $state(3);
	let editModel = $state('');
	let editMaxIterations = $state(10);
	let editTimeout = $state('');
	let editBranchPrefix = $state('');
	let editCommitPrefix = $state('');

	// Claude settings form state
	let envEntries = $state<{ key: string; value: string }[]>([]);
	let statusLineType = $state('');
	let statusLineCommand = $state('');
	let newEnvKey = $state('');
	let newEnvValue = $state('');

	const profiles = ['auto', 'fast', 'safe', 'strict'];
	const gateTypes = ['auto', 'human', 'ai'];

	const profileDescriptions: Record<string, string> = {
		auto: 'Fully automated, no human intervention',
		fast: 'Minimal gates, speed over safety',
		safe: 'AI reviews, human only for merge',
		strict: 'Human gates on spec/review/merge'
	};

	// Configuration documentation for inline help
	const configDocs: Record<string, { description: string; tip?: string; common?: boolean }> = {
		profile: {
			description: 'Controls how much automation vs human oversight',
			tip: 'Start with "auto" for experimentation, use "safe" or "strict" for production',
			common: true
		},
		gates_default: {
			description: 'Default approval type for phase transitions',
			tip: '"auto" proceeds automatically, "human" requires approval, "ai" uses AI review'
		},
		retry_enabled: {
			description: 'Automatically retry failed phases with feedback',
			tip: 'When tests fail, orc will retry implementation with the failure context',
			common: true
		},
		retry_max: {
			description: 'Maximum retry attempts before giving up'
		},
		model: {
			description: 'Claude model for task execution',
			tip: 'claude-sonnet-4 is recommended for most tasks',
			common: true
		},
		max_iterations: {
			description: 'Max Claude conversation turns per phase',
			tip: 'Increase for complex tasks that need more back-and-forth'
		},
		timeout: {
			description: 'Max duration per phase (e.g., "30m", "1h")',
			tip: 'Prevents runaway tasks'
		},
		branch_prefix: {
			description: 'Prefix for task branches',
			tip: 'e.g., "orc/" creates branches like "orc/TASK-001"'
		},
		commit_prefix: {
			description: 'Prefix for commit messages',
			tip: 'e.g., "[orc]" creates commits like "[orc] TASK-001: implement"'
		}
	};

	// Workflow recommendations for Getting Started
	const workflowGuides = [
		{
			title: 'Quick Start',
			description: 'Get started with minimal configuration',
			steps: ['Create task: orc new "description"', 'Run it: orc run TASK-001', 'Watch progress in web UI'],
			icon: 'play'
		},
		{
			title: 'Safe Mode',
			description: 'Add oversight for production work',
			steps: ['Set profile to "safe"', 'Configure prompts for your codebase', 'Enable review rounds'],
			icon: 'shield'
		},
		{
			title: 'Team Setup',
			description: 'Collaborate with activity tracking',
			steps: ['Enable task claiming', 'Configure shared database', 'Set visibility preferences'],
			icon: 'users'
		}
	];

	// Quick access items
	const quickAccessItems = [
		{ label: 'Prompts', href: '/prompts', icon: 'prompts', description: 'Phase prompt templates' },
		{ label: 'CLAUDE.md', href: '/claudemd', icon: 'file', description: 'Project instructions' },
		{ label: 'Skills', href: '/skills', icon: 'skills', description: 'Custom skills' },
		{ label: 'Hooks', href: '/hooks', icon: 'hooks', description: 'Event hooks' },
		{ label: 'MCP', href: '/mcp', icon: 'mcp', description: 'MCP servers' },
		{ label: 'Tools', href: '/tools', icon: 'tools', description: 'Tool permissions' },
		{ label: 'Agents', href: '/agents', icon: 'agents', description: 'Sub-agents' },
		{ label: 'Scripts', href: '/scripts', icon: 'scripts', description: 'Registered scripts' }
	];

	onMount(async () => {
		try {
			const [settingsRes, globalRes, projectRes, configRes] = await Promise.all([
				getSettings(),
				getGlobalSettings(),
				getProjectSettings(),
				getConfig()
			]);

			mergedSettings = settingsRes;
			globalSettings = globalRes;
			projectSettings = projectRes;
			config = configRes;

			// Initialize form state
			resetConfigForm();
			resetClaudeForm();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load settings';
		} finally {
			loading = false;
		}
	});

	function resetConfigForm() {
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

	function resetClaudeForm() {
		if (projectSettings?.env) {
			envEntries = Object.entries(projectSettings.env).map(([key, value]) => ({ key, value }));
		}
		if (projectSettings?.statusLine) {
			statusLineType = projectSettings.statusLine.type || '';
			statusLineCommand = projectSettings.statusLine.command || '';
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
			success = 'Configuration saved';
			setTimeout(() => (success = null), 3000);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save config';
		} finally {
			saving = false;
		}
	}

	async function saveClaudeSettings() {
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
			mergedSettings = await getSettings();
			success = 'Settings saved';
			setTimeout(() => (success = null), 3000);
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
	{#if success}
		<div class="toast success">
			<Icon name="check" size={16} />
			{success}
		</div>
	{/if}

	{#if error}
		<div class="toast error">
			<Icon name="close" size={16} />
			{error}
			<button class="toast-dismiss" onclick={() => (error = null)} aria-label="Dismiss">
				<Icon name="close" size={14} />
			</button>
		</div>
	{/if}

	<!-- Tabs -->
	<div class="tabs">
		<button
			class="tab"
			class:active={activeTab === 'config'}
			onclick={() => (activeTab = 'config')}
		>
			<Icon name="config" size={16} />
			<span>Orc Config</span>
		</button>
		<button
			class="tab"
			class:active={activeTab === 'claude'}
			onclick={() => (activeTab = 'claude')}
		>
			<Icon name="settings" size={16} />
			<span>Claude Settings</span>
		</button>
		<button
			class="tab"
			class:active={activeTab === 'quick'}
			onclick={() => (activeTab = 'quick')}
		>
			<Icon name="dashboard" size={16} />
			<span>Quick Access</span>
		</button>
	</div>

	{#if loading}
		<div class="loading-state">
			<div class="spinner"></div>
			<span>Loading settings...</span>
		</div>
	{:else}
		<!-- Config Tab -->
		{#if activeTab === 'config'}
			<div class="tab-content">
				<div class="content-header">
					<div>
						<h2>Orc Configuration</h2>
						<p class="subtitle">.orc/config.yaml - Task orchestration settings</p>
					</div>
					<div class="header-actions">
						<div class="search-box">
							<Icon name="search" size={14} />
							<input
								type="text"
								placeholder="Search settings..."
								bind:value={configSearch}
								class="search-input"
							/>
						</div>
						<button class="btn-primary" onclick={saveConfig} disabled={saving}>
							{saving ? 'Saving...' : 'Save'}
						</button>
					</div>
				</div>

				<div class="config-grid">
					<!-- Automation Section -->
					<div class="config-section" class:hidden={configSearch && !['profile', 'gate', 'retry', 'automation'].some(k => k.includes(configSearch.toLowerCase()))}>
						<div class="section-header">
							<div class="section-icon automation">
								<Icon name="settings" size={18} />
							</div>
							<h3>Automation</h3>
						</div>

						<div class="form-grid">
							<div class="form-group" class:highlight={configDocs.profile.common}>
								<div class="label-row">
									<label for="profile">Profile</label>
									{#if configDocs.profile.common}
										<span class="common-badge">Common</span>
									{/if}
								</div>
								<select id="profile" bind:value={editProfile}>
									{#each profiles as p}
										<option value={p}>{p}</option>
									{/each}
								</select>
								<span class="hint">{profileDescriptions[editProfile]}</span>
								{#if configDocs.profile.tip}
									<span class="tip"><Icon name="info" size={12} /> {configDocs.profile.tip}</span>
								{/if}
							</div>

							<div class="form-group">
								<div class="label-row">
									<label for="gates_default">Default Gate</label>
								</div>
								<select id="gates_default" bind:value={editGatesDefault}>
									{#each gateTypes as g}
										<option value={g}>{g}</option>
									{/each}
								</select>
								<span class="hint">{configDocs.gates_default.description}</span>
								{#if configDocs.gates_default.tip}
									<span class="tip"><Icon name="info" size={12} /> {configDocs.gates_default.tip}</span>
								{/if}
							</div>

							<div class="form-group toggle-row" class:highlight={configDocs.retry_enabled.common}>
								<div class="toggle-info">
									<div class="label-row">
										<label for="retry_enabled">Enable Retry</label>
										{#if configDocs.retry_enabled.common}
											<span class="common-badge">Common</span>
										{/if}
									</div>
									<span class="hint">{configDocs.retry_enabled.description}</span>
									{#if configDocs.retry_enabled.tip}
										<span class="tip"><Icon name="info" size={12} /> {configDocs.retry_enabled.tip}</span>
									{/if}
								</div>
								<label class="toggle">
									<input type="checkbox" id="retry_enabled" bind:checked={editRetryEnabled} />
									<span class="toggle-slider"></span>
								</label>
							</div>

							<div class="form-group">
								<label for="retry_max">Max Retries</label>
								<input type="number" id="retry_max" bind:value={editRetryMax} min="0" max="10" />
								<span class="hint">{configDocs.retry_max.description}</span>
							</div>
						</div>
					</div>

					<!-- Execution Section -->
					<div class="config-section" class:hidden={configSearch && !['model', 'iteration', 'timeout', 'execution'].some(k => k.includes(configSearch.toLowerCase()))}>
						<div class="section-header">
							<div class="section-icon execution">
								<Icon name="play" size={18} />
							</div>
							<h3>Execution</h3>
						</div>

						<div class="form-grid">
							<div class="form-group full-width" class:highlight={configDocs.model.common}>
								<div class="label-row">
									<label for="model">Model</label>
									{#if configDocs.model.common}
										<span class="common-badge">Common</span>
									{/if}
								</div>
								<input type="text" id="model" bind:value={editModel} placeholder="claude-sonnet-4-20250514" />
								<span class="hint">{configDocs.model.description}</span>
								{#if configDocs.model.tip}
									<span class="tip"><Icon name="info" size={12} /> {configDocs.model.tip}</span>
								{/if}
							</div>

							<div class="form-group">
								<label for="max_iterations">Max Iterations</label>
								<input type="number" id="max_iterations" bind:value={editMaxIterations} min="1" max="100" />
								<span class="hint">{configDocs.max_iterations.description}</span>
								{#if configDocs.max_iterations.tip}
									<span class="tip"><Icon name="info" size={12} /> {configDocs.max_iterations.tip}</span>
								{/if}
							</div>

							<div class="form-group">
								<label for="timeout">Timeout</label>
								<input type="text" id="timeout" bind:value={editTimeout} placeholder="30m" />
								<span class="hint">{configDocs.timeout.description}</span>
								{#if configDocs.timeout.tip}
									<span class="tip"><Icon name="info" size={12} /> {configDocs.timeout.tip}</span>
								{/if}
							</div>
						</div>
					</div>

					<!-- Git Section -->
					<div class="config-section" class:hidden={configSearch && !['git', 'branch', 'commit', 'prefix'].some(k => k.includes(configSearch.toLowerCase()))}>
						<div class="section-header">
							<div class="section-icon git">
								<Icon name="branch" size={18} />
							</div>
							<h3>Git</h3>
						</div>

						<div class="form-grid">
							<div class="form-group">
								<label for="branch_prefix">Branch Prefix</label>
								<input type="text" id="branch_prefix" bind:value={editBranchPrefix} placeholder="orc/" />
								<span class="hint">{configDocs.branch_prefix.description}</span>
								{#if configDocs.branch_prefix.tip}
									<span class="tip"><Icon name="info" size={12} /> {configDocs.branch_prefix.tip}</span>
								{/if}
							</div>

							<div class="form-group">
								<label for="commit_prefix">Commit Prefix</label>
								<input type="text" id="commit_prefix" bind:value={editCommitPrefix} placeholder="[orc]" />
								<span class="hint">{configDocs.commit_prefix.description}</span>
								{#if configDocs.commit_prefix.tip}
									<span class="tip"><Icon name="info" size={12} /> {configDocs.commit_prefix.tip}</span>
								{/if}
							</div>
						</div>
					</div>
				</div>

				<!-- CLI tip -->
				<div class="cli-tip">
					<Icon name="terminal" size={14} />
					<span>Run <code>orc config docs</code> for full configuration reference with all options</span>
				</div>
			</div>
		{/if}

		<!-- Claude Settings Tab -->
		{#if activeTab === 'claude'}
			<div class="tab-content">
				<div class="content-header">
					<div>
						<h2>Claude Settings</h2>
						<p class="subtitle">.claude/settings.json - Claude Code environment</p>
					</div>
					<button class="btn-primary" onclick={saveClaudeSettings} disabled={saving}>
						{saving ? 'Saving...' : 'Save'}
					</button>
				</div>

				<div class="config-grid">
					<!-- Environment Variables -->
					<div class="config-section full-width">
						<div class="section-header">
							<div class="section-icon env">
								<Icon name="config" size={18} />
							</div>
							<h3>Environment Variables</h3>
						</div>

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
									<button class="btn-icon btn-remove" onclick={() => removeEnvVar(index)} title="Remove">
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

					<!-- Status Line -->
					<div class="config-section">
						<div class="section-header">
							<div class="section-icon status">
								<Icon name="info" size={18} />
							</div>
							<h3>Status Line</h3>
						</div>

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

					<!-- Effective Settings Preview -->
					<div class="config-section">
						<div class="section-header">
							<div class="section-icon preview">
								<Icon name="file" size={18} />
							</div>
							<h3>Effective Settings</h3>
						</div>
						<div class="preview-content">
							<pre>{JSON.stringify(mergedSettings, null, 2)}</pre>
						</div>
					</div>
				</div>
			</div>
		{/if}

		<!-- Quick Access Tab -->
		{#if activeTab === 'quick'}
			<div class="tab-content">
				<div class="content-header">
					<div>
						<h2>Settings & Configuration</h2>
						<p class="subtitle">Configure orc and Claude for your workflow</p>
					</div>
				</div>

				<!-- Getting Started Guides -->
				<div class="guides-section">
					<h3 class="guides-title">Getting Started</h3>
					<div class="guides-grid">
						{#each workflowGuides as guide}
							<div class="guide-card">
								<div class="guide-icon">
									<Icon name={guide.icon} size={20} />
								</div>
								<div class="guide-content">
									<h4>{guide.title}</h4>
									<p>{guide.description}</p>
									<ul class="guide-steps">
										{#each guide.steps as step}
											<li>{step}</li>
										{/each}
									</ul>
								</div>
							</div>
						{/each}
					</div>
				</div>

				<!-- Configuration Pages -->
				<div class="config-pages-section">
					<h3 class="section-title">Configuration Pages</h3>
					<div class="quick-access-grid">
						{#each quickAccessItems as item}
							<a href={item.href} class="quick-card">
								<div class="quick-icon">
									<Icon name={item.icon} size={24} />
								</div>
								<div class="quick-info">
									<h4>{item.label}</h4>
									<p>{item.description}</p>
								</div>
								<div class="quick-arrow">
									<Icon name="chevron-right" size={16} />
								</div>
							</a>
						{/each}
					</div>
				</div>

				<!-- Keyboard Tip -->
				<div class="keyboard-tip">
					<Icon name="keyboard" size={14} />
					<span>Press <kbd>Cmd+K</kbd> to open command palette and jump to any page</span>
				</div>
			</div>
		{/if}
	{/if}
</div>

<style>
	.settings-page {
		max-width: 900px;
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
	}

	/* Tabs */
	.tabs {
		display: flex;
		gap: var(--space-1);
		background: var(--bg-secondary);
		padding: var(--space-1);
		border-radius: var(--radius-lg);
		margin-bottom: var(--space-5);
	}

	.tab {
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

	.tab:hover {
		color: var(--text-primary);
		background: var(--bg-tertiary);
	}

	.tab.active {
		background: var(--bg-primary);
		color: var(--accent-primary);
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
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

	/* Tab Content */
	.tab-content {
		animation: fadeIn var(--duration-fast) var(--ease-out);
	}

	@keyframes fadeIn {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}

	.content-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		margin-bottom: var(--space-5);
	}

	.content-header h2 {
		font-size: var(--text-lg);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		margin: 0;
	}

	.subtitle {
		font-size: var(--text-sm);
		color: var(--text-muted);
		margin: var(--space-1) 0 0;
	}

	.btn-primary {
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
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

	/* Config Grid */
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

	/* Config Section */
	.config-section {
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		padding: var(--space-5);
	}

	.config-section.full-width {
		grid-column: span 2;
	}

	@media (max-width: 768px) {
		.config-section.full-width {
			grid-column: span 1;
		}
	}

	.section-header {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		margin-bottom: var(--space-4);
	}

	.section-header h3 {
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
		border-radius: var(--radius-md);
	}

	.section-icon.automation {
		background: rgba(139, 92, 246, 0.15);
		color: var(--accent-primary);
	}

	.section-icon.execution {
		background: rgba(245, 158, 11, 0.15);
		color: var(--status-warning);
	}

	.section-icon.git {
		background: rgba(239, 68, 68, 0.15);
		color: #f87171;
	}

	.section-icon.env {
		background: rgba(16, 185, 129, 0.15);
		color: var(--status-success);
	}

	.section-icon.status {
		background: rgba(59, 130, 246, 0.15);
		color: #3b82f6;
	}

	.section-icon.preview {
		background: rgba(107, 114, 128, 0.15);
		color: var(--text-muted);
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

	.form-group.full-width {
		grid-column: span 2;
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

	.hint {
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	/* Toggle Row */
	.toggle-row {
		flex-direction: row;
		align-items: center;
		justify-content: space-between;
		grid-column: span 2;
		padding: var(--space-3);
		background: var(--bg-tertiary);
		border-radius: var(--radius-md);
	}

	.toggle-info {
		display: flex;
		flex-direction: column;
		gap: var(--space-0-5);
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

	/* Preview */
	.preview-content {
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		padding: var(--space-3);
		max-height: 200px;
		overflow: auto;
	}

	.preview-content pre {
		margin: 0;
		font-family: var(--font-mono);
		font-size: var(--text-xs);
		color: var(--text-primary);
		white-space: pre-wrap;
	}

	/* Quick Access Grid */
	.quick-access-grid {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: var(--space-4);
	}

	@media (max-width: 640px) {
		.quick-access-grid {
			grid-template-columns: 1fr;
		}
	}

	.quick-card {
		display: flex;
		align-items: center;
		gap: var(--space-4);
		padding: var(--space-4);
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		text-decoration: none;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.quick-card:hover {
		border-color: var(--accent-primary);
		background: var(--bg-tertiary);
		transform: translateY(-2px);
	}

	.quick-icon {
		width: 48px;
		height: 48px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--accent-subtle);
		color: var(--accent-primary);
		border-radius: var(--radius-md);
		flex-shrink: 0;
	}

	.quick-info {
		flex: 1;
		min-width: 0;
	}

	.quick-info h4 {
		font-size: var(--text-sm);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		margin: 0;
	}

	.quick-info p {
		font-size: var(--text-xs);
		color: var(--text-muted);
		margin: var(--space-1) 0 0;
	}

	.quick-arrow {
		color: var(--text-muted);
		opacity: 0;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.quick-card:hover .quick-arrow {
		opacity: 1;
		color: var(--accent-primary);
		transform: translateX(4px);
	}

	/* Header Actions */
	.header-actions {
		display: flex;
		align-items: center;
		gap: var(--space-3);
	}

	.search-box {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-muted);
	}

	.search-box:focus-within {
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 3px var(--accent-glow);
	}

	.search-box .search-input {
		border: none;
		background: transparent;
		outline: none;
		color: var(--text-primary);
		font-size: var(--text-sm);
		width: 140px;
	}

	.search-box .search-input::placeholder {
		color: var(--text-muted);
	}

	/* Label Row */
	.label-row {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	/* Common Badge */
	.common-badge {
		font-size: var(--text-2xs);
		font-weight: var(--font-medium);
		color: var(--accent-primary);
		background: var(--accent-subtle);
		padding: 1px 6px;
		border-radius: var(--radius-full);
	}

	/* Highlighted form groups */
	.form-group.highlight {
		position: relative;
	}

	.form-group.highlight::before {
		content: '';
		position: absolute;
		left: -8px;
		top: 0;
		bottom: 0;
		width: 3px;
		background: var(--accent-primary);
		border-radius: 2px;
	}

	/* Tip text */
	.tip {
		display: flex;
		align-items: flex-start;
		gap: var(--space-1-5);
		font-size: var(--text-xs);
		color: var(--accent-primary);
		margin-top: var(--space-1);
		padding: var(--space-2);
		background: var(--accent-subtle);
		border-radius: var(--radius-sm);
	}

	/* Hidden sections */
	.config-section.hidden {
		display: none;
	}

	/* CLI Tip */
	.cli-tip {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		margin-top: var(--space-5);
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

	/* Getting Started Guides */
	.guides-section {
		margin-bottom: var(--space-6);
	}

	.guides-title,
	.section-title {
		font-size: var(--text-sm);
		font-weight: var(--font-semibold);
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		margin-bottom: var(--space-4);
	}

	.guides-grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: var(--space-4);
	}

	@media (max-width: 900px) {
		.guides-grid {
			grid-template-columns: 1fr;
		}
	}

	.guide-card {
		display: flex;
		gap: var(--space-3);
		padding: var(--space-4);
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
	}

	.guide-icon {
		flex-shrink: 0;
		width: 36px;
		height: 36px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--accent-subtle);
		color: var(--accent-primary);
		border-radius: var(--radius-md);
	}

	.guide-content h4 {
		font-size: var(--text-sm);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		margin: 0 0 var(--space-1);
	}

	.guide-content > p {
		font-size: var(--text-xs);
		color: var(--text-muted);
		margin: 0 0 var(--space-3);
	}

	.guide-steps {
		list-style: none;
		padding: 0;
		margin: 0;
	}

	.guide-steps li {
		font-size: var(--text-xs);
		color: var(--text-secondary);
		padding: var(--space-1) 0;
		padding-left: var(--space-4);
		position: relative;
	}

	.guide-steps li::before {
		content: '';
		position: absolute;
		left: 0;
		top: 50%;
		transform: translateY(-50%);
		width: 6px;
		height: 6px;
		background: var(--accent-muted);
		border-radius: 50%;
	}

	/* Config Pages Section */
	.config-pages-section {
		margin-bottom: var(--space-6);
	}

	/* Keyboard Tip */
	.keyboard-tip {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-2);
		padding: var(--space-3);
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-md);
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.keyboard-tip kbd {
		padding: var(--space-0-5) var(--space-1-5);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-sm);
		font-family: var(--font-mono);
		font-size: var(--text-2xs);
	}
</style>
