/**
 * MCP page (/environment/mcp)
 * Displays and edits MCP server configuration from .mcp.json
 * Tabbed layout: Library + Export/Import (matching Hooks.tsx pattern)
 */

import { useState, useEffect, useCallback } from 'react';
import { create } from '@bufbuild/protobuf';
import * as Tabs from '@radix-ui/react-tabs';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Icon } from '@/components/ui/Icon';
import { Modal } from '@/components/overlays/Modal';
import { toast } from '@/stores';
import { useDocumentTitle } from '@/hooks';
import { mcpClient } from '@/lib/client';
import {
	ListMCPServersRequestSchema,
	GetMCPServerRequestSchema,
	CreateMCPServerRequestSchema,
	UpdateMCPServerRequestSchema,
	DeleteMCPServerRequestSchema,
	ExportMCPServersRequestSchema,
	ScanMCPServersRequestSchema,
	ImportMCPServersRequestSchema,
	MCPScope,
	type MCPServerInfo,
	type DiscoveredMCPServer,
} from '@/gen/orc/v1/mcp_pb';
import './environment.css';

interface EnvVar {
	key: string;
	value: string;
}

// Local form data type for editing
interface MCPFormData {
	name: string;
	type: 'stdio' | 'sse';
	command: string;
	args: string[];
	env: Record<string, string>;
	url: string;
	disabled: boolean;
}

