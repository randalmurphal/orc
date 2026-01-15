<script lang="ts">
	import { page } from '$app/stores';
	import Icon from './Icon.svelte';

	interface BreadcrumbItem {
		label: string;
		href?: string;
	}

	// Route config maps paths to labels
	const routeLabels: Record<string, string> = {
		'': 'Home',
		'dashboard': 'Dashboard',
		'board': 'Board',
		'environment': 'Environment',
		'claude': 'Claude Code',
		'orchestrator': 'Orchestrator',
		'skills': 'Skills',
		'hooks': 'Hooks',
		'agents': 'Agents',
		'tools': 'Tools',
		'mcp': 'MCP Servers',
		'prompts': 'Prompts',
		'scripts': 'Scripts',
		'automation': 'Automation',
		'export': 'Export',
		'docs': 'Documentation',
		'preferences': 'Preferences',
		'knowledge': 'Knowledge Queue'
	};

	// Category segments that don't have their own page - link to parent instead
	const categorySegments = new Set(['claude', 'orchestrator']);

	const items = $derived.by(() => {
		const pathname = $page.url.pathname;
		const segments = pathname.split('/').filter(Boolean);

		// Only show breadcrumbs for environment and preferences pages
		if (segments[0] !== 'environment' && segments[0] !== 'preferences') {
			return [];
		}

		const crumbs: BreadcrumbItem[] = [];
		let currentPath = '';

		for (let i = 0; i < segments.length; i++) {
			const segment = segments[i];
			currentPath += '/' + segment;

			const label = routeLabels[segment] || segment;
			const isLast = i === segments.length - 1;

			// For category segments (claude, orchestrator), link to /environment instead
			let href: string | undefined;
			if (isLast) {
				href = undefined;
			} else if (categorySegments.has(segment)) {
				href = '/environment';
			} else {
				href = currentPath;
			}

			crumbs.push({
				label,
				href
			});
		}

		return crumbs;
	});
</script>

{#if items.length > 0}
	<nav class="breadcrumbs" aria-label="Breadcrumb">
		<ol>
			{#each items as item, i}
				<li>
					{#if item.href}
						<a href={item.href}>{item.label}</a>
					{:else}
						<span class="current">{item.label}</span>
					{/if}
					{#if i < items.length - 1}
						<Icon name="chevron-right" size={14} />
					{/if}
				</li>
			{/each}
		</ol>
	</nav>
{/if}

<style>
	.breadcrumbs {
		margin-bottom: var(--space-4);
	}

	ol {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		list-style: none;
		margin: 0;
		padding: 0;
		font-size: var(--text-sm);
	}

	li {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		color: var(--text-muted);
	}

	a {
		color: var(--text-muted);
		text-decoration: none;
		transition: color var(--duration-fast) var(--ease-out);
	}

	a:hover {
		color: var(--text-primary);
	}

	.current {
		color: var(--text-secondary);
		font-weight: 500;
	}

	:global(.breadcrumbs svg) {
		color: var(--text-muted);
		opacity: 0.5;
	}
</style>
