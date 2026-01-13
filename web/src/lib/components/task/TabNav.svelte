<script lang="ts">
	import { onMount, onDestroy } from 'svelte';

	export type TabId = 'timeline' | 'changes' | 'transcript' | 'attachments';

	interface TabConfig {
		id: TabId;
		label: string;
		badge?: string | number | null;
		badgeType?: 'default' | 'success' | 'warning' | 'danger' | 'info';
	}

	interface Props {
		tabs: TabConfig[];
		activeTab: TabId;
		onTabChange: (tab: TabId) => void;
	}

	let { tabs, activeTab, onTabChange }: Props = $props();

	let tabsRef: HTMLDivElement | null = null;

	function handleKeyDown(e: KeyboardEvent) {
		if (!tabsRef?.contains(document.activeElement)) return;

		const currentIndex = tabs.findIndex((t) => t.id === activeTab);
		let newIndex = currentIndex;

		if (e.key === 'ArrowRight') {
			e.preventDefault();
			newIndex = (currentIndex + 1) % tabs.length;
		} else if (e.key === 'ArrowLeft') {
			e.preventDefault();
			newIndex = (currentIndex - 1 + tabs.length) % tabs.length;
		} else if (e.key === 'Home') {
			e.preventDefault();
			newIndex = 0;
		} else if (e.key === 'End') {
			e.preventDefault();
			newIndex = tabs.length - 1;
		}

		if (newIndex !== currentIndex) {
			onTabChange(tabs[newIndex].id);
			// Focus the new tab button
			const buttons = tabsRef?.querySelectorAll('.tab-button');
			(buttons?.[newIndex] as HTMLButtonElement)?.focus();
		}
	}

	onMount(() => {
		document.addEventListener('keydown', handleKeyDown);
	});

	onDestroy(() => {
		document.removeEventListener('keydown', handleKeyDown);
	});

	function getBadgeClass(type?: string): string {
		switch (type) {
			case 'success':
				return 'badge-success';
			case 'warning':
				return 'badge-warning';
			case 'danger':
				return 'badge-danger';
			case 'info':
				return 'badge-info';
			default:
				return 'badge-default';
		}
	}
</script>

<div class="tab-nav" bind:this={tabsRef} role="tablist" aria-label="Task details tabs">
	{#each tabs as tab (tab.id)}
		<button
			class="tab-button"
			class:active={activeTab === tab.id}
			role="tab"
			aria-selected={activeTab === tab.id}
			aria-controls={`tabpanel-${tab.id}`}
			tabindex={activeTab === tab.id ? 0 : -1}
			onclick={() => onTabChange(tab.id)}
		>
			<span class="tab-label">{tab.label}</span>
			{#if tab.badge !== null && tab.badge !== undefined}
				<span class="tab-badge {getBadgeClass(tab.badgeType)}">
					{tab.badge}
				</span>
			{/if}
		</button>
	{/each}
	<div class="tab-indicator" aria-hidden="true"></div>
</div>

<style>
	.tab-nav {
		display: flex;
		gap: var(--space-1);
		padding: var(--space-1);
		background: var(--bg-secondary);
		border: 1px solid var(--border-subtle);
		border-radius: var(--radius-lg);
		overflow-x: auto;
		scrollbar-width: none;
		-ms-overflow-style: none;
	}

	.tab-nav::-webkit-scrollbar {
		display: none;
	}

	.tab-button {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-4);
		background: transparent;
		border: none;
		border-radius: var(--radius-md);
		color: var(--text-secondary);
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
		white-space: nowrap;
		position: relative;
	}

	.tab-button:hover:not(.active) {
		background: var(--bg-tertiary);
		color: var(--text-primary);
	}

	.tab-button.active {
		background: var(--bg-primary);
		color: var(--text-primary);
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
	}

	.tab-button.active::after {
		content: '';
		position: absolute;
		bottom: 0;
		left: 50%;
		transform: translateX(-50%);
		width: calc(100% - var(--space-4));
		height: 2px;
		background: var(--accent-primary);
		border-radius: var(--radius-full);
	}

	.tab-button:focus-visible {
		outline: 2px solid var(--accent-primary);
		outline-offset: 2px;
	}

	.tab-label {
		display: inline-block;
	}

	.tab-badge {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 18px;
		height: 18px;
		padding: 0 var(--space-1-5);
		font-size: var(--text-2xs);
		font-weight: var(--font-semibold);
		border-radius: var(--radius-full);
	}

	.badge-default {
		background: var(--bg-tertiary);
		color: var(--text-secondary);
	}

	.badge-success {
		background: var(--status-success-bg);
		color: var(--status-success);
	}

	.badge-warning {
		background: var(--status-warning-bg);
		color: var(--status-warning);
	}

	.badge-danger {
		background: var(--status-danger-bg);
		color: var(--status-danger);
	}

	.badge-info {
		background: var(--status-info-bg);
		color: var(--status-info);
	}

	/* Active tab badge adjustments */
	.tab-button.active .badge-default {
		background: var(--bg-tertiary);
	}

	/* Responsive: horizontal scroll on mobile */
	@media (max-width: 640px) {
		.tab-nav {
			gap: var(--space-0-5);
			padding: var(--space-0-5);
		}

		.tab-button {
			padding: var(--space-1-5) var(--space-3);
			font-size: var(--text-xs);
		}

		.tab-badge {
			min-width: 16px;
			height: 16px;
			font-size: 10px;
		}
	}
</style>
