<script lang="ts">
	import { goto } from '$app/navigation';
	import { formatShortcut } from '$lib/utils/platform';

	interface Props {
		open: boolean;
		onClose: () => void;
	}

	let { open, onClose }: Props = $props();

	let searchQuery = $state('');
	let selectedIndex = $state(0);
	let inputRef = $state<HTMLInputElement | null>(null);

	interface Command {
		id: string;
		label: string;
		description?: string;
		icon: string;
		shortcut?: string;
		action: () => void;
		category: string;
	}

	const commands: Command[] = [
		{
			id: 'new-task',
			label: 'New Task',
			description: 'Create a new task',
			icon: '\u002B', // +
			shortcut: 'N',
			action: () => {
				onClose();
				// Trigger new task modal
				window.dispatchEvent(new CustomEvent('orc:new-task'));
			},
			category: 'Tasks'
		},
		{
			id: 'go-tasks',
			label: 'Go to Tasks',
			description: 'View all tasks',
			icon: '\u25A0', // ■
			action: () => {
				onClose();
				goto('/');
			},
			category: 'Navigation'
		},
		{
			id: 'go-prompts',
			label: 'Go to Prompts',
			description: 'Manage prompt templates',
			icon: '\u2630', // ☰
			action: () => {
				onClose();
				goto('/prompts');
			},
			category: 'Navigation'
		},
		{
			id: 'go-claudemd',
			label: 'Go to CLAUDE.md',
			description: 'Edit project instructions',
			icon: '\u2630', // ☰
			action: () => {
				onClose();
				goto('/claudemd');
			},
			category: 'Navigation'
		},
		{
			id: 'go-skills',
			label: 'Go to Skills',
			description: 'Manage Claude skills',
			icon: '\u26A1', // ⚡
			action: () => {
				onClose();
				goto('/skills');
			},
			category: 'Navigation'
		},
		{
			id: 'go-hooks',
			label: 'Go to Hooks',
			description: 'Configure event hooks',
			icon: '\u21BB', // ↻
			action: () => {
				onClose();
				goto('/hooks');
			},
			category: 'Navigation'
		},
		{
			id: 'go-mcp',
			label: 'Go to MCP Servers',
			description: 'Manage MCP integrations',
			icon: '\u229A', // ⊚
			action: () => {
				onClose();
				goto('/mcp');
			},
			category: 'Navigation'
		},
		{
			id: 'go-tools',
			label: 'Go to Tools',
			description: 'Tool permissions',
			icon: '\u2692', // ⚒
			action: () => {
				onClose();
				goto('/tools');
			},
			category: 'Navigation'
		},
		{
			id: 'go-agents',
			label: 'Go to Agents',
			description: 'Sub-agent configurations',
			icon: '\u2726', // ✦
			action: () => {
				onClose();
				goto('/agents');
			},
			category: 'Navigation'
		},
		{
			id: 'go-scripts',
			label: 'Go to Scripts',
			description: 'Script registry',
			icon: '\u2630', // ☰
			action: () => {
				onClose();
				goto('/scripts');
			},
			category: 'Navigation'
		},
		{
			id: 'go-settings',
			label: 'Go to Settings',
			description: 'Project settings',
			icon: '\u2699', // ⚙
			action: () => {
				onClose();
				goto('/settings');
			},
			category: 'Navigation'
		},
		{
			id: 'go-config',
			label: 'Go to Config',
			description: 'Orc configuration',
			icon: '\u2699', // ⚙
			action: () => {
				onClose();
				goto('/config');
			},
			category: 'Navigation'
		},
		{
			id: 'switch-project',
			label: 'Switch Project',
			description: 'Change active project',
			icon: '\u21C4', // ⇄
			shortcut: 'P',
			action: () => {
				onClose();
				window.dispatchEvent(new CustomEvent('orc:switch-project'));
			},
			category: 'Projects'
		},
		{
			id: 'toggle-sidebar',
			label: 'Toggle Sidebar',
			description: 'Show/hide navigation',
			icon: '\u2630', // ☰
			shortcut: 'B',
			action: () => {
				onClose();
				window.dispatchEvent(new CustomEvent('orc:toggle-sidebar'));
			},
			category: 'View'
		}
	];

	const filteredCommands = $derived(() => {
		if (!searchQuery.trim()) return commands;
		const query = searchQuery.toLowerCase();
		return commands.filter(
			(cmd) =>
				cmd.label.toLowerCase().includes(query) ||
				cmd.description?.toLowerCase().includes(query) ||
				cmd.category.toLowerCase().includes(query)
		);
	});

	// Group by category
	const groupedCommands = $derived(() => {
		const filtered = filteredCommands();
		const groups: Record<string, Command[]> = {};
		for (const cmd of filtered) {
			if (!groups[cmd.category]) {
				groups[cmd.category] = [];
			}
			groups[cmd.category].push(cmd);
		}
		return groups;
	});

	// Flat list for keyboard navigation
	const flatCommands = $derived(() => filteredCommands());

	$effect(() => {
		if (open && inputRef) {
			inputRef.focus();
			searchQuery = '';
			selectedIndex = 0;
		}
	});

	function handleKeydown(e: KeyboardEvent) {
		const cmds = flatCommands();

		switch (e.key) {
			case 'ArrowDown':
				e.preventDefault();
				selectedIndex = Math.min(selectedIndex + 1, cmds.length - 1);
				break;
			case 'ArrowUp':
				e.preventDefault();
				selectedIndex = Math.max(selectedIndex - 1, 0);
				break;
			case 'Enter':
				e.preventDefault();
				if (cmds[selectedIndex]) {
					cmds[selectedIndex].action();
				}
				break;
			case 'Escape':
				onClose();
				break;
		}
	}

	function handleBackdropClick(e: MouseEvent) {
		if (e.target === e.currentTarget) {
			onClose();
		}
	}

	// Reset selected index when search changes
	$effect(() => {
		searchQuery;
		selectedIndex = 0;
	});
