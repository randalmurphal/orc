import { useState, useEffect, useCallback } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import {
	listAgents,
	getAgent,
	createAgent,
	updateAgent,
	deleteAgent,
	type SubAgent,
} from '@/lib/api';
import './Agents.css';

/**
 * Agents page (/environment/agents)
 *
 * Manages Claude Code sub-agents (.claude/agents/)
 */
export function Agents() {
	const [searchParams] = useSearchParams();
	const [agents, setAgents] = useState<SubAgent[]>([]);
	const [selectedAgent, setSelectedAgent] = useState<SubAgent | null>(null);
	const [isCreating, setIsCreating] = useState(false);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [success, setSuccess] = useState<string | null>(null);

	// Form state
	const [formName, setFormName] = useState('');
	const [formDescription, setFormDescription] = useState('');
	const [formModel, setFormModel] = useState('');
	const [formTools, setFormTools] = useState('');

	const scope = searchParams.get('scope') as 'global' | null;
	const isGlobal = scope === 'global';

	const loadAgents = useCallback(async () => {
		try {
			const data = await listAgents(isGlobal ? 'global' : undefined);
			setAgents(data);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load agents');
		}
	}, [isGlobal]);

	useEffect(() => {
		setLoading(true);
		setError(null);
		loadAgents().finally(() => setLoading(false));
	}, [loadAgents]);

	const selectAgent = async (agent: SubAgent) => {
		setError(null);
		setSuccess(null);
		setIsCreating(false);

		try {
			const fullAgent = await getAgent(agent.name);
			setSelectedAgent(fullAgent);
			setFormName(fullAgent.name);
			setFormDescription(fullAgent.description || '');
			setFormModel(fullAgent.model || '');
			setFormTools(
				typeof fullAgent.tools === 'string'
					? fullAgent.tools
					: Array.isArray(fullAgent.tools)
						? (fullAgent.tools as string[]).join(', ')
						: ''
			);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load agent');
		}
	};

	const startCreate = () => {
		setError(null);
		setSuccess(null);
		setSelectedAgent(null);
		setIsCreating(true);

		setFormName('');
		setFormDescription('');
		setFormModel('');
		setFormTools('');
	};

	const handleSave = async () => {
		if (!formName.trim()) {
			setError('Name is required');
			return;
		}

		setSaving(true);
		setError(null);
		setSuccess(null);

		const tools = formTools
			.split(',')
			.map((t) => t.trim())
			.filter((t) => t);

		const agent: Partial<SubAgent> = {
			name: formName.trim(),
			description: formDescription.trim() || undefined,
			model: formModel.trim() || undefined,
			tools: tools.length > 0 ? tools : undefined,
		};

		try {
			if (isCreating) {
				await createAgent(agent as SubAgent);
				setSuccess('Agent created');
			} else if (selectedAgent) {
				await updateAgent(selectedAgent.name, agent as SubAgent);
				setSuccess('Agent updated');
			}

			await loadAgents();
			setIsCreating(false);

			// Reload
			const updated = await getAgent(formName.trim());
			setSelectedAgent(updated);

			globalThis.setTimeout(() => setSuccess(null), 3000);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to save agent');
		} finally {
			setSaving(false);
		}
	};

	const handleDelete = async () => {
		if (!selectedAgent) return;

		if (!confirm(`Delete agent "${selectedAgent.name}"?`)) return;

		setSaving(true);
		setError(null);
		setSuccess(null);

		try {
			await deleteAgent(selectedAgent.name);
			await loadAgents();
			setSelectedAgent(null);
			setIsCreating(false);
			setSuccess('Agent deleted');
			globalThis.setTimeout(() => setSuccess(null), 3000);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to delete agent');
		} finally {
			setSaving(false);
		}
	};

	return (
		<div className="agents-page">
			<header className="agents-header">
				<div className="header-content">
					<div>
						<h1>{isGlobal ? 'Global ' : ''}Sub-Agents</h1>
						<p className="subtitle">Configure Claude Code sub-agents</p>
					</div>
					<div className="header-actions">
						<div className="scope-toggle">
							<Link
								to="/environment/agents"
								className={`scope-btn ${!isGlobal ? 'active' : ''}`}
							>
								Project
							</Link>
							<Link
								to="/environment/agents?scope=global"
								className={`scope-btn ${isGlobal ? 'active' : ''}`}
							>
								Global
							</Link>
						</div>
						<button className="btn btn-primary" onClick={startCreate}>
							New Agent
						</button>
					</div>
				</div>
			</header>

			{error && <div className="alert alert-error">{error}</div>}
			{success && <div className="alert alert-success">{success}</div>}

			{loading ? (
				<div className="loading-state">Loading agents...</div>
			) : (
				<div className="agents-layout">
					{/* Agent List */}
					<aside className="agent-list">
						<h2>Agents</h2>
						{agents.length === 0 ? (
							<p className="empty-message">No agents configured</p>
						) : (
							<ul>
								{agents.map((agent) => (
									<li key={agent.name}>
										<button
											className={`agent-item ${selectedAgent?.name === agent.name ? 'selected' : ''}`}
											onClick={() => selectAgent(agent)}
										>
											<span className="agent-name">{agent.name}</span>
											{agent.description && (
												<span className="agent-desc">{agent.description}</span>
											)}
										</button>
									</li>
								))}
							</ul>
						)}
					</aside>

					{/* Editor Panel */}
					<div className="editor-panel">
						{selectedAgent || isCreating ? (
							<>
								<div className="editor-header">
									<h2>{isCreating ? 'New Agent' : selectedAgent?.name}</h2>
									{selectedAgent && !isCreating && (
										<button
											className="btn btn-danger"
											onClick={handleDelete}
											disabled={saving}
										>
											Delete
										</button>
									)}
								</div>

								<form
									className="agent-form"
									onSubmit={(e) => {
										e.preventDefault();
										handleSave();
									}}
								>
									<div className="form-group">
										<label htmlFor="name">Name</label>
										<input
											id="name"
											type="text"
											value={formName}
											onChange={(e) => setFormName(e.target.value)}
											placeholder="my-agent"
											disabled={!isCreating}
										/>
									</div>

									<div className="form-group">
										<label htmlFor="description">Description</label>
										<input
											id="description"
											type="text"
											value={formDescription}
											onChange={(e) => setFormDescription(e.target.value)}
											placeholder="What this agent does"
										/>
									</div>

									<div className="form-group">
										<label htmlFor="model">Model (optional)</label>
										<input
											id="model"
											type="text"
											value={formModel}
											onChange={(e) => setFormModel(e.target.value)}
											placeholder="claude-sonnet-4-20250514"
										/>
										<span className="form-hint">Leave empty to use default model</span>
									</div>

									<div className="form-group">
										<label htmlFor="tools">Tools (optional)</label>
										<input
											id="tools"
											type="text"
											value={formTools}
											onChange={(e) => setFormTools(e.target.value)}
											placeholder="Read, Bash, Edit"
										/>
										<span className="form-hint">Comma-separated list of allowed tools</span>
									</div>

									<div className="form-actions">
										<button type="submit" className="btn btn-primary" disabled={saving}>
											{saving ? 'Saving...' : isCreating ? 'Create' : 'Update'}
										</button>
									</div>
								</form>
							</>
						) : (
							<div className="no-selection">
								<p>Select an agent from the list or create a new one.</p>
							</div>
						)}
					</div>
				</div>
			)}
		</div>
	);
}
