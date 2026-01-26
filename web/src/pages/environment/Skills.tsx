/**
 * Skills page (/environment/skills)
 * Displays and previews SKILL.md files from .claude/skills/
 */

import { useState, useEffect, useCallback } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { create } from '@bufbuild/protobuf';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import { Modal } from '@/components/overlays/Modal';
import { useDocumentTitle } from '@/hooks';
import { configClient } from '@/lib/client';
import {
	type Skill,
	SettingsScope,
	ListSkillsRequestSchema,
} from '@/gen/orc/v1/config_pb';
import './environment.css';

type ScopeTab = 'project' | 'global';

// Convert UI scope tab to protobuf SettingsScope enum
function toSettingsScope(scope: ScopeTab): SettingsScope {
	return scope === 'global' ? SettingsScope.GLOBAL : SettingsScope.PROJECT;
}

// Convert protobuf SettingsScope enum to display string
function scopeToString(scope: SettingsScope): string {
	switch (scope) {
		case SettingsScope.GLOBAL:
			return 'global';
		case SettingsScope.PROJECT:
			return 'project';
		default:
			return 'unknown';
	}
}

export function Skills() {
	useDocumentTitle('Skills');
	const [scope, setScope] = useState<ScopeTab>('project');
	const [skills, setSkills] = useState<Skill[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);

	// Preview modal state
	const [previewingSkill, setPreviewingSkill] = useState<string | null>(null);
	const [previewContent, setPreviewContent] = useState<Skill | null>(null);

	const loadSkills = useCallback(async () => {
		try {
			setLoading(true);
			setError(null);
			const response = await configClient.listSkills(
				create(ListSkillsRequestSchema, { scope: toSettingsScope(scope) })
			);
			setSkills(response.skills);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Failed to load skills');
		} finally {
			setLoading(false);
		}
	}, [scope]);

	useEffect(() => {
		loadSkills();
	}, [loadSkills]);

	const handlePreview = (skillName: string) => {
		// Find the skill in the loaded list (protobuf includes full content)
		const skill = skills.find((s) => s.name === skillName);
		if (skill) {
			setPreviewingSkill(skillName);
			setPreviewContent(skill);
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

			<Tabs.Root value={scope} onValueChange={(v) => setScope(v as ScopeTab)}>
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
										<span className="skill-card-scope">{scopeToString(skill.scope)}</span>
										{skill.userInvocable && (
											<span className="skill-card-badge">User Invocable</span>
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
				{previewContent ? (
					<div className="skill-preview">
						<div className="skill-preview-meta">
							<div className="skill-preview-description">
								{previewContent.description}
							</div>
						</div>

						<div className="skill-preview-flags">
							{previewContent.userInvocable && (
								<span className="skill-preview-flag">
									<Icon name="user" size={12} />
									User Invocable
								</span>
							)}
							{previewContent.inputSchema && (
								<span className="skill-preview-flag">
									<Icon name="code" size={12} />
									Has Input Schema
								</span>
							)}
						</div>

						{previewContent.inputSchema && (
							<div className="skill-preview-section">
								<h5>Input Schema</h5>
								<pre className="skill-preview-content">{previewContent.inputSchema}</pre>
							</div>
						)}

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
