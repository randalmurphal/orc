/**
 * Hooks page (/environment/hooks)
 * Displays and manages hooks stored in GlobalDB hook_scripts table.
 */

import { useState, useEffect, useCallback } from 'react';
import { create } from '@bufbuild/protobuf';
import * as Tabs from '@radix-ui/react-tabs';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Icon } from '@/components/ui/Icon';
import { Modal } from '@/components/overlays/Modal';
import type { IconName } from '@/components/ui/Icon';
import { toast } from '@/stores';
import { useDocumentTitle } from '@/hooks';
import { configClient } from '@/lib/client';
import {
	type Hook,
	type DiscoveredItem,
	ListHooksRequestSchema,
	CreateHookRequestSchema,
	UpdateHookRequestSchema,
	DeleteHookRequestSchema,
	ExportHooksRequestSchema,
	ImportHooksRequestSchema,
	ScanClaudeDirRequestSchema,
	SettingsScope,
} from '@/gen/orc/v1/config_pb';
import './environment.css';

// Available hook event types (string-based, matching GlobalDB)
const HOOK_EVENT_TYPES = ['PreToolUse', 'PostToolUse', 'Notification', 'Stop'];

// Hook event type metadata
const HOOK_EVENT_INFO: Record<string, { icon: IconName; description: string }> = {
	PreToolUse: {
		icon: 'play',
		description: 'Runs before a tool is executed',
	},
	PostToolUse: {
		icon: 'check',
		description: 'Runs after a tool completes',
	},
	Notification: {
		icon: 'info',
		description: 'Runs on notifications',
	},
	Stop: {
		icon: 'pause',
		description: 'Runs when execution stops',
	},
};

// Editor form state for creating/editing a hook
interface HookFormState {
	name: string;
	description: string;
	content: string;
	eventType: string;
}

const defaultFormState: HookFormState = {
	name: '',
	description: '',
	content: '',
	eventType: 'PreToolUse',
};

