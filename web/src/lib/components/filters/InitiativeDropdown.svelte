<script lang="ts">
	import Icon from '$lib/components/ui/Icon.svelte';
	import {
		initiatives,
		currentInitiativeId,
		currentInitiative,
		initiativeProgress,
		selectInitiative
	} from '$lib/stores/initiative';
	import { tasks as tasksStore } from '$lib/stores/tasks';
	import type { Initiative } from '$lib/types';

	// Special value for showing only unassigned tasks
	const UNASSIGNED_VALUE = '__unassigned__';

	// Dropdown open state
	let isOpen = $state(false);
	let dropdownRef = $state<HTMLDivElement | null>(null);

	// Get reactive values from stores
	let allInitiatives = $derived($initiatives);
	let selectedId = $derived($currentInitiativeId);
	let selectedInitiative = $derived($currentInitiative);
	let progress = $derived($initiativeProgress);
	let allTasks = $derived($tasksStore);

	// Sort initiatives: active first, then by title
	let sortedInitiatives = $derived(
		[...allInitiatives].sort((a, b) => {
			if (a.status === 'active' && b.status !== 'active') return -1;
			if (b.status === 'active' && a.status !== 'active') return 1;
			return a.title.localeCompare(b.title);
		})
	);

	// Count unassigned tasks (those with no initiative_id)
	let unassignedCount = $derived(
		allTasks.filter(t => !t.initiative_id).length
	);

	// Get task count for an initiative
	function getTaskCount(id: string): number {
		const p = progress.get(id);
		return p?.total ?? 0;
	}

	// Truncate long titles
	function truncateTitle(title: string, maxLength = 24): string {
		if (title.length <= maxLength) return title;
		return title.slice(0, maxLength - 1) + '…';
	}

	// Handle selection
	function handleSelect(id: string | null) {
		if (id === UNASSIGNED_VALUE) {
			// Special handling for unassigned - we'll use a sentinel value
			// The actual filtering happens in the parent component
			selectInitiative(UNASSIGNED_VALUE);
		} else {
			selectInitiative(id);
		}
		isOpen = false;
	}

	// Get display text for current selection
	let displayText = $derived.by(() => {
		if (selectedId === UNASSIGNED_VALUE) {
			return 'Unassigned';
		}
		if (selectedInitiative) {
			return truncateTitle(selectedInitiative.title);
		}
		return 'All initiatives';
	});

	// Is unassigned selected?
	let isUnassigned = $derived(selectedId === UNASSIGNED_VALUE);

	// Close dropdown when clicking outside
	function handleClickOutside(event: MouseEvent) {
		if (dropdownRef && !dropdownRef.contains(event.target as Node)) {
			isOpen = false;
		}
	}

	// Close dropdown on Escape
	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && isOpen) {
			isOpen = false;
		}
	}

	$effect(() => {
		if (isOpen) {
			document.addEventListener('click', handleClickOutside);
			document.addEventListener('keydown', handleKeydown);
		}
		return () => {
			document.removeEventListener('click', handleClickOutside);
			document.removeEventListener('keydown', handleKeydown);
		};
	});
</script>

