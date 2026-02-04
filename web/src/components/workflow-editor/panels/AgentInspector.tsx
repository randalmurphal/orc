import type { Agent } from '@/gen/orc/v1/config_pb';
import './AgentInspector.css';

interface AgentInspectorProps {
	agent: Agent | null;
	onClose: () => void;
}

/**
 * AgentInspector - Panel for displaying agent details in the workflow editor.
 *
 * Shows agent configuration including:
 * - Name and description
 * - Model configuration
 * - Prompt (for sub-agents)
 * - Tools and skills
 */
export function AgentInspector({ agent, onClose }: AgentInspectorProps) {
	if (!agent) {
		return (
			<div className="agent-inspector" data-testid="agent-inspector">
				<div className="agent-inspector-empty">
					No agent selected
				</div>
			</div>
		);
	}

	return (
		<div className="agent-inspector" data-testid="agent-inspector">
			{/* Header */}
			<div className="agent-inspector-header">
				<div className="agent-inspector-title">
					<span className="agent-inspector-icon">
						{agent.isBuiltin ? '⚙' : '🤖'}
					</span>
					<h3>{agent.name}</h3>
					{agent.isBuiltin && (
						<span className="agent-inspector-badge builtin">Built-in</span>
					)}
				</div>
				<button
					type="button"
					className="agent-inspector-close"
					onClick={onClose}
					aria-label="Close agent inspector"
				>
					×
				</button>
			</div>

			{/* Description */}
			{agent.description && (
				<div className="agent-inspector-section">
					<p className="agent-inspector-description">{agent.description}</p>
				</div>
			)}

			{/* Configuration Details */}
			<div className="agent-inspector-section">
				<h4 className="agent-inspector-section-title">Configuration</h4>
				<div className="agent-inspector-fields">
					{/* Model */}
					<div className="agent-inspector-field">
						<span className="agent-inspector-label">Model</span>
						<span className="agent-inspector-value">
							{agent.model || 'Default (inherited)'}
						</span>
					</div>

					{/* Timeout */}
					{agent.timeout && (
						<div className="agent-inspector-field">
							<span className="agent-inspector-label">Timeout</span>
							<span className="agent-inspector-value">{agent.timeout}</span>
						</div>
					)}

					{/* Scope */}
					<div className="agent-inspector-field">
						<span className="agent-inspector-label">Scope</span>
						<span className="agent-inspector-value">
							{formatScope(agent.scope)}
						</span>
					</div>
				</div>
			</div>

			{/* Prompt (for sub-agents) */}
			{agent.prompt && (
				<div className="agent-inspector-section">
					<h4 className="agent-inspector-section-title">Prompt</h4>
					<div className="agent-inspector-prompt">
						<pre>{agent.prompt}</pre>
					</div>
				</div>
			)}

			{/* System Prompt (for executors) */}
			{agent.systemPrompt && (
				<div className="agent-inspector-section">
					<h4 className="agent-inspector-section-title">System Prompt</h4>
					<div className="agent-inspector-prompt">
						<pre>{agent.systemPrompt}</pre>
					</div>
				</div>
			)}

			{/* Tools */}
			{agent.tools && (
				<div className="agent-inspector-section">
					<h4 className="agent-inspector-section-title">Tools</h4>
					<div className="agent-inspector-tools">
						{formatTools(agent.tools)}
					</div>
				</div>
			)}

			{/* Skills */}
			{agent.skillRefs && agent.skillRefs.length > 0 && (
				<div className="agent-inspector-section">
					<h4 className="agent-inspector-section-title">Skills</h4>
					<div className="agent-inspector-tags">
						{agent.skillRefs.map((skill) => (
							<span key={skill} className="agent-inspector-tag">
								{skill}
							</span>
						))}
					</div>
				</div>
			)}

			{/* Usage hint */}
			<div className="agent-inspector-hint">
				<p>Click an agent while a phase is selected to assign it as a sub-agent.</p>
			</div>
		</div>
	);
}

function formatScope(scope: number): string {
	switch (scope) {
		case 0:
			return 'Unknown';
		case 1:
			return 'Global';
		case 2:
			return 'Project';
		case 3:
			return 'Local';
		default:
			return `Scope ${scope}`;
	}
}

function formatTools(tools: { allow: string[]; deny: string[] }): React.ReactNode {
	const parts: string[] = [];

	if (tools.allow && tools.allow.length > 0) {
		if (tools.allow.includes('*')) {
			parts.push('All tools allowed');
		} else {
			parts.push(`Allowed: ${tools.allow.join(', ')}`);
		}
	}

	if (tools.deny && tools.deny.length > 0) {
		parts.push(`Denied: ${tools.deny.join(', ')}`);
	}

	if (parts.length === 0) {
		return <span className="agent-inspector-muted">Default tools</span>;
	}

	return parts.map((part, i) => <div key={i}>{part}</div>);
}
