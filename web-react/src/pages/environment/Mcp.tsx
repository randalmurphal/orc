import { useState, useEffect, useCallback } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import {
	listMCPServers,
	getMCPServer,
	createMCPServer,
	updateMCPServer,
	deleteMCPServer,
	type MCPServerInfo,
	type MCPServer,
	type MCPServerCreate,
} from '@/lib/api';
import './Mcp.css';

/**
 * MCP page (/environment/mcp)
 *
 * Manages MCP servers (.mcp.json)
 */
export function Mcp() {
	const [searchParams] = useSearchParams();
	const [servers, setServers] = useState<MCPServerInfo[]>([]);
	const [selectedServer, setSelectedServer] = useState<MCPServer | null>(null);
	const [isCreating, setIsCreating] = useState(false);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [success, setSuccess] = useState<string | null>(null);

	// Form state
	const [formName, setFormName] = useState('');
	const [formType, setFormType] = useState('stdio');
	const [formCommand, setFormCommand] = useState('');
	const [formArgs, setFormArgs] = useState('');
	const [formUrl, setFormUrl] = useState('');
	const [formDisabled, setFormDisabled] = useState(false);
	const [formEnv, setFormEnv] = useState<[string, string][]>([]);

	const scope = searchParams.get('scope') as 'global' | null;
	const isGlobal = scope === 'global';

	const loadServers = useCallback(async () => {
		try {
			const data = await listMCPServers(isGlobal ? 'global' : undefined);
			setServers(data);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load MCP servers');
		}
	}, [isGlobal]);

	useEffect(() => {
		setLoading(true);
		setError(null);
		loadServers().finally(() => setLoading(false));
	}, [loadServers]);

	const selectServer = async (info: MCPServerInfo) => {
		setError(null);
		setSuccess(null);
		setIsCreating(false);

		try {
			const server = await getMCPServer(info.name);
			setSelectedServer(server);
			setFormName(server.name);
			setFormType(server.type || 'stdio');
			setFormCommand(server.command || '');
			setFormArgs(server.args?.join(' ') || '');
			setFormUrl(server.url || '');
			setFormDisabled(server.disabled);
			setFormEnv(Object.entries(server.env || {}));
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load server');
		}
	};

	const startCreate = () => {
		setError(null);
		setSuccess(null);
		setSelectedServer(null);
		setIsCreating(true);

		setFormName('');
		setFormType('stdio');
		setFormCommand('');
		setFormArgs('');
		setFormUrl('');
		setFormDisabled(false);
		setFormEnv([]);
	};

	const handleSave = async () => {
		if (!formName.trim()) {
			setError('Name is required');
			return;
		}

		setSaving(true);
		setError(null);
		setSuccess(null);

		const env: Record<string, string> = {};
		for (const [key, value] of formEnv) {
			if (key.trim()) {
				env[key.trim()] = value;
			}
		}

		const server: MCPServerCreate = {
			name: formName.trim(),
			type: formType,
			command: formType === 'stdio' ? formCommand.trim() : undefined,
			args: formType === 'stdio' && formArgs.trim()
				? formArgs.split(/\s+/).filter((a) => a.trim())
				: undefined,
			url: formType === 'sse' ? formUrl.trim() : undefined,
			disabled: formDisabled,
			env: Object.keys(env).length > 0 ? env : undefined,
		};

		try {
			if (isCreating) {
				await createMCPServer(server);
				setSuccess('MCP server created');
			} else if (selectedServer) {
				await updateMCPServer(selectedServer.name, server);
				setSuccess('MCP server updated');
			}

			await loadServers();
			setIsCreating(false);

			// Reload the server to get updated data
			const updated = await getMCPServer(formName.trim());
			setSelectedServer(updated);

			globalThis.setTimeout(() => setSuccess(null), 3000);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to save server');
		} finally {
			setSaving(false);
		}
	};

	const handleDelete = async () => {
		if (!selectedServer) return;

		if (!confirm(`Delete MCP server "${selectedServer.name}"?`)) return;

		setSaving(true);
		setError(null);
		setSuccess(null);

		try {
			await deleteMCPServer(selectedServer.name);
			await loadServers();
			setSelectedServer(null);
			setIsCreating(false);
			setSuccess('MCP server deleted');
			globalThis.setTimeout(() => setSuccess(null), 3000);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to delete server');
		} finally {
			setSaving(false);
		}
	};

	const addEnvVar = () => {
		setFormEnv([...formEnv, ['', '']]);
	};

	const updateEnvVar = (index: number, key: string, value: string) => {
		const updated = [...formEnv];
		updated[index] = [key, value];
		setFormEnv(updated);
	};

	const removeEnvVar = (index: number) => {
		setFormEnv(formEnv.filter((_, i) => i !== index));
	};

	return (
		<div className="mcp-page">
			<header className="mcp-header">
				<div className="header-content">
					<div>
						<h1>{isGlobal ? 'Global ' : ''}MCP Servers</h1>
						<p className="subtitle">Configure Model Context Protocol servers</p>
					</div>
					<div className="header-actions">
						<div className="scope-toggle">
							<Link
								to="/environment/mcp"
								className={`scope-btn ${!isGlobal ? 'active' : ''}`}
							>
								Project
							</Link>
							<Link
								to="/environment/mcp?scope=global"
								className={`scope-btn ${isGlobal ? 'active' : ''}`}
							>
								Global
							</Link>
						</div>
						<button className="btn btn-primary" onClick={startCreate}>
							Add Server
						</button>
					</div>
				</div>
			</header>

			{error && <div className="alert alert-error">{error}</div>}
			{success && <div className="alert alert-success">{success}</div>}

			{loading ? (
				<div className="loading-state">Loading MCP servers...</div>
			) : (
				<div className="mcp-layout">
					{/* Server List */}
					<aside className="server-list">
						<h2>Servers</h2>
						{servers.length === 0 ? (
							<p className="empty-message">No MCP servers configured</p>
						) : (
							<ul>
								{servers.map((server) => (
									<li key={server.name}>
										<button
											className={`server-item ${selectedServer?.name === server.name ? 'selected' : ''} ${server.disabled ? 'disabled' : ''}`}
											onClick={() => selectServer(server)}
										>
											<span className="server-name">{server.name}</span>
											<span className="server-type">{server.type}</span>
											{server.disabled && <span className="badge">Disabled</span>}
										</button>
									</li>
								))}
							</ul>
						)}
					</aside>

					{/* Editor Panel */}
					<div className="editor-panel">
						{selectedServer || isCreating ? (
							<>
								<div className="editor-header">
									<h2>{isCreating ? 'New MCP Server' : selectedServer?.name}</h2>
									{selectedServer && !isCreating && (
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
									className="server-form"
									onSubmit={(e) => {
										e.preventDefault();
										handleSave();
									}}
								>
									<div className="form-row">
										<div className="form-group">
											<label htmlFor="name">Name</label>
											<input
												id="name"
												type="text"
												value={formName}
												onChange={(e) => setFormName(e.target.value)}
												placeholder="my-mcp-server"
												disabled={!isCreating}
											/>
										</div>

										<div className="form-group">
											<label htmlFor="type">Type</label>
											<select
												id="type"
												value={formType}
												onChange={(e) => setFormType(e.target.value)}
											>
												<option value="stdio">stdio (local command)</option>
												<option value="sse">SSE (remote URL)</option>
											</select>
										</div>
									</div>

									{formType === 'stdio' ? (
										<>
											<div className="form-group">
												<label htmlFor="command">Command</label>
												<input
													id="command"
													type="text"
													value={formCommand}
													onChange={(e) => setFormCommand(e.target.value)}
													placeholder="npx -y @modelcontextprotocol/server-xxx"
												/>
											</div>

											<div className="form-group">
												<label htmlFor="args">Arguments</label>
												<input
													id="args"
													type="text"
													value={formArgs}
													onChange={(e) => setFormArgs(e.target.value)}
													placeholder="--arg1 value1"
												/>
												<span className="form-hint">Space-separated arguments</span>
											</div>
										</>
									) : (
										<div className="form-group">
											<label htmlFor="url">URL</label>
											<input
												id="url"
												type="text"
												value={formUrl}
												onChange={(e) => setFormUrl(e.target.value)}
												placeholder="https://example.com/mcp"
											/>
										</div>
									)}

									<div className="form-group form-group-checkbox">
										<label>
											<input
												type="checkbox"
												checked={formDisabled}
												onChange={(e) => setFormDisabled(e.target.checked)}
											/>
											Disabled
										</label>
									</div>

									{/* Environment Variables */}
									<div className="form-section">
										<h3>Environment Variables</h3>
										<div className="env-vars">
											{formEnv.map(([key, value], index) => (
												<div className="env-var-row" key={index}>
													<input
														type="text"
														value={key}
														onChange={(e) => updateEnvVar(index, e.target.value, value)}
														placeholder="KEY"
													/>
													<span>=</span>
													<input
														type="text"
														value={value}
														onChange={(e) => updateEnvVar(index, key, e.target.value)}
														placeholder="value"
													/>
													<button
														type="button"
														className="btn-icon"
														onClick={() => removeEnvVar(index)}
													>
														&times;
													</button>
												</div>
											))}
											<button
												type="button"
												className="btn btn-secondary btn-sm"
												onClick={addEnvVar}
											>
												+ Add Variable
											</button>
										</div>
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
								<p>Select an MCP server from the list or add a new one.</p>
							</div>
						)}
					</div>
				</div>
			)}
		</div>
	);
}
