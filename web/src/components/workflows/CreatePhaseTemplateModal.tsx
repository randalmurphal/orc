/**
 * CreatePhaseTemplateModal - Modal for creating a new phase template from scratch.
 *
 * Features:
 * - Create new phase template with ID auto-generated from Name
 * - Prompt source toggle: Inline (DB) or File
 * - Inline prompt editor with {{VARIABLE}} highlighting
 * - Data flow: Input Variables (with suggestions), Output Variable Name
 * - Execution settings: Agent, Gate Type, Max Iterations, Thinking, Checkpoint
 * - 7 collapsible Claude Config sections (same as EditPhaseTemplateModal)
 */

import { useState, useCallback, useEffect, useRef } from 'react';
import { Modal } from '@/components/overlays/Modal';
import { Button, Icon } from '@/components/ui';
import { workflowClient, configClient, mcpClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';
import type { PhaseTemplate } from '@/gen/orc/v1/workflow_pb';
import type { Agent, Hook, Skill } from '@/gen/orc/v1/config_pb';
import type { MCPServerInfo } from '@/gen/orc/v1/mcp_pb';
import { GateType, PromptSource } from '@/gen/orc/v1/workflow_pb';
import { CollapsibleSettingsSection } from '@/components/core/CollapsibleSettingsSection';
import { LibraryPicker } from '@/components/core/LibraryPicker';
import { TagInput } from '@/components/core/TagInput';
import { KeyValueEditor } from '@/components/core/KeyValueEditor';
import './CreatePhaseTemplateModal.css';

export interface CreatePhaseTemplateModalProps {
	/** Whether the modal is open */
	open: boolean;
	/** Callback when modal should close */
	onClose: () => void;
	/** Callback when template is successfully created */
	onCreated: (template: PhaseTemplate) => void;
}

const GATE_TYPE_OPTIONS = [
	{ value: GateType.AUTO, label: 'Auto' },
	{ value: GateType.HUMAN, label: 'Human' },
	{ value: GateType.SKIP, label: 'Skip' },
];

/** Common variable suggestions for input variables */
const VARIABLE_SUGGESTIONS = [
	'SPEC_CONTENT',
	'PROJECT_ROOT',
	'TASK_DESCRIPTION',
	'WORKTREE_PATH',
	'INITIATIVE_VISION',
	'INITIATIVE_DECISIONS',
	'RETRY_CONTEXT',
	'TDD_TEST_CONTENT',
	'BREAKDOWN_CONTENT',
];

/** Convert a name to a URL-friendly slug */
function slugify(name: string): string {
	return name
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '')
		.replace(/-+/g, '-');
}

/** Serialize structured state back to claude_config JSON */
function serializeClaudeConfig(state: {
	hooks: string[];
	skillRefs: string[];
	mcpServers: string[];
	allowedTools: string[];
	disallowedTools: string[];
	env: Record<string, string>;
	extra: Record<string, unknown>;
	mcpServerData: Record<string, unknown>;
}): string {
	const config: Record<string, unknown> = { ...state.extra };

	if (state.hooks.length > 0) config.hooks = state.hooks;
	if (state.skillRefs.length > 0) config.skill_refs = state.skillRefs;
	if (Object.keys(state.mcpServers).length > 0) {
		const servers: Record<string, unknown> = {};
		for (const name of state.mcpServers) {
			servers[name] = state.mcpServerData[name] || {};
		}
		config.mcp_servers = servers;
	}
	if (state.allowedTools.length > 0) config.allowed_tools = state.allowedTools;
	if (state.disallowedTools.length > 0) config.disallowed_tools = state.disallowedTools;
	if (Object.keys(state.env).length > 0) config.env = state.env;

	return JSON.stringify(config, null, 2);
}

/**
 * VariableTagInput - Tag input with variable suggestions dropdown
 */
interface VariableTagInputProps {
	tags: string[];
	onChange: (tags: string[]) => void;
	suggestions?: string[];
	placeholder?: string;
}

