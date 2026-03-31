import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { WorkflowPhase } from '@/gen/orc/v1/workflow_pb';
import type { Hook, Skill } from '@/gen/orc/v1/config_pb';
import type { MCPServerInfo } from '@/gen/orc/v1/mcp_pb';
import {
	hydrateSelectedMCPServers,
	mergeRuntimeConfigs,
	parseRuntimeConfig,
	serializeRuntimeConfig,
	type HookDefinition,
	type RuntimeConfigState,
} from '@/lib/runtimeConfigUtils';
import { CollapsibleSettingsSection } from '@/components/core/CollapsibleSettingsSection';
import { KeyValueEditor } from '@/components/core/KeyValueEditor';
import { LibraryPicker } from '@/components/core/LibraryPicker';
import { TagInput } from '@/components/core/TagInput';
import { fetchMCPServerConfig } from './shared';

interface RuntimeConfigEditorProps {
	phase: WorkflowPhase;
	disabled: boolean;
	onSave: (json: string) => void;
	hooks: Hook[];
	hooksLoading: boolean;
	hooksError: string;
	skills: Skill[];
	skillsLoading: boolean;
	skillsError: string;
	mcpServers: MCPServerInfo[];
	mcpLoading: boolean;
	mcpError: string;
}

export function RuntimeConfigEditor({
	phase,
	disabled,
	onSave,
	hooks,
	hooksLoading,
	hooksError,
	skills,
	skillsLoading,
	skillsError,
	mcpServers,
	mcpLoading,
	mcpError,
}: RuntimeConfigEditorProps) {
	const [selectedHooks, setSelectedHooks] = useState<string[]>([]);
	const [selectedSkills, setSelectedSkills] = useState<string[]>([]);
	const [selectedMCPServers, setSelectedMCPServers] = useState<string[]>([]);
	const [allowedTools, setAllowedTools] = useState<string[]>([]);
	const [disallowedTools, setDisallowedTools] = useState<string[]>([]);
	const [envVars, setEnvVars] = useState<Record<string, string>>({});
	const [mcpServerData, setMcpServerData] = useState<Record<string, unknown>>({});
	const [hookConfig, setHookConfig] = useState<Record<string, unknown>>({});
	const [hookEventTypes, setHookEventTypes] = useState<Record<string, string>>({});
	const [extraFields, setExtraFields] = useState<Record<string, unknown>>({});
	const [jsonText, setJsonText] = useState('');
	const [jsonError, setJsonError] = useState('');
	const jsonActiveRef = useRef(false);

	useEffect(() => {
		const config = parseRuntimeConfig(phase.runtimeConfigOverride);
		setSelectedHooks(config.hooks);
		setSelectedSkills(config.skillRefs);
		setSelectedMCPServers(config.mcpServers);
		setAllowedTools(config.allowedTools);
		setDisallowedTools(config.disallowedTools);
		setEnvVars(config.env);
		setMcpServerData(config.mcpServerData ?? {});
		setHookConfig(config.hookConfig ?? {});
		setHookEventTypes(config.hookEventTypes ?? {});
		setExtraFields(config.extra);
		jsonActiveRef.current = false;
	}, [phase.id, phase.runtimeConfigOverride]);

	useEffect(() => {
		let mounted = true;
		hydrateSelectedMCPServers(
			selectedMCPServers,
			mcpServerData,
			fetchMCPServerConfig,
		).then((hydrated) => {
			if (mounted) {
				setMcpServerData(hydrated);
			}
		}).catch(() => {});

		return () => {
			mounted = false;
		};
	}, [selectedMCPServers]);

	useEffect(() => {
		if (!jsonActiveRef.current) {
			setJsonText(serializeRuntimeConfig({
				hooks: selectedHooks,
				skillRefs: selectedSkills,
				mcpServers: selectedMCPServers,
				allowedTools,
				disallowedTools,
				env: envVars,
				mcpServerData,
				hookConfig,
				hookEventTypes,
				extra: extraFields,
			}, {
				hookDefinitions: hooks.map((hook): HookDefinition => ({
					name: hook.name,
					eventType: hook.eventType,
				})),
			}));
		}
	}, [selectedHooks, selectedSkills, selectedMCPServers, allowedTools, disallowedTools, envVars, mcpServerData, hookConfig, hookEventTypes, extraFields, hooks]);

	const saveConfig = useCallback(
		async (overrides: Partial<RuntimeConfigState>) => {
			const nextMcpServers = overrides.mcpServers ?? selectedMCPServers;
			const json = serializeRuntimeConfig({
				hooks: overrides.hooks ?? selectedHooks,
				skillRefs: overrides.skillRefs ?? selectedSkills,
				mcpServers: nextMcpServers,
				allowedTools: overrides.allowedTools ?? allowedTools,
				disallowedTools: overrides.disallowedTools ?? disallowedTools,
				env: overrides.env ?? envVars,
				mcpServerData: await hydrateSelectedMCPServers(
					nextMcpServers,
					overrides.mcpServerData ?? mcpServerData,
					fetchMCPServerConfig,
				),
				hookConfig: overrides.hookConfig ?? hookConfig,
				hookEventTypes: overrides.hookEventTypes ?? hookEventTypes,
				extra: overrides.extra ?? extraFields,
			}, {
				hookDefinitions: hooks.map((hook): HookDefinition => ({
					name: hook.name,
					eventType: hook.eventType,
				})),
			});
			onSave(json);
		},
		[selectedHooks, selectedSkills, selectedMCPServers, allowedTools, disallowedTools, envVars, mcpServerData, hookConfig, hookEventTypes, extraFields, onSave, hooks],
	);

	const handleJsonBlur = useCallback(() => {
		try {
			const parsed = JSON.parse(jsonText);
			if (typeof parsed !== 'object' || parsed === null) {
				setJsonError('Invalid JSON');
				return;
			}
			const config = parseRuntimeConfig(jsonText);
			setSelectedHooks(config.hooks);
			setSelectedSkills(config.skillRefs);
			setSelectedMCPServers(config.mcpServers);
			setAllowedTools(config.allowedTools);
			setDisallowedTools(config.disallowedTools);
			setEnvVars(config.env);
			setMcpServerData(config.mcpServerData ?? {});
			setHookConfig(config.hookConfig ?? {});
			setHookEventTypes(config.hookEventTypes ?? {});
			setExtraFields(config.extra);
			setJsonError('');
			jsonActiveRef.current = false;
			onSave(jsonText);
		} catch {
			setJsonError('Invalid JSON');
		}
	}, [jsonText, onSave]);

	const template = phase.template;
	const templateConfigStr = (template as Record<string, unknown> | undefined)?.runtimeConfig as string | undefined;
	const merged = useMemo(
		() => mergeRuntimeConfigs(templateConfigStr, phase.runtimeConfigOverride),
		[templateConfigStr, phase.runtimeConfigOverride],
	);
	const inheritedCount = templateConfigStr ? parseRuntimeConfig(templateConfigStr) : null;

	return (
		<div className="claude-config-summary">
			<h4 className="claude-config-summary__title">Runtime Config</h4>

			{inheritedCount && (
				(inheritedCount.hooks.length > 0 ||
					inheritedCount.skillRefs.length > 0 ||
					inheritedCount.mcpServers.length > 0 ||
					inheritedCount.allowedTools.length > 0 ||
					inheritedCount.disallowedTools.length > 0 ||
					Object.keys(inheritedCount.env).length > 0) && (
					<div className="phase-inspector-setting-hint" style={{ marginBottom: '8px' }}>
						Inherited from template: {[
							inheritedCount.hooks.length > 0 && `${inheritedCount.hooks.length} hooks`,
							inheritedCount.skillRefs.length > 0 && `${inheritedCount.skillRefs.length} skills`,
							inheritedCount.mcpServers.length > 0 && `${inheritedCount.mcpServers.length} MCP servers`,
							inheritedCount.allowedTools.length > 0 && `${inheritedCount.allowedTools.length} allowed tools`,
							inheritedCount.disallowedTools.length > 0 && `${inheritedCount.disallowedTools.length} disallowed tools`,
							Object.keys(inheritedCount.env).length > 0 && `${Object.keys(inheritedCount.env).length} env vars`,
						].filter(Boolean).join(', ')}
					</div>
				)
			)}

			<CollapsibleSettingsSection title="Hooks" badgeCount={merged.hooks.length}>
				<InheritedChips items={inheritedCount?.hooks} />
				<LibraryPicker
					type="hooks"
					items={hooks}
					selectedNames={selectedHooks}
					onSelectionChange={(names) => {
						setSelectedHooks(names);
						jsonActiveRef.current = false;
						void saveConfig({ hooks: names });
					}}
					error={hooksError}
					loading={hooksLoading}
					disabled={disabled}
				/>
			</CollapsibleSettingsSection>

			<CollapsibleSettingsSection title="MCP Servers" badgeCount={merged.mcpServers.length}>
				<InheritedChips items={inheritedCount?.mcpServers} />
				<LibraryPicker
					type="mcpServers"
					items={mcpServers}
					selectedNames={selectedMCPServers}
					onSelectionChange={(names) => {
						setSelectedMCPServers(names);
						jsonActiveRef.current = false;
						void saveConfig({ mcpServers: names });
					}}
					error={mcpError}
					loading={mcpLoading}
					disabled={disabled}
				/>
			</CollapsibleSettingsSection>

			<CollapsibleSettingsSection title="Skills" badgeCount={merged.skillRefs.length}>
				<InheritedChips items={inheritedCount?.skillRefs} />
				<LibraryPicker
					type="skills"
					items={skills}
					selectedNames={selectedSkills}
					onSelectionChange={(names) => {
						setSelectedSkills(names);
						jsonActiveRef.current = false;
						void saveConfig({ skillRefs: names });
					}}
					error={skillsError}
					loading={skillsLoading}
					disabled={disabled}
				/>
			</CollapsibleSettingsSection>

			<CollapsibleSettingsSection title="Allowed Tools" badgeCount={merged.allowedTools.length}>
				<InheritedChips items={inheritedCount?.allowedTools} />
				<TagInput
					tags={allowedTools}
					onChange={(tags) => {
						setAllowedTools(tags);
						jsonActiveRef.current = false;
						void saveConfig({ allowedTools: tags });
					}}
					placeholder="Add tool name..."
					disabled={disabled}
				/>
			</CollapsibleSettingsSection>

			<CollapsibleSettingsSection title="Disallowed Tools" badgeCount={merged.disallowedTools.length}>
				<InheritedChips items={inheritedCount?.disallowedTools} />
				<TagInput
					tags={disallowedTools}
					onChange={(tags) => {
						setDisallowedTools(tags);
						jsonActiveRef.current = false;
						void saveConfig({ disallowedTools: tags });
					}}
					placeholder="Add tool name..."
					disabled={disabled}
				/>
			</CollapsibleSettingsSection>

			<CollapsibleSettingsSection title="Env Vars" badgeCount={Object.keys(merged.env).length}>
				<InheritedChips items={inheritedCount?.env ? Object.keys(inheritedCount.env) : undefined} label="env vars" />
				<KeyValueEditor
					entries={envVars}
					onChange={(entries) => {
						setEnvVars(entries);
						jsonActiveRef.current = false;
						void saveConfig({ env: entries });
					}}
					disabled={disabled}
				/>
			</CollapsibleSettingsSection>

			<CollapsibleSettingsSection title="JSON Override" badgeCount={0}>
				<div className="claude-config-json-override">
					<textarea
						className={`claude-config-json-textarea ${jsonError ? 'claude-config-json-textarea--error' : ''}`}
						value={jsonText}
						onChange={(e) => {
							setJsonText(e.target.value);
							jsonActiveRef.current = true;
							setJsonError('');
						}}
						onBlur={handleJsonBlur}
						rows={6}
						disabled={disabled}
						aria-label="Claude config JSON override"
					/>
					{jsonError && <span className="claude-config-json-error">{jsonError}</span>}
				</div>
			</CollapsibleSettingsSection>
		</div>
	);
}

function InheritedChips({ items }: { items?: string[]; label?: string }) {
	if (!items || items.length === 0) return null;
	return (
		<div className="inherited-chips">
			<span className="inherited-chips__label">From template:</span>
			{items.map((item) => (
				<span key={item} className="inherited-chips__chip">{item}</span>
			))}
		</div>
	);
}
