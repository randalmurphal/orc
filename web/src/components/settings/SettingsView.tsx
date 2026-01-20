/**
 * SettingsView component - Slash commands section content with CommandList and ConfigEditor.
 *
 * Features:
 * - Page header with title, subtitle, and "New Command" action button
 * - Split view: CommandList on left, ConfigEditor on right
 * - State management for selected command
 * - Mock data for development
 */

import { useState, useCallback } from 'react';
import { Button } from '../ui/Button';
import { Icon } from '../ui/Icon';
import { CommandList, type Command } from './CommandList';
import { ConfigEditor } from './ConfigEditor';
import './SettingsView.css';

// Mock data for development
const MOCK_COMMANDS: Command[] = [
	{
		id: 'commit',
		name: '/commit',
		description: 'Create a git commit with a generated message based on staged changes',
		scope: 'project',
		path: '.claude/commands/commit.md',
	},
	{
		id: 'review',
		name: '/review',
		description: 'Review the current diff and provide feedback on code quality',
		scope: 'project',
		path: '.claude/commands/review.md',
	},
	{
		id: 'test',
		name: '/test',
		description: 'Run tests related to the current changes and report results',
		scope: 'project',
		path: '.claude/commands/test.md',
	},
	{
		id: 'doc',
		name: '/doc',
		description: 'Generate documentation for selected code or functions',
		scope: 'global',
		path: '~/.claude/commands/doc.md',
	},
	{
		id: 'explain',
		name: '/explain',
		description: 'Explain the selected code or concept in detail',
		scope: 'global',
		path: '~/.claude/commands/explain.md',
	},
];

// Mock content for a selected command
const MOCK_COMMAND_CONTENT = `# Commit Command

Generate a descriptive commit message based on staged changes.

## Behavior

1. Analyze the staged changes using \`git diff --cached\`
2. Generate a commit message following conventional commits format
3. Present the message for approval before committing

## Template

\`\`\`
<type>(<scope>): <description>

<body>
\`\`\`

## Types

- **feat**: A new feature
- **fix**: A bug fix
- **docs**: Documentation only changes
- **style**: Changes that do not affect the meaning of the code
- **refactor**: A code change that neither fixes a bug nor adds a feature
- **test**: Adding missing tests or correcting existing tests
- **chore**: Changes to the build process or auxiliary tools
`;

export function SettingsView() {
	const [commands, setCommands] = useState<Command[]>(MOCK_COMMANDS);
	const [selectedId, setSelectedId] = useState<string | undefined>(MOCK_COMMANDS[0]?.id);
	const [editorContent, setEditorContent] = useState(MOCK_COMMAND_CONTENT);

	const selectedCommand = commands.find((c) => c.id === selectedId);

	const handleSelect = useCallback((id: string) => {
		setSelectedId(id);
		// In a real implementation, this would fetch the command content from the API
		setEditorContent(MOCK_COMMAND_CONTENT);
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

	const handleSave = useCallback(() => {
		// TODO: Save content to API when endpoint is available
	}, []);

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
