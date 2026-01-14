/**
 * CommandPalette - Search and execute commands
 *
 * Features:
 * - Search input with fuzzy filtering
 * - Command sections (Tasks, Navigation, Environment, Settings, Projects, View)
 * - Keyboard navigation (↑/↓ to navigate, Enter to select, Escape to close)
 * - Search match highlighting
 */

import { useState, useEffect, useRef, useMemo, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { createPortal } from 'react-dom';
import { Icon } from '@/components/ui/Icon';
import { formatShortcut } from '@/lib/platform';
import './CommandPalette.css';

interface CommandPaletteProps {
	open: boolean;
	onClose: () => void;
}

interface Command {
	id: string;
	label: string;
	description?: string;
	icon: string;
	shortcut?: string;
	action: () => void;
	category: string;
}

function escapeRegex(str: string): string {
	return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function highlightMatch(text: string, query: string): React.ReactNode {
	if (!query.trim()) return text;
	const regex = new RegExp(`(${escapeRegex(query)})`, 'gi');
	const parts = text.split(regex);
	return parts.map((part, i) =>
		regex.test(part) ? <mark key={i}>{part}</mark> : part
	);
}

export function CommandPalette({ open, onClose }: CommandPaletteProps) {
	const navigate = useNavigate();
	const [searchQuery, setSearchQuery] = useState('');
	const [selectedIndex, setSelectedIndex] = useState(0);
	const inputRef = useRef<HTMLInputElement>(null);

	// Define commands
	const commands = useMemo<Command[]>(() => [
		// Tasks
		{
			id: 'new-task',
			label: 'New Task',
			description: 'Create a new task',
			icon: '+',
			shortcut: 'N',
			action: () => {
				onClose();
				window.dispatchEvent(new CustomEvent('orc:new-task'));
			},
			category: 'Tasks',
		},
		{
			id: 'go-tasks',
			label: 'Go to Tasks',
			description: 'View all tasks',
			icon: '■',
			action: () => {
				onClose();
				navigate('/');
			},
			category: 'Navigation',
		},
		{
			id: 'go-dashboard',
			label: 'Go to Dashboard',
			description: 'Overview and quick stats',
			icon: '■',
			action: () => {
				onClose();
				navigate('/dashboard');
			},
			category: 'Navigation',
		},
		// Environment
		{
			id: 'go-environment',
			label: 'Go to Environment',
			description: 'Claude ecosystem overview',
			icon: '⚙',
			action: () => {
				onClose();
				navigate('/environment');
			},
			category: 'Environment',
		},
		{
			id: 'go-skills',
			label: 'Go to Skills',
			description: 'Manage Claude skills',
			icon: '⚡',
			action: () => {
				onClose();
				navigate('/environment/claude/skills');
			},
			category: 'Environment',
		},
		{
			id: 'go-hooks',
			label: 'Go to Hooks',
			description: 'Configure event hooks',
			icon: '↻',
			action: () => {
				onClose();
				navigate('/environment/claude/hooks');
			},
			category: 'Environment',
		},
		{
			id: 'go-agents',
			label: 'Go to Agents',
			description: 'Sub-agent configurations',
			icon: '✦',
			action: () => {
				onClose();
				navigate('/environment/claude/agents');
			},
			category: 'Environment',
		},
		{
			id: 'go-tools',
			label: 'Go to Tools',
			description: 'Tool permissions',
			icon: '⚒',
			action: () => {
				onClose();
				navigate('/environment/claude/tools');
			},
			category: 'Environment',
		},
		{
			id: 'go-mcp',
			label: 'Go to MCP Servers',
			description: 'Manage MCP integrations',
			icon: '⊚',
			action: () => {
				onClose();
				navigate('/environment/claude/mcp');
			},
			category: 'Environment',
		},
		{
			id: 'go-prompts',
			label: 'Go to Prompts',
			description: 'Manage prompt templates',
			icon: '☰',
			action: () => {
				onClose();
				navigate('/environment/orchestrator/prompts');
			},
			category: 'Environment',
		},
		{
			id: 'go-scripts',
			label: 'Go to Scripts',
			description: 'Script registry',
			icon: '☰',
			action: () => {
				onClose();
				navigate('/environment/orchestrator/scripts');
			},
			category: 'Environment',
		},
		{
			id: 'go-automation',
			label: 'Go to Automation',
			description: 'Orc configuration',
			icon: '⚙',
			action: () => {
				onClose();
				navigate('/environment/orchestrator/automation');
			},
			category: 'Environment',
		},
		{
			id: 'go-export',
			label: 'Go to Export',
			description: 'Export configuration',
			icon: '⬆',
			action: () => {
				onClose();
				navigate('/environment/orchestrator/export');
			},
			category: 'Environment',
		},
		{
			id: 'go-knowledge',
			label: 'Go to Knowledge',
			description: 'Knowledge queue (patterns, gotchas, decisions)',
			icon: '📚',
			action: () => {
				onClose();
				navigate('/environment/knowledge');
			},
			category: 'Environment',
		},
		{
			id: 'go-docs',
			label: 'Go to Documentation',
			description: 'CLAUDE.md hierarchy',
			icon: '☰',
			action: () => {
				onClose();
				navigate('/environment/docs');
			},
			category: 'Environment',
		},
		{
			id: 'go-preferences',
			label: 'Go to Preferences',
			description: 'Personal and global settings',
			icon: '⚙',
			action: () => {
				onClose();
				navigate('/preferences');
			},
			category: 'Settings',
		},
		// Projects & View
		{
			id: 'switch-project',
			label: 'Switch Project',
			description: 'Change active project',
			icon: '⇄',
			shortcut: 'P',
			action: () => {
				onClose();
				window.dispatchEvent(new CustomEvent('orc:switch-project'));
			},
			category: 'Projects',
		},
		{
			id: 'toggle-sidebar',
			label: 'Toggle Sidebar',
			description: 'Show/hide navigation',
			icon: '☰',
			shortcut: 'B',
			action: () => {
				onClose();
				window.dispatchEvent(new CustomEvent('orc:toggle-sidebar'));
			},
			category: 'View',
		},
	], [onClose, navigate]);

	// Filter commands
	const filteredCommands = useMemo(() => {
		if (!searchQuery.trim()) return commands;
		const query = searchQuery.toLowerCase();
		return commands.filter(
			(cmd) =>
				cmd.label.toLowerCase().includes(query) ||
				cmd.description?.toLowerCase().includes(query) ||
				cmd.category.toLowerCase().includes(query)
		);
	}, [commands, searchQuery]);

	// Group by category
	const groupedCommands = useMemo(() => {
		const groups: Record<string, Command[]> = {};
		for (const cmd of filteredCommands) {
			if (!groups[cmd.category]) {
				groups[cmd.category] = [];
			}
			groups[cmd.category].push(cmd);
		}
		return groups;
	}, [filteredCommands]);

	// Reset state when modal opens
	useEffect(() => {
		if (open) {
			setSearchQuery('');
			setSelectedIndex(0);
			setTimeout(() => inputRef.current?.focus(), 50);
		}
	}, [open]);

	// Reset selected index when search changes
	useEffect(() => {
		setSelectedIndex(0);
	}, [searchQuery]);

	// Handle keyboard navigation
	const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
		switch (e.key) {
			case 'ArrowDown':
				e.preventDefault();
				setSelectedIndex((prev) => Math.min(prev + 1, filteredCommands.length - 1));
				break;
			case 'ArrowUp':
				e.preventDefault();
				setSelectedIndex((prev) => Math.max(prev - 1, 0));
				break;
			case 'Enter':
				e.preventDefault();
				if (filteredCommands[selectedIndex]) {
					filteredCommands[selectedIndex].action();
				}
				break;
			case 'Escape':
				onClose();
				break;
		}
	}, [filteredCommands, selectedIndex, onClose]);

	// Handle backdrop click
	const handleBackdropClick = (e: React.MouseEvent) => {
		if (e.target === e.currentTarget) {
			onClose();
		}
	};

	if (!open) return null;

	const content = (
		<div
			className="palette-backdrop"
			role="dialog"
			aria-modal="true"
			aria-label="Command palette"
			onClick={handleBackdropClick}
			onKeyDown={handleKeyDown}
		>
			<div className="palette-content">
				{/* Search Input */}
				<div className="palette-search">
					<Icon name="search" size={16} className="search-icon" />
					<input
						ref={inputRef}
						type="text"
						value={searchQuery}
						onChange={(e) => setSearchQuery(e.target.value)}
						placeholder="Type a command or search..."
						className="search-input"
						aria-label="Search commands"
					/>
					<kbd className="search-hint">esc</kbd>
				</div>

				{/* Results */}
				<div className="palette-results">
					{Object.keys(groupedCommands).length > 0 ? (
						Object.entries(groupedCommands).map(([category, cmds]) => (
							<div key={category} className="result-group">
								<div className="group-label">{category}</div>
								{cmds.map((cmd) => {
									const globalIndex = filteredCommands.indexOf(cmd);
									return (
										<button
											key={cmd.id}
											className={`result-item ${globalIndex === selectedIndex ? 'selected' : ''}`}
											onClick={() => cmd.action()}
											onMouseEnter={() => setSelectedIndex(globalIndex)}
										>
											<span className="item-icon">{cmd.icon}</span>
											<div className="item-content">
												<span className="item-label">
													{highlightMatch(cmd.label, searchQuery)}
												</span>
												{cmd.description && (
													<span className="item-description">
														{highlightMatch(cmd.description, searchQuery)}
													</span>
												)}
											</div>
											{cmd.shortcut && (
												<kbd className="item-shortcut">
													{formatShortcut(cmd.shortcut)}
												</kbd>
											)}
										</button>
									);
								})}
							</div>
						))
					) : (
						<div className="no-results">
							<span className="no-results-icon">?</span>
							<p>No commands found</p>
						</div>
					)}
				</div>

				{/* Footer */}
				<div className="palette-footer">
					<div className="footer-hint">
						<kbd>↑</kbd><kbd>↓</kbd> to navigate
					</div>
					<div className="footer-hint">
						<kbd>↵</kbd> to select
					</div>
				</div>
			</div>
		</div>
	);

	return createPortal(content, document.body);
}
