/**
 * MCP page (/environment/mcp)
 * Displays and edits MCP server configuration from .mcp.json
 */

import { useState, useEffect, useCallback } from 'react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Icon } from '@/components/ui/Icon';
import { Modal } from '@/components/overlays/Modal';
import { toast } from '@/stores';
import {
	listMCPServers,
	getMCPServer,
	createMCPServer,
	updateMCPServer,
	deleteMCPServer,
	type MCPServerInfo,
	type MCPServerCreate,
} from '@/lib/api';
import './environment.css';

interface EnvVar {
	key: string;
	value: string;
}

export function Mcp() {
	const [servers, setServers] = useState<MCPServerInfo[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Editor modal state
	const [editingServer, setEditingServer] = useState<string | null>(null);
	const [isNewServer, setIsNewServer] = useState(false);
	const [formData, setFormData] = useState<MCPServerCreate>({
		name: '',
		type: 'stdio',
		command: '',
		args: [],
		env: {},
		disabled: false,
	});
	const [argsText, setArgsText] = useState('');
	const [envVars, setEnvVars] = useState<EnvVar[]>([]);
	const [saving, setSaving] = useState(false);
	const [editorLoading, setEditorLoading] = useState(false);

	const loadServers = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const data = await listMCPServers();
			setServers(data);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load MCP servers');
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		loadServers();
	}, [loadServers]);

	const handleNew = () => {
		setFormData({
			name: '',
			type: 'stdio',
			command: '',
			args: [],
			env: {},
			disabled: false,
		});
		setArgsText('');
		setEnvVars([{ key: '', value: '' }]);
		setIsNewServer(true);
		setEditingServer('__new__');
	};

	const handleEdit = async (serverName: string) => {
		setEditorLoading(true);
		setEditingServer(serverName);
		setIsNewServer(false);
		try {
			const server = await getMCPServer(serverName);
			setFormData({
				name: server.name,
				type: server.type,
				command: server.command || '',
				args: server.args || [],
				env: server.env || {},
				url: server.url || '',
				disabled: server.disabled,
			});
			setArgsText((server.args || []).join('\n'));
			const envEntries = Object.entries(server.env || {}).map(([key, value]) => ({
				key,
				value,
			}));
			setEnvVars(envEntries.length > 0 ? envEntries : [{ key: '', value: '' }]);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to load server');
			setEditingServer(null);
		} finally {
			setEditorLoading(false);
		}
	};

	const handleSave = async () => {
		if (!formData.name?.trim()) {
			toast.error('Server name is required');
			return;
		}

		// Build the request
		const args = argsText
			.split('\n')
			.map((a) => a.trim())
			.filter(Boolean);
		const env: Record<string, string> = {};
		envVars.forEach((v) => {
			if (v.key.trim()) {
				env[v.key.trim()] = v.value;
			}
		});

		const request: MCPServerCreate = {
			name: formData.name.trim(),
			type: formData.type,
			command: formData.type === 'stdio' ? formData.command : undefined,
			args: formData.type === 'stdio' && args.length > 0 ? args : undefined,
			env: Object.keys(env).length > 0 ? env : undefined,
			url: formData.type === 'sse' ? formData.url : undefined,
			disabled: formData.disabled,
		};

		try {
			setSaving(true);
			if (isNewServer) {
				await createMCPServer(request);
				toast.success('MCP server created');
			} else {
				await updateMCPServer(editingServer!, request);
				toast.success('MCP server updated');
			}
			setEditingServer(null);
			await loadServers();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to save server');
		} finally {
			setSaving(false);
		}
	};

	const handleDelete = async (serverName: string) => {
		if (!confirm(`Delete MCP server "${serverName}"?`)) {
			return;
		}
		try {
			await deleteMCPServer(serverName);
			toast.success('MCP server deleted');
			await loadServers();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to delete server');
		}
	};

	const handleToggle = async (server: MCPServerInfo) => {
		try {
			await updateMCPServer(server.name, { disabled: !server.disabled });
			toast.success(`${server.name} ${server.disabled ? 'enabled' : 'disabled'}`);
			await loadServers();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to update server');
		}
	};

	const handleAddEnvVar = () => {
		setEnvVars([...envVars, { key: '', value: '' }]);
	};

	const handleRemoveEnvVar = (index: number) => {
		setEnvVars(envVars.filter((_, i) => i !== index));
	};

	const handleUpdateEnvVar = (index: number, field: 'key' | 'value', value: string) => {
		const updated = [...envVars];
		updated[index] = { ...updated[index], [field]: value };
		setEnvVars(updated);
	};

	if (loading) {
		return (
			<div className="page environment-mcp-page">
				<div className="env-loading">Loading MCP servers...</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className="page environment-mcp-page">
				<div className="env-error">
					<p>{error}</p>
					<Button variant="secondary" onClick={loadServers}>
						Retry
					</Button>
				</div>
			</div>
		);
	}

	return (
		<div className="page environment-mcp-page">
			<div className="env-page-header">
				<div>
					<h3>MCP Servers</h3>
					<p className="env-page-description">
						Configure Model Context Protocol servers that extend Claude Code capabilities.
					</p>
				</div>
				<div className="env-page-header-actions">
					<Button variant="primary" onClick={handleNew}>
						<Icon name="plus" size={14} />
						Add Server
					</Button>
				</div>
			</div>

			{servers.length === 0 ? (
				<div className="env-empty">
					<Icon name="server" size={48} />
					<p>No MCP servers configured</p>
					<p className="env-empty-hint">
						Add servers to extend Claude Code with external tools and capabilities.
					</p>
				</div>
			) : (
				<div className="env-card-grid">
					{servers.map((server) => (
						<div
							key={server.name}
							className={`env-card mcp-card ${server.disabled ? 'disabled' : ''}`}
						>
							<div className="env-card-header">
								<h4 className="env-card-title">
									<Icon name={server.type === 'sse' ? 'globe' : 'terminal'} size={16} />
									{server.name}
								</h4>
								<div className="env-card-actions">
									<Button
										variant="ghost"
										size="sm"
										onClick={() => handleToggle(server)}
										aria-label={server.disabled ? 'Enable' : 'Disable'}
									>
										<Icon name={server.disabled ? 'eye' : 'eye-off'} size={14} />
									</Button>
									<Button
										variant="ghost"
										size="sm"
										onClick={() => handleEdit(server.name)}
									>
										<Icon name="edit" size={14} />
									</Button>
									<Button
										variant="ghost"
										size="sm"
										onClick={() => handleDelete(server.name)}
									>
										<Icon name="trash" size={14} />
									</Button>
								</div>
							</div>
							<div className="mcp-card-type">{server.type}</div>
							{server.type === 'stdio' && server.command && (
								<div className="mcp-card-command">{server.command}</div>
							)}
							{server.type === 'sse' && server.url && (
								<div className="mcp-card-url">{server.url}</div>
							)}
							<div className="mcp-card-meta">
								{server.args_count > 0 && (
									<span className="mcp-card-badge">
										{server.args_count} arg{server.args_count !== 1 ? 's' : ''}
									</span>
								)}
								{server.has_env && (
									<span className="mcp-card-badge">
										{server.env_count} env var{server.env_count !== 1 ? 's' : ''}
									</span>
								)}
								{server.disabled && (
									<span className="mcp-card-badge disabled">Disabled</span>
								)}
							</div>
						</div>
					))}
				</div>
			)}

			{/* Editor Modal */}
			<Modal
				open={editingServer !== null}
				onClose={() => setEditingServer(null)}
				title={isNewServer ? 'Add MCP Server' : `Edit ${editingServer}`}
				size="lg"
			>
				{editorLoading ? (
					<div className="env-loading">Loading server...</div>
				) : (
					<div className="mcp-editor">
						<div className="mcp-form-row">
							<div className="mcp-form-field">
								<label>Server Name</label>
								<Input
									value={formData.name || ''}
									onChange={(e) => setFormData({ ...formData, name: e.target.value })}
									placeholder="my-server"
									disabled={!isNewServer}
									size="sm"
								/>
							</div>
							<div className="mcp-form-field">
								<label>Type</label>
								<select
									value={formData.type || 'stdio'}
									onChange={(e) =>
										setFormData({ ...formData, type: e.target.value as 'stdio' | 'sse' })
									}
									className="input-field"
								>
									<option value="stdio">stdio (local process)</option>
									<option value="sse">sse (HTTP server)</option>
								</select>
							</div>
						</div>

						{formData.type === 'stdio' ? (
							<>
								<div className="mcp-form-field">
									<label>Command</label>
									<Input
										value={formData.command || ''}
										onChange={(e) =>
											setFormData({ ...formData, command: e.target.value })
										}
										placeholder="npx -y @anthropic/mcp-server"
										size="sm"
									/>
								</div>
								<div className="mcp-form-field">
									<label>Arguments (one per line)</label>
									<textarea
										value={argsText}
										onChange={(e) => setArgsText(e.target.value)}
										className="textarea-field mcp-args-textarea"
										placeholder="--flag&#10;value"
										rows={3}
									/>
								</div>
							</>
						) : (
							<div className="mcp-form-field">
								<label>Server URL</label>
								<Input
									value={formData.url || ''}
									onChange={(e) => setFormData({ ...formData, url: e.target.value })}
									placeholder="http://localhost:3000/sse"
									size="sm"
								/>
							</div>
						)}

						<div className="mcp-form-field">
							<div className="mcp-form-field-header">
								<label>Environment Variables</label>
								<Button variant="ghost" size="sm" onClick={handleAddEnvVar}>
									<Icon name="plus" size={14} />
									Add
								</Button>
							</div>
							<div className="mcp-env-vars">
								{envVars.map((v, i) => (
									<div key={i} className="mcp-env-row">
										<Input
											value={v.key}
											onChange={(e) => handleUpdateEnvVar(i, 'key', e.target.value)}
											placeholder="KEY"
											size="sm"
										/>
										<Input
											value={v.value}
											onChange={(e) => handleUpdateEnvVar(i, 'value', e.target.value)}
											placeholder="value"
											size="sm"
										/>
										<Button
											variant="ghost"
											size="sm"
											onClick={() => handleRemoveEnvVar(i)}
											aria-label="Remove"
										>
											<Icon name="x" size={14} />
										</Button>
									</div>
								))}
							</div>
						</div>

						<div className="mcp-form-field mcp-checkbox">
							<input
								type="checkbox"
								id="mcp-disabled"
								checked={formData.disabled || false}
								onChange={(e) => setFormData({ ...formData, disabled: e.target.checked })}
							/>
							<label htmlFor="mcp-disabled">Disabled</label>
						</div>

						<div className="mcp-editor-actions">
							<Button variant="secondary" onClick={() => setEditingServer(null)}>
								Cancel
							</Button>
							<Button variant="primary" onClick={handleSave} loading={saving}>
								{isNewServer ? 'Create Server' : 'Save Changes'}
							</Button>
						</div>
					</div>
				)}
			</Modal>
		</div>
	);
}