export function Hooks() {
	useDocumentTitle('Hooks');
	const [hooks, setHooks] = useState<Hook[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Editor modal state
	const [editingHook, setEditingHook] = useState<Hook | null>(null);
	const [isCreating, setIsCreating] = useState(false);
	const [formState, setFormState] = useState<HookFormState>(defaultFormState);
	const [saving, setSaving] = useState(false);

	const loadData = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const response = await configClient.listHooks(
				create(ListHooksRequestSchema, {})
			);
			setHooks(response.hooks);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load hooks');
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		loadData();
	}, [loadData]);

	const handleCreate = () => {
		setFormState(defaultFormState);
		setIsCreating(true);
		setEditingHook(null);
	};

	const handleEdit = (hook: Hook) => {
		setFormState({
			name: hook.name,
			description: hook.description,
			content: hook.content,
			eventType: hook.eventType || 'PreToolUse',
		});
		setEditingHook(hook);
		setIsCreating(false);
	};

	const handleClone = (hook: Hook) => {
		setFormState({
			name: hook.name + '-copy',
			description: hook.description,
			content: hook.content,
			eventType: hook.eventType || 'PreToolUse',
		});
		setEditingHook(null);
		setIsCreating(true);
	};

	const handleClose = () => {
		setEditingHook(null);
		setIsCreating(false);
		setFormState(defaultFormState);
	};

	const handleFormChange = (field: keyof HookFormState, value: string) => {
		setFormState((prev) => ({ ...prev, [field]: value }));
	};

	const handleSave = async () => {
		if (!formState.name.trim() || !formState.content.trim()) {
			toast.error('Name and content are required');
			return;
		}
		if (!formState.eventType) {
			toast.error('Event type is required');
			return;
		}

		try {
			setSaving(true);

			if (isCreating) {
				await configClient.createHook(
					create(CreateHookRequestSchema, {
						name: formState.name.trim(),
						content: formState.content.trim(),
						eventType: formState.eventType,
						description: formState.description.trim(),
					})
				);
				toast.success('Hook created');
			} else if (editingHook) {
				await configClient.updateHook(
					create(UpdateHookRequestSchema, {
						id: editingHook.id,
						name: formState.name.trim(),
						description: formState.description.trim(),
						content: formState.content.trim(),
						eventType: formState.eventType,
					})
				);
				toast.success('Hook updated');
			}

			handleClose();
			await loadData();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to save hook');
		} finally {
			setSaving(false);
		}
	};

	const handleDelete = async (hook: Hook) => {
		if (!confirm(`Delete hook "${hook.name}"?`)) {
			return;
		}
		try {
			await configClient.deleteHook(
				create(DeleteHookRequestSchema, {
					id: hook.id,
				})
			);
			toast.success('Hook deleted');
			await loadData();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to delete hook');
		}
	};

	// Export/Import state
	const [exportDest, setExportDest] = useState<SettingsScope>(SettingsScope.PROJECT);
	const [selectedExportIds, setSelectedExportIds] = useState<Set<string>>(new Set());
	const [exporting, setExporting] = useState(false);
	const [scanSource, setScanSource] = useState<SettingsScope>(SettingsScope.PROJECT);
	const [scanning, setScanning] = useState(false);
	const [discoveredItems, setDiscoveredItems] = useState<DiscoveredItem[]>([]);
	const [selectedImportNames, setSelectedImportNames] = useState<Set<string>>(new Set());
	const [importing, setImporting] = useState(false);

	const handleExport = async () => {
		if (selectedExportIds.size === 0) {
			toast.error('Select at least one hook to export');
			return;
		}
		try {
			setExporting(true);
			const resp = await configClient.exportHooks(
				create(ExportHooksRequestSchema, {
					hookIds: Array.from(selectedExportIds),
					destination: exportDest,
				})
			);
			toast.success(`Exported ${resp.writtenPaths.length} hook(s)`);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to export hooks');
		} finally {
			setExporting(false);
		}
	};

	const handleScan = async () => {
		try {
			setScanning(true);
			const resp = await configClient.scanClaudeDir(
				create(ScanClaudeDirRequestSchema, {
					source: scanSource,
				})
			);
			const hookItems = resp.items.filter((i) => i.itemType === 'hook');
			setDiscoveredItems(hookItems);
			setSelectedImportNames(new Set(hookItems.map((i) => i.name)));
			if (hookItems.length === 0) {
				toast.info('No new or modified hooks found');
			}
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to scan directory');
		} finally {
			setScanning(false);
		}
	};

	const handleImport = async () => {
		const itemsToImport = discoveredItems.filter((i) => selectedImportNames.has(i.name));
		if (itemsToImport.length === 0) {
			toast.error('Select at least one hook to import');
			return;
		}
		try {
			setImporting(true);
			const resp = await configClient.importHooks(
				create(ImportHooksRequestSchema, {
					items: itemsToImport,
				})
			);
			toast.success(`Imported ${resp.imported.length} hook(s)`);
			setDiscoveredItems([]);
			setSelectedImportNames(new Set());
			await loadData();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to import hooks');
		} finally {
			setImporting(false);
		}
	};

	const toggleExportId = (id: string) => {
		setSelectedExportIds((prev) => {
			const next = new Set(prev);
			if (next.has(id)) next.delete(id);
			else next.add(id);
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

	// Group hooks by event type for display
	const hooksByEvent = HOOK_EVENT_TYPES.reduce(
		(acc, eventType) => {
			acc[eventType] = hooks.filter((h) => h.eventType === eventType);
			return acc;
		},
		{} as Record<string, Hook[]>
	);

	// Collect hooks with unrecognized event types
	const knownTypes = new Set(HOOK_EVENT_TYPES);
	const otherHooks = hooks.filter((h) => !knownTypes.has(h.eventType));

	if (loading) {
		return (
			<div className="page environment-hooks-page">
				<div className="env-loading">Loading hooks...</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className="page environment-hooks-page">
				<div className="env-error">
					<p>{error}</p>
					<Button variant="secondary" onClick={loadData}>
						Retry
					</Button>
				</div>
			</div>
		);
	}

	const isEditorOpen = isCreating || editingHook !== null;

	return (
		<div className="page environment-hooks-page">
			<div className="env-page-header">
				<div>
					<h3>Hooks</h3>
					<p className="env-page-description">
						Configure shell commands that run at specific points during Claude Code execution.
					</p>
				</div>
				<Button variant="primary" size="sm" onClick={handleCreate}>
					<Icon name="plus" size={14} />
					Add Hook
				</Button>
			</div>

			<Tabs.Root defaultValue="library">
				<Tabs.List className="env-scope-tabs" aria-label="Hooks view">
					<Tabs.Trigger value="library">Library</Tabs.Trigger>
					<Tabs.Trigger value="export-import">Export / Import</Tabs.Trigger>
				</Tabs.List>

				<Tabs.Content value="library">
					<div className="hooks-groups">
						{HOOK_EVENT_TYPES.map((eventType) => {
							const info = HOOK_EVENT_INFO[eventType];
							const eventHooks = hooksByEvent[eventType] || [];

							return (
								<div key={eventType} className="hooks-group">
									<div className="hooks-group-header">
										<div className="hooks-group-title-row">
											<Icon name={info.icon} size={16} />
											<h4 className="hooks-group-title">{eventType}</h4>
											{eventHooks.length > 0 && (
												<span className="hooks-group-count">
													{eventHooks.length} hook{eventHooks.length !== 1 ? 's' : ''}
												</span>
											)}
										</div>
										<p className="hooks-group-description">{info.description}</p>
									</div>
									{eventHooks.length > 0 ? (
										<div className="hooks-list">
											{eventHooks.map((hook) => (
												<HookItem
													key={hook.id}
													hook={hook}
													onEdit={handleEdit}
													onClone={handleClone}
													onDelete={handleDelete}
												/>
											))}
										</div>
									) : (
										<div className="hooks-empty">No hooks configured</div>
									)}
								</div>
							);
						})}
						{otherHooks.length > 0 && (
							<div className="hooks-group">
								<div className="hooks-group-header">
									<div className="hooks-group-title-row">
										<Icon name="code" size={16} />
										<h4 className="hooks-group-title">Other</h4>
										<span className="hooks-group-count">
											{otherHooks.length} hook{otherHooks.length !== 1 ? 's' : ''}
										</span>
									</div>
									<p className="hooks-group-description">Hooks with custom event types</p>
								</div>
								<div className="hooks-list">
									{otherHooks.map((hook) => (
										<HookItem
											key={hook.id}
											hook={hook}
											onEdit={handleEdit}
											onClone={handleClone}
											onDelete={handleDelete}
										/>
									))}
								</div>
							</div>
						)}
					</div>
				</Tabs.Content>

				<Tabs.Content value="export-import">
					<div className="export-import-section">
						<div className="export-import-panel">
							<h4>Export Hooks</h4>
							<p className="env-page-description">Export hooks from the library to .claude/hooks/ directory.</p>
							<div className="export-import-controls">
								<div className="export-dest-selector">
									<Button
										variant={exportDest === SettingsScope.PROJECT ? 'primary' : 'secondary'}
										size="sm"
										onClick={() => setExportDest(SettingsScope.PROJECT)}
									>
										Project .claude/
									</Button>
									<Button
										variant={exportDest === SettingsScope.GLOBAL ? 'primary' : 'secondary'}
										size="sm"
										onClick={() => setExportDest(SettingsScope.GLOBAL)}
									>
										User ~/.claude/
									</Button>
								</div>
							</div>
							{hooks.length === 0 ? (
								<div className="hooks-empty">No hooks in library to export</div>
							) : (
								<div className="export-import-list">
									{hooks.map((hook) => (
										<label key={hook.id} className="export-import-item">
											<input
												type="checkbox"
												checked={selectedExportIds.has(hook.id)}
												onChange={() => toggleExportId(hook.id)}
											/>
											<span className="export-import-item-name">{hook.name}</span>
											<span className="export-import-item-meta">{hook.eventType}</span>
										</label>
									))}
								</div>
							)}
							<Button variant="primary" size="sm" onClick={handleExport} loading={exporting} disabled={selectedExportIds.size === 0}>
								Export Selected ({selectedExportIds.size})
							</Button>
						</div>

						<div className="export-import-panel">
							<h4>Import Hooks</h4>
							<p className="env-page-description">Scan .claude/hooks/ directory and import discovered hooks.</p>
							<div className="export-import-controls">
								<div className="export-dest-selector">
									<Button
										variant={scanSource === SettingsScope.PROJECT ? 'primary' : 'secondary'}
										size="sm"
										onClick={() => setScanSource(SettingsScope.PROJECT)}
									>
										Project .claude/
									</Button>
									<Button
										variant={scanSource === SettingsScope.GLOBAL ? 'primary' : 'secondary'}
										size="sm"
										onClick={() => setScanSource(SettingsScope.GLOBAL)}
									>
										User ~/.claude/
									</Button>
								</div>
								<Button variant="secondary" size="sm" onClick={handleScan} loading={scanning}>
									<Icon name="search" size={14} />
									Scan
								</Button>
							</div>
							{discoveredItems.length === 0 ? (
								<div className="hooks-empty">No items discovered. Click Scan to search.</div>
							) : (
								<div className="export-import-list">
									{discoveredItems.map((item) => (
										<label key={item.name} className="export-import-item">
											<input
												type="checkbox"
												checked={selectedImportNames.has(item.name)}
												onChange={() => toggleImportName(item.name)}
											/>
											<span className="export-import-item-name">{item.name}</span>
											<span className={`export-import-badge export-import-badge-${item.status}`}>
												{item.status}
											</span>
											{item.content && (
												<code className="export-import-preview">
													{item.content.length > 60 ? item.content.slice(0, 60) + '…' : item.content}
												</code>
											)}
										</label>
									))}
								</div>
							)}
							{discoveredItems.length > 0 && (
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
				open={isEditorOpen}
				onClose={handleClose}
				title={isCreating ? 'Create Hook' : `Edit Hook: ${editingHook?.name}`}
				size="lg"
			>
				<div className="hooks-editor">
					<div className="hooks-editor-field">
						<label htmlFor="hook-name">Name</label>
						<Input
							id="hook-name"
							value={formState.name}
							onChange={(e) => handleFormChange('name', e.target.value)}
							placeholder="my-hook"
							size="sm"
						/>
					</div>

					<div className="hooks-editor-field">
						<label htmlFor="hook-description">Description</label>
						<Input
							id="hook-description"
							value={formState.description}
							onChange={(e) => handleFormChange('description', e.target.value)}
							placeholder="What this hook does"
							size="sm"
						/>
					</div>

					<div className="hooks-editor-field">
						<label htmlFor="hook-event-type">Event Type</label>
						<select
							id="hook-event-type"
							className="input-field"
							value={formState.eventType}
							onChange={(e) => handleFormChange('eventType', e.target.value)}
							style={{ padding: 'var(--space-2)' }}
						>
							{HOOK_EVENT_TYPES.map((et) => (
								<option key={et} value={et}>
									{et} - {HOOK_EVENT_INFO[et].description}
								</option>
							))}
						</select>
					</div>

					<div className="hooks-editor-field">
						<label htmlFor="hook-content">Content (script body)</label>
						<textarea
							id="hook-content"
							className="input-field"
							value={formState.content}
							onChange={(e) => handleFormChange('content', e.target.value)}
							placeholder="#!/bin/bash&#10;# Hook script content"
							rows={8}
							style={{ fontFamily: 'var(--font-mono)', fontSize: '0.85rem', padding: 'var(--space-2)', resize: 'vertical' }}
						/>
					</div>

					<div className="hooks-editor-actions">
						<Button variant="secondary" onClick={handleClose}>
							Cancel
						</Button>
						<Button variant="primary" onClick={handleSave} loading={saving}>
							{isCreating ? 'Create Hook' : 'Save Changes'}
						</Button>
					</div>
				</div>
			</Modal>
		</div>
	);
}

// Hook item component
function HookItem({
	hook,
	onEdit,
	onClone,
	onDelete,
}: {
	hook: Hook;
	onEdit: (hook: Hook) => void;
	onClone: (hook: Hook) => void;
	onDelete: (hook: Hook) => void;
}) {
	return (
		<div className="hook-item">
			<div className="hook-item-main">
				<div className="hook-item-info">
					<div className="hook-name">
						{hook.name}
						{hook.isBuiltin && (
							<span className="skill-card-badge" style={{ marginLeft: '0.5rem' }}>Built-in</span>
						)}
					</div>
					{hook.description && (
						<div className="hook-description" style={{ color: 'var(--text-secondary)', fontSize: '0.85rem' }}>
							{hook.description}
						</div>
					)}
					{hook.content && (
						<div className="hook-command">
							<Icon name="terminal" size={12} />
							<code>{hook.content.length > 80 ? hook.content.slice(0, 80) + '…' : hook.content}</code>
						</div>
					)}
				</div>
				<div className="hook-item-actions">
					{!hook.isBuiltin && (
						<Button
							variant="ghost"
							size="sm"
							iconOnly
							onClick={() => onEdit(hook)}
							aria-label="Edit"
						>
							<Icon name="edit" size={14} />
						</Button>
					)}
					<Button
						variant="ghost"
						size="sm"
						iconOnly
						onClick={() => onClone(hook)}
						aria-label="Clone"
					>
						<Icon name="copy" size={14} />
					</Button>
					{!hook.isBuiltin && (
						<Button
							variant="ghost"
							size="sm"
							iconOnly
							onClick={() => onDelete(hook)}
							aria-label="Delete"
						>
							<Icon name="trash" size={14} />
						</Button>
					)}
				</div>
			</div>
		</div>
	);
}
