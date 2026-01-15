<script lang="ts">
	import type { Snippet } from 'svelte';
	import Breadcrumbs from '$lib/components/ui/Breadcrumbs.svelte';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();
</script>

<div class="environment-layout">
	<Breadcrumbs />
	{@render children()}
</div>

<style>
	.environment-layout {
		width: 100%;
	}
</style>
