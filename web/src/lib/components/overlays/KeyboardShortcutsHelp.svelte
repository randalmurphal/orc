<script lang="ts">
	import Modal from './Modal.svelte';
	import { getShortcutManager, SHORTCUTS } from '$lib/shortcuts';

	interface Props {
		open: boolean;
		onClose: () => void;
	}

	let { open, onClose }: Props = $props();

	// Organize shortcuts by category
	const categories = [
		{
			name: 'Global',
			shortcuts: [
				{ keys: '⌘ K', description: 'Open command palette' },
				{ keys: '⌘ N', description: 'Create new task' },
				{ keys: '⌘ B', description: 'Toggle sidebar' },
				{ keys: '⌘ P', description: 'Switch project' },
				{ keys: '/', description: 'Focus search' },
				{ keys: '?', description: 'Show this help' },
				{ keys: 'Esc', description: 'Close overlay' }
			]
		},
		{
			name: 'Navigation',
			shortcuts: [
				{ keys: 'g d', description: 'Go to dashboard' },
				{ keys: 'g t', description: 'Go to tasks' },
				{ keys: 'g s', description: 'Go to settings' },
				{ keys: 'g p', description: 'Go to prompts' },
				{ keys: 'g h', description: 'Go to hooks' },
				{ keys: 'g k', description: 'Go to skills' }
			]
		},
		{
			name: 'Task List',
			shortcuts: [
				{ keys: 'j', description: 'Select next task' },
				{ keys: 'k', description: 'Select previous task' },
				{ keys: 'Enter', description: 'Open selected task' },
				{ keys: 'r', description: 'Run selected task' },
				{ keys: 'p', description: 'Pause selected task' },
				{ keys: 'd', description: 'Delete selected task' }
			]
		}
	];
</script>

<Modal {open} {onClose} title="Keyboard Shortcuts" size="md">
	<div class="shortcuts-help">
		{#each categories as category}
			<section class="category">
				<h3 class="category-title">{category.name}</h3>
				<div class="shortcuts-list">
					{#each category.shortcuts as shortcut}
						<div class="shortcut-row">
							<div class="shortcut-keys">
								{#each shortcut.keys.split(' ') as key}
									<kbd class="key">{key}</kbd>
								{/each}
							</div>
							<span class="shortcut-description">{shortcut.description}</span>
						</div>
					{/each}
				</div>
			</section>
		{/each}
	</div>
</Modal>

<style>
	.shortcuts-help {
		display: flex;
		flex-direction: column;
		gap: var(--space-6);
	}

	.category {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
	}

	.category-title {
		font-size: var(--text-sm);
		font-weight: var(--font-semibold);
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		margin: 0;
	}

	.shortcuts-list {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
	}

	.shortcut-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: var(--space-2) 0;
		border-bottom: 1px solid var(--border-subtle);
	}

	.shortcut-row:last-child {
		border-bottom: none;
	}

	.shortcut-keys {
		display: flex;
		align-items: center;
		gap: var(--space-1);
	}

	.key {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 24px;
		height: 24px;
		padding: 0 var(--space-2);
		background: var(--bg-tertiary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-sm);
		font-family: var(--font-mono);
		font-size: var(--text-xs);
		color: var(--text-primary);
		box-shadow: 0 1px 2px rgba(0, 0, 0, 0.1);
	}

	.shortcut-description {
		font-size: var(--text-sm);
		color: var(--text-secondary);
	}
</style>