function VariableTagInput({ tags, onChange, suggestions = VARIABLE_SUGGESTIONS, placeholder }: VariableTagInputProps) {
	const [inputValue, setInputValue] = useState('');
	const [showSuggestions, setShowSuggestions] = useState(false);
	const inputRef = useRef<HTMLInputElement>(null);

	const filteredSuggestions = suggestions.filter(
		(s) => !tags.includes(s) && s.toLowerCase().includes(inputValue.toLowerCase())
	);

	const addTag = useCallback(
		(value: string) => {
			const trimmed = value.trim().toUpperCase();
			if (!trimmed) return;
			if (tags.includes(trimmed)) return;
			onChange([...tags, trimmed]);
			setInputValue('');
		},
		[tags, onChange]
	);

	const handleKeyDown = useCallback(
		(e: React.KeyboardEvent<HTMLInputElement>) => {
			if (e.key === 'Enter') {
				e.preventDefault();
				addTag(inputValue);
			} else if (e.key === 'Backspace' && inputValue === '' && tags.length > 0) {
				onChange(tags.slice(0, -1));
			} else if (e.key === 'Escape') {
				setShowSuggestions(false);
			}
		},
		[inputValue, tags, onChange, addTag]
	);

	const handleInputChange = useCallback(
		(e: React.ChangeEvent<HTMLInputElement>) => {
			setInputValue(e.target.value);
			setShowSuggestions(true);
		},
		[]
	);

	const removeTag = useCallback(
		(index: number) => {
			onChange(tags.filter((_, i) => i !== index));
		},
		[tags, onChange]
	);

	const handleSuggestionClick = useCallback(
		(suggestion: string) => {
			addTag(suggestion);
			setShowSuggestions(false);
			inputRef.current?.focus();
		},
		[addTag]
	);

	return (
		<div className="variable-tag-input">
			<div className="variable-tag-input__chips">
				{tags.map((tag, index) => (
					<span key={tag} className="variable-tag-input__chip" data-tag={tag}>
						<span className="label-text">{tag}</span>
						<button
							type="button"
							className="variable-tag-input__chip-remove"
							onClick={() => removeTag(index)}
							aria-label={`Remove ${tag}`}
						>
							×
						</button>
					</span>
				))}
			</div>
			<div className="variable-tag-input__input-wrapper">
				<input
					ref={inputRef}
					type="text"
					className="variable-tag-input__input"
					value={inputValue}
					onChange={handleInputChange}
					onKeyDown={handleKeyDown}
					onFocus={() => setShowSuggestions(true)}
					onBlur={() => setTimeout(() => setShowSuggestions(false), 200)}
					placeholder={placeholder || 'Add variable...'}
					aria-label="Input Variables"
				/>
				{showSuggestions && filteredSuggestions.length > 0 && (
					<ul className="variable-tag-input__suggestions" role="listbox">
						{filteredSuggestions.map((suggestion) => (
							<li
								key={suggestion}
								role="option"
								aria-selected={false}
								className="variable-tag-input__suggestion"
								onMouseDown={(e) => {
									e.preventDefault();
									handleSuggestionClick(suggestion);
								}}
							>
								{suggestion}
							</li>
						))}
					</ul>
				)}
			</div>
		</div>
	);
}

/**
 * PromptEditor - Textarea with {{VARIABLE}} highlighting overlay
 */
interface PromptEditorProps {
	value: string;
	onChange: (value: string) => void;
	placeholder?: string;
}