</script>

{#if open}
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="palette-backdrop"
		role="dialog"
		aria-modal="true"
		aria-label="Command palette"
		tabindex="-1"
		onclick={handleBackdropClick}
		onkeydown={handleKeydown}
	>
		<div class="palette-content">
			<!-- Search Input -->
			<div class="palette-search">
				<svg class="search-icon" xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<circle cx="11" cy="11" r="8" />
					<path d="m21 21-4.35-4.35" />
				</svg>
				<input
					bind:this={inputRef}
					bind:value={searchQuery}
					type="text"
					placeholder="Type a command or search..."
					class="search-input"
					aria-label="Search commands"
				/>
				<kbd class="search-hint">esc</kbd>
			</div>

			<!-- Results -->
			<div class="palette-results">
				{#each Object.entries(groupedCommands()) as [category, cmds]}
					<div class="result-group">
						<div class="group-label">{category}</div>
						{#each cmds as cmd, i}
							{@const globalIndex = flatCommands().indexOf(cmd)}
							<button
								class="result-item"
								class:selected={globalIndex === selectedIndex}
								onclick={() => cmd.action()}
								onmouseenter={() => (selectedIndex = globalIndex)}
							>
								<span class="item-icon">{cmd.icon}</span>
								<div class="item-content">
									<span class="item-label">{cmd.label}</span>
									{#if cmd.description}
										<span class="item-description">{cmd.description}</span>
									{/if}
								</div>
								{#if cmd.shortcut}
									<kbd class="item-shortcut">{formatShortcut(cmd.shortcut)}</kbd>
								{/if}
							</button>
						{/each}
					</div>
				{:else}
					<div class="no-results">
						<span class="no-results-icon">?</span>
						<p>No commands found</p>
					</div>
				{/each}
			</div>

			<!-- Footer -->
			<div class="palette-footer">
				<div class="footer-hint">
					<kbd>\u2191</kbd><kbd>\u2193</kbd> to navigate
				</div>
				<div class="footer-hint">
					<kbd>\u21B5</kbd> to select
				</div>
			</div>
		</div>
	</div>
{/if}

<style>
	.palette-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.6);
		backdrop-filter: blur(4px);
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding: var(--space-16) var(--space-4);
		z-index: 1100;
		animation: fade-in var(--duration-fast) var(--ease-out);
	}

	.palette-content {
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-xl);
		box-shadow: var(--shadow-2xl);
		width: 100%;
		max-width: 560px;
		max-height: 70vh;
		overflow: hidden;
		display: flex;
		flex-direction: column;
		animation: modal-content-in var(--duration-fast) var(--ease-out);
	}

	/* Search */
	.palette-search {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-4);
		border-bottom: 1px solid var(--border-subtle);
	}

	.search-icon {
		flex-shrink: 0;
		color: var(--text-muted);
	}

	.search-input {
		flex: 1;
		background: transparent;
		border: none;
		font-size: var(--text-base);
		color: var(--text-primary);
		outline: none;
	}

	.search-input::placeholder {
		color: var(--text-muted);
	}

	.search-hint {
		flex-shrink: 0;
		padding: var(--space-0-5) var(--space-1-5);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-sm);
		font-family: var(--font-mono);
		font-size: var(--text-2xs);
		color: var(--text-muted);
	}

	/* Results */
	.palette-results {
		flex: 1;
		overflow-y: auto;
		padding: var(--space-2);
	}

	.result-group {
		margin-bottom: var(--space-2);
	}

	.group-label {
		padding: var(--space-2) var(--space-3);
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		text-transform: uppercase;
		letter-spacing: var(--tracking-wider);
		color: var(--text-muted);
	}

	.result-item {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		width: 100%;
		padding: var(--space-2-5) var(--space-3);
		background: transparent;
		border: none;
		border-radius: var(--radius-md);
		text-align: left;
		cursor: pointer;
		transition: background var(--duration-fast) var(--ease-out);
	}

	.result-item:hover,
	.result-item.selected {
		background: var(--bg-tertiary);
	}

	.result-item.selected {
		outline: 1px solid var(--accent-muted);
	}

	.item-icon {
		flex-shrink: 0;
		width: 24px;
		height: 24px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--bg-tertiary);
		border-radius: var(--radius-sm);
		font-size: var(--text-sm);
		color: var(--text-secondary);
	}

	.result-item.selected .item-icon {
		background: var(--accent-subtle);
		color: var(--accent-primary);
	}

	.item-content {
		flex: 1;
		min-width: 0;
	}

	.item-label {
		display: block;
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
	}

	.item-description {
		display: block;
		font-size: var(--text-xs);
		color: var(--text-muted);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.item-shortcut {
		flex-shrink: 0;
		padding: var(--space-0-5) var(--space-1-5);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-sm);
		font-family: var(--font-mono);
		font-size: var(--text-2xs);
		color: var(--text-muted);
	}

	/* No results */
	.no-results {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: var(--space-8);
		text-align: center;
	}

	.no-results-icon {
		width: 40px;
		height: 40px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--bg-tertiary);
		border-radius: var(--radius-full);
		font-size: var(--text-lg);
		color: var(--text-muted);
		margin-bottom: var(--space-3);
	}

	.no-results p {
		font-size: var(--text-sm);
		color: var(--text-muted);
	}

	/* Footer */
	.palette-footer {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--space-6);
		padding: var(--space-3);
		border-top: 1px solid var(--border-subtle);
		background: var(--bg-tertiary);
	}

	.footer-hint {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.footer-hint kbd {
		padding: var(--space-0-5) var(--space-1);
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-sm);
		font-family: var(--font-mono);
		font-size: var(--text-2xs);
	}
</style>
