<script lang="ts">
	import { page } from '$app/stores';
	import type { Project } from '$lib/types';
	import { getModifierKey, formatShortcut } from '$lib/utils/platform';

	interface Props {
		currentProject: Project | null;
		onProjectClick?: () => void;
		onNewTask?: () => void;
		onCommandPalette?: () => void;
	}

	let { currentProject, onProjectClick, onNewTask, onCommandPalette }: Props = $props();

	const modKey = getModifierKey();

	// Derive page title from route
	function getPageTitle(pathname: string): string {
		if (pathname === '/' || pathname.startsWith('/tasks')) {
			if (pathname.startsWith('/tasks/')) {
				return 'Task Details';
			}
			return 'Tasks';
		}
		const segment = pathname.split('/')[1];
		const titles: Record<string, string> = {
			prompts: 'Prompts',
			claudemd: 'CLAUDE.md',
			skills: 'Skills',
			hooks: 'Hooks',
			mcp: 'MCP Servers',
			tools: 'Tools',
			agents: 'Agents',
			scripts: 'Scripts',
			settings: 'Settings',
			config: 'Configuration'
		};
		return titles[segment] || segment;
	}

	const pageTitle = $derived(getPageTitle($page.url.pathname));
</script>

<header class="header">
	<div class="header-left">
		<!-- Project Switcher Button -->
		<button class="project-btn" onclick={onProjectClick} title="Switch project ({modKey}+P)">
			<span class="project-icon">
				<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
				</svg>
			</span>
			<span class="project-name">{currentProject?.name || 'Select project'}</span>
			<span class="project-chevron">
				<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<polyline points="6 9 12 15 18 9" />
				</svg>
			</span>
		</button>

		<!-- Page Title / Breadcrumb -->
		<div class="page-info">
			<span class="separator">/</span>
			<h1 class="page-title">{pageTitle}</h1>
		</div>
	</div>

	<div class="header-right">
		<!-- Command Palette Hint -->
		<button class="cmd-hint" onclick={onCommandPalette} title="Command palette ({modKey}+K)">
			<span class="cmd-hint-label">Commands</span>
			<kbd>{formatShortcut('K')}</kbd>
		</button>

		<!-- New Task Button -->
		{#if onNewTask}
			<button class="primary new-task-btn" onclick={onNewTask}>
				<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<line x1="12" y1="5" x2="12" y2="19" />
					<line x1="5" y1="12" x2="19" y2="12" />
				</svg>
				New Task
			</button>
		{/if}
	</div>
</header>

<style>
	.header {
		height: var(--header-height);
		background: var(--bg-primary);
		border-bottom: 1px solid var(--border-subtle);
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0 var(--space-6);
		position: sticky;
		top: 0;
		z-index: var(--z-header);
	}

	.header-left {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.header-right {
		display: flex;
		align-items: center;
		gap: var(--space-4);
	}

	/* Project Button */
	.project-btn {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-1-5) var(--space-3);
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		color: var(--text-primary);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.project-btn:hover {
		background: var(--bg-tertiary);
		border-color: var(--border-strong);
	}

	.project-btn:focus-visible {
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 3px var(--accent-glow);
	}

	.project-icon {
		color: var(--accent-primary);
		display: flex;
		align-items: center;
	}

	.project-name {
		max-width: 150px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.project-chevron {
		color: var(--text-muted);
		display: flex;
		align-items: center;
	}

	/* Page Info */
	.page-info {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.separator {
		color: var(--text-muted);
		font-size: var(--text-lg);
		font-weight: var(--font-regular);
	}

	.page-title {
		font-size: var(--text-base);
		font-weight: var(--font-semibold);
		color: var(--text-primary);
		margin: 0;
		letter-spacing: normal;
		text-transform: none;
	}

	/* Command Hint */
	.cmd-hint {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		color: var(--text-muted);
		font-size: var(--text-xs);
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		padding: var(--space-1-5) var(--space-3);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.cmd-hint:hover {
		background: var(--bg-tertiary);
		border-color: var(--border-strong);
		color: var(--text-secondary);
	}

	.cmd-hint-label {
		font-size: var(--text-sm);
	}

	.cmd-hint kbd {
		font-family: var(--font-mono);
		font-size: var(--text-2xs);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-sm);
		padding: var(--space-0-5) var(--space-1-5);
		box-shadow: 0 1px 0 var(--border-default);
	}

	/* New Task Button */
	.new-task-btn {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}
</style>
