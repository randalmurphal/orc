<script lang="ts">
	import { onMount } from 'svelte';
	import {
		listAgents,
		getAgent,
		createAgent,
		updateAgent,
		deleteAgent,
		listSkills,
		type SubAgent,
		type SkillInfo
	} from '$lib/api';

	let agents: SubAgent[] = [];
	let skills: SkillInfo[] = [];
	let selectedAgent: SubAgent | null = null;
	let isCreating = false;
	let loading = true;
	let saving = false;
	let error: string | null = null;
	let success: string | null = null;

	// Form fields
	let formName = '';
	let formDescription = '';
	let formModel = '';
	let formPrompt = '';
	let formWorkDir = '';
	let formTimeout = '';
	let formSkillRefs: string[] = [];
	let formAllowTools = '';
	let formDenyTools = '';

	onMount(async () => {
		try {
			[agents, skills] = await Promise.all([listAgents(), listSkills()]);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load agents';
		} finally {
			loading = false;
		}
	});

	async function selectAgentByName(name: string) {
		error = null;
		success = null;
		isCreating = false;

		try {
			selectedAgent = await getAgent(name);
			formName = selectedAgent.name;
			formDescription = selectedAgent.description;
			formModel = selectedAgent.model || '';
			formPrompt = selectedAgent.prompt || '';
			formWorkDir = selectedAgent.work_dir || '';
			formTimeout = selectedAgent.timeout || '';
			formSkillRefs = selectedAgent.skill_refs || [];
			formAllowTools = selectedAgent.tools?.allow?.join(', ') || '';
			formDenyTools = selectedAgent.tools?.deny?.join(', ') || '';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load agent';
		}
	}

	function startCreate() {
		error = null;
		success = null;
		selectedAgent = null;
		isCreating = true;

		formName = '';
		formDescription = '';
		formModel = '';
		formPrompt = '';
		formWorkDir = '';
		formTimeout = '';
		formSkillRefs = [];
		formAllowTools = '';
		formDenyTools = '';
	}

	function toggleSkillRef(skillName: string) {
		if (formSkillRefs.includes(skillName)) {
			formSkillRefs = formSkillRefs.filter((s) => s !== skillName);
		} else {
			formSkillRefs = [...formSkillRefs, skillName];
		}
	}

	async function handleSave() {
		if (!formName.trim() || !formDescription.trim()) {
			error = 'Name and description are required';
			return;
		}

		saving = true;
		error = null;
		success = null;

		const allowTools = formAllowTools
			.split(',')
			.map((t) => t.trim())
			.filter((t) => t);
		const denyTools = formDenyTools
			.split(',')
			.map((t) => t.trim())
			.filter((t) => t);

		const agent: SubAgent = {
			name: formName.trim(),
			description: formDescription.trim()
		};

		if (formModel) agent.model = formModel;
		if (formPrompt) agent.prompt = formPrompt;
		if (formWorkDir) agent.work_dir = formWorkDir;
		if (formTimeout) agent.timeout = formTimeout;
		if (formSkillRefs.length > 0) agent.skill_refs = formSkillRefs;
		if (allowTools.length > 0 || denyTools.length > 0) {
			agent.tools = {};
			if (allowTools.length > 0) agent.tools.allow = allowTools;
			if (denyTools.length > 0) agent.tools.deny = denyTools;
		}

		try {
			if (isCreating) {
				await createAgent(agent);
				success = 'Agent created successfully';
			} else if (selectedAgent) {
				await updateAgent(selectedAgent.name, agent);
				success = 'Agent updated successfully';
			}

			agents = await listAgents();
			selectedAgent = agent;
			isCreating = false;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save agent';
		} finally {
			saving = false;
		}
	}

	async function handleDelete() {
		if (!selectedAgent) return;

		if (!confirm(`Delete agent "${selectedAgent.name}"?`)) return;

		saving = true;
		error = null;
		success = null;

		try {
			await deleteAgent(selectedAgent.name);
			agents = await listAgents();
			selectedAgent = null;
			isCreating = false;
			success = 'Agent deleted successfully';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete agent';
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head>
	<title>Agents - orc</title>
</svelte:head>

<div class="agents-page">
	<header class="page-header">
		<div class="header-content">
			<div>
				<h1>Sub-Agents</h1>
				<p class="subtitle">Define agents for task delegation</p>
			</div>
			<button class="btn btn-primary" on:click={startCreate}>New Agent</button>
		</div>
	</header>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if success}
		<div class="alert alert-success">{success}</div>
	{/if}

	{#if loading}
		<div class="loading">Loading agents...</div>
	{:else}
		<div class="agents-layout">
			<!-- Agent List -->
			<aside class="agent-list">
				<h2>Agents</h2>
				{#if agents.length === 0}
					<p class="empty-message">No agents configured</p>
				{:else}
					<ul>
						{#each agents as agent}
							<li>
								<button
									class="agent-item"
									class:selected={selectedAgent?.name === agent.name}
									on:click={() => selectAgentByName(agent.name)}
								>
									<span class="agent-name">{agent.name}</span>
									{#if agent.description}
										<span class="agent-desc">{agent.description}</span>
									{/if}
								</button>
							</li>
						{/each}
					</ul>
				{/if}
			</aside>

			<!-- Editor Panel -->
			<div class="editor-panel">
				{#if selectedAgent || isCreating}
					<div class="editor-header">
						<h2>{isCreating ? 'New Agent' : selectedAgent?.name}</h2>
						{#if selectedAgent && !isCreating}
							<button class="btn btn-danger" on:click={handleDelete} disabled={saving}>
								Delete
							</button>
						{/if}
					</div>

					<form class="agent-form" on:submit|preventDefault={handleSave}>
						<div class="form-row">
							<div class="form-group">
								<label for="name">Name</label>
								<input
									id="name"
									type="text"
									bind:value={formName}
									placeholder="my-agent"
									disabled={!isCreating}
								/>
							</div>

							<div class="form-group">
								<label for="model">Model (optional)</label>
								<select id="model" bind:value={formModel}>
									<option value="">Default</option>
									<option value="sonnet">Sonnet</option>
									<option value="opus">Opus</option>
									<option value="haiku">Haiku</option>
								</select>
							</div>
						</div>

						<div class="form-group">
							<label for="description">Description</label>
							<input
								id="description"
								type="text"
								bind:value={formDescription}
								placeholder="Brief description of what this agent does"
							/>
						</div>

						<div class="form-row">
							<div class="form-group">
								<label for="work-dir">Work Directory (optional)</label>
								<input
									id="work-dir"
									type="text"
									bind:value={formWorkDir}
									placeholder="./src"
								/>
							</div>

							<div class="form-group">
								<label for="timeout">Timeout (optional)</label>
								<input
									id="timeout"
									type="text"
									bind:value={formTimeout}
									placeholder="30m"
								/>
							</div>
						</div>

						<div class="form-row">
							<div class="form-group">
								<label for="allow-tools">Allowed Tools (optional)</label>
								<input
									id="allow-tools"
									type="text"
									bind:value={formAllowTools}
									placeholder="Read, Grep, Glob"
								/>
								<span class="form-hint">Comma-separated list</span>
							</div>

							<div class="form-group">
								<label for="deny-tools">Denied Tools (optional)</label>
								<input
									id="deny-tools"
									type="text"
									bind:value={formDenyTools}
									placeholder="Bash, Write"
								/>
								<span class="form-hint">Comma-separated list</span>
							</div>
						</div>

						{#if skills.length > 0}
							<div class="form-group">
								<span class="form-label">Skill References</span>
								<div class="skill-refs" role="group" aria-label="Skill References">
									{#each skills as skill}
										<button
											type="button"
											class="skill-chip"
											class:selected={formSkillRefs.includes(skill.name)}
											on:click={() => toggleSkillRef(skill.name)}
										>
											{skill.name}
										</button>
									{/each}
								</div>
							</div>
						{/if}

						<div class="form-group form-group-grow">
							<label for="prompt">System Prompt (optional)</label>
							<textarea
								id="prompt"
								bind:value={formPrompt}
								placeholder="Additional instructions for this agent..."
								rows="8"
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
						<p>Select an agent from the list or create a new one.</p>
						<p class="hint">
							Sub-agents are specialized workers that can be invoked during task execution
						</p>
					</div>
				{/if}
			</div>
		</div>
	{/if}
</div>

<style>
	.agents-page {
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

	.agents-layout {
		display: grid;
		grid-template-columns: 250px 1fr;
		gap: 1.5rem;
		min-height: 600px;
	}

	/* Agent List */
	.agent-list {
		background: var(--bg-secondary);
		border-radius: 8px;
		padding: 1rem;
		border: 1px solid var(--border-color);
	}

	.agent-list h2 {
		font-size: 0.875rem;
		font-weight: 600;
		margin: 0 0 0.75rem;
		color: var(--text-secondary);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.agent-list ul {
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

	.agent-item {
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

	.agent-item:hover {
		background: var(--bg-tertiary, rgba(0, 0, 0, 0.05));
	}

	.agent-item.selected {
		background: var(--primary-bg, #dbeafe);
		color: var(--primary-text, #1d4ed8);
	}

	.agent-name {
		font-weight: 500;
	}

	.agent-desc {
		font-size: 0.75rem;
		color: var(--text-secondary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		max-width: 100%;
	}

	.agent-item.selected .agent-desc {
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

	.agent-form {
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

	.form-group label,
	.form-label {
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--text-primary);
	}

	.form-group input,
	.form-group select,
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
		min-height: 150px;
	}

	.form-group input:focus,
	.form-group select:focus,
	.form-group textarea:focus {
		outline: none;
		border-color: var(--primary, #3b82f6);
	}

	.form-hint {
		font-size: 0.75rem;
		color: var(--text-secondary);
	}

	.skill-refs {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}

	.skill-chip {
		padding: 0.375rem 0.75rem;
		border: 1px solid var(--border-color);
		border-radius: 16px;
		background: var(--bg-primary);
		color: var(--text-primary);
		font-size: 0.75rem;
		cursor: pointer;
		transition: all 0.15s;
	}

	.skill-chip:hover {
		background: var(--bg-tertiary, rgba(0, 0, 0, 0.05));
	}

	.skill-chip.selected {
		background: var(--primary-bg, #dbeafe);
		border-color: var(--primary, #3b82f6);
		color: var(--primary-text, #1d4ed8);
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

	@media (max-width: 768px) {
		.agents-layout {
			grid-template-columns: 1fr;
		}

		.agent-list {
			max-height: 200px;
			overflow-y: auto;
		}

		.form-row {
			grid-template-columns: 1fr;
		}
	}
</style>
