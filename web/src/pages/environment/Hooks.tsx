/**
 * Hooks page (/environment/hooks)
 * Displays and manages hooks stored in GlobalDB hook_scripts table.
 */

import { useState, useEffect, useCallback } from 'react';
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
	ListHooksRequestSchema,
	CreateHookRequestSchema,
	UpdateHookRequestSchema,
	DeleteHookRequestSchema,
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