export function Mcp() {
	useDocumentTitle('MCP Servers');
	const [servers, setServers] = useState<MCPServerInfo[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Editor modal state
	const [editingServer, setEditingServer] = useState<string | null>(null);
	const [isNewServer, setIsNewServer] = useState(false);
	const [formData, setFormData] = useState<MCPFormData>({
		name: '',
		type: 'stdio',
		command: '',
		args: [],
		env: {},
		url: '',
		disabled: false,
	});
	const [argsText, setArgsText] = useState('');
	const [envVars, setEnvVars] = useState<EnvVar[]>([]);
	const [saving, setSaving] = useState(false);
	const [editorLoading, setEditorLoading] = useState(false);

	// Export/Import state
	const [exportSource, setExportSource] = useState<MCPScope>(MCPScope.MCP_SCOPE_PROJECT);
	const [exportDest, setExportDest] = useState<MCPScope>(MCPScope.MCP_SCOPE_GLOBAL);
	const [exportServers, setExportServers] = useState<MCPServerInfo[]>([]);
	const [selectedExportNames, setSelectedExportNames] = useState<Set<string>>(new Set());
	const [exporting, setExporting] = useState(false);
	const [exportLoading, setExportLoading] = useState(false);
	const [scanSource, setScanSource] = useState<MCPScope>(MCPScope.MCP_SCOPE_GLOBAL);
	const [scanCompareTo, setScanCompareTo] = useState<MCPScope>(MCPScope.MCP_SCOPE_PROJECT);
	const [scanning, setScanning] = useState(false);
	const [discoveredServers, setDiscoveredServers] = useState<DiscoveredMCPServer[]>([]);
	const [selectedImportNames, setSelectedImportNames] = useState<Set<string>>(new Set());
	const [importing, setImporting] = useState(false);

	const loadServers = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const response = await mcpClient.listMCPServers(
				create(ListMCPServersRequestSchema, {})
			);
			setServers(response.servers);
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
			url: '',
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
			const response = await mcpClient.getMCPServer(
				create(GetMCPServerRequestSchema, { name: serverName })
			);
			const server = response.server;
			if (!server) {
				throw new Error('Server not found');
			}
			setFormData({
				name: server.name,
				type: server.type === 'sse' ? 'sse' : 'stdio',
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

		try {
			setSaving(true);
			if (isNewServer) {
				await mcpClient.createMCPServer(
					create(CreateMCPServerRequestSchema, {
						name: formData.name.trim(),
						type: formData.type,
						command: formData.type === 'stdio' ? formData.command : undefined,
						args: formData.type === 'stdio' && args.length > 0 ? args : [],
						env: Object.keys(env).length > 0 ? env : {},
						url: formData.type === 'sse' ? formData.url : undefined,
						disabled: formData.disabled,
					})
				);
				toast.success('MCP server created');
			} else {
				await mcpClient.updateMCPServer(
					create(UpdateMCPServerRequestSchema, {
						name: editingServer!,
						type: formData.type,
						command: formData.type === 'stdio' ? formData.command : undefined,
						args: formData.type === 'stdio' ? args : [],
						env: Object.keys(env).length > 0 ? env : {},
						url: formData.type === 'sse' ? formData.url : undefined,
						disabled: formData.disabled,
					})
				);
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
			await mcpClient.deleteMCPServer(
				create(DeleteMCPServerRequestSchema, { name: serverName })
			);
			toast.success('MCP server deleted');
			await loadServers();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to delete server');
		}
	};

	const handleToggle = async (server: MCPServerInfo) => {
		try {
			await mcpClient.updateMCPServer(
				create(UpdateMCPServerRequestSchema, {
					name: server.name,
					disabled: !server.disabled,
				})
			);
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

	// Export/Import handlers
	const loadExportServers = useCallback(async () => {
		try {
			setExportLoading(true);
			const response = await mcpClient.listMCPServers(
				create(ListMCPServersRequestSchema, { scope: exportSource })
			);
			setExportServers(response.servers);
			setSelectedExportNames(new Set());
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to load servers for export');
		} finally {
			setExportLoading(false);
		}
	}, [exportSource]);

	const handleExport = async () => {
		if (selectedExportNames.size === 0) {
			toast.error('Select at least one server to export');
			return;
		}
		try {
			setExporting(true);
			const resp = await mcpClient.exportMCPServers(
				create(ExportMCPServersRequestSchema, {
					serverNames: Array.from(selectedExportNames),
					source: exportSource,
					destination: exportDest,
				})
			);
			toast.success(`Exported ${resp.exportedCount} server(s)`);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to export servers');
		} finally {
			setExporting(false);
		}
	};

	const handleScan = async () => {
		try {
			setScanning(true);
			const resp = await mcpClient.scanMCPServers(
				create(ScanMCPServersRequestSchema, {
					source: scanSource,
					compareTo: scanCompareTo,
				})
			);
			setDiscoveredServers(resp.servers);
			setSelectedImportNames(new Set(resp.servers.map((s) => s.name)));
			if (resp.servers.length === 0) {
				toast.info('No new or modified servers found');
			}
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to scan servers');
		} finally {
			setScanning(false);
		}
	};

	const handleImport = async () => {
		const namesToImport = Array.from(selectedImportNames);
		if (namesToImport.length === 0) {
			toast.error('Select at least one server to import');
			return;
		}
		try {
			setImporting(true);
			const resp = await mcpClient.importMCPServers(
				create(ImportMCPServersRequestSchema, {
					serverNames: namesToImport,
					source: scanSource,
					destination: scanCompareTo,
				})
			);
			toast.success(`Imported ${resp.importedCount} server(s)`);
			setDiscoveredServers([]);
			setSelectedImportNames(new Set());
			await loadServers();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to import servers');
		} finally {
			setImporting(false);
		}
	};

	const toggleExportName = (name: string) => {
		setSelectedExportNames((prev) => {
			const next = new Set(prev);
			if (next.has(name)) next.delete(name);
			else next.add(name);
			return next;
		});
	};

	const toggleImportName = (name: string) => {
		setSelectedImportNames((prev) => {
			const next = new Set(prev);
			if (next.has(name)) next.delete(name);
			else next.add(name);
			return next;
		});
	};

	const scopeLabel = (scope: MCPScope) =>
		scope === MCPScope.MCP_SCOPE_GLOBAL ? 'Global ~/.claude/' : 'Project .mcp.json';

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

			<Tabs.Root defaultValue="library">
				<Tabs.List className="env-scope-tabs" aria-label="MCP view">
					<Tabs.Trigger value="library">Library</Tabs.Trigger>
					<Tabs.Trigger value="export-import">Export / Import</Tabs.Trigger>
				</Tabs.List>

				<Tabs.Content value="library">
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
										{server.argsCount > 0 && (
											<span className="mcp-card-badge">
												{server.argsCount} arg{server.argsCount !== 1 ? 's' : ''}
											</span>
										)}
										{server.hasEnv && (
											<span className="mcp-card-badge">
												{server.envCount} env var{server.envCount !== 1 ? 's' : ''}
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
				</Tabs.Content>

				<Tabs.Content value="export-import">
					<div className="export-import-section">
						{/* Export Panel */}
						<div className="export-import-panel">
							<h4>Export MCP Servers</h4>
							<p className="env-page-description">Copy MCP servers from one scope to another.</p>
							<div className="export-import-controls">
								<div className="export-dest-selector">
									<span style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>From:</span>
									<Button
										variant={exportSource === MCPScope.MCP_SCOPE_PROJECT ? 'primary' : 'secondary'}
										size="sm"
										onClick={() => setExportSource(MCPScope.MCP_SCOPE_PROJECT)}
									>
										Project
									</Button>
									<Button
										variant={exportSource === MCPScope.MCP_SCOPE_GLOBAL ? 'primary' : 'secondary'}
										size="sm"
										onClick={() => setExportSource(MCPScope.MCP_SCOPE_GLOBAL)}
									>
										Global
									</Button>
								</div>
								<div className="export-dest-selector">
									<span style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>To:</span>
									<Button
										variant={exportDest === MCPScope.MCP_SCOPE_PROJECT ? 'primary' : 'secondary'}
										size="sm"
										onClick={() => setExportDest(MCPScope.MCP_SCOPE_PROJECT)}
									>
										Project
									</Button>
									<Button
										variant={exportDest === MCPScope.MCP_SCOPE_GLOBAL ? 'primary' : 'secondary'}
										size="sm"
										onClick={() => setExportDest(MCPScope.MCP_SCOPE_GLOBAL)}
									>
										Global
									</Button>
								</div>
								<Button variant="secondary" size="sm" onClick={loadExportServers} loading={exportLoading}>
									<Icon name="refresh" size={14} />
									Load
								</Button>
							</div>
							{exportServers.length === 0 ? (
								<div className="hooks-empty">No servers in {scopeLabel(exportSource)}. Click Load to refresh.</div>
							) : (
								<div className="export-import-list">
									{exportServers.map((server) => (
										<label key={server.name} className="export-import-item">
											<input
												type="checkbox"
												checked={selectedExportNames.has(server.name)}
												onChange={() => toggleExportName(server.name)}
											/>
											<span className="export-import-item-name">{server.name}</span>
											<span className="export-import-item-meta">{server.type}</span>
										</label>
									))}
								</div>
							)}
							<Button variant="primary" size="sm" onClick={handleExport} loading={exporting} disabled={selectedExportNames.size === 0}>
								Export Selected ({selectedExportNames.size})
							</Button>
						</div>

						{/* Import Panel */}
						<div className="export-import-panel">
							<h4>Import MCP Servers</h4>
							<p className="env-page-description">Scan for new or modified servers and import them.</p>
							<div className="export-import-controls">
								<div className="export-dest-selector">
									<span style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>Scan:</span>
									<Button
										variant={scanSource === MCPScope.MCP_SCOPE_GLOBAL ? 'primary' : 'secondary'}
										size="sm"
										onClick={() => { setScanSource(MCPScope.MCP_SCOPE_GLOBAL); setScanCompareTo(MCPScope.MCP_SCOPE_PROJECT); }}
									>
										Global
									</Button>
									<Button
										variant={scanSource === MCPScope.MCP_SCOPE_PROJECT ? 'primary' : 'secondary'}
										size="sm"
										onClick={() => { setScanSource(MCPScope.MCP_SCOPE_PROJECT); setScanCompareTo(MCPScope.MCP_SCOPE_GLOBAL); }}
									>
										Project
									</Button>
								</div>
								<Button variant="secondary" size="sm" onClick={handleScan} loading={scanning}>
									<Icon name="search" size={14} />
									Scan
								</Button>
							</div>
							{discoveredServers.length === 0 ? (
								<div className="hooks-empty">No items discovered. Click Scan to search.</div>
							) : (
								<div className="export-import-list">
									{discoveredServers.map((server) => (
										<label key={server.name} className="export-import-item">
											<input
												type="checkbox"
												checked={selectedImportNames.has(server.name)}
												onChange={() => toggleImportName(server.name)}
											/>
											<span className="export-import-item-name">{server.name}</span>
											<span className={`export-import-badge export-import-badge-${server.status}`}>
												{server.status}
											</span>
											<span className="export-import-item-meta">{server.type}</span>
										</label>
									))}
								</div>
							)}
							{discoveredServers.length > 0 && (
								<Button variant="primary" size="sm" onClick={handleImport} loading={importing} disabled={selectedImportNames.size === 0}>
									Import Selected ({selectedImportNames.size})
								</Button>
							)}
						</div>
					</div>
				</Tabs.Content>
			</Tabs.Root>

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
