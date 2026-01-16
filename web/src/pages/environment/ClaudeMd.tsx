/**
 * CLAUDE.md page (/environment/claudemd)
 * Displays CLAUDE.md files from different scopes
 */

import { useState, useEffect, useCallback } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import { getClaudeMDHierarchy, type ClaudeMDHierarchy, type ClaudeMD } from '@/lib/api';
import './environment.css';

type Scope = 'global' | 'user' | 'project';

import type { IconName } from '@/components/ui/Icon';

const SCOPE_INFO: Record<Scope, { label: string; icon: IconName; description: string }> = {
	global: {
		label: 'Global',
		icon: 'globe',
		description: 'System-wide instructions (~/.claude/CLAUDE.md)',
	},
	user: {
		label: 'User',
		icon: 'user',
		description: 'Personal instructions (~/CLAUDE.md)',
	},
	project: {
		label: 'Project',
		icon: 'folder',
		description: 'Project-specific instructions (./CLAUDE.md)',
	},
};

export function ClaudeMd() {
	const [hierarchy, setHierarchy] = useState<ClaudeMDHierarchy | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [activeScope, setActiveScope] = useState<Scope>('project');

	const loadHierarchy = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const data = await getClaudeMDHierarchy();
			setHierarchy(data);

			// Set initial scope to first available
			if (data.project) {
				setActiveScope('project');
			} else if (data.user) {
				setActiveScope('user');
			} else if (data.global) {
				setActiveScope('global');
			}
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load CLAUDE.md files');
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		loadHierarchy();
	}, [loadHierarchy]);

	const getActiveContent = (): ClaudeMD | null => {
		if (!hierarchy) return null;
		switch (activeScope) {
			case 'global':
				return hierarchy.global || null;
			case 'user':
				return hierarchy.user || null;
			case 'project':
				return hierarchy.project || null;
		}
	};

	const hasContent = (scope: Scope): boolean => {
		if (!hierarchy) return false;
		switch (scope) {
			case 'global':
				return !!hierarchy.global?.content;
			case 'user':
				return !!hierarchy.user?.content;
			case 'project':
				return !!hierarchy.project?.content;
		}
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
					<Button variant="secondary" onClick={loadHierarchy}>
						Retry
					</Button>
				</div>
			</div>
		);
	}

	const activeContent = getActiveContent();
	const scopeInfo = SCOPE_INFO[activeScope];

	return (
		<div className="page environment-claudemd-page">
			<div className="env-page-header">
				<div>
					<h3>CLAUDE.md</h3>
					<p className="env-page-description">
						Project instructions that customize Claude Code behavior. Files are loaded in
						order: global, user, then project.
					</p>
				</div>
			</div>

			<Tabs.Root value={activeScope} onValueChange={(v) => setActiveScope(v as Scope)}>
				<Tabs.List className="env-scope-tabs">
					{(['global', 'user', 'project'] as Scope[]).map((scope) => {
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

			{hierarchy?.local && hierarchy.local.length > 0 && (
				<div className="claudemd-local-section">
					<h4>Local CLAUDE.md Files</h4>
					<p className="claudemd-local-description">
						Additional CLAUDE.md files found in subdirectories.
					</p>
					<div className="claudemd-local-list">
						{hierarchy.local.map((file, i) => (
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