function PromptEditor({ value, onChange, placeholder }: PromptEditorProps) {
	const textareaRef = useRef<HTMLTextAreaElement>(null);
	const highlightRef = useRef<HTMLDivElement>(null);

	// Sync scroll between textarea and highlight overlay
	const handleScroll = useCallback(() => {
		if (textareaRef.current && highlightRef.current) {
			highlightRef.current.scrollTop = textareaRef.current.scrollTop;
			highlightRef.current.scrollLeft = textareaRef.current.scrollLeft;
		}
	}, []);

	// Render text with highlighted variables
	const renderHighlightedText = useCallback((text: string) => {
		const parts: React.ReactNode[] = [];
		// Match {{VARIABLE_NAME}} patterns
		const regex = /(\{\{[A-Z_][A-Z0-9_]*\}\})/g;
		let lastIndex = 0;
		let match;

		while ((match = regex.exec(text)) !== null) {
			// Add text before the match
			if (match.index > lastIndex) {
				parts.push(
					<span key={`text-${lastIndex}`} className="prompt-editor-text">
						{text.slice(lastIndex, match.index)}
					</span>
				);
			}
			// Add the highlighted variable
			parts.push(
				<span key={`var-${match.index}`} className="prompt-editor-highlight variable-highlight" data-variable-highlight>
					{match[1]}
				</span>
			);
			lastIndex = regex.lastIndex;
		}

		// Add remaining text
		if (lastIndex < text.length) {
			parts.push(
				<span key={`text-${lastIndex}`} className="prompt-editor-text">
					{text.slice(lastIndex)}
				</span>
			);
		}

		return parts;
	}, []);

	return (
		<div className="prompt-editor">
			<div
				ref={highlightRef}
				className="prompt-editor__highlight-overlay"
				aria-hidden="true"
			>
				{renderHighlightedText(value)}
			</div>
			<textarea
				ref={textareaRef}
				className="prompt-editor__textarea"
				value={value}
				onChange={(e) => onChange(e.target.value)}
				onScroll={handleScroll}
				placeholder={placeholder || 'Enter your prompt template...'}
				aria-label="Prompt Content"
				rows={8}
			/>
		</div>
	);
}

/**
 * CreatePhaseTemplateModal allows creating a new phase template from scratch.
 */
