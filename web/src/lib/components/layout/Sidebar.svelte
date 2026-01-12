<script lang="ts">
	import { page } from '$app/stores';
	import { browser } from '$app/environment';
	import { formatShortcut } from '$lib/utils/platform';
	import Icon from '$lib/components/ui/Icon.svelte';
	import { sidebarPinned } from '$lib/stores/sidebar';

	interface NavItem {
		label: string;
		href: string;
		icon: string;
	}

	interface NavGroup {
		label: string;
		icon: string;
		items: NavItem[];
		basePath: string; // Used for active state detection
	}

	interface NavSection {
		id: string;
		label: string;
		items?: NavItem[];
		groups?: NavGroup[];
	}

	// Navigation structure
	const sections: NavSection[] = [
		{
			id: 'work',
			label: 'Work',
			items: [
				{ label: 'Dashboard', href: '/dashboard', icon: 'dashboard' },
				{ label: 'Tasks', href: '/', icon: 'tasks' },
				{ label: 'Board', href: '/board', icon: 'board' }
			]
		},
		{
			id: 'environment',
			label: 'Environment',
			groups: [
				{
					label: 'Claude Code',
					icon: 'terminal',
					basePath: '/environment/claude',
					items: [
						{ label: 'Skills', href: '/environment/claude/skills', icon: 'skills' },
						{ label: 'Hooks', href: '/environment/claude/hooks', icon: 'hooks' },
						{ label: 'Agents', href: '/environment/claude/agents', icon: 'agents' },
						{ label: 'Tools', href: '/environment/claude/tools', icon: 'tools' },
						{ label: 'MCP Servers', href: '/environment/claude/mcp', icon: 'mcp' }
					]
				},
				{
					label: 'Orchestrator',
					icon: 'layers',
					basePath: '/environment/orchestrator',
					items: [
						{ label: 'Prompts', href: '/environment/orchestrator/prompts', icon: 'prompts' },
						{ label: 'Scripts', href: '/environment/orchestrator/scripts', icon: 'scripts' },
						{ label: 'Automation', href: '/environment/orchestrator/automation', icon: 'config' },
						{ label: 'Export', href: '/environment/orchestrator/export', icon: 'export' },
						{ label: 'Knowledge', href: '/environment/knowledge', icon: 'database' }
					]
				}
			]
		}
	];

	// Standalone items (not in sections)
	const environmentOverview: NavItem = {
		label: 'Overview',
		href: '/environment',
		icon: 'layers'
	};

	const docsItem: NavItem = {
		label: 'Documentation',
		href: '/environment/docs',
		icon: 'file-text'
	};

	const preferencesItem: NavItem = {
		label: 'Preferences',
		href: '/preferences',
		icon: 'user'
	};

	// Sidebar state - expanded follows pinned state on mount and pin changes
	let expanded = $state($sidebarPinned);
	// Pin state is synced with store
	let pinned = $derived($sidebarPinned);

	// Keep expanded in sync when pinned changes externally
	$effect(() => {
		if (pinned) {
			expanded = true;
		}
	});

	// Section/group expansion state (persisted in localStorage)
	const STORAGE_KEY_SECTIONS = 'orc-sidebar-sections';
	const STORAGE_KEY_GROUPS = 'orc-sidebar-groups';

	function loadExpandedState(): { sections: Set<string>; groups: Set<string> } {
		if (!browser) return { sections: new Set(), groups: new Set() };
		try {
			const sectionsJson = localStorage.getItem(STORAGE_KEY_SECTIONS);
			const groupsJson = localStorage.getItem(STORAGE_KEY_GROUPS);
			return {
				sections: sectionsJson ? new Set(JSON.parse(sectionsJson)) : new Set(),
				groups: groupsJson ? new Set(JSON.parse(groupsJson)) : new Set()
			};
		} catch {
			return { sections: new Set(), groups: new Set() };
		}
	}

	function saveExpandedState(sections: Set<string>, groups: Set<string>) {
		if (!browser) return;
		localStorage.setItem(STORAGE_KEY_SECTIONS, JSON.stringify([...sections]));
		localStorage.setItem(STORAGE_KEY_GROUPS, JSON.stringify([...groups]));
	}

	let { sections: expandedSections, groups: expandedGroups } = loadExpandedState();
	let expandedSectionsState = $state(expandedSections);
	let expandedGroupsState = $state(expandedGroups);

	function toggleSection(sectionId: string) {
		if (expandedSectionsState.has(sectionId)) {
			expandedSectionsState.delete(sectionId);
		} else {
			expandedSectionsState.add(sectionId);
		}
		expandedSectionsState = new Set(expandedSectionsState);
		saveExpandedState(expandedSectionsState, expandedGroupsState);
	}

	function toggleGroup(groupLabel: string) {
		if (expandedGroupsState.has(groupLabel)) {
			expandedGroupsState.delete(groupLabel);
		} else {
			expandedGroupsState.add(groupLabel);
		}
		expandedGroupsState = new Set(expandedGroupsState);
		saveExpandedState(expandedSectionsState, expandedGroupsState);
	}

	function isActive(href: string): boolean {
		const pathname = $page.url.pathname;
		if (href === '/') {
			return pathname === '/' || pathname.startsWith('/tasks');
		}
		if (href === '/dashboard') {
			return pathname === '/dashboard';
		}
		if (href === '/environment') {
			return pathname === '/environment';
		}
		return pathname.startsWith(href);
	}

	function isGroupActive(basePath: string): boolean {
		return $page.url.pathname.startsWith(basePath);
	}

	function handleMouseEnter() {
		if (!pinned) {
			expanded = true;
		}
	}

	function handleMouseLeave() {
		if (!pinned) {
			expanded = false;
		}
	}

	function togglePin() {
		sidebarPinned.toggle();
		expanded = !pinned; // Use current pinned value (before toggle completes)
	}

	// Get work and environment sections
	const workSection = sections.find((s) => s.id === 'work')!;
	const envSection = sections.find((s) => s.id === 'environment')!;