<div class="initiative-dropdown" bind:this={dropdownRef}>
	<button
		class="dropdown-trigger"
		class:active={selectedId !== null}
		onclick={() => (isOpen = !isOpen)}
		aria-haspopup="listbox"
		aria-expanded={isOpen}
	>
		<span class="trigger-text">{displayText}</span>
		<Icon name={isOpen ? 'chevron-up' : 'chevron-down'} size={14} />
	</button>

	{#if isOpen}
		<div class="dropdown-menu" role="listbox">
			<!-- All initiatives -->
			<button
				class="dropdown-item"
				class:selected={selectedId === null}
				onclick={() => handleSelect(null)}
				role="option"
				aria-selected={selectedId === null}
			>
				<span class="item-indicator">
					{#if selectedId === null}
						<span class="indicator-dot filled"></span>
					{:else}
						<span class="indicator-dot"></span>
					{/if}
				</span>
				<span class="item-label">All initiatives</span>
			</button>

			<!-- Unassigned tasks -->
			<button
				class="dropdown-item"
				class:selected={isUnassigned}
				onclick={() => handleSelect(UNASSIGNED_VALUE)}
				role="option"
				aria-selected={isUnassigned}
			>
				<span class="item-indicator">
					{#if isUnassigned}
						<span class="indicator-dot filled"></span>
					{:else}
						<span class="indicator-dot"></span>
					{/if}
				</span>
				<span class="item-label">Unassigned</span>
				<span class="item-count">{unassignedCount}</span>
			</button>

			{#if sortedInitiatives.length > 0}
				<div class="dropdown-divider"></div>

				<!-- Initiative list -->
				{#each sortedInitiatives as initiative (initiative.id)}
					{@const count = getTaskCount(initiative.id)}
					<button
						class="dropdown-item"
						class:selected={selectedId === initiative.id}
						onclick={() => handleSelect(initiative.id)}
						role="option"
						aria-selected={selectedId === initiative.id}
						title={initiative.title}
					>
						<span class="item-indicator">
							{#if selectedId === initiative.id}
								<span class="indicator-dot filled"></span>
							{:else}
								<span class="indicator-dot"></span>
							{/if}
						</span>
						<span class="item-label">{truncateTitle(initiative.title)}</span>
						<span class="item-count">{count}</span>
					</button>
				{/each}
			{/if}
		</div>
	{/if}
</div>

<style>
	.initiative-dropdown {
		position: relative;
	}

	.dropdown-trigger {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		background: var(--bg-secondary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		color: var(--text-primary);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
		min-width: 140px;
	}

	.dropdown-trigger:hover {
		border-color: var(--border-strong);
	}

	.dropdown-trigger:focus {
		outline: none;
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 3px var(--accent-glow);
	}

	.dropdown-trigger.active {
		background: var(--accent-subtle);
		border-color: var(--accent-primary);
		color: var(--accent-primary);
	}

	.trigger-text {
		flex: 1;
		text-align: left;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.dropdown-menu {
		position: absolute;
		top: calc(100% + var(--space-1));
		left: 0;
		min-width: 200px;
		max-height: 300px;
		overflow-y: auto;
		background: var(--bg-primary);
		border: 1px solid var(--border-default);
		border-radius: var(--radius-lg);
		box-shadow: var(--shadow-lg);
		z-index: var(--z-dropdown);
		padding: var(--space-1);
		animation: dropdown-enter var(--duration-fast) var(--ease-out);
	}

	@keyframes dropdown-enter {
		from {
			opacity: 0;
			transform: translateY(-4px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.dropdown-item {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		width: 100%;
		padding: var(--space-2) var(--space-3);
		background: transparent;
		border: none;
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		color: var(--text-secondary);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
		text-align: left;
	}

	.dropdown-item:hover {
		background: var(--bg-tertiary);
		color: var(--text-primary);
	}

	.dropdown-item.selected {
		background: var(--accent-subtle);
		color: var(--accent-primary);
	}

	.item-indicator {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 16px;
		height: 16px;
		flex-shrink: 0;
	}

	.indicator-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		border: 1.5px solid var(--text-muted);
		background: transparent;
		transition: all var(--duration-fast) var(--ease-out);
	}

	.indicator-dot.filled {
		background: var(--accent-primary);
		border-color: var(--accent-primary);
	}

	.dropdown-item:hover .indicator-dot {
		border-color: var(--text-secondary);
	}

	.dropdown-item.selected .indicator-dot {
		border-color: var(--accent-primary);
	}

	.item-label {
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.item-count {
		font-size: var(--text-xs);
		font-family: var(--font-mono);
		color: var(--text-muted);
		padding: var(--space-0-5) var(--space-1-5);
		background: var(--bg-tertiary);
		border-radius: var(--radius-full);
		flex-shrink: 0;
	}

	.dropdown-item.selected .item-count {
		background: var(--accent-primary);
		color: var(--text-inverse);
	}

	.dropdown-divider {
		height: 1px;
		background: var(--border-subtle);
		margin: var(--space-1) var(--space-2);
	}
</style>
