/**
 * Agents page (/environment/agents)
 * Displays available sub-agent definitions
 */

import { useState, useEffect, useCallback } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import { Modal } from '@/components/overlays/Modal';
import { useDocumentTitle } from '@/hooks';
import { listAgents, getAgent, type SubAgent } from '@/lib/api';
import './environment.css';

type Scope = 'project' | 'global';

export function Agents() {
	useDocumentTitle('Agents');
	const [scope, setScope] = useState<Scope>('project');
	const [agents, setAgents] = useState<SubAgent[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Preview modal state
	const [previewingAgent, setPreviewingAgent] = useState<string | null>(null);
	const [previewContent, setPreviewContent] = useState<SubAgent | null>(null);
	const [previewLoading, setPreviewLoading] = useState(false);

	const loadAgents = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const data = await listAgents(scope === 'global' ? 'global' : undefined);
			setAgents(data);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load agents');
		} finally {
			setLoading(false);
		}
	}, [scope]);

	useEffect(() => {
		loadAgents();
	}, [loadAgents]);

	const handlePreview = async (agentName: string) => {
		setPreviewingAgent(agentName);
		setPreviewLoading(true);
		try {
			const agent = await getAgent(agentName);
			setPreviewContent(agent);
		} catch (_err) {
			setPreviewingAgent(null);
		} finally {
			setPreviewLoading(false);
		}
	};

	if (loading) {
		return (
			<div className="page environment-agents-page">
				<div className="env-loading">Loading agents...</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className="page environment-agents-page">
				<div className="env-error">
					<p>{error}</p>
					<Button variant="secondary" onClick={loadAgents}>
						Retry
					</Button>
				</div>
			</div>
		);
	}

	return (
		<div className="page environment-agents-page">
			<div className="env-page-header">
				<div>
					<h3>Agents</h3>
					<p className="env-page-description">
						Sub-agent definitions for specialized Claude Code tasks.
					</p>
				</div>
			</div>

			<Tabs.Root value={scope} onValueChange={(v) => setScope(v as Scope)}>
				<Tabs.List className="env-scope-tabs">
					<Tabs.Trigger value="project" className="env-scope-tab">
						<Icon name="folder" size={14} />
						Project
					</Tabs.Trigger>
					<Tabs.Trigger value="global" className="env-scope-tab">
						<Icon name="user" size={14} />
						Global
					</Tabs.Trigger>
				</Tabs.List>

				<Tabs.Content value={scope}>
					{agents.length === 0 ? (
						<div className="env-empty">
							<Icon name="agents" size={48} />
							<p>No agents found in {scope} scope</p>
							<p className="env-empty-hint">
								Agents are defined in settings.json or discovered from .md files.
							</p>
						</div>
					) : (
						<div className="env-card-grid">
							{agents.map((agent) => (
								<div
									key={agent.name}
									className="env-card agent-card"
									onClick={() => handlePreview(agent.name)}
								>
									<div className="env-card-header">
										<h4 className="env-card-title">
											<Icon name="user" size={16} />
											{agent.name}
										</h4>
									</div>
									<p className="env-card-description">{agent.description}</p>
									<div className="agent-card-meta">
										{agent.model && (
											<span className="agent-card-model">{agent.model}</span>
										)}
										{agent.path && (
											<code className="agent-card-path">{agent.path}</code>
										)}
									</div>
								</div>
							))}
						</div>
					)}
				</Tabs.Content>
			</Tabs.Root>

			{/* Preview Modal */}
			<Modal
				open={previewingAgent !== null}
				onClose={() => setPreviewingAgent(null)}
				title={
					<div className="agent-preview-title">
						<Icon name="user" size={20} />
						<span>{previewingAgent}</span>
					</div>
				}
				size="lg"
			>
				{previewLoading ? (
					<div className="env-loading">Loading agent...</div>
				) : previewContent ? (
					<div className="agent-preview">
						<div className="agent-preview-description">
							{previewContent.description}
						</div>

						{previewContent.model && (
							<div className="agent-preview-field">
								<span className="agent-preview-label">Model:</span>
								<code>{previewContent.model}</code>
							</div>
						)}

						{previewContent.timeout && (
							<div className="agent-preview-field">
								<span className="agent-preview-label">Timeout:</span>
								<code>{previewContent.timeout}</code>
							</div>
						)}

						{previewContent.work_dir && (
							<div className="agent-preview-field">
								<span className="agent-preview-label">Working Directory:</span>
								<code>{previewContent.work_dir}</code>
							</div>
						)}

						{previewContent.skill_refs && previewContent.skill_refs.length > 0 && (
							<div className="agent-preview-field">
								<span className="agent-preview-label">Skills:</span>
								<div className="agent-preview-skills">
									{previewContent.skill_refs.map((skill) => (
										<code key={skill} className="agent-preview-skill">
											{skill}
										</code>
									))}
								</div>
							</div>
						)}

						{previewContent.tools && (
							<div className="agent-preview-field">
								<span className="agent-preview-label">Tools:</span>
								<code>
									{typeof previewContent.tools === 'string'
										? previewContent.tools
										: JSON.stringify(previewContent.tools)}
								</code>
							</div>
						)}

						{previewContent.prompt && (
							<div className="agent-preview-section">
								<h5>Prompt</h5>
								<pre className="agent-preview-content">{previewContent.prompt}</pre>
							</div>
						)}
					</div>
				) : null}
			</Modal>
		</div>
	);
}
