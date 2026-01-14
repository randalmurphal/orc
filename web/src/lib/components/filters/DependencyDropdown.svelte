<script lang="ts">
	import Icon from '$lib/components/ui/Icon.svelte';
	import { currentDependencyStatus, selectDependencyStatus, DEPENDENCY_OPTIONS, type DependencyStatusFilter } from '$lib/stores/dependency';
	import { tasks as tasksStore } from '$lib/stores/tasks';

	// Dropdown open state
	let isOpen = $state(false);
	let dropdownRef = $state<HTMLDivElement | null>(null);

	// Get reactive values from stores
	let selectedStatus = $derived($currentDependencyStatus);
	let allTasks = $derived($tasksStore);

	// Count tasks by dependency status
	let statusCounts = $derived.by(() => {
		const counts = {
			all: allTasks.length,
			blocked: 0,
			ready: 0,
			none: 0
		};
		for (const task of allTasks) {
			if (task.dependency_status === 'blocked') counts.blocked++;
			else if (task.dependency_status === 'ready') counts.ready++;
			else if (task.dependency_status === 'none') counts.none++;
		}
		return counts;
	});

	// Get count for a specific filter
	function getCount(value: DependencyStatusFilter): number {
		return statusCounts[value] ?? 0;
	}

	// Handle selection
	function handleSelect(value: DependencyStatusFilter) {
		selectDependencyStatus(value === 'all' ? null : value);
		isOpen = false;
	}

	// Get display text for current selection
	let displayText = $derived.by(() => {
		const option = DEPENDENCY_OPTIONS.find(o => o.value === selectedStatus);
		return option?.label ?? 'All dependencies';
	});

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

<div class="dependency-dropdown" bind:this={dropdownRef}>
	<button
		class="dropdown-trigger"
		class:active={selectedStatus !== 'all'}
		onclick={() => (isOpen = !isOpen)}
		aria-haspopup="listbox"
		aria-expanded={isOpen}
	>
		<span class="trigger-text">{displayText}</span>
		<Icon name={isOpen ? 'chevron-up' : 'chevron-down'} size={14} />
	</button>

	{#if isOpen}
		<div class="dropdown-menu" role="listbox">
			{#each DEPENDENCY_OPTIONS as option (option.value)}
				{@const count = getCount(option.value)}
				<button
					class="dropdown-item"
					class:selected={selectedStatus === option.value}
					onclick={() => handleSelect(option.value)}
					role="option"
					aria-selected={selectedStatus === option.value}
				>
					<span class="item-indicator">
						{#if selectedStatus === option.value}
							<span class="indicator-dot filled"></span>
						{:else}
							<span class="indicator-dot"></span>
						{/if}
					</span>
					<span class="item-label">{option.label}</span>
					{#if option.value !== 'all'}
						<span class="item-count">{count}</span>
					{/if}
				</button>
			{/each}
		</div>
	{/if}
</div>

<style>
	.dependency-dropdown {
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
		min-width: 180px;
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
</style>
