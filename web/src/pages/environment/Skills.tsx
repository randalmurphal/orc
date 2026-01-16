/**
 * Skills page (/environment/skills)
 * Displays and previews SKILL.md files from .claude/skills/
 */

import { useState, useEffect, useCallback } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import { Modal } from '@/components/overlays/Modal';
import { toast } from '@/stores';
import { useDocumentTitle } from '@/hooks';
import { listSkills, getSkill, type SkillInfo, type Skill } from '@/lib/api';
import './environment.css';

type Scope = 'project' | 'global';

export function Skills() {
	useDocumentTitle('Skills');
	const [scope, setScope] = useState<Scope>('project');
	const [skills, setSkills] = useState<SkillInfo[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Preview modal state
	const [previewingSkill, setPreviewingSkill] = useState<string | null>(null);
	const [previewContent, setPreviewContent] = useState<Skill | null>(null);
	const [previewLoading, setPreviewLoading] = useState(false);

	const loadSkills = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const data = await listSkills(scope);
			setSkills(data);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load skills');
		} finally {
			setLoading(false);
		}
	}, [scope]);

	useEffect(() => {
		loadSkills();
	}, [loadSkills]);

	const handlePreview = async (skillName: string) => {
		setPreviewingSkill(skillName);
		setPreviewLoading(true);
		try {
			const skill = await getSkill(skillName, scope);
			setPreviewContent(skill);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to load skill');
			setPreviewingSkill(null);
		} finally {
			setPreviewLoading(false);
		}
	};

	if (loading) {
		return (
			<div className="page environment-skills-page">
				<div className="env-loading">Loading skills...</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className="page environment-skills-page">
				<div className="env-error">
					<p>{error}</p>
					<Button variant="secondary" onClick={loadSkills}>
						Retry
					</Button>
				</div>
			</div>
		);
	}

	return (
		<div className="page environment-skills-page">
			<div className="env-page-header">
				<div>
					<h3>Skills</h3>
					<p className="env-page-description">
						Browse SKILL.md files that provide specialized capabilities to Claude Code.
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
						<Icon name="globe" size={14} />
						Global
					</Tabs.Trigger>
				</Tabs.List>

				<Tabs.Content value={scope}>
					{skills.length === 0 ? (
						<div className="env-empty">
							<Icon name="book" size={48} />
							<p>No skills found in {scope} scope</p>
							<p className="env-empty-hint">
								Skills are defined in{' '}
								<code>{scope === 'global' ? '~/.claude/skills/' : '.claude/skills/'}</code>
							</p>
						</div>
					) : (
						<div className="env-card-grid">
							{skills.map((skill) => (
								<div
									key={skill.name}
									className="env-card skill-card"
									onClick={() => handlePreview(skill.name)}
								>
									<div className="env-card-header">
										<h4 className="env-card-title">
											<Icon name="book" size={16} />
											{skill.name}
										</h4>
									</div>
									<p className="env-card-description">{skill.description}</p>
									<div className="skill-card-meta">
										<code className="skill-card-path">{skill.path}</code>
									</div>
								</div>
							))}
						</div>
					)}
				</Tabs.Content>
			</Tabs.Root>

			{/* Preview Modal */}
			<Modal
				open={previewingSkill !== null}
				onClose={() => setPreviewingSkill(null)}
				title={
					<div className="skill-preview-title">
						<Icon name="book" size={20} />
						<span>{previewingSkill}</span>
					</div>
				}
				size="lg"
			>
				{previewLoading ? (
					<div className="env-loading">Loading skill...</div>
				) : previewContent ? (
					<div className="skill-preview">
						<div className="skill-preview-meta">
							<div className="skill-preview-description">
								{previewContent.description}
							</div>
							{previewContent.version && (
								<span className="skill-preview-badge">v{previewContent.version}</span>
							)}
						</div>

						{previewContent.allowed_tools && previewContent.allowed_tools.length > 0 && (
							<div className="skill-preview-tools">
								<span className="skill-preview-label">Allowed Tools:</span>
								<div className="skill-preview-tools-list">
									{previewContent.allowed_tools.map((tool) => (
										<code key={tool} className="skill-preview-tool">
											{tool}
										</code>
									))}
								</div>
							</div>
						)}

						<div className="skill-preview-flags">
							{previewContent.has_references && (
								<span className="skill-preview-flag">
									<Icon name="link" size={12} />
									References
								</span>
							)}
							{previewContent.has_scripts && (
								<span className="skill-preview-flag">
									<Icon name="terminal" size={12} />
									Scripts
								</span>
							)}
							{previewContent.has_assets && (
								<span className="skill-preview-flag">
									<Icon name="image" size={12} />
									Assets
								</span>
							)}
						</div>

						<div className="skill-preview-section">
							<h5>Content</h5>
							<pre className="skill-preview-content">{previewContent.content}</pre>
						</div>
					</div>
				) : null}
			</Modal>
		</div>
	);
}
