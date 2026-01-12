<script lang="ts">
	import { onMount } from 'svelte';
	import Icon from '$lib/components/ui/Icon.svelte';
	import {
		listSkills,
		listHooks,
		listAgents,
		listMCPServers,
		listTools,
		getToolPermissions,
		getConfig,
		listPrompts,
		listScripts,
		getClaudeMDHierarchy,
		type SkillInfo,
		type Hook,
		type SubAgent,
		type MCPServer,
		type ToolInfo,
		type ToolPermissions,
		type Config,
		type PromptInfo,
		type ProjectScript,
		type ClaudeMDHierarchy
	} from '$lib/api';

	// State
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Data
	let skills = $state<SkillInfo[]>([]);
	let hooks = $state<Record<string, Hook[]>>({});
	let agents = $state<SubAgent[]>([]);
	let mcpServers = $state<MCPServer[]>([]);
	let tools = $state<ToolInfo[]>([]);
	let toolPermissions = $state<ToolPermissions | null>(null);
	let config = $state<Config | null>(null);
	let prompts = $state<PromptInfo[]>([]);
	let scripts = $state<ProjectScript[]>([]);
	let claudeMD = $state<ClaudeMDHierarchy | null>(null);

	// Expanded sections
	let expandedSections = $state<Set<string>>(new Set());

	function toggleSection(section: string) {
		if (expandedSections.has(section)) {
			expandedSections.delete(section);
		} else {
			expandedSections.add(section);
		}
		expandedSections = new Set(expandedSections);
	}

	onMount(async () => {
		try {
			const [
				skillsRes,
				hooksRes,
				agentsRes,
				mcpRes,
				toolsRes,
				permissionsRes,
				configRes,
				promptsRes,
				scriptsRes,
				claudeMDRes
			] = await Promise.all([
				listSkills().catch(() => []),
				listHooks().catch(() => ({})),
				listAgents().catch(() => []),
				listMCPServers().catch(() => []),
				listTools().catch(() => []),
				getToolPermissions().catch(() => null),
				getConfig().catch(() => null),
				listPrompts().catch(() => []),
				listScripts().catch(() => []),
				getClaudeMDHierarchy().catch(() => null)
			]);

			skills = skillsRes;
			hooks = hooksRes;
			agents = agentsRes;
			mcpServers = mcpRes;
			tools = toolsRes;
			toolPermissions = permissionsRes;
			config = configRes;
			prompts = promptsRes;
			scripts = scriptsRes;
			claudeMD = claudeMDRes;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load environment data';
		} finally {
			loading = false;
		}
	});

	// Computed stats
	const hookCount = $derived(Object.values(hooks).flat().length);
	const activeHookEvents = $derived(Object.keys(hooks).length);
	const mcpConnected = $derived(mcpServers.filter((s) => !s.disabled).length);
	const deniedToolsCount = $derived(toolPermissions?.deny?.length || 0);
	const overriddenPrompts = $derived(prompts.filter((p) => p.has_override).length);
	const claudeMDLevels = $derived(
		claudeMD
			? [claudeMD.global, claudeMD.user, claudeMD.project].filter((c) => c?.content).length
			: 0
	);

	// Status helpers
	function getClaudeCodeStatus(): { status: 'healthy' | 'warning' | 'issues'; message: string } {
		const issues = [];
		if (mcpServers.some((s) => s.disabled)) issues.push('disabled MCP servers');
		if (deniedToolsCount > 0) issues.push(`${deniedToolsCount} denied tools`);

		if (issues.length > 0) {
			return { status: 'warning', message: issues.join(', ') };
		}
		return { status: 'healthy', message: 'All systems operational' };
	}

	function getOrchestratorStatus(): { status: 'healthy' | 'warning' | 'issues'; message: string } {
		const issues = [];
		if (overriddenPrompts > 0) issues.push(`${overriddenPrompts} overridden prompts`);

		if (issues.length > 0) {
			return { status: 'warning', message: issues.join(', ') };
		}
		return { status: 'healthy', message: 'Default configuration' };
	}

	const claudeStatus = $derived(getClaudeCodeStatus());
	const orcStatus = $derived(getOrchestratorStatus());