</script>

<aside
	class="sidebar"
	class:expanded
	class:pinned
	onmouseenter={handleMouseEnter}
	onmouseleave={handleMouseLeave}
	role="navigation"
	aria-label="Main navigation"
>
	<!-- Logo -->
	<div class="logo-section">
		<a href="/" class="logo">
			<span class="logo-icon">&gt;_</span>
			{#if expanded}
				<span class="logo-text">ORC</span>
			{/if}
		</a>
		{#if expanded}
			<button
				class="pin-btn"
				class:active={pinned}
				onclick={togglePin}
				title={pinned ? 'Unpin sidebar' : 'Pin sidebar'}
				aria-pressed={pinned}
			>
				<Icon name="pin" size={14} class={pinned ? 'pin-filled' : ''} />
			</button>
		{/if}
	</div>

	<!-- Scrollable Navigation -->
	<div class="nav-container">
		<!-- Work Section -->
		<nav class="nav-section">
			{#if expanded}
				<div class="section-header">Work</div>
			{/if}
			<ul class="nav-list">
				{#each workSection.items || [] as item}
					<li>
						<a
							href={item.href}
							class="nav-item"
							class:active={isActive(item.href)}
							title={!expanded ? item.label : undefined}
						>
							<span class="nav-icon">
								<Icon name={item.icon} size={18} />
							</span>
							{#if expanded}
								<span class="nav-label">{item.label}</span>
							{/if}
						</a>
					</li>
				{/each}
			</ul>
		</nav>

		<!-- Divider -->
		<div class="nav-divider"></div>

		<!-- Environment Section -->
		<nav class="nav-section environment-section">
			{#if expanded}
				<button
					class="section-header clickable"
					onclick={() => toggleSection('environment')}
					aria-expanded={expandedSectionsState.has('environment')}
				>
					<span>Environment</span>
					<Icon
						name={expandedSectionsState.has('environment') ? 'chevron-down' : 'chevron-right'}
						size={14}
					/>
				</button>
			{:else}
				<a
					href="/environment"
					class="nav-item"
					class:active={$page.url.pathname.startsWith('/environment')}
					title="Environment"
				>
					<span class="nav-icon">
						<Icon name="layers" size={18} />
					</span>
				</a>
			{/if}

			{#if expanded && expandedSectionsState.has('environment')}
				<!-- Overview link -->
				<ul class="nav-list">
					<li>
						<a
							href={environmentOverview.href}
							class="nav-item sub-item"
							class:active={isActive(environmentOverview.href)}
						>
							<span class="nav-icon">
								<Icon name={environmentOverview.icon} size={16} />
							</span>
							<span class="nav-label">{environmentOverview.label}</span>
						</a>
					</li>
				</ul>

				<!-- Groups -->
				{#each envSection.groups || [] as group}
					<div class="nav-group">
						<button
							class="group-header"
							onclick={() => toggleGroup(group.label)}
							aria-expanded={expandedGroupsState.has(group.label)}
							class:active={isGroupActive(group.basePath)}
						>
							<span class="group-icon">
								<Icon name={group.icon} size={16} />
							</span>
							<span class="group-label">{group.label}</span>
							<Icon
								name={expandedGroupsState.has(group.label) ? 'chevron-down' : 'chevron-right'}
								size={12}
							/>
						</button>

						{#if expandedGroupsState.has(group.label)}
							<ul class="nav-list nested">
								{#each group.items as item}
									<li>
										<a
											href={item.href}
											class="nav-item nested-item"
											class:active={isActive(item.href)}
										>
											<span class="nav-icon">
												<Icon name={item.icon} size={14} />
											</span>
											<span class="nav-label">{item.label}</span>
										</a>
									</li>
								{/each}
							</ul>
						{/if}
					</div>
				{/each}

				<!-- Documentation link -->
				<ul class="nav-list">
					<li>
						<a
							href={docsItem.href}
							class="nav-item sub-item"
							class:active={isActive(docsItem.href)}
						>
							<span class="nav-icon">
								<Icon name={docsItem.icon} size={16} />
							</span>
							<span class="nav-label">{docsItem.label}</span>
						</a>
					</li>
				</ul>
			{/if}
		</nav>
	</div>

	<!-- Bottom Section: Preferences -->
	<div class="bottom-section">
		<div class="nav-divider"></div>
		<nav class="nav-section">
			<ul class="nav-list">
				<li>
					<a
						href={preferencesItem.href}
						class="nav-item"
						class:active={isActive(preferencesItem.href)}
						title={!expanded ? preferencesItem.label : undefined}
					>
						<span class="nav-icon">
							<Icon name={preferencesItem.icon} size={18} />
						</span>
						{#if expanded}
							<span class="nav-label">{preferencesItem.label}</span>
						{/if}
					</a>
				</li>
			</ul>
		</nav>
	</div>

	<!-- Keyboard hint -->
	{#if expanded}
		<div class="keyboard-hint">
			<kbd>{formatShortcut('B')}</kbd> to toggle
		</div>
	{/if}
</aside>

<style>
	.sidebar {
		position: fixed;
		top: 0;
		left: 0;
		bottom: 0;
		width: var(--sidebar-width-collapsed);
		background: var(--bg-secondary);
		border-right: 1px solid var(--border-subtle);
		display: flex;
		flex-direction: column;
		z-index: var(--z-sidebar);
		transition: width var(--duration-normal) var(--ease-out);
		overflow: hidden;
	}

	.sidebar.expanded {
		width: var(--sidebar-width-expanded);
	}

	.sidebar.pinned {
		width: var(--sidebar-width-expanded);
	}

	/* Logo Section */
	.logo-section {
		display: flex;
		align-items: center;
		justify-content: space-between;
		height: var(--header-height);
		padding: 0 var(--space-3);
		border-bottom: 1px solid var(--border-subtle);
		flex-shrink: 0;
	}

	.logo {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		text-decoration: none;
		color: var(--accent-primary);
		font-family: var(--font-mono);
		font-weight: var(--font-bold);
		font-size: var(--text-lg);
	}

	.logo:hover {
		color: var(--accent-hover);
	}

	.logo-icon {
		font-size: var(--text-lg);
		text-shadow: 0 0 10px var(--accent-glow);
	}

	.logo-text {
		letter-spacing: var(--tracking-tight);
		animation: fade-in var(--duration-fast) var(--ease-out);
	}

	.pin-btn {
		width: 28px;
		height: 28px;
		padding: var(--space-1);
		background: transparent;
		border: none;
		border-radius: var(--radius-md);
		color: var(--text-muted);
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.pin-btn:hover {
		background: var(--bg-tertiary);
		color: var(--text-secondary);
	}

	.pin-btn.active {
		color: var(--accent-primary);
	}

	.pin-btn.active :global(.pin-filled) {
		fill: currentColor;
	}

	/* Navigation Container */
	.nav-container {
		flex: 1;
		overflow-y: auto;
		overflow-x: hidden;
	}

	/* Navigation */
	.nav-section {
		padding: var(--space-2) 0;
	}

	.environment-section {
		padding-bottom: var(--space-1);
	}

	.section-header {
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-muted);
		padding: var(--space-2) var(--space-4) var(--space-1);
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.section-header.clickable {
		cursor: pointer;
		background: transparent;
		border: none;
		width: 100%;
		text-align: left;
		transition: color var(--duration-fast) var(--ease-out);
	}

	.section-header.clickable:hover {
		color: var(--text-secondary);
	}

	.nav-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: var(--space-0-5);
	}

	.nav-list.nested {
		margin-left: var(--space-4);
	}

	.nav-item {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-2-5) var(--space-3);
		margin: 0 var(--space-2);
		color: var(--text-secondary);
		text-decoration: none;
		border-radius: var(--radius-md);
		transition: all var(--duration-fast) var(--ease-out);
		white-space: nowrap;
	}

	.nav-item.sub-item {
		padding: var(--space-2) var(--space-3);
		margin-left: var(--space-3);
	}

	.nav-item.nested-item {
		padding: var(--space-1-5) var(--space-3);
		margin: 0;
		font-size: var(--text-sm);
	}

	.nav-item:hover {
		background: var(--bg-tertiary);
		color: var(--text-primary);
	}

	.nav-item.active {
		background: var(--accent-subtle);
		color: var(--accent-primary);
	}

	.nav-item.active .nav-icon {
		color: var(--accent-primary);
	}

	.nav-icon {
		flex-shrink: 0;
		width: 18px;
		height: 18px;
		display: flex;
		align-items: center;
		justify-content: center;
		color: var(--text-muted);
		transition: color var(--duration-fast) var(--ease-out);
	}

	.nav-item:hover .nav-icon {
		color: var(--text-secondary);
	}

	.nav-label {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		animation: fade-in var(--duration-fast) var(--ease-out);
	}

	/* Nav Groups */
	.nav-group {
		margin: var(--space-1) 0;
	}

	.group-header {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		width: 100%;
		padding: var(--space-2) var(--space-3);
		margin: 0 var(--space-2);
		background: transparent;
		border: none;
		border-radius: var(--radius-md);
		color: var(--text-secondary);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
		text-align: left;
	}

	.group-header:hover {
		background: var(--bg-tertiary);
		color: var(--text-primary);
	}

	.group-header.active {
		color: var(--accent-primary);
	}

	.group-icon {
		flex-shrink: 0;
		width: 16px;
		height: 16px;
		display: flex;
		align-items: center;
		justify-content: center;
		color: var(--text-muted);
	}

	.group-header:hover .group-icon,
	.group-header.active .group-icon {
		color: inherit;
	}

	.group-label {
		flex: 1;
	}

	/* Divider */
	.nav-divider {
		height: 1px;
		background: var(--border-subtle);
		margin: var(--space-2) var(--space-4);
	}

	/* Bottom Section */
	.bottom-section {
		flex-shrink: 0;
		margin-top: auto;
	}

	/* Keyboard hint */
	.keyboard-hint {
		padding: var(--space-3) var(--space-4);
		border-top: 1px solid var(--border-subtle);
		font-size: var(--text-xs);
		color: var(--text-muted);
		display: flex;
		align-items: center;
		gap: var(--space-2);
		animation: fade-in var(--duration-fast) var(--ease-out);
	}

	.keyboard-hint kbd {
		font-family: var(--font-mono);
		font-size: var(--text-2xs);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-sm);
		padding: var(--space-0-5) var(--space-1);
		box-shadow: 0 1px 0 var(--border-default);
	}
</style>
