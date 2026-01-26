/**
 * Hooks page (/environment/hooks)
 * Displays and edits Claude Code hooks from settings.json
 */

import { useState, useEffect, useCallback } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { create } from '@bufbuild/protobuf';
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
	HookEvent,
	SettingsScope,
	ListHooksRequestSchema,
	CreateHookRequestSchema,
	UpdateHookRequestSchema,
	DeleteHookRequestSchema,
} from '@/gen/orc/v1/config_pb';
import './environment.css';

type ScopeTab = 'project' | 'global';

// Convert UI scope tab to protobuf SettingsScope enum
function toSettingsScope(scope: ScopeTab): SettingsScope {
	return scope === 'global' ? SettingsScope.GLOBAL : SettingsScope.PROJECT;
}

// Convert HookEvent enum to display string
function hookEventToString(event: HookEvent): string {
	switch (event) {
		case HookEvent.PRE_TOOL_USE:
			return 'PreToolUse';
		case HookEvent.POST_TOOL_USE:
			return 'PostToolUse';
		case HookEvent.NOTIFICATION:
			return 'Notification';
		case HookEvent.STOP:
			return 'Stop';
		default:
			return 'Unknown';
	}
}

// Convert string to HookEvent enum
function stringToHookEvent(str: string): HookEvent {
	switch (str) {
		case 'PreToolUse':
			return HookEvent.PRE_TOOL_USE;
		case 'PostToolUse':
			return HookEvent.POST_TOOL_USE;
		case 'Notification':
			return HookEvent.NOTIFICATION;
		case 'Stop':
			return HookEvent.STOP;
		default:
			return HookEvent.UNSPECIFIED;
	}
}

// Available hook events
const HOOK_EVENTS = [
	HookEvent.PRE_TOOL_USE,
	HookEvent.POST_TOOL_USE,
	HookEvent.NOTIFICATION,
	HookEvent.STOP,
];

// Hook event descriptions
const HOOK_EVENT_INFO: Record<HookEvent, { icon: IconName; description: string }> = {
	[HookEvent.UNSPECIFIED]: {
		icon: 'code',
		description: 'Unspecified event',
	},
	[HookEvent.PRE_TOOL_USE]: {
		icon: 'play',
		description: 'Runs before a tool is executed',
	},
	[HookEvent.POST_TOOL_USE]: {
		icon: 'check',
		description: 'Runs after a tool completes',
	},
	[HookEvent.NOTIFICATION]: {
		icon: 'info',
		description: 'Runs on notifications',
	},
	[HookEvent.STOP]: {
		icon: 'pause',
		description: 'Runs when execution stops',
	},
};

// Editor form state for creating/editing a hook
interface HookFormState {
	name: string;
	event: HookEvent;
	matcher: string;
	command: string;
	workingDir: string;
	timeout: number;
	enabled: boolean;
}

const defaultFormState: HookFormState = {
	name: '',
	event: HookEvent.PRE_TOOL_USE,
	matcher: '*',
	command: '',
	workingDir: '',
	timeout: 30,
	enabled: true,
};

