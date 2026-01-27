/**
 * ClaudeMdPage - CLAUDE.md editor with split-view preview
 *
 * Provides editing for Global and Project CLAUDE.md files with:
 * - Tab switching between scopes
 * - ConfigEditor for syntax-highlighted editing
 * - Live markdown preview pane
 */

import { useState, useEffect, useCallback } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { create } from '@bufbuild/protobuf';
import { Icon } from '@/components/ui/Icon';
import type { IconName } from '@/components/ui/Icon';
import { ConfigEditor } from '@/components/settings/ConfigEditor';
import { useDocumentTitle } from '@/hooks';
import { configClient } from '@/lib/client';
import {
	type ClaudeMd,
	SettingsScope,
	GetClaudeMdRequestSchema,
	UpdateClaudeMdRequestSchema,
} from '@/gen/orc/v1/config_pb';
import './ClaudeMdPage.css';

type ScopeTab = 'global' | 'project';

const SCOPE_INFO: Record<ScopeTab, { label: string; icon: IconName; description: string }> = {
	global: {
		label: 'Global',
		icon: 'globe',
		description: 'System-wide instructions (~/.claude/CLAUDE.md)',
	},
	project: {
		label: 'Project',
		icon: 'folder',
		description: 'Project-specific instructions (./CLAUDE.md)',
	},
};

/**
 * Convert markdown to HTML for preview rendering.
 * Handles: headings (h1-h6), lists (ul/li), code blocks, paragraphs.
 */
