/**
 * AddAgentModal - Modal for creating new custom agents.
 *
 * Features:
 * - Name input (required, used as ID)
 * - Description input (required)
 * - Model selector (optional)
 * - System prompt for executor role (optional)
 * - Sub-agent prompt for delegation role (optional)
 * - Claude config JSON (optional)
 * - Tools multi-select (optional)
 * - Creates agent via configClient.createAgent()
 *
 * Agents are a unified concept that can serve two roles:
 * 1. EXECUTOR: The main agent that runs a phase (uses systemPrompt)
 * 2. SUB-AGENT: Delegated to by the executor (uses prompt)
 */

import { useState, useCallback, useEffect } from 'react';
import { Modal } from '@/components/overlays/Modal';
import { Button } from '@/components/ui/Button';
import { configClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';
import type { Agent } from '@/gen/orc/v1/config_pb';
import '../task-detail/TaskEditModal.css';

interface AddAgentModalProps {
	open: boolean;
	onClose: () => void;
	onCreate?: (agent: Agent) => void;
}

const MODEL_OPTIONS = [
	{ value: '', label: 'Default (inherit)' },
	{ value: 'opus', label: 'Opus' },
	{ value: 'sonnet', label: 'Sonnet' },
	{ value: 'haiku', label: 'Haiku' },
] as const;

const TOOL_OPTIONS = [
	{ value: 'Read', label: 'Read - Read file contents' },
	{ value: 'Grep', label: 'Grep - Search file contents' },
	{ value: 'Glob', label: 'Glob - Find files by pattern' },
	{ value: 'Edit', label: 'Edit - Modify files' },
	{ value: 'Write', label: 'Write - Create new files' },
	{ value: 'Bash', label: 'Bash - Run shell commands' },
	{ value: 'WebFetch', label: 'WebFetch - Fetch web content' },
	{ value: 'WebSearch', label: 'WebSearch - Search the web' },
] as const;

export function AddAgentModal({ open, onClose, onCreate }: AddAgentModalProps) {
	const [name, setName] = useState('');
	const [description, setDescription] = useState('');
	const [model, setModel] = useState('');
	const [systemPrompt, setSystemPrompt] = useState(''); // For executor role
	const [prompt, setPrompt] = useState(''); // For sub-agent role
	const [claudeConfig, setClaudeConfig] = useState(''); // JSON config
	const [selectedTools, setSelectedTools] = useState<string[]>(['Read', 'Grep', 'Glob']);
	const [saving, setSaving] = useState(false);

	// Reset form when modal opens
	useEffect(() => {
		if (open) {
			setName('');
			setDescription('');
			setModel('');
			setSystemPrompt('');
			setPrompt('');
			setClaudeConfig('');
			setSelectedTools(['Read', 'Grep', 'Glob']);
		}
	}, [open]);

	const handleToolToggle = useCallback((tool: string) => {
		setSelectedTools((prev) =>
			prev.includes(tool) ? prev.filter((t) => t !== tool) : [...prev, tool]
		);
	}, []);

	const handleCreate = useCallback(async () => {
		if (!name.trim()) {
			toast.error('Name is required');
			return;
		}
		if (!description.trim()) {
			toast.error('Description is required');
			return;
		}

		// Validate claude config JSON if provided
		if (claudeConfig.trim()) {
			try {
				JSON.parse(claudeConfig.trim());
			} catch {
				toast.error('Claude config must be valid JSON');
				return;
			}
		}

		setSaving(true);
		try {
			// Use name as ID (slugified)
			const agentId = name.trim().toLowerCase().replace(/\s+/g, '-');
			const response = await configClient.createAgent({
				id: agentId,
				name: name.trim(),
				description: description.trim(),
				model: model || undefined,
				systemPrompt: systemPrompt.trim() || undefined,
				prompt: prompt.trim() || undefined,
				claudeConfig: claudeConfig.trim() || undefined,
				tools: selectedTools.length > 0 ? { allow: selectedTools } : undefined,
			});

			if (response.agent) {
				toast.success(`Agent '${response.agent.name}' created`);
				onCreate?.(response.agent);
			}
			onClose();
		} catch (e) {
			toast.error(e instanceof Error ? e.message : 'Failed to create agent');
		} finally {
			setSaving(false);
		}
	}, [name, description, model, systemPrompt, prompt, claudeConfig, selectedTools, onCreate, onClose]);

	// Handle Enter key on simple inputs (not textarea)
	const handleKeyDown = useCallback(
		(e: React.KeyboardEvent) => {
			if (e.key === 'Enter' && !e.shiftKey && name.trim() && description.trim()) {
				e.preventDefault();
				handleCreate();
			}
		},
		[handleCreate, name, description]
	);

	return (
		<Modal open={open} title="New Agent" onClose={onClose}>
			<div className="task-edit-form">
				{/* Name */}
				<div className="form-group">
					<label htmlFor="new-agent-name">Name</label>
					<input
						id="new-agent-name"
						type="text"
						value={name}
						onChange={(e) => setName(e.target.value)}
						onKeyDown={handleKeyDown}
						placeholder="my-agent"
						autoFocus
					/>
					<span className="form-hint">
						Unique identifier for this agent (e.g., security-checker, test-writer)
					</span>
				</div>

				{/* Description */}
				<div className="form-group">
					<label htmlFor="new-agent-description">Description *</label>
					<input
						id="new-agent-description"
						type="text"
						value={description}
						onChange={(e) => setDescription(e.target.value)}
						onKeyDown={handleKeyDown}
						placeholder="When to use this agent..."
					/>
					<span className="form-hint">
						Claude uses this to decide when to delegate to this agent
					</span>
				</div>

				{/* Model */}
				<div className="form-group">
					<label htmlFor="new-agent-model">Model</label>
					<select
						id="new-agent-model"
						value={model}
						onChange={(e) => setModel(e.target.value)}
					>
						{MODEL_OPTIONS.map((opt) => (
							<option key={opt.value} value={opt.value}>
								{opt.label}
							</option>
						))}
					</select>
				</div>

				{/* Tools */}
				<div className="form-group">
					<label>Allowed Tools</label>
					<div className="form-checkbox-group">
						{TOOL_OPTIONS.map((tool) => (
							<label key={tool.value} className="form-checkbox-label">
								<input
									type="checkbox"
									checked={selectedTools.includes(tool.value)}
									onChange={() => handleToolToggle(tool.value)}
								/>
								<span>{tool.label}</span>
							</label>
						))}
					</div>
				</div>

				{/* System Prompt - for executor role */}
				<div className="form-group">
					<label htmlFor="new-agent-system-prompt">System Prompt (executor role)</label>
					<textarea
						id="new-agent-system-prompt"
						value={systemPrompt}
						onChange={(e) => setSystemPrompt(e.target.value)}
						placeholder="System prompt when this agent runs a phase..."
						rows={4}
					/>
					<span className="form-hint">
						Used when this agent executes a phase as the main runner
					</span>
				</div>

				{/* Prompt - for sub-agent role */}
				<div className="form-group">
					<label htmlFor="new-agent-prompt">Role Prompt (sub-agent role)</label>
					<textarea
						id="new-agent-prompt"
						value={prompt}
						onChange={(e) => setPrompt(e.target.value)}
						placeholder="Context when another agent delegates to this one..."
						rows={4}
					/>
					<span className="form-hint">
						Used when the executor agent delegates work to this agent
					</span>
				</div>

				{/* Claude Config - advanced JSON */}
				<div className="form-group">
					<label htmlFor="new-agent-config">Claude Config (optional JSON)</label>
					<textarea
						id="new-agent-config"
						value={claudeConfig}
						onChange={(e) => setClaudeConfig(e.target.value)}
						placeholder='{"append_system_prompt": "..."}'
						rows={2}
					/>
					<span className="form-hint">
						Advanced: JSON configuration for Claude API settings
					</span>
				</div>

				{/* Actions */}
				<div className="form-actions">
					<Button type="button" variant="secondary" onClick={onClose}>
						Cancel
					</Button>
					<Button
						type="button"
						variant="primary"
						onClick={handleCreate}
						disabled={!name.trim() || !description.trim()}
						loading={saving}
					>
						Create Agent
					</Button>
				</div>
			</div>
		</Modal>
	);
}
