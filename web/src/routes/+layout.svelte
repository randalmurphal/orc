<script lang="ts">
	import '../app.css';
</script>

<div class="app">
	<header>
		<nav>
			<a href="/" class="logo">orc</a>
			<div class="nav-links">
				<a href="/">Tasks</a>
				<a href="/config">Config</a>
			</div>
		</nav>
	</header>

	<main>
		<slot />
	</main>
</div>

<style>
	.app {
		display: flex;
		flex-direction: column;
		min-height: 100vh;
	}

	header {
		background: var(--bg-secondary);
		border-bottom: 1px solid var(--border-color);
		padding: 0 1.5rem;
	}

	nav {
		display: flex;
		align-items: center;
		justify-content: space-between;
		max-width: 1200px;
		margin: 0 auto;
		height: 56px;
	}

	.logo {
		font-size: 1.25rem;
		font-weight: 700;
		color: var(--text-primary);
	}

	.nav-links {
		display: flex;
		gap: 1.5rem;
	}

	.nav-links a {
		color: var(--text-secondary);
		font-size: 0.875rem;
	}

	.nav-links a:hover {
		color: var(--text-primary);
		text-decoration: none;
	}

	main {
		flex: 1;
		max-width: 1200px;
		margin: 0 auto;
		padding: 2rem 1.5rem;
		width: 100%;
	}
</style>
