<script lang="ts">
	import Icon from '$lib/components/ui/Icon.svelte';

	export type BoardViewMode = 'flat' | 'swimlane';

	interface Props {
		value: BoardViewMode;
		onChange: (mode: BoardViewMode) => void;
	}

	let { value, onChange }: Props = $props();

	let isOpen = $state(false);
	let dropdownRef = $state<HTMLDivElement | null>(null);

	const viewOptions: { id: BoardViewMode; label: string; description: string }[] = [
		{ id: 'flat', label: 'Flat', description: 'All tasks in columns' },
		{ id: 'swimlane', label: 'By Initiative', description: 'Grouped by initiative' }
	];

	let selectedOption = $derived(
		viewOptions.find(opt => opt.id === value) ?? viewOptions[0]
	);

	function handleSelect(mode: BoardViewMode) {
		onChange(mode);
		isOpen = false;
	}

	function handleClickOutside(event: MouseEvent) {
		if (dropdownRef && !dropdownRef.contains(event.target as Node)) {
			isOpen = false;
		}
	}

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

<div class="view-mode-dropdown" bind:this={dropdownRef}>
	<button
		class="dropdown-trigger"
		onclick={() => (isOpen = !isOpen)}
		aria-haspopup="listbox"
		aria-expanded={isOpen}
	>
		<Icon name="layout" size={14} />
		<span class="trigger-text">{selectedOption.label}</span>
		<Icon name={isOpen ? 'chevron-up' : 'chevron-down'} size={14} />
	</button>

	{#if isOpen}
		<div class="dropdown-menu" role="listbox">
			{#each viewOptions as option (option.id)}
				<button
					class="dropdown-item"
					class:selected={value === option.id}
					onclick={() => handleSelect(option.id)}
					role="option"
					aria-selected={value === option.id}
				>
					<span class="item-indicator">
						{#if value === option.id}
							<span class="indicator-dot filled"></span>
						{:else}
							<span class="indicator-dot"></span>
						{/if}
					</span>
					<div class="item-content">
						<span class="item-label">{option.label}</span>
						<span class="item-description">{option.description}</span>
					</div>
				</button>
			{/each}
		</div>
	{/if}
</div>

<style>
	.view-mode-dropdown {
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
		min-width: 130px;
	}

	.dropdown-trigger:hover {
		border-color: var(--border-strong);
	}

	.dropdown-trigger:focus {
		outline: none;
		border-color: var(--accent-primary);
		box-shadow: 0 0 0 3px var(--accent-glow);
	}

	.trigger-text {
		flex: 1;
		text-align: left;
		white-space: nowrap;
	}

	.dropdown-menu {
		position: absolute;
		top: calc(100% + var(--space-1));
		left: 0;
		min-width: 180px;
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
		align-items: flex-start;
		gap: var(--space-2);
		width: 100%;
		padding: var(--space-2) var(--space-3);
		background: transparent;
		border: none;
		border-radius: var(--radius-md);
		cursor: pointer;
		transition: all var(--duration-fast) var(--ease-out);
		text-align: left;
	}

	.dropdown-item:hover {
		background: var(--bg-tertiary);
	}

	.dropdown-item.selected {
		background: var(--accent-subtle);
	}

	.item-indicator {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 16px;
		height: 20px;
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

	.item-content {
		display: flex;
		flex-direction: column;
		gap: var(--space-0-5);
	}

	.item-label {
		font-size: var(--text-sm);
		font-weight: var(--font-medium);
		color: var(--text-primary);
	}

	.dropdown-item.selected .item-label {
		color: var(--accent-primary);
	}

	.item-description {
		font-size: var(--text-xs);
		color: var(--text-muted);
	}

	.dropdown-item:hover .indicator-dot {
		border-color: var(--text-secondary);
	}

	.dropdown-item.selected .indicator-dot {
		border-color: var(--accent-primary);
	}
</style>