function renderMarkdown(markdown: string): string {
	const escapeHtml = (text: string): string => {
		return text
			.replace(/&/g, '&amp;')
			.replace(/</g, '&lt;')
			.replace(/>/g, '&gt;')
			.replace(/"/g, '&quot;')
			.replace(/'/g, '&#039;');
	};

	const lines = markdown.split('\n');
	const result: string[] = [];
	let inCodeBlock = false;
	let inList = false;

	for (let i = 0; i < lines.length; i++) {
		const line = lines[i];

		// Code blocks
		if (line.startsWith('```')) {
			if (inCodeBlock) {
				result.push('</code></pre>');
				inCodeBlock = false;
			} else {
				if (inList) {
					result.push('</ul>');
					inList = false;
				}
				result.push('<pre><code>');
				inCodeBlock = true;
			}
			continue;
		}

		if (inCodeBlock) {
			result.push(escapeHtml(line));
			continue;
		}

		// Headings
		const headingMatch = line.match(/^(#{1,6})\s+(.*)$/);
		if (headingMatch) {
			if (inList) {
				result.push('</ul>');
				inList = false;
			}
			const level = headingMatch[1].length;
			const text = escapeHtml(headingMatch[2]);
			result.push(`<h${level}>${text}</h${level}>`);
			continue;
		}

		// List items
		const listMatch = line.match(/^[-*]\s+(.*)$/);
		if (listMatch) {
			if (!inList) {
				result.push('<ul>');
				inList = true;
			}
			result.push(`<li>${escapeHtml(listMatch[1])}</li>`);
			continue;
		}

		// Empty line ends list
		if (line.trim() === '' && inList) {
			result.push('</ul>');
			inList = false;
			result.push('<br>');
			continue;
		}

		// Regular text as paragraph
		if (line.trim()) {
			if (inList) {
				result.push('</ul>');
				inList = false;
			}
			result.push(`<p>${escapeHtml(line)}</p>`);
		}
	}

	// Close any open tags
	if (inList) {
		result.push('</ul>');
	}
	if (inCodeBlock) {
		result.push('</code></pre>');
	}

	return result.join('\n');
}

export function ClaudeMdPage() {
	useDocumentTitle('CLAUDE.md Editor');
	const [files, setFiles] = useState<ClaudeMd[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [activeScope, setActiveScope] = useState<ScopeTab>('project');

	// Track edited content separately from loaded content
	const [editedContent, setEditedContent] = useState<Record<ScopeTab, string>>({
		global: '',
		project: '',
	});

	const loadFiles = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const response = await configClient.getClaudeMd(create(GetClaudeMdRequestSchema, {}));
			setFiles(response.files);

			// Initialize edited content from loaded files
			const projectFile = response.files.find((f) => f.scope === SettingsScope.PROJECT);
			const globalFile = response.files.find((f) => f.scope === SettingsScope.GLOBAL);

			setEditedContent({
				project: projectFile?.content ?? '',
				global: globalFile?.content ?? '',
			});

			// Set initial scope to first available with content
			if (projectFile?.content) {
				setActiveScope('project');
			} else if (globalFile?.content) {
				setActiveScope('global');
			}
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load CLAUDE.md files');
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		loadFiles();
	}, [loadFiles]);

	// Get file for a scope
	const getFileForScope = useCallback(
		(scope: ScopeTab): ClaudeMd | undefined => {
			const targetScope = scope === 'project' ? SettingsScope.PROJECT : SettingsScope.GLOBAL;
			return files.find((f) => f.scope === targetScope);
		},
		[files]
	);

	const hasContent = useCallback(
		(scope: ScopeTab): boolean => {
			const file = getFileForScope(scope);
			return Boolean(file?.content);
		},
		[getFileForScope]
	);

	// Handle content change for a specific scope
	const handleContentChange = useCallback(
		(scope: ScopeTab, content: string) => {
			setEditedContent((prev) => ({
				...prev,
				[scope]: content,
			}));
		},
		[]
	);

	// Save handler for a specific scope
	const handleSave = useCallback(
		async (scope: ScopeTab) => {
			try {
				const targetScope =
					scope === 'project' ? SettingsScope.PROJECT : SettingsScope.GLOBAL;
				await configClient.updateClaudeMd(
					create(UpdateClaudeMdRequestSchema, {
						scope: targetScope,
						content: editedContent[scope],
					})
				);
				// Reload to get updated files
				await loadFiles();
			} catch (err) {
				setError(err instanceof Error ? err.message : 'Failed to save CLAUDE.md');
			}
		},
		[editedContent, loadFiles]
	);

	// Check if any files exist
	const hasAnyFiles = files.length > 0;

	if (loading) {
		return (
			<div className="page claudemd-page">
				<div className="claudemd-loading">Loading CLAUDE.md files...</div>
			</div>
		);
	}

	if (error && !hasAnyFiles) {
		return (
			<div className="page claudemd-page">
				<div className="claudemd-error" data-testid="claudemd-error">
					<Icon name="alert-circle" size={16} />
					<span>{error}</span>
				</div>
			</div>
		);
	}

	if (!hasAnyFiles) {
		return (
			<div className="page claudemd-page">
				<div className="claudemd-header">
					<div>
						<h3>CLAUDE.md Editor</h3>
						<p className="claudemd-description">
							Edit your project's CLAUDE.md instructions file.
						</p>
					</div>
				</div>
				<div className="claudemd-empty" data-testid="claudemd-empty-state">
					<Icon name="file-text" size={48} />
					<h4>No CLAUDE.md Files Found</h4>
					<p>
						No CLAUDE.md files exist yet. Create a CLAUDE.md file in your project root
						or in ~/.claude/ to get started.
					</p>
				</div>
			</div>
		);
	}

	return (
		<div className="page claudemd-page">
			<div className="claudemd-header">
				<div>
					<h3>CLAUDE.md Editor</h3>
					<p className="claudemd-description">
						Edit your project's CLAUDE.md instructions file.
					</p>
				</div>
			</div>

			{error && (
				<div className="claudemd-error" data-testid="claudemd-error">
					<Icon name="alert-circle" size={16} />
					<span>{error}</span>
				</div>
			)}

			<Tabs.Root value={activeScope} onValueChange={(v) => setActiveScope(v as ScopeTab)}>
				<Tabs.List className="claudemd-tabs">
					{(['project', 'global'] as ScopeTab[]).map((scope) => {
						const info = SCOPE_INFO[scope];
						const has = hasContent(scope);
						return (
							<Tabs.Trigger
								key={scope}
								value={scope}
								className={`claudemd-tab ${!has ? 'empty' : ''}`}
								role="tab"
							>
								<Icon name={info.icon} size={14} />
								{info.label}
								{has && <span className="claudemd-tab-indicator" />}
							</Tabs.Trigger>
						);
					})}
				</Tabs.List>

				{(['project', 'global'] as ScopeTab[]).map((scope) => {
					const scopeInfo = SCOPE_INFO[scope];
					const scopeFile = getFileForScope(scope);
					const scopeContent = editedContent[scope];
					const renderedMarkdown = renderMarkdown(scopeContent);

					return (
						<Tabs.Content key={scope} value={scope} className="claudemd-content">
							<div className="claudemd-scope-info">
								<Icon name={scopeInfo.icon} size={16} />
								<span>{scopeInfo.description}</span>
								{scopeFile?.path && (
									<code className="claudemd-path">{scopeFile.path}</code>
								)}
							</div>

							<div className="claudemd-split-view">
								<div className="claudemd-editor-pane" data-testid="claudemd-editor-pane">
									<ConfigEditor
										filePath={scopeFile?.path ?? `${scope}/CLAUDE.md`}
										content={scopeContent}
										onChange={(content) => handleContentChange(scope, content)}
										onSave={() => handleSave(scope)}
										language="markdown"
									/>
								</div>

								<div className="claudemd-preview-pane" data-testid="claudemd-preview-pane">
									<div className="claudemd-preview-header">
										<Icon name="eye" size={14} />
										<span>Preview</span>
									</div>
									<div
										className="claudemd-preview-content"
										dangerouslySetInnerHTML={{ __html: renderedMarkdown }}
									/>
								</div>
							</div>
						</Tabs.Content>
					);
				})}
			</Tabs.Root>
		</div>
	);
}
