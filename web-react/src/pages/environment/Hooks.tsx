import { useState, useEffect, useCallback } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import {
	listHooks,
	getHookTypes,
	updateHook,
	deleteHook,
	type HooksMap,
	type HookEvent,
	type Hook,
	type HookEntry,
} from '@/lib/api';
import './Hooks.css';

type Scope = 'global' | 'project';

/**
 * Hooks page (/environment/hooks)
 *
 * Manages Claude Code hooks (settings.json format) at two levels:
 * - Global (~/.claude/settings.json)
 * - Project (.claude/settings.json)
 */
export function Hooks() {
	const [searchParams] = useSearchParams();
	const [hooks, setHooks] = useState<HooksMap>({});
	const [hookTypes, setHookTypes] = useState<HookEvent[]>([]);
	const [selectedEvent, setSelectedEvent] = useState<string | null>(null);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [success, setSuccess] = useState<string | null>(null);
	const [isEditing, setIsEditing] = useState(false);

	// Form state
	const [editingHooks, setEditingHooks] = useState<Hook[]>([]);

	const scope = searchParams.get('scope') as Scope | null;
	const isGlobal = scope === 'global';
	const scopeParam: 'global' | 'project' | undefined = isGlobal ? 'global' : 'project';

	const loadData = useCallback(async () => {
		setLoading(true);
		setError(null);

		try {
			const [hooksData, types] = await Promise.all([
				listHooks(scopeParam),
				getHookTypes(),
			]);
			setHooks(hooksData);
			setHookTypes(types);

			// Select first event with hooks if available
			const firstEvent = Object.keys(hooksData)[0] || types[0] || null;
			setSelectedEvent(firstEvent);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load hooks');
		} finally {
			setLoading(false);
		}
	}, [scopeParam]);

	useEffect(() => {
		loadData();
	}, [loadData]);

	const selectEvent = (event: string) => {
		setSelectedEvent(event);
		setIsEditing(false);
		setEditingHooks(hooks[event] || []);
	};

	const startEditing = () => {
		if (selectedEvent) {
			setEditingHooks(hooks[selectedEvent] || []);
			setIsEditing(true);
		}
	};

	const cancelEditing = () => {
		setIsEditing(false);
		if (selectedEvent) {
			setEditingHooks(hooks[selectedEvent] || []);
		}
	};

	const addHook = () => {
		setEditingHooks([
			...editingHooks,
			{
				matcher: '',
				hooks: [{ type: 'command', command: '' }],
			},
		]);
	};

	const removeHook = (index: number) => {
		setEditingHooks(editingHooks.filter((_, i) => i !== index));
	};

	const updateHookMatcher = (index: number, matcher: string) => {
		const updated = [...editingHooks];
		updated[index] = { ...updated[index], matcher };
		setEditingHooks(updated);
	};

	const addHookEntry = (hookIndex: number) => {
		const updated = [...editingHooks];
		updated[hookIndex] = {
			...updated[hookIndex],
			hooks: [...updated[hookIndex].hooks, { type: 'command', command: '' }],
		};
		setEditingHooks(updated);
	};

	const removeHookEntry = (hookIndex: number, entryIndex: number) => {
		const updated = [...editingHooks];
		updated[hookIndex] = {
			...updated[hookIndex],
			hooks: updated[hookIndex].hooks.filter((_, i) => i !== entryIndex),
		};
		setEditingHooks(updated);
	};

	const updateHookEntry = (
		hookIndex: number,
		entryIndex: number,
		entry: Partial<HookEntry>
	) => {
		const updated = [...editingHooks];
		updated[hookIndex] = {
			...updated[hookIndex],
			hooks: updated[hookIndex].hooks.map((h, i) =>
				i === entryIndex ? { ...h, ...entry } : h
			),
		};
		setEditingHooks(updated);
	};

	const handleSave = async () => {
		if (!selectedEvent) return;

		setSaving(true);
		setError(null);
		setSuccess(null);

		try {
			// Filter out empty hooks
			const validHooks = editingHooks.filter(
				(h) => h.hooks.length > 0 && h.hooks.some((e) => e.command.trim())
			);

			if (validHooks.length > 0) {
				await updateHook(selectedEvent, validHooks, scopeParam);
			} else {
				await deleteHook(selectedEvent, scopeParam);
			}

			await loadData();
			setIsEditing(false);
			setSuccess('Hooks saved successfully');
			setTimeout(() => setSuccess(null), 3000);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to save hooks');
		} finally {
			setSaving(false);
		}
	};

	const getEventDescription = (event: string): string => {
		switch (event) {
			case 'PreToolUse':
				return 'Runs before Claude uses any tool';
			case 'PostToolUse':
				return 'Runs after Claude uses any tool';
			case 'PreCompact':
				return 'Runs before conversation is compacted';
			case 'PrePrompt':
				return 'Runs before the prompt is sent to Claude';
			case 'Stop':
				return 'Runs when Claude finishes';
			default:
				return '';
		}
	};

	return (
		<div className="hooks-page">
			<header className="hooks-header">
				<div className="header-content">
					<div>
						<h1>{isGlobal ? 'Global ' : ''}Claude Code Hooks</h1>
						<p className="subtitle">
							Configure hook commands that run at specific events
						</p>
					</div>
					<div className="header-actions">
						<div className="scope-toggle">
							<Link
								to="/environment/hooks"
								className={`scope-btn ${!isGlobal ? 'active' : ''}`}
							>
								Project
							</Link>
							<Link
								to="/environment/hooks?scope=global"
								className={`scope-btn ${isGlobal ? 'active' : ''}`}
							>
								Global
							</Link>
						</div>
					</div>
				</div>
			</header>

			{error && <div className="alert alert-error">{error}</div>}
			{success && <div className="alert alert-success">{success}</div>}

			{loading ? (
				<div className="loading-state">Loading hooks...</div>
			) : (
				<div className="hooks-layout">
					{/* Event List */}
					<aside className="event-list">
						<h2>Events</h2>
						<ul>
							{hookTypes.map((event) => (
								<li key={event}>
									<button
										className={`event-item ${selectedEvent === event ? 'selected' : ''}`}
										onClick={() => selectEvent(event)}
									>
										<span className="event-name">{event}</span>
										{hooks[event] && hooks[event].length > 0 && (
											<span className="hook-count">{hooks[event].length}</span>
										)}
									</button>
								</li>
							))}
						</ul>
					</aside>

					{/* Editor Panel */}
					<div className="editor-panel">
						{selectedEvent ? (
							<>
								<div className="editor-header">
									<div>
										<h2>{selectedEvent}</h2>
										<p className="event-desc">{getEventDescription(selectedEvent)}</p>
									</div>
									{!isEditing ? (
										<button className="btn btn-primary" onClick={startEditing}>
											Edit
										</button>
									) : (
										<div className="header-buttons">
											<button
												className="btn btn-secondary"
												onClick={cancelEditing}
												disabled={saving}
											>
												Cancel
											</button>
											<button
												className="btn btn-primary"
												onClick={handleSave}
												disabled={saving}
											>
												{saving ? 'Saving...' : 'Save'}
											</button>
										</div>
									)}
								</div>

								<div className="hooks-content">
									{isEditing ? (
										<div className="hooks-editor">
											{editingHooks.length === 0 ? (
												<div className="empty-editor">
													<p>No hooks configured for this event</p>
													<button className="btn btn-primary" onClick={addHook}>
														Add Hook
													</button>
												</div>
											) : (
												<>
													{editingHooks.map((hook, hookIndex) => (
														<div className="hook-card" key={hookIndex}>
															<div className="hook-card-header">
																<div className="form-group">
																	<label htmlFor={`matcher-${hookIndex}`}>
																		Matcher (glob pattern)
																	</label>
																	<input
																		id={`matcher-${hookIndex}`}
																		type="text"
																		value={hook.matcher}
																		onChange={(e) =>
																			updateHookMatcher(hookIndex, e.target.value)
																		}
																		placeholder="* (matches all tools)"
																	/>
																</div>
																<button
																	className="btn-icon btn-danger"
																	onClick={() => removeHook(hookIndex)}
																	title="Remove hook"
																>
																	&times;
																</button>
															</div>

															<div className="hook-entries">
																{hook.hooks.map((entry, entryIndex) => (
																	<div className="hook-entry" key={entryIndex}>
																		<select
																			value={entry.type}
																			onChange={(e) =>
																				updateHookEntry(hookIndex, entryIndex, {
																					type: e.target.value,
																				})
																			}
																		>
																			<option value="command">Command</option>
																			<option value="url">URL</option>
																		</select>
																		<input
																			type="text"
																			value={entry.command}
																			onChange={(e) =>
																				updateHookEntry(hookIndex, entryIndex, {
																					command: e.target.value,
																				})
																			}
																			placeholder={
																				entry.type === 'command'
																					? 'echo "Hook triggered"'
																					: 'https://example.com/webhook'
																			}
																		/>
																		<button
																			className="btn-icon"
																			onClick={() =>
																				removeHookEntry(hookIndex, entryIndex)
																			}
																			title="Remove entry"
																		>
																			&times;
																		</button>
																	</div>
																))}
																<button
																	className="btn-sm btn-secondary"
																	onClick={() => addHookEntry(hookIndex)}
																>
																	+ Add Command
																</button>
															</div>
														</div>
													))}
													<button className="btn btn-secondary" onClick={addHook}>
														+ Add Hook
													</button>
												</>
											)}
										</div>
									) : (
										<div className="hooks-view">
											{!hooks[selectedEvent] || hooks[selectedEvent].length === 0 ? (
												<div className="no-hooks">
													<p>No hooks configured for this event</p>
													<button className="btn btn-primary" onClick={startEditing}>
														Add Hook
													</button>
												</div>
											) : (
												<div className="hooks-list">
													{hooks[selectedEvent].map((hook, i) => (
														<div className="hook-view" key={i}>
															<div className="hook-matcher">
																<span className="label">Matcher:</span>
																<code>{hook.matcher || '*'}</code>
															</div>
															<div className="hook-commands">
																{hook.hooks.map((entry, j) => (
																	<div className="hook-command" key={j}>
																		<span className="entry-type">{entry.type}:</span>
																		<code>{entry.command}</code>
																	</div>
																))}
															</div>
														</div>
													))}
												</div>
											)}
										</div>
									)}
								</div>
							</>
						) : (
							<div className="no-selection">
								<p>Select an event type from the list</p>
							</div>
						)}
					</div>
				</div>
			)}
		</div>
	);
}
