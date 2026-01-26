/**
 * Scripts page (/environment/scripts)
 * Displays project script registry
 */

import { useState, useEffect, useCallback } from 'react';
import { create } from '@bufbuild/protobuf';
import { Button } from '@/components/ui/Button';
import { Icon, type IconName } from '@/components/ui/Icon';
import { toast } from '@/stores';
import { useDocumentTitle } from '@/hooks';
import { configClient } from '@/lib/client';
import {
	type Script,
	ListScriptsRequestSchema,
	DiscoverScriptsRequestSchema,
} from '@/gen/orc/v1/config_pb';
import './environment.css';

export function Scripts() {
	useDocumentTitle('Scripts');
	const [scripts, setScripts] = useState<Script[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [discovering, setDiscovering] = useState(false);

	const loadScripts = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const response = await configClient.listScripts(create(ListScriptsRequestSchema, {}));
			setScripts(response.scripts);
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
			const response = await configClient.discoverScripts(create(DiscoverScriptsRequestSchema, {}));
			toast.success(`Discovered ${response.scripts.length} script${response.scripts.length !== 1 ? 's' : ''}`);
			await loadScripts();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to discover scripts');
		} finally {
			setDiscovering(false);
		}
	};

	const getLanguageIcon = (script: Script): IconName => {
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

	const getLanguageLabel = (script: Script): string => {
		// Use the language field if available, otherwise derive from extension
		if (script.language) {
			return script.language;
		}
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
