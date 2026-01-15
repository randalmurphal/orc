<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	onMount(() => {
		goto('/environment/orchestrator/prompts', { replaceState: true });
	});
</script>

<div class="redirect">Redirecting to Prompts...</div>

<style>
	.redirect {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 200px;
		color: var(--text-muted);
	}
</style>
