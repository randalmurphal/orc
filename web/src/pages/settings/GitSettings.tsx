/**
 * Git Settings page (/settings/git)
 * Shows project-level git defaults for branch naming, target branch, and PR settings.
 */

import { useDocumentTitle } from '@/hooks';
import { Icon } from '@/components/ui/Icon';
import './GitSettings.css';

export function GitSettingsPage() {
	useDocumentTitle('Git Settings');

	return (
		<div className="page git-settings-page">
			<div className="git-settings-header">
				<div>
					<h3>Git Settings</h3>
					<p className="git-settings-description">
						Project-level defaults for branch naming, target branch, and PR creation.
						These can be overridden per-task.
					</p>
				</div>
			</div>

			<div className="git-settings-section">
				<h4>Branch Naming</h4>
				<p className="git-settings-info">
					By default, task branches are named using the task ID (e.g., <code>TASK-001</code>).
					If a task is linked to an initiative, the initiative's branch prefix is prepended.
				</p>
				<div className="git-settings-example">
					<div className="git-settings-example-item">
						<span className="git-settings-example-label">Default:</span>
						<code>TASK-001</code>
					</div>
					<div className="git-settings-example-item">
						<span className="git-settings-example-label">With Initiative:</span>
						<code>feature/auth/TASK-001</code>
					</div>
					<div className="git-settings-example-item">
						<span className="git-settings-example-label">Custom Override:</span>
						<code>my-custom-branch</code>
					</div>
				</div>
			</div>

			<div className="git-settings-section">
				<h4>Target Branch</h4>
				<p className="git-settings-info">
					The branch that PRs will target. Configurable in <code>.orc/config.yaml</code>:
				</p>
				<pre className="git-settings-code">
{`completion:
  target_branch: main  # Default target for PRs`}
				</pre>
			</div>

			<div className="git-settings-section">
				<h4>PR Settings</h4>
				<p className="git-settings-info">
					Default PR options when tasks complete. Configurable in <code>.orc/config.yaml</code>:
				</p>
				<pre className="git-settings-code">
{`completion:
  pr:
    draft: false           # Create PRs as drafts
    labels: []             # Labels to apply
    reviewers: []          # Reviewers to request
    team_reviewers: []     # Team reviewers
    maintainer_can_modify: true`}
				</pre>
			</div>

			<div className="git-settings-section">
				<h4>Task-Level Overrides</h4>
				<p className="git-settings-info">
					Individual tasks can override these defaults via CLI flags or the task edit modal:
				</p>
				<ul className="git-settings-overrides">
					<li>
						<code>--branch</code> - Custom branch name
					</li>
					<li>
						<code>--target-branch</code> - Override target branch
					</li>
					<li>
						<code>--pr-draft</code> - Create PR as draft
					</li>
					<li>
						<code>--pr-labels</code> - PR labels (comma-separated)
					</li>
					<li>
						<code>--pr-reviewers</code> - PR reviewers (comma-separated)
					</li>
				</ul>
			</div>

			<div className="git-settings-info-box">
				<Icon name="info" size={16} />
				<p>
					Edit <code>.orc/config.yaml</code> directly to change project-level defaults,
					or use the task edit modal to override settings for individual tasks.
				</p>
			</div>
		</div>
	);
}
