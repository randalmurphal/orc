/**
 * Scripts page (/environment/scripts)
 * Displays project script registry
 */

import { useState, useEffect, useCallback } from 'react';
import { Button } from '@/components/ui/Button';
import { Icon, type IconName } from '@/components/ui/Icon';
import { toast } from '@/stores';
import { useDocumentTitle } from '@/hooks';
import { listScripts, discoverScripts, type ProjectScript } from '@/lib/api';
import './environment.css';

export function Scripts() {
	useDocumentTitle('Scripts');
	const [scripts, setScripts] = useState<ProjectScript[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [discovering, setDiscovering] = useState(false);

	const loadScripts = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const data = await listScripts();
			setScripts(data);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load scripts');
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		loadScripts();
	}, [loadScripts]);

	const handleDiscover = async () => {
		try {
			setDiscovering(true);
			const discovered = await discoverScripts();
			toast.success(`Discovered ${discovered.length} script${discovered.length !== 1 ? 's' : ''}`);
			await loadScripts();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to discover scripts');
		} finally {
			setDiscovering(false);
		}
	};

	const getLanguageIcon = (script: ProjectScript): IconName => {
		const ext = script.path.split('.').pop()?.toLowerCase();
		switch (ext) {
			case 'py':
			case 'js':
			case 'ts':
				return 'code';
			case 'sh':
			case 'bash':
			case 'zsh':
				return 'terminal';
			default:
				return 'file';
		}
	};

	const getLanguageLabel = (script: ProjectScript): string => {
		const ext = script.path.split('.').pop()?.toLowerCase();
		switch (ext) {
			case 'py':
				return 'Python';
			case 'js':
				return 'JavaScript';
			case 'ts':
				return 'TypeScript';
			case 'sh':
			case 'bash':
				return 'Bash';
			case 'zsh':
				return 'Zsh';
			default:
				return ext?.toUpperCase() || 'Unknown';
		}
	};

	if (loading) {
		return (
			<div className="page environment-scripts-page">
				<div className="env-loading">Loading scripts...</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className="page environment-scripts-page">
				<div className="env-error">
					<p>{error}</p>
					<Button variant="secondary" onClick={loadScripts}>
						Retry
					</Button>
				</div>
			</div>
		);
	}

	return (
		<div className="page environment-scripts-page">
			<div className="env-page-header">
				<div>
					<h3>Scripts</h3>
					<p className="env-page-description">
						Project scripts that can be referenced in task specifications.
					</p>
				</div>
				<div className="env-page-header-actions">
					<Button variant="secondary" onClick={handleDiscover} loading={discovering}>
						<Icon name="search" size={14} />
						Discover Scripts
					</Button>
				</div>
			</div>

			{scripts.length === 0 ? (
				<div className="env-empty">
					<Icon name="code" size={48} />
					<p>No scripts registered</p>
					<p className="env-empty-hint">
						Click "Discover Scripts" to scan the project for executable scripts.
					</p>
				</div>
			) : (
				<table className="scripts-table">
					<thead>
						<tr>
							<th>Name</th>
							<th>Path</th>
							<th>Description</th>
							<th>Language</th>
						</tr>
					</thead>
					<tbody>
						{scripts.map((script) => (
							<tr key={script.name}>
								<td className="script-name">
									<Icon name={getLanguageIcon(script)} size={14} />
									{script.name}
								</td>
								<td className="script-path">
									<code>{script.path}</code>
								</td>
								<td className="script-description">{script.description || '—'}</td>
								<td className="script-language">{getLanguageLabel(script)}</td>
							</tr>
						))}
					</tbody>
				</table>
			)}
		</div>
	);
}
