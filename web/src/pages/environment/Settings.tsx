/**
 * Environment Settings page (/environment/settings)
 * Displays and edits Claude Code settings (global + project)
 */

import { useState, useEffect, useCallback } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { create } from '@bufbuild/protobuf';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Icon } from '@/components/ui/Icon';
import { toast } from '@/stores';
import { useDocumentTitle } from '@/hooks';
import { configClient } from '@/lib/client';
import {
	type SettingsHierarchy,
	SettingsScope,
	GetSettingsHierarchyRequestSchema,
	UpdateSettingsRequestSchema,
	SettingsSchema,
} from '@/gen/orc/v1/config_pb';
import './environment.css';

type ScopeTab = 'global' | 'project';

// Convert UI scope tab to protobuf SettingsScope enum
function toSettingsScope(scope: ScopeTab): SettingsScope {
	return scope === 'global' ? SettingsScope.GLOBAL : SettingsScope.PROJECT;
}

// Permission key-value pair for editing
interface PermissionEntry {
	key: string;
	value: boolean;
}

export function Settings() {
	useDocumentTitle('Settings');
	const [hierarchy, setHierarchy] = useState<SettingsHierarchy | null>(null);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [scope, setScope] = useState<ScopeTab>('project');

	// Form state matching protobuf Settings schema
	const [tools, setTools] = useState<string[]>([]);
	const [mcpServers, setMcpServers] = useState<string[]>([]);
	const [customInstructions, setCustomInstructions] = useState<string>('');
	const [permissions, setPermissions] = useState<PermissionEntry[]>([]);
	const [hasChanges, setHasChanges] = useState(false);

	const loadSettings = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const response = await configClient.getSettingsHierarchy(
				create(GetSettingsHierarchyRequestSchema, {})
			);
			setHierarchy(response.hierarchy ?? null);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load settings');
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		loadSettings();
	}, [loadSettings]);

	// Update form when scope changes
	useEffect(() => {
		if (!hierarchy) return;

		const settings = scope === 'global' ? hierarchy.global : hierarchy.project;

		// Tools
		setTools(settings?.tools || []);

		// MCP Servers
		setMcpServers(settings?.mcpServers || []);

		// Custom Instructions
		setCustomInstructions(settings?.customInstructions || '');

		// Permissions (convert map to array for editing)
		const perms = settings?.permissions || {};
		setPermissions(Object.entries(perms).map(([key, value]) => ({ key, value })));

		setHasChanges(false);
	}, [hierarchy, scope]);

	const handleToolsChange = (value: string) => {
		// Parse comma-separated tools
		setTools(value.split(',').map((t) => t.trim()).filter(Boolean));
		setHasChanges(true);
	};

	const handleMcpServersChange = (value: string) => {
		// Parse comma-separated MCP servers
		setMcpServers(value.split(',').map((s) => s.trim()).filter(Boolean));
		setHasChanges(true);
	};

	const handleCustomInstructionsChange = (value: string) => {
		setCustomInstructions(value);
		setHasChanges(true);
	};

	const handlePermissionChange = (index: number, field: 'key' | 'value', value: string | boolean) => {
		const newPerms = [...permissions];
		if (field === 'key') {
			newPerms[index] = { ...newPerms[index], key: value as string };
		} else {
			newPerms[index] = { ...newPerms[index], value: value as boolean };
		}
		setPermissions(newPerms);
		setHasChanges(true);
	};

	const handleAddPermission = () => {
		setPermissions([...permissions, { key: '', value: true }]);
		setHasChanges(true);
	};

	const handleRemovePermission = (index: number) => {
		setPermissions(permissions.filter((_, i) => i !== index));
		setHasChanges(true);
	};

	const handleSave = async () => {
		try {
			setSaving(true);

			// Build settings object
			const settings = create(SettingsSchema, {
				tools,
				mcpServers,
				customInstructions: customInstructions || undefined,
				permissions: Object.fromEntries(
					permissions.filter((p) => p.key.trim()).map((p) => [p.key.trim(), p.value])
				),
			});

			await configClient.updateSettings(
				create(UpdateSettingsRequestSchema, {
					scope: toSettingsScope(scope),
					settings,
				})
			);

			toast.success(`${scope === 'global' ? 'Global' : 'Project'} settings saved`);
			setHasChanges(false);

			// Reload to get updated state
			await loadSettings();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to save settings');
		} finally {
			setSaving(false);
		}
	};

	const handleReset = () => {
		// Re-apply current settings from hierarchy
		if (!hierarchy) return;

		const settings = scope === 'global' ? hierarchy.global : hierarchy.project;

		setTools(settings?.tools || []);
		setMcpServers(settings?.mcpServers || []);
		setCustomInstructions(settings?.customInstructions || '');
		const perms = settings?.permissions || {};
		setPermissions(Object.entries(perms).map(([key, value]) => ({ key, value })));
		setHasChanges(false);
	};

	if (loading) {
		return (
			<div className="page environment-settings-page">
				<div className="env-loading">Loading settings...</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className="page environment-settings-page">
				<div className="env-error">
					<p>{error}</p>
					<Button variant="secondary" onClick={loadSettings}>
						Retry
					</Button>
				</div>
			</div>
		);
	}

	return (
		<div className="page environment-settings-page">
			<div className="env-page-header">
				<h3>Claude Code Settings</h3>
				<div className="env-page-header-actions">
					{hasChanges && (
						<>
							<Button variant="ghost" onClick={handleReset}>
								Reset
							</Button>
							<Button variant="primary" onClick={handleSave} loading={saving}>
								Save Changes
							</Button>
						</>
					)}
				</div>
			</div>

			<Tabs.Root value={scope} onValueChange={(v) => setScope(v as ScopeTab)}>
				<Tabs.List className="env-scope-tabs" aria-label="Settings scope">
					<Tabs.Trigger value="project" className="env-scope-tab">
						<Icon name="folder" size={16} />
						Project
					</Tabs.Trigger>
					<Tabs.Trigger value="global" className="env-scope-tab">
						<Icon name="globe" size={16} />
						Global
					</Tabs.Trigger>
				</Tabs.List>

				<Tabs.Content value={scope} className="settings-content">
					{/* Tools */}
					<div className="settings-section">
						<h4 className="settings-section-title">
							<Icon name="tools" size={18} />
							Enabled Tools
						</h4>
						<p className="settings-section-description">
							Comma-separated list of tools to enable for Claude Code.
						</p>
						<Input
							placeholder="Read, Write, Edit, Bash, Glob, Grep..."
							value={tools.join(', ')}
							onChange={(e) => handleToolsChange(e.target.value)}
							size="sm"
						/>
					</div>

					{/* MCP Servers */}
					<div className="settings-section">
						<h4 className="settings-section-title">
							<Icon name="server" size={18} />
							MCP Servers
						</h4>
						<p className="settings-section-description">
							Comma-separated list of allowed MCP server names.
						</p>
						<Input
							placeholder="filesystem, database, custom-server..."
							value={mcpServers.join(', ')}
							onChange={(e) => handleMcpServersChange(e.target.value)}
							size="sm"
						/>
					</div>

					{/* Custom Instructions */}
					<div className="settings-section">
						<h4 className="settings-section-title">
							<Icon name="file-text" size={18} />
							Custom Instructions
						</h4>
						<p className="settings-section-description">
							Additional instructions to include in Claude Code prompts.
						</p>
						<textarea
							className="settings-textarea"
							placeholder="Enter custom instructions..."
							value={customInstructions}
							onChange={(e) => handleCustomInstructionsChange(e.target.value)}
							rows={4}
						/>
					</div>

					{/* Permissions */}
					<div className="settings-section">
						<h4 className="settings-section-title">
							<Icon name="shield" size={18} />
							Permission Overrides
						</h4>
						<div className="settings-env-vars">
							{permissions.length === 0 ? (
								<p className="env-empty" style={{ padding: 'var(--space-4)', textAlign: 'left' }}>
									No permission overrides configured
								</p>
							) : (
								permissions.map((p, i) => (
									<div key={i} className="settings-env-row">
										<Input
											placeholder="permission.name"
											value={p.key}
											onChange={(e) => handlePermissionChange(i, 'key', e.target.value)}
											size="sm"
										/>
										<label className="settings-checkbox-label">
											<input
												type="checkbox"
												checked={p.value}
												onChange={(e) => handlePermissionChange(i, 'value', e.target.checked)}
											/>
											Allowed
										</label>
										<Button
											variant="ghost"
											size="sm"
											iconOnly
											aria-label="Remove permission"
											onClick={() => handleRemovePermission(i)}
										>
											<Icon name="trash" size={16} />
										</Button>
									</div>
								))
							)}
							<Button
								variant="ghost"
								size="sm"
								className="settings-add-btn"
								leftIcon={<Icon name="plus" size={16} />}
								onClick={handleAddPermission}
							>
								Add Permission
							</Button>
						</div>
					</div>
				</Tabs.Content>
			</Tabs.Root>
		</div>
	);
}