export function CreatePhaseTemplateModal({
	open,
	onClose,
	onCreated,
}: CreatePhaseTemplateModalProps) {
	// Form state
	const [templateId, setTemplateId] = useState('');
	const [name, setName] = useState('');
	const [description, setDescription] = useState('');
	const [idManuallyEdited, setIdManuallyEdited] = useState(false);

	// Prompt state
	const [promptSource, setPromptSource] = useState<'inline' | 'file'>('inline');
	const [promptContent, setPromptContent] = useState('');
	const [promptPath, setPromptPath] = useState('');

	// Data flow state
	const [inputVariables, setInputVariables] = useState<string[]>([]);
	const [outputVarName, setOutputVarName] = useState('');

	// Execution settings
	const [agentId, setAgentId] = useState('');
	const [maxIterations, setMaxIterations] = useState(20);
	const [gateType, setGateType] = useState<GateType>(GateType.AUTO);
	const [thinkingEnabled, setThinkingEnabled] = useState(false);
	const [checkpoint, setCheckpoint] = useState(false);

	// Claude config structured state
	const [selectedHooks, setSelectedHooks] = useState<string[]>([]);
	const [selectedSkills, setSelectedSkills] = useState<string[]>([]);
	const [selectedMCPServers, setSelectedMCPServers] = useState<string[]>([]);
	const [allowedTools, setAllowedTools] = useState<string[]>([]);
	const [disallowedTools, setDisallowedTools] = useState<string[]>([]);
	const [envVars, setEnvVars] = useState<Record<string, string>>({});
	const [extraFields] = useState<Record<string, unknown>>({});
	const [mcpServerData, setMcpServerData] = useState<Record<string, unknown>>({});

	// JSON override state
	const [jsonOverride, setJsonOverride] = useState('{}');
	const [jsonError, setJsonError] = useState('');
	const jsonOverrideActiveRef = useRef(false);

	// Library data
	const [hooks, setHooks] = useState<Hook[]>([]);
	const [skills, setSkills] = useState<Skill[]>([]);
	const [mcpServers, setMcpServers] = useState<MCPServerInfo[]>([]);
	const [hooksError, setHooksError] = useState('');
	const [skillsError, setSkillsError] = useState('');
	const [mcpError, setMcpError] = useState('');
	const [hooksLoading, setHooksLoading] = useState(true);
	const [skillsLoading, setSkillsLoading] = useState(true);
	const [mcpLoading, setMcpLoading] = useState(true);

	// Agents list
	const [agents, setAgents] = useState<Agent[]>([]);
	const [agentsLoading, setAgentsLoading] = useState(true);

	// Saving state
	const [saving, setSaving] = useState(false);

	// Reset form when modal opens
	useEffect(() => {
		if (open) {
			setTemplateId('');
			setName('');
			setDescription('');
			setIdManuallyEdited(false);
			setPromptSource('inline');
			setPromptContent('');
			setPromptPath('');
			setInputVariables([]);
			setOutputVarName('');
			setAgentId('');
			setMaxIterations(20);
			setGateType(GateType.AUTO);
			setThinkingEnabled(false);
			setCheckpoint(false);
			setSelectedHooks([]);
			setSelectedSkills([]);
			setSelectedMCPServers([]);
			setAllowedTools([]);
			setDisallowedTools([]);
			setEnvVars({});
			setMcpServerData({});
			setJsonOverride('{}');
			setJsonError('');
			jsonOverrideActiveRef.current = false;
		}
	}, [open]);

	// Fetch agents, hooks, skills, MCP servers on mount
	useEffect(() => {
		let mounted = true;

		configClient.listAgents({}).then((response) => {
			if (mounted) {
				setAgents(response.agents);
				setAgentsLoading(false);
			}
		}).catch(() => {
			if (mounted) setAgentsLoading(false);
		});

		configClient.listHooks({}).then((response) => {
			if (mounted) {
				setHooks(response.hooks);
				setHooksLoading(false);
			}
		}).catch(() => {
			if (mounted) {
				setHooksError('Failed to load hooks');
				setHooksLoading(false);
			}
		});

		configClient.listSkills({}).then((response) => {
			if (mounted) {
				setSkills(response.skills);
				setSkillsLoading(false);
			}
		}).catch(() => {
			if (mounted) {
				setSkillsError('Failed to load skills');
				setSkillsLoading(false);
			}
		});

		mcpClient.listMCPServers({}).then((response) => {
			if (mounted) {
				setMcpServers(response.servers);
				setMcpLoading(false);
			}
		}).catch(() => {
			if (mounted) {
				setMcpError('Failed to load MCP servers');
				setMcpLoading(false);
			}
		});

		return () => { mounted = false; };
	}, []);

	// Auto-generate ID from name (unless manually edited)
	const handleNameChange = useCallback((newName: string) => {
		setName(newName);
		if (!idManuallyEdited) {
			setTemplateId(slugify(newName));
		}
	}, [idManuallyEdited]);

	// Mark ID as manually edited
	const handleIdChange = useCallback((newId: string) => {
		setTemplateId(newId);
		setIdManuallyEdited(true);
	}, []);

	// Update JSON override when structured fields change
	useEffect(() => {
		if (!jsonOverrideActiveRef.current) {
			const json = serializeClaudeConfig({
				hooks: selectedHooks,
				skillRefs: selectedSkills,
				mcpServers: selectedMCPServers,
				allowedTools,
				disallowedTools,
				env: envVars,
				extra: extraFields,
				mcpServerData,
			});
			setJsonOverride(json);
		}
	}, [selectedHooks, selectedSkills, selectedMCPServers, allowedTools, disallowedTools, envVars, extraFields, mcpServerData]);

	// Handle JSON override blur
	const handleJsonBlur = useCallback(() => {
		try {
			const parsed = JSON.parse(jsonOverride);
			if (typeof parsed !== 'object' || parsed === null) {
				setJsonError('Invalid JSON');
				return;
			}

			// Re-parse into structured fields
			setSelectedHooks(Array.isArray(parsed.hooks) ? parsed.hooks : []);
			setSelectedSkills(Array.isArray(parsed.skill_refs) ? parsed.skill_refs : []);
			setAllowedTools(Array.isArray(parsed.allowed_tools) ? parsed.allowed_tools : []);
			setDisallowedTools(Array.isArray(parsed.disallowed_tools) ? parsed.disallowed_tools : []);
			setEnvVars(typeof parsed.env === 'object' && parsed.env !== null ? parsed.env : {});

			// MCP servers
			if (parsed.mcp_servers && typeof parsed.mcp_servers === 'object') {
				setSelectedMCPServers(Object.keys(parsed.mcp_servers));
				setMcpServerData(parsed.mcp_servers);
			} else {
				setSelectedMCPServers([]);
				setMcpServerData({});
			}

			setJsonError('');
			jsonOverrideActiveRef.current = false;
		} catch {
			setJsonError('Invalid JSON');
		}
	}, [jsonOverride]);

	// Validate required fields
	const isValid = templateId.trim() !== '' && name.trim() !== '';

	// Handle create
	const handleCreate = useCallback(async () => {
		if (!isValid) return;

		setSaving(true);
		try {
			const claudeConfig = serializeClaudeConfig({
				hooks: selectedHooks,
				skillRefs: selectedSkills,
				mcpServers: selectedMCPServers,
				allowedTools,
				disallowedTools,
				env: envVars,
				extra: extraFields,
				mcpServerData,
			});

			const response = await workflowClient.createPhaseTemplate({
				id: templateId.trim(),
				name: name.trim(),
				description: description.trim() || undefined,
				promptSource: promptSource === 'file' ? PromptSource.FILE : PromptSource.DB,
				promptContent: promptSource === 'inline' ? promptContent || undefined : undefined,
				promptPath: promptSource === 'file' ? promptPath || undefined : undefined,
				maxIterations: maxIterations,
				gateType: gateType,
				thinkingEnabled: thinkingEnabled || undefined,
				checkpoint: checkpoint,
				agentId: agentId || undefined,
				claudeConfig: claudeConfig !== '{}' ? claudeConfig : undefined,
				outputVarName: outputVarName.trim() || undefined,
				producesArtifact: false,
				inputVariables: inputVariables.length > 0 ? inputVariables : undefined,
				subAgentIds: [],
			});

			if (response.template) {
				toast.success(`Phase template created: ${response.template.name}`);
				onCreated(response.template);
				onClose();
			}
		} catch (e) {
			const errorMsg = e instanceof Error ? e.message : 'Unknown error';
			toast.error(`Failed to create template: ${errorMsg}`);
		} finally {
			setSaving(false);
		}
	}, [
		isValid,
		templateId,
		name,
		description,
		promptSource,
		promptContent,
		promptPath,
		maxIterations,
		gateType,
		thinkingEnabled,
		checkpoint,
		agentId,
		inputVariables,
		selectedHooks,
		selectedSkills,
		selectedMCPServers,
		allowedTools,
		disallowedTools,
		envVars,
		extraFields,
		mcpServerData,
		outputVarName,
		onCreated,
		onClose,
	]);

	return (
		<Modal
			open={open}
			onClose={onClose}
			title="Create Phase Template"
			size="md"
			ariaLabel="Create phase template dialog"
		>
			<form
				onSubmit={(e) => {
					e.preventDefault();
					handleCreate();
				}}
				className="create-template-form"
			>
				{/* Metadata Section */}
				<div className="create-template-section">
					<h3 className="create-template-section-title">Metadata</h3>

					{/* Name */}
					<div className="form-group">
						<label htmlFor="create-template-name" className="form-label">
							Name <span className="form-required">*</span>
						</label>
						<input
							id="create-template-name"
							type="text"
							className="form-input"
							value={name}
							onChange={(e) => handleNameChange(e.target.value)}
							placeholder="e.g. Code Analysis"
						/>
					</div>

					{/* Template ID */}
					<div className="form-group">
						<label htmlFor="create-template-id" className="form-label">
							Template ID <span className="form-required">*</span>
						</label>
						<input
							id="create-template-id"
							type="text"
							className="form-input form-input-mono"
							value={templateId}
							onChange={(e) => handleIdChange(e.target.value)}
							placeholder="e.g. code-analysis"
						/>
						<span className="form-help">
							Unique identifier (auto-generated from name)
						</span>
					</div>

					{/* Description */}
					<div className="form-group">
						<label htmlFor="create-template-description" className="form-label">
							Description
						</label>
						<textarea
							id="create-template-description"
							className="form-textarea"
							value={description}
							onChange={(e) => setDescription(e.target.value)}
							placeholder="Describe what this phase does..."
							rows={2}
						/>
					</div>
				</div>

				{/* Prompt Section */}
				<div className="create-template-section">
					<h3 className="create-template-section-title">Prompt</h3>

					<div className="form-group">
						<div className="create-template-toggle">
							<button
								type="button"
								className={`create-template-toggle-btn ${promptSource === 'inline' ? 'create-template-toggle-btn--active' : ''}`}
								onClick={() => setPromptSource('inline')}
							>
								Inline
							</button>
							<button
								type="button"
								className={`create-template-toggle-btn ${promptSource === 'file' ? 'create-template-toggle-btn--active' : ''}`}
								onClick={() => setPromptSource('file')}
							>
								File
							</button>
						</div>
					</div>

					{promptSource === 'inline' && (
						<div className="form-group">
							<PromptEditor
								value={promptContent}
								onChange={setPromptContent}
								placeholder="Enter your prompt template..."
							/>
							<span className="form-help">
								Use {'{{VARIABLE_NAME}}'} for template variables
							</span>
						</div>
					)}

					{promptSource === 'file' && (
						<div className="form-group">
							<label htmlFor="create-template-prompt-path" className="form-label">
								Prompt Path
							</label>
							<div className="create-template-path-input">
								<span className="create-template-path-prefix">.orc/prompts/</span>
								<input
									id="create-template-prompt-path"
									type="text"
									className="form-input"
									value={promptPath}
									onChange={(e) => setPromptPath(e.target.value)}
									placeholder="path/to/prompt.md"
								/>
							</div>
						</div>
					)}
				</div>

				{/* Data Flow Section */}
				<div className="create-template-section">
					<h3 className="create-template-section-title">Data Flow</h3>

					{/* Input Variables */}
					<div className="form-group">
						<label className="form-label">Input Variables</label>
						<VariableTagInput
							tags={inputVariables}
							onChange={setInputVariables}
							placeholder="Add variable..."
						/>
						<span className="form-help">
							Variables this phase expects to receive
						</span>
					</div>

					{/* Output Variable Name */}
					<div className="form-group">
						<label htmlFor="create-template-output-var" className="form-label">
							Output Variable
						</label>
						<input
							id="create-template-output-var"
							type="text"
							className="form-input form-input-mono"
							value={outputVarName}
							onChange={(e) => setOutputVarName(e.target.value)}
							placeholder="e.g. SPEC_CONTENT"
						/>
						<span className="form-help">
							Downstream phases reference this name
						</span>
					</div>
				</div>

				{/* Execution Section */}
				<div className="create-template-section">
					<h3 className="create-template-section-title">Execution</h3>

					{/* Agent and Gate Type */}
					<div className="form-row">
						<div className="form-group form-group-half">
							<label htmlFor="create-template-agent" className="form-label">
								Agent
							</label>
							<select
								id="create-template-agent"
								className="form-select"
								value={agentId}
								onChange={(e) => setAgentId(e.target.value)}
								disabled={agentsLoading}
							>
								<option value="">Default</option>
								{agents.map((agent) => (
									<option key={agent.name} value={agent.name}>
										{agent.name}
										{agent.model ? ` (${agent.model})` : ''}
									</option>
								))}
							</select>
						</div>

						<div className="form-group form-group-half">
							<label htmlFor="create-template-gate" className="form-label">
								Gate Type
							</label>
							<select
								id="create-template-gate"
								className="form-select"
								value={gateType}
								onChange={(e) => setGateType(Number(e.target.value) as GateType)}
							>
								{GATE_TYPE_OPTIONS.map((option) => (
									<option key={option.value} value={option.value}>
										{option.label}
									</option>
								))}
							</select>
						</div>
					</div>

					{/* Max iterations */}
					<div className="form-group">
						<label htmlFor="create-template-iterations" className="form-label">
							Max Iterations
						</label>
						<input
							id="create-template-iterations"
							type="number"
							className="form-input"
							value={maxIterations}
							onChange={(e) => setMaxIterations(Number(e.target.value))}
							min={1}
							max={1000}
						/>
					</div>

					{/* Options */}
					<div className="form-group">
						<label className="form-label">Options</label>
						<div className="create-template-options">
							<label className="form-checkbox">
								<input
									type="checkbox"
									checked={thinkingEnabled}
									onChange={(e) => setThinkingEnabled(e.target.checked)}
								/>
								<span className="form-checkbox-label">Enable Thinking</span>
							</label>
							<label className="form-checkbox">
								<input
									type="checkbox"
									checked={checkpoint}
									onChange={(e) => setCheckpoint(e.target.checked)}
								/>
								<span className="form-checkbox-label">Checkpoint</span>
							</label>
						</div>
					</div>
				</div>

				{/* Claude Config Settings Sections */}
				<div className="create-template-section">
					<h3 className="create-template-section-title">Claude Config</h3>

					<CollapsibleSettingsSection title="Hooks" badgeCount={selectedHooks.length} badgeText={String(selectedHooks.length)}>
						<LibraryPicker
							type="hooks"
							items={hooks}
							selectedNames={selectedHooks}
							onSelectionChange={(names) => {
								setSelectedHooks(names);
								jsonOverrideActiveRef.current = false;
							}}
							error={hooksError}
							loading={hooksLoading}
						/>
					</CollapsibleSettingsSection>

					<CollapsibleSettingsSection title="MCP Servers" badgeCount={selectedMCPServers.length} badgeText={String(selectedMCPServers.length)}>
						<LibraryPicker
							type="mcpServers"
							items={mcpServers}
							selectedNames={selectedMCPServers}
							onSelectionChange={(names) => {
								setSelectedMCPServers(names);
								jsonOverrideActiveRef.current = false;
							}}
							error={mcpError}
							loading={mcpLoading}
						/>
					</CollapsibleSettingsSection>

					<CollapsibleSettingsSection title="Skills" badgeCount={selectedSkills.length} badgeText={String(selectedSkills.length)}>
						<LibraryPicker
							type="skills"
							items={skills}
							selectedNames={selectedSkills}
							onSelectionChange={(names) => {
								setSelectedSkills(names);
								jsonOverrideActiveRef.current = false;
							}}
							error={skillsError}
							loading={skillsLoading}
						/>
					</CollapsibleSettingsSection>

					<CollapsibleSettingsSection title="Allowed Tools" badgeCount={allowedTools.length} badgeText={String(allowedTools.length)}>
						<TagInput
							tags={allowedTools}
							onChange={(tags) => {
								setAllowedTools(tags);
								jsonOverrideActiveRef.current = false;
							}}
							placeholder="Add tool name..."
						/>
					</CollapsibleSettingsSection>

					<CollapsibleSettingsSection title="Disallowed Tools" badgeCount={disallowedTools.length} badgeText={String(disallowedTools.length)}>
						<TagInput
							tags={disallowedTools}
							onChange={(tags) => {
								setDisallowedTools(tags);
								jsonOverrideActiveRef.current = false;
							}}
							placeholder="Add tool name..."
						/>
					</CollapsibleSettingsSection>

					<CollapsibleSettingsSection title="Env Vars" badgeCount={Object.keys(envVars).length} badgeText={String(Object.keys(envVars).length)}>
						<KeyValueEditor
							entries={envVars}
							onChange={(entries) => {
								setEnvVars(entries);
								jsonOverrideActiveRef.current = false;
							}}
						/>
					</CollapsibleSettingsSection>

					<CollapsibleSettingsSection title="JSON Override" badgeCount={0}>
						<div className="create-template-json-override">
							<textarea
								className={`create-template-json-textarea ${jsonError ? 'create-template-json-textarea--error' : ''}`}
								value={jsonOverride}
								onChange={(e) => {
									setJsonOverride(e.target.value);
									jsonOverrideActiveRef.current = true;
									setJsonError('');
								}}
								onBlur={handleJsonBlur}
								rows={8}
								aria-label="JSON override"
							/>
							{jsonError && (
								<span className="create-template-json-error">{jsonError}</span>
							)}
						</div>
					</CollapsibleSettingsSection>
				</div>

				{/* Actions */}
				<div className="form-actions">
					<Button
						type="button"
						variant="ghost"
						onClick={onClose}
						disabled={saving}
					>
						Cancel
					</Button>
					<Button
						type="submit"
						variant="primary"
						disabled={saving || !isValid}
						leftIcon={<Icon name="plus" size={12} />}
					>
						{saving ? 'Creating...' : 'Create'}
					</Button>
				</div>
			</form>
		</Modal>
	);
}
