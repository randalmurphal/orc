/**
 * SettingsView component - Slash commands section content with CommandList and ConfigEditor.
 *
 * Features:
 * - Page header with title, subtitle, and "New Command" action button
 * - Split view: CommandList on left, ConfigEditor on right
 * - State management for selected command
 * - Mock data for development
 */

import { useState, useCallback, useEffect, useMemo } from 'react';
import { Button } from '../ui/Button';
import { Icon } from '../ui/Icon';
import { CommandList, type Command } from './CommandList';
import { ConfigEditor } from './ConfigEditor';
import { NewCommandModal } from './NewCommandModal';
import { configClient } from '@/lib/client';
import type { Skill } from '@/gen/orc/v1/config_pb';
import { SettingsScope } from '@/gen/orc/v1/config_pb';
import './SettingsView.css';

// Helper to convert SettingsScope enum to Command scope string
function scopeToString(scope: SettingsScope): 'project' | 'global' {
	return scope === SettingsScope.GLOBAL ? 'global' : 'project';
}

// Helper to generate a synthetic path for display
function skillToPath(skill: Skill): string {
	const base = skill.scope === SettingsScope.GLOBAL ? '~/.claude/commands' : '.claude/commands';
	return `${base}/${skill.name}.md`;
}

export function SettingsView() {
	const [skills, setSkills] = useState<Skill[]>([]);
	const [selectedId, setSelectedId] = useState<string | undefined>(undefined);
	const [editorContent, setEditorContent] = useState('');
	const [isModalOpen, setIsModalOpen] = useState(false);

	// Convert skills to commands for CommandList
	const commands: Command[] = useMemo(() => {
		return skills.map((skill) => ({
			id: skill.name,
			name: `/${skill.name}`,
			description: skill.description,
			scope: scopeToString(skill.scope),
			path: skillToPath(skill),
		}));
	}, [skills]);

	const selectedCommand = commands.find((c) => c.id === selectedId);
	const selectedSkill = skills.find((s) => s.name === selectedId);

	// Fetch skills from API on mount
	useEffect(() => {
		const fetchSkills = async () => {
			try {
				const response = await configClient.listSkills({});
				setSkills(response.skills);

				// Auto-select first skill if available
				if (response.skills.length > 0) {
					setSelectedId(response.skills[0].name);
				}
			} catch (err) {
				console.error('Failed to fetch skills:', err);
			}
		};

		fetchSkills();
	}, []);

	// Update editor content when selection changes (skills already have content)
	useEffect(() => {
		if (selectedSkill) {
			setEditorContent(selectedSkill.content);
		} else {
			setEditorContent('');
		}
	}, [selectedSkill]);

	const handleSelect = useCallback((id: string) => {
		setSelectedId(id);
	}, []);

	const handleDelete = useCallback(async (id: string) => {
		try {
			const skillToDelete = skills.find((s) => s.name === id);
			if (!skillToDelete) return;

			await configClient.deleteSkill({ id: skillToDelete.id });
			setSkills((prev) => prev.filter((s) => s.name !== id));

			if (selectedId === id) {
				const remaining = skills.filter((s) => s.name !== id);
				setSelectedId(remaining[0]?.name);
			}
		} catch (err) {
			console.error('Failed to delete skill:', err);
		}
	}, [selectedId, skills]);

	const handleContentChange = useCallback((content: string) => {
		setEditorContent(content);
	}, []);

	const handleSave = useCallback(async () => {
		if (!selectedId || !selectedSkill) return;

		try {
			await configClient.updateSkill({
				id: selectedSkill.id,
				name: selectedId,
				description: selectedSkill.description,
				content: editorContent,
			});
		} catch (err) {
			console.error('Failed to save command:', err);
		}
	}, [selectedId, selectedSkill, editorContent]);

	const handleNewCommand = useCallback(() => {
		setIsModalOpen(true);
	}, []);

	const handleModalClose = useCallback(() => {
		setIsModalOpen(false);
	}, []);

	const handleSkillCreate = useCallback((skill: Skill) => {
		setSkills((prev) => [...prev, skill]);
		setSelectedId(skill.name);
	}, []);

	return (
		<div className="settings-view">
			{/* Page Header */}
			<header className="settings-view__header">
				<div className="settings-view__header-content">
					<h2 className="settings-view__title">Slash Commands</h2>
					<p className="settings-view__subtitle">
						Custom commands for Claude Code (~/.claude/commands)
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

			{/* New Command Modal */}
			<NewCommandModal
				open={isModalOpen}
				onClose={handleModalClose}
				onCreate={handleSkillCreate}
			/>
		</div>
	);
}
