<script lang="ts">
	import '../app.css';
	import { page } from '$app/stores';
	import ProjectSwitcher from '$lib/components/ProjectSwitcher.svelte';

	const navItems = [
		{ href: '/', label: 'Tasks' },
		{ href: '/prompts', label: 'Prompts' },
		{ href: '/claudemd', label: 'CLAUDE.md' },
		{ href: '/skills', label: 'Skills' },
		{ href: '/hooks', label: 'Hooks' },
		{ href: '/mcp', label: 'MCP' },
		{ href: '/tools', label: 'Tools' },
		{ href: '/agents', label: 'Agents' },
		{ href: '/scripts', label: 'Scripts' },
		{ href: '/settings', label: 'Settings' },
		{ href: '/config', label: 'Config' }
	];

	function isActive(href: string, currentPath: string): boolean {
		if (href === '/') {
			// Tasks is active on / and /tasks/*
			return currentPath === '/' || currentPath.startsWith('/tasks');
		}
		return currentPath.startsWith(href);
	}
</script>

<div class="app">
	<header>
		<nav>
			<a href="/" class="logo">orc</a>
			<ProjectSwitcher />
			<div class="nav-links">
				{#each navItems as item}
					<a
						href={item.href}
						class:active={isActive(item.href, $page.url.pathname)}
					>
						{item.label}
					</a>
				{/each}
			</div>
		</nav>
	</header>

	<main>
		<slot />
	</main>
</div>

<style>
	.app {
		display: flex;
		flex-direction: column;
		min-height: 100vh;
	}

	header {
		background: var(--bg-secondary);
		border-bottom: 1px solid var(--border-color);
		padding: 0 1.5rem;
	}

	nav {
		display: flex;
		align-items: center;
		justify-content: space-between;
		max-width: 1200px;
		margin: 0 auto;
		height: 56px;
	}

	.logo {
		font-size: 1.25rem;
		font-weight: 700;
		color: var(--text-primary);
	}

	.nav-links {
		display: flex;
		gap: 1.5rem;
	}

	.nav-links a {
		color: var(--text-secondary);
		font-size: 0.875rem;
		padding: 0.25rem 0;
		border-bottom: 2px solid transparent;
		transition: color 0.15s, border-color 0.15s;
	}

	.nav-links a:hover {
		color: var(--text-primary);
		text-decoration: none;
	}

	.nav-links a.active {
		color: var(--accent-primary);
		border-bottom-color: var(--accent-primary);
	}

	main {
		flex: 1;
		max-width: 1200px;
		margin: 0 auto;
		padding: 2rem 1.5rem;
		width: 100%;
	}
</style>
