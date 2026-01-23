/**
 * SettingsView component - Slash commands section content with CommandList and ConfigEditor.
 *
 * Features:
 * - Page header with title, subtitle, and "New Command" action button
 * - Split view: CommandList on left, ConfigEditor on right
 * - State management for selected command
 * - Mock data for development
 */

import { useState, useCallback, useEffect } from 'react';
import { Button } from '../ui/Button';
import { Icon } from '../ui/Icon';
import { CommandList, type Command } from './CommandList';
import { ConfigEditor } from './ConfigEditor';
import * as api from '@/lib/api';
import './SettingsView.css';

export function SettingsView() {
	const [commands, setCommands] = useState<Command[]>([]);
	const [selectedId, setSelectedId] = useState<string | undefined>(undefined);
	const [editorContent, setEditorContent] = useState('');

	const selectedCommand = commands.find((c) => c.id === selectedId);

	// Fetch skills from API on mount
	useEffect(() => {
		const fetchSkills = async () => {
			try {
				const skills = await api.listSkills();
				const commandsFromSkills: Command[] = skills.map((skill) => ({
					id: skill.name,
					name: `/${skill.name}`,
					description: skill.description,
					scope: skill.path.includes('/.claude/') ? 'global' : 'project',
					path: skill.path,
				}));
				setCommands(commandsFromSkills);

				// Auto-select first command if available
				if (commandsFromSkills.length > 0) {
					setSelectedId(commandsFromSkills[0].id);
				}
			} catch (err) {
				console.error('Failed to fetch skills:', err);
			}
		};

		fetchSkills();
	}, []);

	// Fetch command content when selection changes
	useEffect(() => {
		if (!selectedId) return;

		const fetchCommandContent = async () => {
			try {
				// scope is optional, let API decide based on skill location
				const skill = await api.getSkill(selectedId, undefined);
				setEditorContent(skill?.content || '');
			} catch (err) {
				console.error('Failed to fetch command content:', err);
				setEditorContent('');
			}
		};

		fetchCommandContent();
	}, [selectedId]);

	const handleSelect = useCallback((id: string) => {
		setSelectedId(id);
	}, []);

	const handleDelete = useCallback((id: string) => {
		setCommands((prev) => prev.filter((c) => c.id !== id));
		if (selectedId === id) {
			setSelectedId(commands[0]?.id !== id ? commands[0]?.id : commands[1]?.id);
		}
	}, [selectedId, commands]);

	const handleContentChange = useCallback((content: string) => {
		setEditorContent(content);
	}, []);

	const handleSave = useCallback(async () => {
		if (!selectedId || !selectedCommand) return;

		try {
			await api.updateSkill(selectedId, {
				name: selectedId,
				description: selectedCommand.description,
				content: editorContent,
			});
		} catch (err) {
			console.error('Failed to save command:', err);
		}
	}, [selectedId, selectedCommand, editorContent]);

	const handleNewCommand = useCallback(() => {
		// TODO: Open modal to create a new command when implemented
	}, []);

	return (
		<div className="settings-view">
			{/* Page Header */}
			<header className="settings-view__header">
				<div className="settings-view__header-content">
					<h2 className="settings-view__title">Slash Commands</h2>
					<p className="settings-view__subtitle">
						Create and manage custom slash commands for Claude
					</p>
				</div>
				<Button
					variant="primary"
					size="sm"
					leftIcon={<Icon name="plus" size={14} />}
					onClick={handleNewCommand}
				>
					New Command
				</Button>
			</header>

			{/* Content Area */}
			<div className="settings-view__content">
				{/* Command List */}
				<div className="settings-view__list">
					<CommandList
						commands={commands}
						selectedId={selectedId}
						onSelect={handleSelect}
						onDelete={handleDelete}
					/>
				</div>

				{/* Editor */}
				<div className="settings-view__editor">
					{selectedCommand ? (
						<ConfigEditor
							filePath={selectedCommand.path || ''}
							content={editorContent}
							onChange={handleContentChange}
							onSave={handleSave}
							language="markdown"
						/>
					) : (
						<div className="settings-view__empty">
							<Icon name="terminal" size={32} />
							<p>Select a command to edit</p>
						</div>
					)}
				</div>
			</div>
		</div>
	);
}
