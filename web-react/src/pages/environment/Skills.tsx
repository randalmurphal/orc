import { useState, useEffect, useCallback } from 'react';
import { useSearchParams, Link } from 'react-router-dom';
import {
	listSkills,
	getSkill,
	createSkill,
	updateSkill,
	deleteSkill,
	type SkillInfo,
	type Skill as SkillType,
} from '@/lib/api';
import './Skills.css';

type Scope = 'global' | 'project';

/**
 * Skills page (/environment/skills)
 *
 * Manages Claude Code skills (SKILL.md format) at two levels:
 * - Global (~/.claude/skills/)
 * - Project (.claude/skills/)
 */
export function Skills() {
	const [searchParams, setSearchParams] = useSearchParams();
	const [skills, setSkills] = useState<SkillInfo[]>([]);
	const [selectedSkill, setSelectedSkill] = useState<SkillType | null>(null);
	const [selectedSkillDir, setSelectedSkillDir] = useState<string | null>(null);
	const [isCreating, setIsCreating] = useState(false);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [success, setSuccess] = useState<string | null>(null);

	// Form fields
	const [formName, setFormName] = useState('');
	const [formDescription, setFormDescription] = useState('');
	const [formContent, setFormContent] = useState('');
	const [formAllowedTools, setFormAllowedTools] = useState('');

	const scope = searchParams.get('scope') as Scope | null;
	const isGlobal = scope === 'global';
	const scopeParam = isGlobal ? 'global' : undefined;
	const skillsBasePath = isGlobal ? '~/.claude/skills/' : '.claude/skills/';

	const loadSkills = useCallback(async () => {
		try {
			const skillsList = await listSkills(scopeParam);
			setSkills(skillsList);
			setSelectedSkill(null);
			setSelectedSkillDir(null);
			setIsCreating(false);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load skills');
		}
	}, [scopeParam]);

	useEffect(() => {
		setLoading(true);
		setError(null);
		loadSkills().finally(() => setLoading(false));
	}, [loadSkills]);

	const getSkillDirName = (skillPath: string): string => {
		return skillPath.split('/').pop() || skillPath;
	};

	const selectSkill = async (skill: SkillInfo) => {
		setError(null);
		setSuccess(null);
		setIsCreating(false);

		try {
			const dirName = getSkillDirName(skill.path);
			setSelectedSkillDir(dirName);
			const fullSkill = await getSkill(dirName, scopeParam);
			setSelectedSkill(fullSkill);
			setFormName(fullSkill.name);
			setFormDescription(fullSkill.description);
			setFormContent(fullSkill.content);
			setFormAllowedTools(fullSkill.allowed_tools?.join(', ') || '');
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to load skill');
			setSelectedSkillDir(null);
		}
	};

	const startCreate = () => {
		setError(null);
		setSuccess(null);
		setSelectedSkill(null);
		setSelectedSkillDir(null);
		setIsCreating(true);

		setFormName('');
		setFormDescription('');
		setFormContent('');
		setFormAllowedTools('');
	};

	const handleSave = async () => {
		if (!formName.trim() || !formDescription.trim()) {
			setError('Name and description are required');
			return;
		}

		setSaving(true);
		setError(null);
		setSuccess(null);

		const allowedTools = formAllowedTools
			.split(',')
			.map((t) => t.trim())
			.filter((t) => t);

		const skill: SkillType = {
			name: formName.trim(),
			description: formDescription.trim(),
			content: formContent.trim(),
			allowed_tools: allowedTools.length > 0 ? allowedTools : undefined,
		};

		try {
			if (isCreating) {
				await createSkill(skill, scopeParam);
				setSuccess('Skill created successfully');
				setSelectedSkillDir(formName.trim());
			} else if (selectedSkill && selectedSkillDir) {
				await updateSkill(selectedSkillDir, skill, scopeParam);
				setSuccess('Skill updated successfully');
			}

			await loadSkills();
			setSelectedSkill(skill);
			setIsCreating(false);

			setTimeout(() => setSuccess(null), 3000);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to save skill');
		} finally {
			setSaving(false);
		}
	};

	const handleDelete = async () => {
		if (!selectedSkill || !selectedSkillDir) return;

		if (!confirm(`Delete skill "${selectedSkill.name}"?`)) return;

		setSaving(true);
		setError(null);
		setSuccess(null);

		try {
			await deleteSkill(selectedSkillDir, scopeParam);
			await loadSkills();
			setSelectedSkill(null);
			setSelectedSkillDir(null);
			setIsCreating(false);
			setSuccess('Skill deleted successfully');
			setTimeout(() => setSuccess(null), 3000);
		} catch (e) {
			setError(e instanceof Error ? e.message : 'Failed to delete skill');
		} finally {
			setSaving(false);
		}
	};

	return (
		<div className="skills-page">
			<header className="skills-header">
				<div className="header-content">
					<div>
						<h1>{isGlobal ? 'Global ' : ''}Claude Code Skills</h1>
						<p className="subtitle">Manage skills in {skillsBasePath} (SKILL.md format)</p>
					</div>
					<div className="header-actions">
						<div className="scope-toggle">
							<Link
								to="/environment/skills"
								className={`scope-btn ${!isGlobal ? 'active' : ''}`}
							>
								Project
							</Link>
							<Link
								to="/environment/skills?scope=global"
								className={`scope-btn ${isGlobal ? 'active' : ''}`}
							>
								Global
							</Link>
						</div>
						<button className="btn btn-primary" onClick={startCreate}>
							New Skill
						</button>
					</div>
				</div>
			</header>

			{error && <div className="alert alert-error">{error}</div>}
			{success && <div className="alert alert-success">{success}</div>}

			{loading ? (
				<div className="loading-state">Loading skills...</div>
			) : (
				<div className="skills-layout">
					{/* Skill List */}
					<aside className="skill-list">
						<h2>Skills</h2>
						{skills.length === 0 ? (
							<p className="empty-message">No skills configured</p>
						) : (
							<ul>
								{skills.map((skill) => (
									<li key={skill.path}>
										<button
											className={`skill-item ${selectedSkill?.name === skill.name ? 'selected' : ''}`}
											onClick={() => selectSkill(skill)}
										>
											<span className="skill-name">{skill.name}</span>
											{skill.description && (
												<span className="skill-desc">{skill.description}</span>
											)}
										</button>
									</li>
								))}
							</ul>
						)}
					</aside>

					{/* Editor Panel */}
					<div className="editor-panel">
						{selectedSkill || isCreating ? (
							<>
								<div className="editor-header">
									<h2>{isCreating ? 'New Skill' : selectedSkill?.name}</h2>
									{selectedSkill && !isCreating && (
										<button
											className="btn btn-danger"
											onClick={handleDelete}
											disabled={saving}
										>
											Delete
										</button>
									)}
								</div>

								<form
									className="skill-form"
									onSubmit={(e) => {
										e.preventDefault();
										handleSave();
									}}
								>
									<div className="form-row">
										<div className="form-group">
											<label htmlFor="name">Name</label>
											<input
												id="name"
												type="text"
												value={formName}
												onChange={(e) => setFormName(e.target.value)}
												placeholder="my-skill"
											/>
											<span className="form-hint">
												{skillsBasePath}
												{formName || 'name'}/SKILL.md
											</span>
										</div>

										<div className="form-group">
											<label htmlFor="allowed-tools">Allowed Tools (optional)</label>
											<input
												id="allowed-tools"
												type="text"
												value={formAllowedTools}
												onChange={(e) => setFormAllowedTools(e.target.value)}
												placeholder="Read, Bash, Edit"
											/>
											<span className="form-hint">Comma-separated list of tools</span>
										</div>
									</div>

									<div className="form-group">
										<label htmlFor="description">Description</label>
										<input
											id="description"
											type="text"
											value={formDescription}
											onChange={(e) => setFormDescription(e.target.value)}
											placeholder="Brief description of what this skill does"
										/>
									</div>

									<div className="form-group form-group-grow">
										<label htmlFor="content">Content (Markdown)</label>
										<textarea
											id="content"
											value={formContent}
											onChange={(e) => setFormContent(e.target.value)}
											placeholder="Enter the skill instructions..."
											rows={15}
										/>
									</div>

									<div className="form-actions">
										<button type="submit" className="btn btn-primary" disabled={saving}>
											{saving ? 'Saving...' : isCreating ? 'Create' : 'Update'}
										</button>
									</div>
								</form>
							</>
						) : (
							<div className="no-selection">
								<p>Select a skill from the list or create a new one.</p>
								<p className="hint">
									Skills are reusable prompts that can be invoked with <code>/skill-name</code>
								</p>
							</div>
						)}
					</div>
				</div>
			)}
		</div>
	);
}