</script>

<svelte:head>
	<title>Environment - orc</title>
</svelte:head>

<div class="environment-page">
	<header class="page-header">
		<div class="header-content">
			<h1>Environment</h1>
			<p class="subtitle">Claude Code and Orchestrator configuration</p>
		</div>
	</header>

	{#if error}
		<div class="alert alert-error">
			<Icon name="error" size={16} />
			{error}
		</div>
	{/if}

	{#if loading}
		<div class="loading-state">
			<div class="spinner"></div>
			<span>Loading environment...</span>
		</div>
	{:else}
		<div class="sections">
			<!-- Claude Code Section -->
			<div class="section" class:expanded={expandedSections.has('claude')}>
				<button class="section-header" onclick={() => toggleSection('claude')}>
					<div class="section-info">
						<div class="section-icon claude">
							<Icon name="terminal" size={20} />
						</div>
						<div class="section-title">
							<h2>Claude Code</h2>
							<p class="section-summary">
								Skills ({skills.length}) • Hooks ({hookCount}) • Agents ({agents.length}) • MCP ({mcpConnected}
								connected)
							</p>
						</div>
					</div>
					<div class="section-status">
						<span class="status-badge {claudeStatus.status}">{claudeStatus.status}</span>
						<Icon
							name={expandedSections.has('claude') ? 'chevron-down' : 'chevron-right'}
							size={16}
						/>
					</div>
				</button>

				{#if expandedSections.has('claude')}
					<div class="section-content">
						{#if claudeStatus.status !== 'healthy'}
							<div class="status-message warning">
								<Icon name="warning" size={14} />
								{claudeStatus.message}
							</div>
						{/if}

						<div class="config-grid">
							<a href="/environment/claude/skills" class="config-card">
								<div class="card-icon">
									<Icon name="skills" size={18} />
								</div>
								<div class="card-info">
									<h3>Skills</h3>
									<p>{skills.length} configured</p>
								</div>
								<Icon name="chevron-right" size={14} class="card-arrow" />
							</a>

							<a href="/environment/claude/hooks" class="config-card">
								<div class="card-icon">
									<Icon name="hooks" size={18} />
								</div>
								<div class="card-info">
									<h3>Hooks</h3>
									<p>{hookCount} hooks on {activeHookEvents} events</p>
								</div>
								<Icon name="chevron-right" size={14} class="card-arrow" />
							</a>

							<a href="/environment/claude/agents" class="config-card">
								<div class="card-icon">
									<Icon name="agents" size={18} />
								</div>
								<div class="card-info">
									<h3>Agents</h3>
									<p>{agents.length} sub-agents</p>
								</div>
								<Icon name="chevron-right" size={14} class="card-arrow" />
							</a>

							<a href="/environment/claude/tools" class="config-card">
								<div class="card-icon">
									<Icon name="tools" size={18} />
								</div>
								<div class="card-info">
									<h3>Tools</h3>
									<p>
										{tools.length} available{deniedToolsCount > 0
											? `, ${deniedToolsCount} denied`
											: ''}
									</p>
								</div>
								<Icon name="chevron-right" size={14} class="card-arrow" />
							</a>

							<a href="/environment/claude/mcp" class="config-card">
								<div class="card-icon">
									<Icon name="mcp" size={18} />
								</div>
								<div class="card-info">
									<h3>MCP Servers</h3>
									<p>{mcpConnected} of {mcpServers.length} enabled</p>
								</div>
								<Icon name="chevron-right" size={14} class="card-arrow" />
							</a>
						</div>
					</div>
				{/if}
			</div>

			<!-- Orchestrator Section -->
			<div class="section" class:expanded={expandedSections.has('orchestrator')}>
				<button class="section-header" onclick={() => toggleSection('orchestrator')}>
					<div class="section-info">
						<div class="section-icon orchestrator">
							<Icon name="layers" size={20} />
						</div>
						<div class="section-title">
							<h2>Orchestrator</h2>
							<p class="section-summary">
								Profile: {config?.profile || 'auto'} • {prompts.length} prompts • {scripts.length} scripts
							</p>
						</div>
					</div>
					<div class="section-status">
						<span class="status-badge {orcStatus.status}">{orcStatus.status}</span>
						<Icon
							name={expandedSections.has('orchestrator') ? 'chevron-down' : 'chevron-right'}
							size={16}
						/>
					</div>
				</button>

				{#if expandedSections.has('orchestrator')}
					<div class="section-content">
						{#if orcStatus.status !== 'healthy'}
							<div class="status-message warning">
								<Icon name="info" size={14} />
								{orcStatus.message}
							</div>
						{/if}

						{#if config}
							<div class="quick-settings">
								<div class="setting-item">
									<span class="setting-label">Profile</span>
									<span class="setting-value badge">{config.profile}</span>
								</div>
								<div class="setting-item">
									<span class="setting-label">Retry</span>
									<span class="setting-value">
										{config.automation.retry_enabled ? 'Enabled' : 'Disabled'}
										{#if config.automation.retry_enabled}
											(max {config.automation.retry_max})
										{/if}
									</span>
								</div>
								<div class="setting-item">
									<span class="setting-label">Gates</span>
									<span class="setting-value">{config.automation.gates_default}</span>
								</div>
								<div class="setting-item">
									<span class="setting-label">Model</span>
									<span class="setting-value mono">{config.execution.model}</span>
								</div>
							</div>
						{/if}

						<div class="config-grid">
							<a href="/environment/orchestrator/prompts" class="config-card">
								<div class="card-icon">
									<Icon name="prompts" size={18} />
								</div>
								<div class="card-info">
									<h3>Prompts</h3>
									<p>
										{prompts.length} phases{overriddenPrompts > 0
											? `, ${overriddenPrompts} overridden`
											: ''}
									</p>
								</div>
								<Icon name="chevron-right" size={14} class="card-arrow" />
							</a>

							<a href="/environment/orchestrator/scripts" class="config-card">
								<div class="card-icon">
									<Icon name="scripts" size={18} />
								</div>
								<div class="card-info">
									<h3>Scripts</h3>
									<p>{scripts.length} registered</p>
								</div>
								<Icon name="chevron-right" size={14} class="card-arrow" />
							</a>

							<a href="/environment/orchestrator/automation" class="config-card">
								<div class="card-icon">
									<Icon name="config" size={18} />
								</div>
								<div class="card-info">
									<h3>Automation</h3>
									<p>Profile, gates, retry settings</p>
								</div>
								<Icon name="chevron-right" size={14} class="card-arrow" />
							</a>

							<a href="/environment/orchestrator/export" class="config-card">
								<div class="card-icon">
									<Icon name="export" size={18} />
								</div>
								<div class="card-info">
									<h3>Export</h3>
									<p>Task artifact export settings</p>
								</div>
								<Icon name="chevron-right" size={14} class="card-arrow" />
							</a>
						</div>
					</div>
				{/if}
			</div>

			<!-- Documentation Section -->
			<div class="section" class:expanded={expandedSections.has('docs')}>
				<button class="section-header" onclick={() => toggleSection('docs')}>
					<div class="section-info">
						<div class="section-icon docs">
							<Icon name="file-text" size={20} />
						</div>
						<div class="section-title">
							<h2>Documentation</h2>
							<p class="section-summary">CLAUDE.md: {claudeMDLevels} level(s) configured</p>
						</div>
					</div>
					<div class="section-status">
						<span class="status-badge healthy">configured</span>
						<Icon
							name={expandedSections.has('docs') ? 'chevron-down' : 'chevron-right'}
							size={16}
						/>
					</div>
				</button>

				{#if expandedSections.has('docs')}
					<div class="section-content">
						<div class="docs-hierarchy">
							{#if claudeMD?.global?.content}
								<div class="docs-level">
									<span class="level-badge global">Global</span>
									<span class="level-path">~/.claude/CLAUDE.md</span>
									<span class="level-size"
										>{claudeMD.global.content.length.toLocaleString()} chars</span
									>
								</div>
							{/if}
							{#if claudeMD?.user?.content}
								<div class="docs-level">
									<span class="level-badge user">User</span>
									<span class="level-path">~/CLAUDE.md</span>
									<span class="level-size"
										>{claudeMD.user.content.length.toLocaleString()} chars</span
									>
								</div>
							{/if}
							{#if claudeMD?.project?.content}
								<div class="docs-level">
									<span class="level-badge project">Project</span>
									<span class="level-path">.claude/CLAUDE.md or CLAUDE.md</span>
									<span class="level-size"
										>{claudeMD.project.content.length.toLocaleString()} chars</span
									>
								</div>
							{/if}
							{#if !claudeMD?.global?.content && !claudeMD?.user?.content && !claudeMD?.project?.content}
								<p class="no-docs">No CLAUDE.md files configured</p>
							{/if}
						</div>

						<a href="/environment/docs" class="config-card full-width">
							<div class="card-icon">
								<Icon name="file-text" size={18} />
							</div>
							<div class="card-info">
								<h3>Edit Documentation</h3>
								<p>View and edit project CLAUDE.md</p>
							</div>
							<Icon name="chevron-right" size={14} class="card-arrow" />
						</a>
					</div>
				{/if}
			</div>
		</div>

		<!-- Keyboard hint -->
		<div class="keyboard-hint">
			<Icon name="terminal" size={14} />
			<span>Press <kbd>Cmd+K</kbd> to quickly jump to any configuration page</span>
		</div>
	{/if}
</div>

<style>
	.environment-page {
		max-width: 900px;
	}

	.page-header {
		margin-bottom: var(--space-6);
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

	/* Alert */
	.alert {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-3) var(--space-4);
		border-radius: var(--radius-md);
		margin-bottom: var(--space-4);
	}

	.alert-error {
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

	/* Sections */
	.sections {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	.section {
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		overflow: hidden;
		transition: border-color var(--duration-fast) var(--ease-out);
	}

	.section:hover {
		border-color: var(--border-default);
	}

	.section.expanded {
		border-color: var(--accent-primary);
	}

	.section-header {
		width: 100%;
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--space-4) var(--space-5);
		background: transparent;
		border: none;
		cursor: pointer;
		text-align: left;
		transition: background var(--duration-fast) var(--ease-out);
	}

	.section-header:hover {
		background: var(--bg-tertiary);
	}

	.section-info {
		display: flex;
		align-items: center;
		gap: var(--space-4);
	}

	.section-icon {
		width: 44px;
		height: 44px;
		display: flex;
		align-items: center;
		justify-content: center;
		border-radius: var(--radius-md);
		flex-shrink: 0;
	}

	.section-icon.claude {
		background: rgba(139, 92, 246, 0.15);
		color: var(--accent-primary);
	}

	.section-icon.orchestrator {
		background: rgba(245, 158, 11, 0.15);
		color: var(--status-warning);
	}

	.section-icon.docs {
		background: rgba(59, 130, 246, 0.15);
		color: #3b82f6;
	}

	.section-title h2 {
		font-size: var(--text-base);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		margin: 0;
	}

	.section-summary {
		font-size: var(--text-sm);
		color: var(--text-muted);
		margin: var(--space-1) 0 0;
	}

	.section-status {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		color: var(--text-muted);
	}

	.status-badge {
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		padding: var(--space-1) var(--space-2);
		border-radius: var(--radius-full);
		text-transform: capitalize;
	}

	.status-badge.healthy {
		background: rgba(16, 185, 129, 0.15);
		color: var(--status-success);
	}

	.status-badge.warning {
		background: rgba(245, 158, 11, 0.15);
		color: var(--status-warning);
	}

	.status-badge.issues {
		background: rgba(239, 68, 68, 0.15);
		color: var(--status-danger);
	}

	/* Section Content */
	.section-content {
		padding: 0 var(--space-5) var(--space-5);
		animation: slideDown var(--duration-fast) var(--ease-out);
	}

	@keyframes slideDown {
		from {
			opacity: 0;
			transform: translateY(-8px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.status-message {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		margin-bottom: var(--space-4);
	}

	.status-message.warning {
		background: rgba(245, 158, 11, 0.1);
		color: var(--status-warning);
	}

	/* Quick Settings */
	.quick-settings {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: var(--space-3);
		padding: var(--space-3);
		background: var(--bg-tertiary);
		border-radius: var(--radius-md);
		margin-bottom: var(--space-4);
	}

	@media (max-width: 640px) {
		.quick-settings {
			grid-template-columns: repeat(2, 1fr);
		}
	}

	.setting-item {
		display: flex;
		flex-direction: column;
		gap: var(--space-0-5);
	}

	.setting-label {
		font-size: var(--text-xs);
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.setting-value {
		font-size: var(--text-sm);
		color: var(--text-primary);
		font-weight: var(--font-medium);
	}

	.setting-value.badge {
		display: inline-flex;
		background: var(--accent-subtle);
		color: var(--accent-primary);
		padding: var(--space-0-5) var(--space-2);
		border-radius: var(--radius-sm);
		font-size: var(--text-xs);
		width: fit-content;
	}

	.setting-value.mono {
		font-family: var(--font-mono);
		font-size: var(--text-xs);
	}

	/* Config Grid */
	.config-grid {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: var(--space-3);
	}

	@media (max-width: 640px) {
		.config-grid {
			grid-template-columns: 1fr;
		}
	}

	.config-card {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-3) var(--space-4);
		background: var(--bg-primary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-md);
		text-decoration: none;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.config-card:hover {
		border-color: var(--accent-primary);
		background: var(--bg-tertiary);
	}

	.config-card.full-width {
		grid-column: span 2;
	}

	@media (max-width: 640px) {
		.config-card.full-width {
			grid-column: span 1;
		}
	}

	.card-icon {
		width: 36px;
		height: 36px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--bg-tertiary);
		border-radius: var(--radius-md);
		color: var(--text-secondary);
		flex-shrink: 0;
	}

	.config-card:hover .card-icon {
		background: var(--accent-subtle);
		color: var(--accent-primary);
	}

	.card-info {
		flex: 1;
		min-width: 0;
	}

	.card-info h3 {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
		margin: 0;
	}

	.card-info p {
		font-size: var(--text-xs);
		color: var(--text-muted);
		margin: var(--space-0-5) 0 0;
	}

	.config-card :global(.card-arrow) {
		color: var(--text-muted);
		opacity: 0;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.config-card:hover :global(.card-arrow) {
		opacity: 1;
		color: var(--accent-primary);
		transform: translateX(2px);
	}

	/* Docs Hierarchy */
	.docs-hierarchy {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
		margin-bottom: var(--space-4);
	}

	.docs-level {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-2) var(--space-3);
		background: var(--bg-tertiary);
		border-radius: var(--radius-md);
	}

	.level-badge {
		font-size: var(--text-xs);
		font-weight: var(--font-medium);
		padding: var(--space-0-5) var(--space-2);
		border-radius: var(--radius-sm);
		min-width: 60px;
		text-align: center;
	}

	.level-badge.global {
		background: rgba(139, 92, 246, 0.15);
		color: var(--accent-primary);
	}

	.level-badge.user {
		background: rgba(59, 130, 246, 0.15);
		color: #3b82f6;
	}

	.level-badge.project {
		background: rgba(16, 185, 129, 0.15);
		color: var(--status-success);
	}

	.level-path {
		flex: 1;
		font-family: var(--font-mono);
		font-size: var(--text-xs);
		color: var(--text-secondary);
	}

	.level-size {
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.no-docs {
		font-size: var(--text-sm);
		color: var(--text-muted);
		font-style: italic;
	}

	/* Keyboard Hint */
	.keyboard-hint {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-2);
		margin-top: var(--space-6);
		padding: var(--space-3);
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-md);
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.keyboard-hint kbd {
		padding: var(--space-0-5) var(--space-1-5);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-sm);
		font-family: var(--font-mono);
		font-size: var(--text-2xs);
	}
</style>
