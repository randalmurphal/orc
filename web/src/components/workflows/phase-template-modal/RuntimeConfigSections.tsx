import type { Hook, Skill } from '@/gen/orc/v1/config_pb';
import type { MCPServerInfo } from '@/gen/orc/v1/mcp_pb';
import { CollapsibleSettingsSection } from '@/components/core/CollapsibleSettingsSection';
import { KeyValueEditor } from '@/components/core/KeyValueEditor';
import { LibraryPicker } from '@/components/core/LibraryPicker';
import { TagInput } from '@/components/core/TagInput';

interface RuntimeConfigSectionsProps {
	selectedHooks: string[];
	onSelectedHooksChange: (value: string[]) => void;
	selectedMcpServers: string[];
	onSelectedMcpServersChange: (value: string[]) => void;
	selectedSkills: string[];
	onSelectedSkillsChange: (value: string[]) => void;
	allowedTools: string[];
	onAllowedToolsChange: (value: string[]) => void;
	disallowedTools: string[];
	onDisallowedToolsChange: (value: string[]) => void;
	envVars: Record<string, string>;
	onEnvVarsChange: (value: Record<string, string>) => void;
	jsonOverride: string;
	onJsonOverrideChange: (value: string) => void;
	onJsonOverrideBlur: () => void;
	jsonError: string;
	hooks: Hook[];
	hooksError: string;
	hooksLoading: boolean;
	skills: Skill[];
	skillsError: string;
	skillsLoading: boolean;
	mcpServers: MCPServerInfo[];
	mcpError: string;
	mcpLoading: boolean;
	jsonWrapperClassName: string;
	jsonTextareaClassName: string;
	jsonErrorClassName: string;
}

export function RuntimeConfigSections({
	selectedHooks,
	onSelectedHooksChange,
	selectedMcpServers,
	onSelectedMcpServersChange,
	selectedSkills,
	onSelectedSkillsChange,
	allowedTools,
	onAllowedToolsChange,
	disallowedTools,
	onDisallowedToolsChange,
	envVars,
	onEnvVarsChange,
	jsonOverride,
	onJsonOverrideChange,
	onJsonOverrideBlur,
	jsonError,
	hooks,
	hooksError,
	hooksLoading,
	skills,
	skillsError,
	skillsLoading,
	mcpServers,
	mcpError,
	mcpLoading,
	jsonWrapperClassName,
	jsonTextareaClassName,
	jsonErrorClassName,
}: RuntimeConfigSectionsProps) {
	return (
		<>
			<CollapsibleSettingsSection title="Hooks" badgeCount={selectedHooks.length} badgeText={String(selectedHooks.length)}>
				<LibraryPicker
					type="hooks"
					items={hooks}
					selectedNames={selectedHooks}
					onSelectionChange={onSelectedHooksChange}
					error={hooksError}
					loading={hooksLoading}
				/>
			</CollapsibleSettingsSection>

			<CollapsibleSettingsSection title="MCP Servers" badgeCount={selectedMcpServers.length} badgeText={String(selectedMcpServers.length)}>
				<LibraryPicker
					type="mcpServers"
					items={mcpServers}
					selectedNames={selectedMcpServers}
					onSelectionChange={onSelectedMcpServersChange}
					error={mcpError}
					loading={mcpLoading}
				/>
			</CollapsibleSettingsSection>

			<CollapsibleSettingsSection title="Skills" badgeCount={selectedSkills.length} badgeText={String(selectedSkills.length)}>
				<LibraryPicker
					type="skills"
					items={skills}
					selectedNames={selectedSkills}
					onSelectionChange={onSelectedSkillsChange}
					error={skillsError}
					loading={skillsLoading}
				/>
			</CollapsibleSettingsSection>

			<CollapsibleSettingsSection title="Allowed Tools" badgeCount={allowedTools.length} badgeText={String(allowedTools.length)}>
				<TagInput tags={allowedTools} onChange={onAllowedToolsChange} placeholder="Add tool name..." />
			</CollapsibleSettingsSection>

			<CollapsibleSettingsSection title="Disallowed Tools" badgeCount={disallowedTools.length} badgeText={String(disallowedTools.length)}>
				<TagInput tags={disallowedTools} onChange={onDisallowedToolsChange} placeholder="Add tool name..." />
			</CollapsibleSettingsSection>

			<CollapsibleSettingsSection title="Env Vars" badgeCount={Object.keys(envVars).length} badgeText={String(Object.keys(envVars).length)}>
				<KeyValueEditor entries={envVars} onChange={onEnvVarsChange} />
			</CollapsibleSettingsSection>

			<CollapsibleSettingsSection title="JSON Override" badgeCount={0}>
				<div className={jsonWrapperClassName}>
					<textarea
						className={`${jsonTextareaClassName} ${jsonError ? `${jsonTextareaClassName}--error` : ''}`}
						value={jsonOverride}
						onChange={(event) => onJsonOverrideChange(event.target.value)}
						onBlur={onJsonOverrideBlur}
						rows={8}
						aria-label="JSON override"
					/>
					{jsonError && <span className={jsonErrorClassName}>{jsonError}</span>}
				</div>
			</CollapsibleSettingsSection>
		</>
	);
}
