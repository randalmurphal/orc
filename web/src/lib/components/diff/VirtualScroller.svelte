<script lang="ts" generics="T">
	import { onMount } from 'svelte';
	import type { Snippet } from 'svelte';

	interface Props {
		items: T[];
		itemHeight?: number;
		buffer?: number;
		children: Snippet<[{ item: T; index: number }]>;
	}

	let { items, itemHeight = 22, buffer = 10, children }: Props = $props();

	let containerEl = $state<HTMLElement | null>(null);
	let scrollTop = $state(0);
	let containerHeight = $state(400);

	const visibleStart = $derived(Math.max(0, Math.floor(scrollTop / itemHeight) - buffer));
	const visibleEnd = $derived(
		Math.min(items.length, visibleStart + Math.ceil(containerHeight / itemHeight) + buffer * 2)
	);
	const visibleItems = $derived(items.slice(visibleStart, visibleEnd));
	const topPadding = $derived(visibleStart * itemHeight);
	const bottomPadding = $derived((items.length - visibleEnd) * itemHeight);
	const totalHeight = $derived(items.length * itemHeight);

	onMount(() => {
		if (containerEl) {
			containerHeight = containerEl.clientHeight;

			const resizeObserver = new ResizeObserver((entries) => {
				for (const entry of entries) {
					containerHeight = entry.contentRect.height;
				}
			});

			resizeObserver.observe(containerEl);

			return () => {
				resizeObserver.disconnect();
			};
		}
	});

	function handleScroll() {
		if (containerEl) {
			scrollTop = containerEl.scrollTop;
		}
	}
</script>

<div bind:this={containerEl} class="virtual-scroller" onscroll={handleScroll}>
	<div class="virtual-content" style:height="{totalHeight}px">
		<div class="virtual-spacer" style:height="{topPadding}px"></div>
		{#each visibleItems as item, i (visibleStart + i)}
			{@render children({ item, index: visibleStart + i })}
		{/each}
		<div class="virtual-spacer" style:height="{bottomPadding}px"></div>
	</div>
</div>

<style>
	.virtual-scroller {
		height: 100%;
		overflow-y: auto;
		/* Max height configurable via parent container */
	}

	.virtual-content {
		position: relative;
	}

	.virtual-spacer {
		flex-shrink: 0;
	}
</style>
