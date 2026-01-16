/**
 * Hooks page (/environment/hooks)
 * Displays and edits Claude Code hooks from settings.json
 */

import { useState, useEffect, useCallback } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Icon } from '@/components/ui/Icon';
import { Modal } from '@/components/overlays/Modal';
import type { IconName } from '@/components/ui/Icon';
import { toast } from '@/stores';
import { useDocumentTitle } from '@/hooks';
import {
	listHooks,
	getHookTypes,
	updateHook,
	type HooksMap,
	type Hook,
	type HookEvent,
} from '@/lib/api';
import './environment.css';

type Scope = 'project' | 'global';

// Hook event descriptions
const HOOK_EVENT_INFO: Record<string, { icon: IconName; description: string }> = {
	PreToolUse: {
		icon: 'play',
		description: 'Runs before a tool is executed',
	},
	PostToolUse: {
		icon: 'check',
		description: 'Runs after a tool completes',
	},
	PreCompact: {
		icon: 'minimize-2',
		description: 'Runs before context compaction',
	},
	PrePrompt: {
		icon: 'message-square',
		description: 'Runs before sending a prompt',
	},
	Stop: {
		icon: 'pause',
		description: 'Runs when execution stops',
	},
};

export function Hooks() {
	useDocumentTitle('Hooks');
	const [scope, setScope] = useState<Scope>('project');
	const [hooks, setHooks] = useState<HooksMap>({});
	const [hookTypes, setHookTypes] = useState<HookEvent[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Editor modal state
	const [editingEvent, setEditingEvent] = useState<string | null>(null);
	const [editorHooks, setEditorHooks] = useState<Hook[]>([]);
	const [saving, setSaving] = useState(false);

	const loadData = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const [hooksData, types] = await Promise.all([listHooks(scope), getHookTypes()]);
			setHooks(hooksData);
			setHookTypes(types);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load hooks');
		} finally {
			setLoading(false);
		}
	}, [scope]);

	useEffect(() => {
		loadData();
	}, [loadData]);

	const handleEdit = (event: string) => {
		const eventHooks = hooks[event] || [];
		setEditorHooks(
			eventHooks.length > 0
				? eventHooks.map((h) => ({ ...h, hooks: [...h.hooks] }))
				: [{ matcher: '*', hooks: [{ type: 'command', command: '' }] }]
		);
		setEditingEvent(event);
	};

	const handleAddHook = () => {
		setEditorHooks([...editorHooks, { matcher: '*', hooks: [{ type: 'command', command: '' }] }]);
	};

	const handleRemoveHook = (index: number) => {
		setEditorHooks(editorHooks.filter((_, i) => i !== index));
	};

	const handleUpdateMatcher = (index: number, matcher: string) => {
		const updated = [...editorHooks];
		updated[index] = { ...updated[index], matcher };
		setEditorHooks(updated);
	};

	const handleUpdateCommand = (hookIndex: number, entryIndex: number, command: string) => {
		const updated = [...editorHooks];
		const hookEntries = [...updated[hookIndex].hooks];
		hookEntries[entryIndex] = { ...hookEntries[entryIndex], command };
		updated[hookIndex] = { ...updated[hookIndex], hooks: hookEntries };
		setEditorHooks(updated);
	};

	const handleAddEntry = (hookIndex: number) => {
		const updated = [...editorHooks];
		updated[hookIndex] = {
			...updated[hookIndex],
			hooks: [...updated[hookIndex].hooks, { type: 'command', command: '' }],
		};
		setEditorHooks(updated);
	};

	const handleRemoveEntry = (hookIndex: number, entryIndex: number) => {
		const updated = [...editorHooks];
		updated[hookIndex] = {
			...updated[hookIndex],
			hooks: updated[hookIndex].hooks.filter((_, i) => i !== entryIndex),
		};
		setEditorHooks(updated);
	};

	const handleSave = async () => {
		if (!editingEvent) return;

		// Filter out empty hooks
		const filteredHooks = editorHooks.filter(
			(h) => h.matcher.trim() && h.hooks.some((e) => e.command.trim())
		);

		try {
			setSaving(true);
			await updateHook(editingEvent, filteredHooks, scope);
			toast.success(`${editingEvent} hooks saved`);
			setEditingEvent(null);
			await loadData();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to save hooks');
		} finally {
			setSaving(false);
		}
	};

	const handleDelete = async (event: string) => {
		if (!confirm(`Delete all ${event} hooks?`)) {
			return;
		}
		try {
			await updateHook(event, [], scope);
			toast.success(`${event} hooks deleted`);
			await loadData();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to delete hooks');
		}
	};

	const countHooks = (event: string): number => {
		return (hooks[event] || []).reduce((sum, h) => sum + h.hooks.length, 0);
	};

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

	return (
		<div className="page environment-hooks-page">
			<div className="env-page-header">
				<div>
					<h3>Hooks</h3>
					<p className="env-page-description">
						Configure shell commands that run at specific points during Claude Code execution.
					</p>
				</div>
			</div>

			<Tabs.Root value={scope} onValueChange={(v) => setScope(v as Scope)}>
				<Tabs.List className="env-scope-tabs">
					<Tabs.Trigger value="project" className="env-scope-tab">
						<Icon name="folder" size={14} />
						Project
					</Tabs.Trigger>
					<Tabs.Trigger value="global" className="env-scope-tab">
						<Icon name="user" size={14} />
						Global
					</Tabs.Trigger>
				</Tabs.List>

				<Tabs.Content value={scope}>
					<div className="hooks-groups">
						{hookTypes.map((event) => {
							const info = HOOK_EVENT_INFO[event] || {
								icon: 'code',
								description: 'Custom hook event',
							};
							const eventHooks = hooks[event] || [];
							const count = countHooks(event);

							return (
								<div key={event} className="hooks-group">
									<div className="hooks-group-header">
										<div className="hooks-group-title-row">
											<Icon name={info.icon} size={16} />
											<h4 className="hooks-group-title">{event}</h4>
											{count > 0 && (
												<span className="hooks-group-count">
													{count} hook{count !== 1 ? 's' : ''}
												</span>
											)}
										</div>
										<p className="hooks-group-description">{info.description}</p>
									</div>
									<div className="hooks-group-actions">
										<Button variant="ghost" size="sm" onClick={() => handleEdit(event)}>
											<Icon name="edit" size={14} />
											{count > 0 ? 'Edit' : 'Add'}
										</Button>
										{count > 0 && (
											<Button
												variant="ghost"
												size="sm"
												onClick={() => handleDelete(event)}
											>
												<Icon name="trash" size={14} />
											</Button>
										)}
									</div>
									{eventHooks.length > 0 && (
										<div className="hooks-list">
											{eventHooks.map((hook, i) => (
												<div key={i} className="hook-item">
													<div className="hook-matcher">
														<Icon name="target" size={12} />
														<code>{hook.matcher}</code>
													</div>
													<div className="hook-commands">
														{hook.hooks.map((entry, j) => (
															<div key={j} className="hook-command">
																<Icon name="terminal" size={12} />
																<code>{entry.command}</code>
															</div>
														))}
													</div>
												</div>
											))}
										</div>
									)}
									{eventHooks.length === 0 && (
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
				open={editingEvent !== null}
				onClose={() => setEditingEvent(null)}
				title={`Edit ${editingEvent} Hooks`}
				size="lg"
			>
				<div className="hooks-editor">
					<p className="hooks-editor-hint">
						Each hook has a <strong>matcher</strong> pattern (e.g., <code>*</code> for all,{' '}
						<code>Edit</code> for specific tool) and one or more <strong>commands</strong> to
						run.
					</p>

					{editorHooks.map((hook, hookIndex) => (
						<div key={hookIndex} className="hooks-editor-item">
							<div className="hooks-editor-item-header">
								<div className="hooks-editor-matcher">
									<label>Matcher Pattern</label>
									<Input
										value={hook.matcher}
										onChange={(e) => handleUpdateMatcher(hookIndex, e.target.value)}
										placeholder="* or tool name"
										size="sm"
									/>
								</div>
								<Button
									variant="ghost"
									size="sm"
									onClick={() => handleRemoveHook(hookIndex)}
									aria-label="Remove hook"
								>
									<Icon name="trash" size={14} />
								</Button>
							</div>

							<div className="hooks-editor-commands">
								<label>Commands</label>
								{hook.hooks.map((entry, entryIndex) => (
									<div key={entryIndex} className="hooks-editor-command-row">
										<Input
											value={entry.command}
											onChange={(e) =>
												handleUpdateCommand(hookIndex, entryIndex, e.target.value)
											}
											placeholder="Shell command to run"
											size="sm"
										/>
										{hook.hooks.length > 1 && (
											<Button
												variant="ghost"
												size="sm"
												onClick={() => handleRemoveEntry(hookIndex, entryIndex)}
												aria-label="Remove command"
											>
												<Icon name="x" size={14} />
											</Button>
										)}
									</div>
								))}
								<Button
									variant="ghost"
									size="sm"
									onClick={() => handleAddEntry(hookIndex)}
									className="hooks-editor-add-command"
								>
									<Icon name="plus" size={14} />
									Add Command
								</Button>
							</div>
						</div>
					))}

					<Button variant="secondary" onClick={handleAddHook} className="hooks-editor-add">
						<Icon name="plus" size={14} />
						Add Hook
					</Button>

					<div className="hooks-editor-actions">
						<Button variant="secondary" onClick={() => setEditingEvent(null)}>
							Cancel
						</Button>
						<Button variant="primary" onClick={handleSave} loading={saving}>
							Save Hooks
						</Button>
					</div>
				</div>
			</Modal>
		</div>
	);
}