export function Hooks() {
	useDocumentTitle('Hooks');
	const [scope, setScope] = useState<ScopeTab>('project');
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
				create(ListHooksRequestSchema, { scope: toSettingsScope(scope) })
			);
			setHooks(response.hooks);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load hooks');
		} finally {
			setLoading(false);
		}
	}, [scope]);

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
			event: hook.event,
			matcher: hook.matcher || '*',
			command: hook.command,
			workingDir: hook.workingDir || '',
			timeout: hook.timeout || 30,
			enabled: hook.enabled,
		});
		setEditingHook(hook);
		setIsCreating(false);
	};

	const handleClose = () => {
		setEditingHook(null);
		setIsCreating(false);
		setFormState(defaultFormState);
	};

	const handleFormChange = (field: keyof HookFormState, value: string | number | boolean | HookEvent) => {
		setFormState((prev) => ({ ...prev, [field]: value }));
	};

	const handleSave = async () => {
		if (!formState.name.trim() || !formState.command.trim()) {
			toast.error('Name and command are required');
			return;
		}

		try {
			setSaving(true);

			if (isCreating) {
				await configClient.createHook(
					create(CreateHookRequestSchema, {
						name: formState.name.trim(),
						event: formState.event,
						matcher: formState.matcher || undefined,
						command: formState.command.trim(),
						workingDir: formState.workingDir || undefined,
						timeout: formState.timeout,
						scope: toSettingsScope(scope),
					})
				);
				toast.success('Hook created');
			} else if (editingHook) {
				await configClient.updateHook(
					create(UpdateHookRequestSchema, {
						name: editingHook.name,
						scope: toSettingsScope(scope),
						event: formState.event,
						matcher: formState.matcher || undefined,
						command: formState.command.trim(),
						workingDir: formState.workingDir || undefined,
						timeout: formState.timeout,
						enabled: formState.enabled,
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
					name: hook.name,
					scope: toSettingsScope(scope),
				})
			);
			toast.success('Hook deleted');
			await loadData();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to delete hook');
		}
	};

	const handleToggleEnabled = async (hook: Hook) => {
		try {
			await configClient.updateHook(
				create(UpdateHookRequestSchema, {
					name: hook.name,
					scope: toSettingsScope(scope),
					enabled: !hook.enabled,
				})
			);
			await loadData();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to update hook');
		}
	};

	// Group hooks by event type for display
	const hooksByEvent = HOOK_EVENTS.reduce(
		(acc, event) => {
			acc[event] = hooks.filter((h) => h.event === event);
			return acc;
		},
		{} as Record<HookEvent, Hook[]>
	);

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

			<Tabs.Root value={scope} onValueChange={(v) => setScope(v as ScopeTab)}>
				<Tabs.List className="env-scope-tabs">
					<Tabs.Trigger value="project" className="env-scope-tab">
						<Icon name="folder" size={14} />
						Project
					</Tabs.Trigger>
					<Tabs.Trigger value="global" className="env-scope-tab">
						<Icon name="globe" size={14} />
						Global
					</Tabs.Trigger>
				</Tabs.List>

				<Tabs.Content value={scope}>
					<div className="hooks-groups">
						{HOOK_EVENTS.map((event) => {
							const info = HOOK_EVENT_INFO[event];
							const eventHooks = hooksByEvent[event] || [];

							return (
								<div key={event} className="hooks-group">
									<div className="hooks-group-header">
										<div className="hooks-group-title-row">
											<Icon name={info.icon} size={16} />
											<h4 className="hooks-group-title">{hookEventToString(event)}</h4>
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
												<div
													key={hook.name}
													className={`hook-item ${!hook.enabled ? 'disabled' : ''}`}
												>
													<div className="hook-item-main">
														<div className="hook-item-info">
															<div className="hook-name">{hook.name}</div>
															{hook.matcher && hook.matcher !== '*' && (
																<div className="hook-matcher">
																	<Icon name="target" size={12} />
																	<code>{hook.matcher}</code>
																</div>
															)}
															<div className="hook-command">
																<Icon name="terminal" size={12} />
																<code>{hook.command}</code>
															</div>
														</div>
														<div className="hook-item-actions">
															<Button
																variant="ghost"
																size="sm"
																iconOnly
																onClick={() => handleToggleEnabled(hook)}
																aria-label={hook.enabled ? 'Disable' : 'Enable'}
															>
																<Icon
																	name={hook.enabled ? 'eye' : 'eye-off'}
																	size={16}
																/>
															</Button>
															<Button
																variant="ghost"
																size="sm"
																iconOnly
																onClick={() => handleEdit(hook)}
																aria-label="Edit"
															>
																<Icon name="edit" size={14} />
															</Button>
															<Button
																variant="ghost"
																size="sm"
																iconOnly
																onClick={() => handleDelete(hook)}
																aria-label="Delete"
															>
																<Icon name="trash" size={14} />
															</Button>
														</div>
													</div>
												</div>
											))}
										</div>
									) : (
										<div className="hooks-empty">No hooks configured</div>
									)}
								</div>
							);
						})}
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
							disabled={!isCreating}
						/>
					</div>

					<div className="hooks-editor-field">
						<label htmlFor="hook-event">Event</label>
						<select
							id="hook-event"
							className="input-field"
							value={hookEventToString(formState.event)}
							onChange={(e) => handleFormChange('event', stringToHookEvent(e.target.value))}
							style={{ padding: 'var(--space-2)' }}
						>
							{HOOK_EVENTS.map((event) => (
								<option key={event} value={hookEventToString(event)}>
									{hookEventToString(event)} - {HOOK_EVENT_INFO[event].description}
								</option>
							))}
						</select>
					</div>

					<div className="hooks-editor-field">
						<label htmlFor="hook-matcher">Matcher Pattern</label>
						<Input
							id="hook-matcher"
							value={formState.matcher}
							onChange={(e) => handleFormChange('matcher', e.target.value)}
							placeholder="* (all) or specific tool name"
							size="sm"
						/>
						<p className="hooks-editor-hint">
							Use <code>*</code> to match all tools, or specify a tool name like{' '}
							<code>Edit</code> or <code>Bash</code>.
						</p>
					</div>

					<div className="hooks-editor-field">
						<label htmlFor="hook-command">Command</label>
						<Input
							id="hook-command"
							value={formState.command}
							onChange={(e) => handleFormChange('command', e.target.value)}
							placeholder="Shell command to execute"
							size="sm"
						/>
					</div>

					<div className="hooks-editor-field">
						<label htmlFor="hook-workdir">Working Directory (optional)</label>
						<Input
							id="hook-workdir"
							value={formState.workingDir}
							onChange={(e) => handleFormChange('workingDir', e.target.value)}
							placeholder="/path/to/directory"
							size="sm"
						/>
					</div>

					<div className="hooks-editor-field">
						<label htmlFor="hook-timeout">Timeout (seconds)</label>
						<Input
							id="hook-timeout"
							type="number"
							min={1}
							max={300}
							value={formState.timeout}
							onChange={(e) => handleFormChange('timeout', parseInt(e.target.value) || 30)}
							size="sm"
						/>
					</div>

					{!isCreating && (
						<div className="hooks-editor-field">
							<label className="settings-checkbox-label">
								<input
									type="checkbox"
									checked={formState.enabled}
									onChange={(e) => handleFormChange('enabled', e.target.checked)}
								/>
								Enabled
							</label>
						</div>
					)}

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
