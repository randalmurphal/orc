<script lang="ts">
	import { page } from '$app/stores';
	import { formatShortcut } from '$lib/utils/platform';

	interface NavItem {
		label: string;
		href: string;
		icon: string;
		section?: 'main' | 'config';
	}

	const navItems: NavItem[] = [
		{ label: 'Dashboard', href: '/dashboard', icon: 'dashboard', section: 'main' },
		{ label: 'Tasks', href: '/', icon: 'tasks', section: 'main' },
		{ label: 'Prompts', href: '/prompts', icon: 'prompts', section: 'main' },
		{ label: 'CLAUDE.md', href: '/claudemd', icon: 'file', section: 'main' },
		{ label: 'Skills', href: '/skills', icon: 'skills', section: 'main' },
		{ label: 'Hooks', href: '/hooks', icon: 'hooks', section: 'main' },
		{ label: 'MCP', href: '/mcp', icon: 'mcp', section: 'main' },
		{ label: 'Tools', href: '/tools', icon: 'tools', section: 'main' },
		{ label: 'Agents', href: '/agents', icon: 'agents', section: 'main' },
		{ label: 'Scripts', href: '/scripts', icon: 'scripts', section: 'main' },
		{ label: 'Settings', href: '/settings', icon: 'settings', section: 'config' },
		{ label: 'Config', href: '/config', icon: 'config', section: 'config' }
	];

	let expanded = $state(false);
	let pinned = $state(false);

	function isActive(href: string): boolean {
		if (href === '/') {
			return $page.url.pathname === '/' || $page.url.pathname.startsWith('/tasks');
		}
		if (href === '/dashboard') {
			return $page.url.pathname === '/dashboard';
		}
		return $page.url.pathname.startsWith(href);
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
		pinned = !pinned;
		expanded = pinned;
	}

	const mainItems = navItems.filter((item) => item.section === 'main');
	const configItems = navItems.filter((item) => item.section === 'config');
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
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="14"
					height="14"
					viewBox="0 0 24 24"
					fill={pinned ? 'currentColor' : 'none'}
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
				>
					<line x1="12" y1="17" x2="12" y2="22" />
					<path d="M5 17h14v-1.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V6h1a2 2 0 0 0 0-4H8a2 2 0 0 0 0 4h1v4.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24Z" />
				</svg>
			</button>
		{/if}
	</div>

	<!-- Main Navigation -->
	<nav class="nav-section">
		<ul class="nav-list">
			{#each mainItems as item}
				<li>
					<a
						href={item.href}
						class="nav-item"
						class:active={isActive(item.href)}
						title={!expanded ? item.label : undefined}
					>
						<span class="nav-icon">
							{#if item.icon === 'dashboard'}
								<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
									<rect x="3" y="3" width="7" height="9" />
									<rect x="14" y="3" width="7" height="5" />
									<rect x="14" y="12" width="7" height="9" />
									<rect x="3" y="16" width="7" height="5" />
								</svg>
							{:else if item.icon === 'tasks'}
								<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
									<rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
									<line x1="9" y1="9" x2="15" y2="9" />
									<line x1="9" y1="13" x2="15" y2="13" />
									<line x1="9" y1="17" x2="13" y2="17" />
								</svg>
							{:else if item.icon === 'prompts'}
								<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
									<polyline points="4 7 4 4 20 4 20 7" />
									<line x1="9" y1="20" x2="15" y2="20" />
									<line x1="12" y1="4" x2="12" y2="20" />
								</svg>
							{:else if item.icon === 'file'}
								<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
									<path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z" />
									<polyline points="14 2 14 8 20 8" />
									<line x1="16" y1="13" x2="8" y2="13" />
									<line x1="16" y1="17" x2="8" y2="17" />
									<line x1="10" y1="9" x2="8" y2="9" />
								</svg>
							{:else if item.icon === 'skills'}
								<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
									<polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2" />
								</svg>
							{:else if item.icon === 'hooks'}
								<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
									<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
									<path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
								</svg>
							{:else if item.icon === 'mcp'}
								<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
									<rect x="4" y="4" width="16" height="16" rx="2" ry="2" />
									<rect x="9" y="9" width="6" height="6" />
									<line x1="9" y1="1" x2="9" y2="4" />
									<line x1="15" y1="1" x2="15" y2="4" />
									<line x1="9" y1="20" x2="9" y2="23" />
									<line x1="15" y1="20" x2="15" y2="23" />
									<line x1="20" y1="9" x2="23" y2="9" />
									<line x1="20" y1="14" x2="23" y2="14" />
									<line x1="1" y1="9" x2="4" y2="9" />
									<line x1="1" y1="14" x2="4" y2="14" />
								</svg>
							{:else if item.icon === 'tools'}
								<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
									<path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" />
								</svg>
							{:else if item.icon === 'agents'}
								<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
									<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
									<circle cx="9" cy="7" r="4" />
									<path d="M23 21v-2a4 4 0 0 0-3-3.87" />
									<path d="M16 3.13a4 4 0 0 1 0 7.75" />
								</svg>
							{:else if item.icon === 'scripts'}
								<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
									<polyline points="16 18 22 12 16 6" />
									<polyline points="8 6 2 12 8 18" />
								</svg>
							{:else if item.icon === 'settings'}
								<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
									<circle cx="12" cy="12" r="3" />
									<path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z" />
								</svg>
							{:else if item.icon === 'config'}
								<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
									<line x1="4" y1="21" x2="4" y2="14" />
									<line x1="4" y1="10" x2="4" y2="3" />
									<line x1="12" y1="21" x2="12" y2="12" />
									<line x1="12" y1="8" x2="12" y2="3" />
									<line x1="20" y1="21" x2="20" y2="16" />
									<line x1="20" y1="12" x2="20" y2="3" />
									<line x1="1" y1="14" x2="7" y2="14" />
									<line x1="9" y1="8" x2="15" y2="8" />
									<line x1="17" y1="16" x2="23" y2="16" />
								</svg>
							{/if}
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

	<!-- Config Section -->
	<nav class="nav-section config-section">
		<ul class="nav-list">
			{#each configItems as item}
				<li>
					<a
						href={item.href}
						class="nav-item"
						class:active={isActive(item.href)}
						title={!expanded ? item.label : undefined}
					>
						<span class="nav-icon">
							{#if item.icon === 'settings'}
								<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
									<circle cx="12" cy="12" r="3" />
									<path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z" />
								</svg>
							{:else if item.icon === 'config'}
								<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
									<line x1="4" y1="21" x2="4" y2="14" />
									<line x1="4" y1="10" x2="4" y2="3" />
									<line x1="12" y1="21" x2="12" y2="12" />
									<line x1="12" y1="8" x2="12" y2="3" />
									<line x1="20" y1="21" x2="20" y2="16" />
									<line x1="20" y1="12" x2="20" y2="3" />
									<line x1="1" y1="14" x2="7" y2="14" />
									<line x1="9" y1="8" x2="15" y2="8" />
									<line x1="17" y1="16" x2="23" y2="16" />
								</svg>
							{/if}
						</span>
						{#if expanded}
							<span class="nav-label">{item.label}</span>
						{/if}
					</a>
				</li>
			{/each}
		</ul>
	</nav>

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

	/* Navigation */
	.nav-section {
		flex: 1;
		padding: var(--space-2) 0;
		overflow-y: auto;
		overflow-x: hidden;
	}

	.config-section {
		flex: none;
		padding-bottom: var(--space-3);
	}

	.nav-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: var(--space-0-5);
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

	/* Divider */
	.nav-divider {
		height: 1px;
		background: var(--border-subtle);
		margin: var(--space-2) var(--space-4);
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
