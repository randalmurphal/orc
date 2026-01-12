<script lang="ts">
	import { onMount } from 'svelte';
	import {
		listSkills,
		getSkill,
		createSkill,
		updateSkill,
		deleteSkill,
		type SkillInfo,
		type Skill
	} from '$lib/api';

	let skills: SkillInfo[] = [];
	let selectedSkill: Skill | null = null;
	let isCreating = false;
	let loading = true;
	let saving = false;
	let error: string | null = null;
	let success: string | null = null;

	// Form fields
	let formName = '';
	let formDescription = '';
	let formContent = '';
	let formAllowedTools = '';

	onMount(async () => {
		try {
			skills = await listSkills();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load skills';
		} finally {
			loading = false;
		}
	});

	async function selectSkillByName(name: string) {
		error = null;
		success = null;
		isCreating = false;

		try {
			selectedSkill = await getSkill(name);
			formName = selectedSkill.name;
			formDescription = selectedSkill.description;
			formContent = selectedSkill.content;
			formAllowedTools = selectedSkill.allowed_tools?.join(', ') || '';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load skill';
		}
	}

	function startCreate() {
		error = null;
		success = null;
		selectedSkill = null;
		isCreating = true;

		formName = '';
		formDescription = '';
		formContent = '';
		formAllowedTools = '';
	}

	async function handleSave() {
		if (!formName.trim() || !formDescription.trim()) {
			error = 'Name and description are required';
			return;
		}

		saving = true;
		error = null;
		success = null;

		const allowedTools = formAllowedTools
			.split(',')
			.map((t) => t.trim())
			.filter((t) => t);

		const skill: Skill = {
			name: formName.trim(),
			description: formDescription.trim(),
			content: formContent.trim(),
			allowed_tools: allowedTools.length > 0 ? allowedTools : undefined
		};

		try {
			if (isCreating) {
				await createSkill(skill);
				success = 'Skill created successfully';
			} else if (selectedSkill) {
				await updateSkill(selectedSkill.name, skill);
				success = 'Skill updated successfully';
			}

			skills = await listSkills();
			selectedSkill = skill;
			isCreating = false;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save skill';
		} finally {
			saving = false;
		}
	}

	async function handleDelete() {
		if (!selectedSkill) return;

		if (!confirm(`Delete skill "${selectedSkill.name}"?`)) return;

		saving = true;
		error = null;
		success = null;

		try {
			await deleteSkill(selectedSkill.name);
			skills = await listSkills();
			selectedSkill = null;
			isCreating = false;
			success = 'Skill deleted successfully';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete skill';
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head>
	<title>Skills - orc</title>
</svelte:head>

<div class="skills-page">
	<header class="page-header">
		<div class="header-content">
			<div>
				<h1>Claude Code Skills</h1>
				<p class="subtitle">Manage skills in .claude/skills/ (SKILL.md format)</p>
			</div>
			<button class="btn btn-primary" on:click={startCreate}>New Skill</button>
		</div>
	</header>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if success}
		<div class="alert alert-success">{success}</div>
	{/if}

	{#if loading}
		<div class="loading">Loading skills...</div>
	{:else}
		<div class="skills-layout">
			<!-- Skill List -->
			<aside class="skill-list">
				<h2>Skills</h2>
				{#if skills.length === 0}
					<p class="empty-message">No skills configured</p>
				{:else}
					<ul>
						{#each skills as skill}
							<li>
								<button
									class="skill-item"
									class:selected={selectedSkill?.name === skill.name}
									on:click={() => selectSkillByName(skill.name)}
								>
									<span class="skill-name">{skill.name}</span>
									{#if skill.description}
										<span class="skill-desc">{skill.description}</span>
									{/if}
								</button>
							</li>
						{/each}
					</ul>
				{/if}
			</aside>

			<!-- Editor Panel -->
			<div class="editor-panel">
				{#if selectedSkill || isCreating}
					<div class="editor-header">
						<h2>{isCreating ? 'New Skill' : selectedSkill?.name}</h2>
						{#if selectedSkill && !isCreating}
							<button class="btn btn-danger" on:click={handleDelete} disabled={saving}>
								Delete
							</button>
						{/if}
					</div>

					<form class="skill-form" on:submit|preventDefault={handleSave}>
						<div class="form-row">
							<div class="form-group">
								<label for="name">Name</label>
								<input
									id="name"
									type="text"
									bind:value={formName}
									placeholder="my-skill"
									disabled={!isCreating}
								/>
								<span class="form-hint"
									>.claude/skills/{formName || 'name'}/SKILL.md</span
								>
							</div>

							<div class="form-group">
								<label for="allowed-tools">Allowed Tools (optional)</label>
								<input
									id="allowed-tools"
									type="text"
									bind:value={formAllowedTools}
									placeholder="Read, Bash, Edit"
								/>
								<span class="form-hint">Comma-separated list of tools</span>
							</div>
						</div>

						<div class="form-group">
							<label for="description">Description</label>
							<input
								id="description"
								type="text"
								bind:value={formDescription}
								placeholder="Brief description of what this skill does"
							/>
						</div>

						<div class="form-group form-group-grow">
							<label for="content">Content (Markdown)</label>
							<textarea
								id="content"
								bind:value={formContent}
								placeholder="Enter the skill instructions..."
								rows="15"
							></textarea>
						</div>

						<div class="form-actions">
							<button type="submit" class="btn btn-primary" disabled={saving}>
								{saving ? 'Saving...' : isCreating ? 'Create' : 'Update'}
							</button>
						</div>
					</form>
				{:else}
					<div class="no-selection">
						<p>Select a skill from the list or create a new one.</p>
						<p class="hint">
							Skills are reusable prompts that can be invoked with <code>/skill-name</code>
						</p>
					</div>
				{/if}
			</div>
		</div>
	{/if}
</div>

<style>
	.skills-page {
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
	}

	.page-header h1 {
		margin: 0;
		font-size: 1.5rem;
	}

	.header-content {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
	}

	.subtitle {
		margin: 0.5rem 0 0;
		color: var(--text-secondary);
		font-size: 0.875rem;
	}

	.alert {
		padding: 0.75rem 1rem;
		border-radius: 6px;
		font-size: 0.875rem;
	}

	.alert-error {
		background: var(--error-bg, #fee2e2);
		color: var(--error-text, #dc2626);
		border: 1px solid var(--error-border, #fecaca);
	}

	.alert-success {
		background: var(--success-bg, #dcfce7);
		color: var(--success-text, #16a34a);
		border: 1px solid var(--success-border, #bbf7d0);
	}

	.loading {
		text-align: center;
		padding: 3rem;
		color: var(--text-secondary);
	}

	.skills-layout {
		display: grid;
		grid-template-columns: 250px 1fr;
		gap: 1.5rem;
		min-height: 600px;
	}

	/* Skill List */
	.skill-list {
		background: var(--bg-secondary);
		border-radius: 8px;
		padding: 1rem;
		border: 1px solid var(--border-color);
	}

	.skill-list h2 {
		font-size: 0.875rem;
		font-weight: 600;
		margin: 0 0 0.75rem;
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.skill-list ul {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.empty-message {
		color: var(--text-secondary);
		font-size: 0.875rem;
		font-style: italic;
	}

	.skill-item {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		width: 100%;
		padding: 0.5rem 0.75rem;
		background: transparent;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		text-align: left;
		color: var(--text-primary);
		font-size: 0.875rem;
		gap: 0.25rem;
	}

	.skill-item:hover {
		background: var(--bg-tertiary, rgba(0, 0, 0, 0.05));
	}

	.skill-item.selected {
		background: var(--primary-bg, #dbeafe);
		color: var(--primary-text, #1d4ed8);
	}

	.skill-name {
		font-weight: 500;
	}

	.skill-desc {
		font-size: 0.75rem;
		color: var(--text-secondary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		max-width: 100%;
	}

	.skill-item.selected .skill-desc {
		color: var(--primary-text, #1d4ed8);
		opacity: 0.7;
	}

	/* Editor Panel */
	.editor-panel {
		display: flex;
		flex-direction: column;
		background: var(--bg-secondary);
		border-radius: 8px;
		border: 1px solid var(--border-color);
		overflow: hidden;
	}

	.editor-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 1rem;
		border-bottom: 1px solid var(--border-color);
	}

	.editor-header h2 {
		margin: 0;
		font-size: 1rem;
	}

	.skill-form {
		padding: 1.5rem;
		display: flex;
		flex-direction: column;
		gap: 1rem;
		flex: 1;
	}

	.form-row {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 1rem;
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.form-group-grow {
		flex: 1;
	}

	.form-group label {
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--text-primary);
	}

	.form-group input,
	.form-group textarea {
		padding: 0.5rem 0.75rem;
		border: 1px solid var(--border-color);
		border-radius: 6px;
		font-size: 0.875rem;
		background: var(--bg-primary);
		color: var(--text-primary);
	}

	.form-group textarea {
		font-family: 'JetBrains Mono', 'Fira Code', monospace;
		resize: vertical;
		flex: 1;
		min-height: 200px;
	}

	.form-group input:focus,
	.form-group textarea:focus {
		outline: none;
		border-color: var(--primary, #3b82f6);
	}

	.form-hint {
		font-size: 0.75rem;
		color: var(--text-secondary);
	}

	.form-actions {
		padding-top: 0.5rem;
	}

	.btn {
		padding: 0.5rem 1rem;
		border-radius: 6px;
		font-size: 0.875rem;
		font-weight: 500;
		cursor: pointer;
		border: 1px solid transparent;
	}

	.btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.btn-primary {
		background: var(--primary, #3b82f6);
		color: white;
	}

	.btn-primary:hover:not(:disabled) {
		background: var(--primary-hover, #2563eb);
	}

	.btn-danger {
		background: var(--error-text, #dc2626);
		color: white;
	}

	.btn-danger:hover:not(:disabled) {
		background: #b91c1c;
	}

	.no-selection {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100%;
		padding: 3rem;
		text-align: center;
		color: var(--text-secondary);
		gap: 1rem;
	}

	.no-selection .hint {
		font-size: 0.875rem;
	}

	.no-selection code {
		background: var(--bg-tertiary, rgba(0, 0, 0, 0.05));
		padding: 0.125rem 0.375rem;
		border-radius: 4px;
		font-family: 'JetBrains Mono', 'Fira Code', monospace;
	}

	@media (max-width: 768px) {
		.skills-layout {
			grid-template-columns: 1fr;
		}

		.skill-list {
			max-height: 200px;
			overflow-y: auto;
		}

		.form-row {
			grid-template-columns: 1fr;
		}
	}
</style>
