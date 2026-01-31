/**
 * LibraryPicker - Multi-select picker with grouping support.
 *
 * Used for selecting hooks, skills, and MCP servers from the existing library.
 * Hooks are grouped by event type. Skills show name + description.
 * MCP servers show name + command preview.
 */

import { useCallback } from 'react';
import type { Hook, Skill } from '@/gen/orc/v1/config_pb';
import type { MCPServerInfo } from '@/gen/orc/v1/mcp_pb';
import './LibraryPicker.css';

export type LibraryPickerType = 'hooks' | 'skills' | 'mcpServers';

export interface LibraryPickerProps {
	/** Type of items being picked */
	type: LibraryPickerType;
	/** Available items from the library */
	items: Hook[] | Skill[] | MCPServerInfo[];
	/** Currently selected item names */
	selectedNames: string[];
	/** Callback when selection changes */
	onSelectionChange: (names: string[]) => void;
	/** Error message to display */
	error?: string;
	/** Whether the picker is loading */
	loading?: boolean;
	/** Whether the picker is disabled */
	disabled?: boolean;
}

const HOOK_EVENT_LABELS: Record<string, string> = {
	'PreToolUse': 'PreToolUse',
	'PostToolUse': 'PostToolUse',
	'Stop': 'Stop',
	'Notification': 'Notification',
};

const EMPTY_MESSAGES: Record<LibraryPickerType, string> = {
	hooks: 'No hooks configured',
	skills: 'No skills configured',
	mcpServers: 'No MCP servers configured',
};

export function LibraryPicker({
	type,
	items,
	selectedNames,
	onSelectionChange,
	error,
	loading = false,
	disabled = false,
}: LibraryPickerProps) {
	const toggleSelection = useCallback(
		(name: string) => {
			if (disabled) return;
			if (selectedNames.includes(name)) {
				onSelectionChange(selectedNames.filter((n) => n !== name));
			} else {
				onSelectionChange([...selectedNames, name]);
			}
		},
		[selectedNames, onSelectionChange, disabled]
	);

	if (error) {
		return <div className="library-picker__error">{error}</div>;
	}

	if (loading) {
		return <div className="library-picker__loading">Loading...</div>;
	}

	if (items.length === 0) {
		return <div className="library-picker__empty">{EMPTY_MESSAGES[type]}</div>;
	}

	if (type === 'hooks') {
		return renderHooks(items as Hook[], selectedNames, toggleSelection);
	}

	if (type === 'skills') {
		return renderSkills(items as Skill[], selectedNames, toggleSelection);
	}

	return renderMCPServers(items as MCPServerInfo[], selectedNames, toggleSelection);
}

function renderHooks(
	hooks: Hook[],
	selectedNames: string[],
	toggleSelection: (name: string) => void
) {
	// Group hooks by event type
	const groups = new Map<string, Hook[]>();
	for (const hook of hooks) {
		const event = hook.eventType || 'Unknown';
		if (!groups.has(event)) {
			groups.set(event, []);
		}
		groups.get(event)!.push(hook);
	}

	return (
		<div className="library-picker">
			{Array.from(groups.entries()).map(([event, groupHooks]) => (
				<div key={event} className="library-picker__group">
					<div className="library-picker__group-header">
						{HOOK_EVENT_LABELS[event] || `Event ${event}`}
					</div>
					{groupHooks.map((hook) => {
						const isSelected = selectedNames.includes(hook.name);
						return (
							<div
								key={hook.name}
								className={`library-picker__item ${isSelected ? 'library-picker__item--selected' : ''}`}
								data-selected={isSelected || undefined}
								onClick={() => toggleSelection(hook.name)}
							>
								<span className="library-picker__item-name">{hook.name}</span>
								<span className="library-picker__item-detail">{hook.eventType}</span>
							</div>
						);
					})}
				</div>
			))}
		</div>
	);
}

function renderSkills(
	skills: Skill[],
	selectedNames: string[],
	toggleSelection: (name: string) => void
) {
	return (
		<div className="library-picker">
			{skills.map((skill) => {
				const isSelected = selectedNames.includes(skill.name);
				return (
					<div
						key={skill.name}
						className={`library-picker__item ${isSelected ? 'library-picker__item--selected' : ''}`}
						data-selected={isSelected || undefined}
						onClick={() => toggleSelection(skill.name)}
					>
						<span className="library-picker__item-name">{skill.name}</span>
						<span className="library-picker__item-detail">{skill.description}</span>
					</div>
				);
			})}
		</div>
	);
}

function renderMCPServers(
	servers: MCPServerInfo[],
	selectedNames: string[],
	toggleSelection: (name: string) => void
) {
	return (
		<div className="library-picker">
			{servers.map((server) => {
				const isSelected = selectedNames.includes(server.name);
				return (
					<div
						key={server.name}
						className={`library-picker__item ${isSelected ? 'library-picker__item--selected' : ''}`}
						data-selected={isSelected || undefined}
						onClick={() => toggleSelection(server.name)}
					>
						<span className="library-picker__item-name">{server.name}</span>
						<span className="library-picker__item-detail library-picker__item-detail--mono">
							{server.command}
						</span>
					</div>
				);
			})}
		</div>
	);
}
