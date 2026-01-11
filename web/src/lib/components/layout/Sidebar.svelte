<script lang="ts">
	import { page } from '$app/stores';
	import { formatShortcut } from '$lib/utils/platform';
	import Icon from '$lib/components/ui/Icon.svelte';

	interface NavItem {
		label: string;
		href: string;
		icon: string;
		section?: 'main' | 'config';
	}

	const navItems: NavItem[] = [
		{ label: 'Dashboard', href: '/dashboard', icon: 'dashboard', section: 'main' },
		{ label: 'Tasks', href: '/', icon: 'tasks', section: 'main' },
		{ label: 'Board', href: '/board', icon: 'board', section: 'main' },
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
				<Icon name="pin" size={14} class={pinned ? 'pin-filled' : ''} />
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

	/* Pin filled state - apply fill via CSS */
	.pin-btn.active :global(.pin-filled) {
		fill: currentColor;
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
