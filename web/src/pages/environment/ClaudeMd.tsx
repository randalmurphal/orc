/**
 * CLAUDE.md page (/environment/claudemd)
 * Displays CLAUDE.md files from different scopes
 */

import { useState, useEffect, useCallback } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { create } from '@bufbuild/protobuf';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import type { IconName } from '@/components/ui/Icon';
import { useDocumentTitle } from '@/hooks';
import { configClient } from '@/lib/client';
import {
	type ClaudeMd as ClaudeMdType,
	SettingsScope,
	GetClaudeMdRequestSchema,
} from '@/gen/orc/v1/config_pb';
import './environment.css';

type ScopeTab = 'global' | 'project';

const SCOPE_INFO: Record<ScopeTab, { label: string; icon: IconName; description: string }> = {
	global: {
		label: 'Global',
		icon: 'globe',
		description: 'System-wide instructions (~/.claude/CLAUDE.md, ~/CLAUDE.md)',
	},
	project: {
		label: 'Project',
		icon: 'folder',
		description: 'Project-specific instructions (./CLAUDE.md)',
	},
};

export function ClaudeMd() {
	useDocumentTitle('CLAUDE.md');
	const [files, setFiles] = useState<ClaudeMdType[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [activeScope, setActiveScope] = useState<ScopeTab>('project');

	const loadFiles = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const response = await configClient.getClaudeMd(create(GetClaudeMdRequestSchema, {}));
			setFiles(response.files);

			// Set initial scope to first available with content
			const hasProject = response.files.some(
				(f) => f.scope === SettingsScope.PROJECT && f.content
			);
			const hasGlobal = response.files.some(
				(f) => f.scope === SettingsScope.GLOBAL && f.content
			);

			if (hasProject) {
				setActiveScope('project');
			} else if (hasGlobal) {
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

	// Get files for the current scope
	const getFilesForScope = (scope: ScopeTab): ClaudeMdType[] => {
		const targetScope =
			scope === 'project' ? SettingsScope.PROJECT : SettingsScope.GLOBAL;
		return files.filter((f) => f.scope === targetScope);
	};

	// Get the primary file for a scope (first one with content)
	const getPrimaryFile = (scope: ScopeTab): ClaudeMdType | null => {
		const scopeFiles = getFilesForScope(scope);
		return scopeFiles.find((f) => f.content) ?? scopeFiles[0] ?? null;
	};

	// Get additional files for a scope (all except the primary)
	const getAdditionalFiles = (scope: ScopeTab): ClaudeMdType[] => {
		const scopeFiles = getFilesForScope(scope);
		const primary = getPrimaryFile(scope);
		if (!primary) return [];
		return scopeFiles.filter((f) => f.path !== primary.path && f.content);
	};

	const hasContent = (scope: ScopeTab): boolean => {
		return getFilesForScope(scope).some((f) => f.content);
	};

	if (loading) {
		return (
			<div className="page environment-claudemd-page">
				<div className="env-loading">Loading CLAUDE.md files...</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className="page environment-claudemd-page">
				<div className="env-error">
					<p>{error}</p>
					<Button variant="secondary" onClick={loadFiles}>
						Retry
					</Button>
				</div>
			</div>
		);
	}

	const activeContent = getPrimaryFile(activeScope);
	const additionalFiles = getAdditionalFiles(activeScope);
	const scopeInfo = SCOPE_INFO[activeScope];

	return (
		<div className="page environment-claudemd-page">
			<div className="env-page-header">
				<div>
					<h3>CLAUDE.md</h3>
					<p className="env-page-description">
						Project instructions that customize Claude Code behavior. Files are loaded in
						order: global, then project.
					</p>
				</div>
			</div>

			<Tabs.Root value={activeScope} onValueChange={(v) => setActiveScope(v as ScopeTab)}>
				<Tabs.List className="env-scope-tabs">
					{(['global', 'project'] as ScopeTab[]).map((scope) => {
						const info = SCOPE_INFO[scope];
						const has = hasContent(scope);
						return (
							<Tabs.Trigger
								key={scope}
								value={scope}
								className={`env-scope-tab ${!has ? 'empty' : ''}`}
							>
								<Icon name={info.icon} size={14} />
								{info.label}
								{has && <span className="claudemd-tab-indicator" />}
							</Tabs.Trigger>
						);
					})}
				</Tabs.List>

				<Tabs.Content value={activeScope}>
					<div className="claudemd-content">
						<div className="claudemd-header">
							<div className="claudemd-scope-info">
								<Icon name={scopeInfo.icon} size={16} />
								<span>{scopeInfo.description}</span>
							</div>
							{activeContent?.path && (
								<code className="claudemd-path">{activeContent.path}</code>
							)}
						</div>

						{activeContent?.content ? (
							<div className="claudemd-preview">
								<pre className="claudemd-preview-content">{activeContent.content}</pre>
							</div>
						) : (
							<div className="claudemd-empty">
								<Icon name="file-text" size={48} />
								<p>No {scopeInfo.label.toLowerCase()} CLAUDE.md file found</p>
								<p className="claudemd-empty-hint">
									Create a file at the expected path to add instructions.
								</p>
							</div>
						)}
					</div>
				</Tabs.Content>
			</Tabs.Root>

			{additionalFiles.length > 0 && (
				<div className="claudemd-local-section">
					<h4>Additional CLAUDE.md Files</h4>
					<p className="claudemd-local-description">
						Other CLAUDE.md files in this scope.
					</p>
					<div className="claudemd-local-list">
						{additionalFiles.map((file, i) => (
							<div key={i} className="claudemd-local-item">
								<Icon name="file-text" size={14} />
								<code>{file.path}</code>
							</div>
						))}
					</div>
				</div>
			)}
		</div>
	);
}
