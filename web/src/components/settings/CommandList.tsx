/**
 * CommandList component - displays slash commands organized by scope (project/global).
 * Each command shows an icon, name, description, and action buttons for editing/deleting.
 */

import { type KeyboardEvent, useCallback, useState } from 'react';
import { Icon } from '../ui/Icon';
import './CommandList.css';

export interface Command {
	id: string;
	name: string;
	description: string;
	scope: 'project' | 'global';
	path?: string;
}

export interface CommandListProps {
	commands: Command[];
	selectedId?: string;
	onSelect: (id: string) => void;
	onDelete: (id: string) => void;
}

interface CommandItemProps {
	command: Command;
	isSelected: boolean;
	onSelect: (id: string) => void;
	onDelete: (id: string) => void;
}

function CommandItem({ command, isSelected, onSelect, onDelete }: CommandItemProps) {
	const [showConfirm, setShowConfirm] = useState(false);

	const handleClick = useCallback(() => {
		onSelect(command.id);
	}, [command.id, onSelect]);

	const handleKeyDown = useCallback(
		(e: KeyboardEvent) => {
			if (e.key === 'Enter' || e.key === ' ') {
				e.preventDefault();
				onSelect(command.id);
			}
		},
		[command.id, onSelect]
	);

	const handleEditClick = useCallback(
		(e: React.MouseEvent) => {
			e.stopPropagation();
			onSelect(command.id);
		},
		[command.id, onSelect]
	);

	const handleDeleteClick = useCallback((e: React.MouseEvent) => {
		e.stopPropagation();
		setShowConfirm(true);
	}, []);

	const handleConfirmDelete = useCallback(
		(e: React.MouseEvent) => {
			e.stopPropagation();
			onDelete(command.id);
			setShowConfirm(false);
		},
		[command.id, onDelete]
	);

	const handleCancelDelete = useCallback((e: React.MouseEvent) => {
		e.stopPropagation();
		setShowConfirm(false);
	}, []);

	const handleConfirmKeyDown = useCallback(
		(e: KeyboardEvent) => {
			if (e.key === 'Enter' || e.key === ' ') {
				e.preventDefault();
				e.stopPropagation();
				onDelete(command.id);
				setShowConfirm(false);
			} else if (e.key === 'Escape') {
				e.preventDefault();
				setShowConfirm(false);
			}
		},
		[command.id, onDelete]
	);

	const handleCancelKeyDown = useCallback((e: KeyboardEvent) => {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			e.stopPropagation();
			setShowConfirm(false);
		} else if (e.key === 'Escape') {
			e.preventDefault();
			setShowConfirm(false);
		}
	}, []);

	const iconClass = `command-icon ${command.scope === 'global' ? 'global' : ''}`;

	return (
		<div
			className={`command-item ${isSelected ? 'selected' : ''}`}
			role="button"
			tabIndex={0}
			onClick={handleClick}
			onKeyDown={handleKeyDown}
			aria-pressed={isSelected}
		>
			<div className={iconClass}>
				<Icon name="terminal" size={16} />
			</div>
			<div className="command-info">
				<div className="command-name">{command.name}</div>
				<div className="command-desc" title={command.description}>
					{command.description}
				</div>
			</div>
			<div className="command-actions">
				{showConfirm ? (
					<>
						<button
							className="command-btn command-btn-confirm"
							onClick={handleConfirmDelete}
							onKeyDown={handleConfirmKeyDown}
							aria-label="Confirm delete"
							type="button"
						>
							<Icon name="check" size={14} />
						</button>
						<button
							className="command-btn command-btn-cancel"
							onClick={handleCancelDelete}
							onKeyDown={handleCancelKeyDown}
							aria-label="Cancel delete"
							type="button"
						>
							<Icon name="x" size={14} />
						</button>
					</>
				) : (
					<>
						<button
							className="command-btn"
							onClick={handleEditClick}
							aria-label={`Edit ${command.name}`}
							type="button"
						>
							<Icon name="edit" size={14} />
						</button>
						<button
							className="command-btn"
							onClick={handleDeleteClick}
							aria-label={`Delete ${command.name}`}
							type="button"
						>
							<Icon name="trash" size={14} />
						</button>
					</>
				)}
			</div>
		</div>
	);
}

export function CommandList({ commands, selectedId, onSelect, onDelete }: CommandListProps) {
	const projectCommands = commands.filter((c) => c.scope === 'project');
	const globalCommands = commands.filter((c) => c.scope === 'global');

	if (commands.length === 0) {
		return (
			<div className="command-list-empty">
				<Icon name="terminal" size={32} />
				<div className="command-list-empty-title">No commands</div>
				<div className="command-list-empty-desc">Create a command to get started</div>
			</div>
		);
	}

	return (
		<div className="command-list-container">
			{projectCommands.length > 0 && (
				<div className="section">
					<div className="section-header">
						<div className="section-title">Project Commands</div>
						<div className="section-desc">
							Commands available in the current project. Stored in .claude/commands/
						</div>
					</div>
					<div className="command-list">
						{projectCommands.map((command) => (
							<CommandItem
								key={command.id}
								command={command}
								isSelected={selectedId === command.id}
								onSelect={onSelect}
								onDelete={onDelete}
							/>
						))}
					</div>
				</div>
			)}

			{globalCommands.length > 0 && (
				<div className="section">
					<div className="section-header">
						<div className="section-title">Global Commands</div>
						<div className="section-desc">
							Available in all projects. Stored in ~/.claude/commands/
						</div>
					</div>
					<div className="command-list">
						{globalCommands.map((command) => (
							<CommandItem
								key={command.id}
								command={command}
								isSelected={selectedId === command.id}
								onSelect={onSelect}
								onDelete={onDelete}
							/>
						))}
					</div>
				</div>
			)}
		</div>
	);
}
