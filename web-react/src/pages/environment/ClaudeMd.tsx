import { useState, useEffect, useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';
import {
	getClaudeMD,
	updateClaudeMD,
	getClaudeMDHierarchy,
	type ClaudeMDHierarchy,
} from '@/lib/api';
import './ClaudeMd.css';

type Scope = 'global' | 'user' | 'project';

/**
 * CLAUDE.md editor page (/environment/claudemd)
 *
 * Manages CLAUDE.md files at three levels:
 * - Global (~/.claude/CLAUDE.md)
 * - User (~/CLAUDE.md)
 * - Project (./CLAUDE.md)
 */
export function ClaudeMd() {
	const [searchParams, setSearchParams] = useSearchParams();
	const [content, setContent] = useState('');
	const [hierarchy, setHierarchy] = useState<ClaudeMDHierarchy | null>(null);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [success, setSuccess] = useState<string | null>(null);

	const selectedSource = (searchParams.get('scope') as Scope) || 'project';

	const loadContent = useCallback(async (scope: Scope = 'project') => {
		setLoading(true);
		setError(null);

		try {
			const hierarchyData = await getClaudeMDHierarchy();
			setHierarchy(hierarchyData);

			const claudeMD = await getClaudeMD(scope);
			setContent(claudeMD.content || '');
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load CLAUDE.md');
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		loadContent(selectedSource);
	}, [selectedSource, loadContent]);

	const selectSource = (source: Scope) => {
		if (source === 'project') {
			setSearchParams({});
		} else {
			setSearchParams({ scope: source });
		}
	};

	const handleSave = async () => {
		setSaving(true);
		setError(null);
		setSuccess(null);

		try {
			await updateClaudeMD(content, selectedSource);
			setSuccess('CLAUDE.md saved successfully');

			// Refresh hierarchy
			const hierarchyData = await getClaudeMDHierarchy();
			setHierarchy(hierarchyData);

			// Clear success message after 3 seconds
			setTimeout(() => setSuccess(null), 3000);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to save CLAUDE.md');
		} finally {
			setSaving(false);
		}
	};

	const getSourceLabel = (source: string): string => {
		switch (source) {
			case 'global':
				return 'Global (~/.claude/CLAUDE.md)';
			case 'user':
				return 'User (~/CLAUDE.md)';
			case 'project':
				return 'Project (./CLAUDE.md)';
			default:
				return source;
		}
	};

	const hasContent = (source: string): boolean => {
		if (!hierarchy) return false;
		switch (source) {
			case 'global':
				return !!hierarchy.global?.content;
			case 'user':
				return !!hierarchy.user?.content;
			case 'project':
				return !!hierarchy.project?.content;
			default:
				return false;
		}
	};

	const pageTitle =
		selectedSource === 'project'
			? 'CLAUDE.md'
			: `${selectedSource.charAt(0).toUpperCase() + selectedSource.slice(1)} CLAUDE.md`;

	return (
		<div className="claudemd-page">
			<header className="claudemd-header">
				<div className="header-content">
					<div>
						<h1>{pageTitle}</h1>
						<p className="subtitle">
							{selectedSource === 'global' && 'Global instructions at ~/.claude/CLAUDE.md'}
							{selectedSource === 'user' && 'User instructions at ~/CLAUDE.md'}
							{selectedSource === 'project' && 'Project instructions for Claude'}
						</p>
					</div>
					<button className="btn btn-primary" onClick={handleSave} disabled={saving}>
						{saving ? 'Saving...' : 'Save'}
					</button>
				</div>
			</header>

			{error && <div className="alert alert-error">{error}</div>}
			{success && <div className="alert alert-success">{success}</div>}

			{loading ? (
				<div className="loading-state">Loading CLAUDE.md...</div>
			) : (
				<div className="claudemd-layout">
					{/* Source Selector */}
					<aside className="source-list">
						<h2>Sources</h2>
						<p className="help-text">CLAUDE.md files are applied in order: global, user, project</p>
						<ul>
							<li>
								<button
									className={`source-item ${selectedSource === 'global' ? 'selected' : ''}`}
									onClick={() => selectSource('global')}
								>
									<span className="source-name">Global</span>
									<span className="source-path">~/.claude/CLAUDE.md</span>
									{!hasContent('global') && <span className="badge badge-new">New</span>}
								</button>
							</li>
							<li>
								<button
									className={`source-item ${selectedSource === 'user' ? 'selected' : ''}`}
									onClick={() => selectSource('user')}
								>
									<span className="source-name">User</span>
									<span className="source-path">~/CLAUDE.md</span>
									{!hasContent('user') && <span className="badge badge-new">New</span>}
								</button>
							</li>
							<li>
								<button
									className={`source-item ${selectedSource === 'project' ? 'selected' : ''}`}
									onClick={() => selectSource('project')}
								>
									<span className="source-name">Project</span>
									<span className="source-path">./CLAUDE.md</span>
									{!hasContent('project') && <span className="badge badge-new">New</span>}
								</button>
							</li>
						</ul>
					</aside>

					{/* Editor Panel */}
					<div className="editor-panel">
						<div className="editor-header">
							<h2>{getSourceLabel(selectedSource)}</h2>
						</div>

						<div className="editor-content">
							<textarea
								value={content}
								onChange={(e) => setContent(e.target.value)}
								placeholder="# Instructions&#10;&#10;Add instructions for Claude here..."
								rows={30}
							/>
						</div>
					</div>
				</div>
			)}
		</div>
	);
}
