import { useCallback } from 'react';
import { CollapsibleSettingsSection } from '@/components/core/CollapsibleSettingsSection';
import { TagInput } from '@/components/core/TagInput';
import { KeyValueEditor } from '@/components/core/KeyValueEditor';
import type { RuntimeConfigState } from '@/lib/runtimeConfigUtils';

/** Generate badge text for a section showing inherited/override breakdown. */
function sectionBadgeText(inheritedCount: number, overrideCount: number): string {
	const total = inheritedCount + overrideCount;
	if (total === 0) return '0';
	if (overrideCount === 0) return `${total} inherited`;
	if (inheritedCount === 0) return `${total} override`;
	return `${total} — ${inheritedCount} inherited, ${overrideCount} override`;
}

/** Get unique override items (items in override that aren't in inherited). */
function uniqueOverrides(inherited: string[], overrides: string[]): string[] {
	const inheritedSet = new Set(inherited);
	return overrides.filter((item) => !inheritedSet.has(item));
}

interface RuntimeConfigSectionsProps {
	templateConfig: RuntimeConfigState;
	overrideHooks: string[];
	overrideSkills: string[];
	overrideMcpServers: string[];
	overrideAllowedTools: string[];
	overrideDisallowedTools: string[];
	overrideEnv: Record<string, string>;
	jsonOverride: string;
	availableHookNames: string[];
	availableSkillNames: string[];
	availableMcpServerNames: string[];
	onOverrideHooksChange: (hooks: string[]) => void;
	onOverrideSkillsChange: (skills: string[]) => void;
	onOverrideMcpServersChange: (servers: string[]) => void;
	onOverrideAllowedToolsChange: (tools: string[]) => void;
	onOverrideDisallowedToolsChange: (tools: string[]) => void;
	onOverrideEnvChange: (env: Record<string, string>) => void;
	onJsonOverrideChange: (json: string) => void;
	onClearOverride: (section: string) => void;
}

export function RuntimeConfigSections({
	templateConfig,
	overrideHooks,
	overrideSkills,
	overrideMcpServers,
	overrideAllowedTools,
	overrideDisallowedTools,
	overrideEnv,
	jsonOverride,
	availableHookNames,
	availableSkillNames,
	availableMcpServerNames,
	onOverrideHooksChange,
	onOverrideSkillsChange,
	onOverrideMcpServersChange,
	onOverrideAllowedToolsChange,
	onOverrideDisallowedToolsChange,
	onOverrideEnvChange,
	onJsonOverrideChange,
	onClearOverride,
}: RuntimeConfigSectionsProps) {
	return (
		<div className="claude-config-sections">
			<ListOverrideSection
				title="Hooks"
				testId="hooks-picker"
				inherited={templateConfig.hooks}
				overrides={overrideHooks}
				availableItems={availableHookNames}
				onChange={onOverrideHooksChange}
				onClear={() => onClearOverride('hooks')}
			/>

			<ListOverrideSection
				title="MCP Servers"
				testId="mcp-servers-picker"
				inherited={templateConfig.mcpServers}
				overrides={overrideMcpServers}
				availableItems={availableMcpServerNames}
				onChange={onOverrideMcpServersChange}
				onClear={() => onClearOverride('mcpServers')}
			/>

			<ListOverrideSection
				title="Skills"
				testId="skills-picker"
				inherited={templateConfig.skillRefs}
				overrides={overrideSkills}
				availableItems={availableSkillNames}
				onChange={onOverrideSkillsChange}
				onClear={() => onClearOverride('skills')}
			/>

			<ToolsOverrideSection
				title="Allowed Tools"
				testId="allowed-tools-input"
				inherited={templateConfig.allowedTools}
				overrides={overrideAllowedTools}
				onChange={onOverrideAllowedToolsChange}
				onClear={() => onClearOverride('allowedTools')}
			/>

			<ToolsOverrideSection
				title="Disallowed Tools"
				testId="disallowed-tools-input"
				inherited={templateConfig.disallowedTools}
				overrides={overrideDisallowedTools}
				onChange={onOverrideDisallowedToolsChange}
				onClear={() => onClearOverride('disallowedTools')}
			/>

			<EnvVarsOverrideSection
				templateEnv={templateConfig.env}
				overrideEnv={overrideEnv}
				onChange={onOverrideEnvChange}
				onClear={() => onClearOverride('env')}
			/>

			<CollapsibleSettingsSection
				title="JSON Override"
				badgeCount={0}
				badgeText={jsonOverride ? '1' : '0'}
			>
				<textarea
					className="claude-config-json-textarea"
					value={jsonOverride}
					onChange={(e) => onJsonOverrideChange(e.target.value)}
					aria-label="JSON Override"
					placeholder='{"hooks": ["my-hook"], ...}'
					rows={4}
				/>
			</CollapsibleSettingsSection>
		</div>
	);
}

interface ListOverrideSectionProps {
	title: string;
	testId: string;
	inherited: string[];
	overrides: string[];
	availableItems?: string[];
	onChange: (items: string[]) => void;
	onClear: () => void;
}

function ListOverrideSection({
	title,
	testId,
	inherited,
	overrides,
	availableItems,
	onChange,
	onClear,
}: ListOverrideSectionProps) {
	const uniqueOverrideItems = uniqueOverrides(inherited, overrides);
	const inheritedCount = inherited.length;
	const overrideCount = uniqueOverrideItems.length;

	const handleAdd = useCallback(() => {
		if (availableItems && availableItems.length > 0) {
			const selected = new Set([...inherited, ...overrides]);
			const nextAvailable = availableItems.find((item) => !selected.has(item));
			if (nextAvailable) {
				onChange([...overrides, nextAvailable]);
			}
			return;
		}
		const baseName = `new-${title.toLowerCase().replace(/\s+/g, '-')}`;
		let name = baseName;
		let counter = 1;
		while (overrides.includes(name) || inherited.includes(name)) {
			name = `${baseName}-${counter++}`;
		}
		onChange([...overrides, name]);
	}, [availableItems, title, overrides, inherited, onChange]);

	const addDisabled = availableItems !== undefined &&
		availableItems.every((item) => inherited.includes(item) || overrides.includes(item));

	return (
		<CollapsibleSettingsSection
			title={title}
			badgeCount={inheritedCount + overrideCount}
			badgeText={sectionBadgeText(inheritedCount, overrideCount)}
		>
			<div data-testid={testId}>
				{inherited.map((item) => (
					<div key={`inherited-${item}`} className="settings-item settings-item--inherited">
						<span className="settings-item__name">{item}</span>
					</div>
				))}

				{uniqueOverrideItems.map((item) => (
					<div key={`override-${item}`} className="settings-item settings-item--override">
						<span className="settings-item__name">{item}</span>
					</div>
				))}

				<div className="settings-item__actions">
					<button
						type="button"
						className="settings-item__add-btn"
						onClick={handleAdd}
						aria-label="Add"
						role="button"
						disabled={addDisabled}
					>
						Add
					</button>
					{overrides.length > 0 && (
						<button
							type="button"
							className="settings-item__clear-btn"
							onClick={onClear}
							aria-label="Clear Override"
						>
							Clear Override
						</button>
					)}
					{overrides.length === 0 && inherited.length > 0 && (
						<button
							type="button"
							className="settings-item__clear-btn"
							onClick={onClear}
							aria-label="Clear Override"
							disabled
						>
							Clear Override
						</button>
					)}
				</div>
			</div>
		</CollapsibleSettingsSection>
	);
}

interface ToolsOverrideSectionProps {
	title: string;
	testId: string;
	inherited: string[];
	overrides: string[];
	onChange: (items: string[]) => void;
	onClear: () => void;
}

function ToolsOverrideSection({
	title,
	testId,
	inherited,
	overrides,
	onChange,
	onClear,
}: ToolsOverrideSectionProps) {
	const uniqueOverrideItems = uniqueOverrides(inherited, overrides);
	const inheritedCount = inherited.length;
	const overrideCount = uniqueOverrideItems.length;

	return (
		<CollapsibleSettingsSection
			title={title}
			badgeCount={inheritedCount + overrideCount}
			badgeText={sectionBadgeText(inheritedCount, overrideCount)}
		>
			<div data-testid={testId}>
				{inherited.map((item) => (
					<div key={`inherited-${item}`} className="settings-item settings-item--inherited">
						<span className="settings-item__name">{item}</span>
					</div>
				))}

				<TagInput
					tags={overrides}
					onChange={onChange}
					placeholder={`Add ${title.toLowerCase()}...`}
				/>

				{overrides.length > 0 && (
					<button
						type="button"
						className="settings-item__clear-btn"
						onClick={onClear}
						aria-label="Clear Override"
					>
						Clear Override
					</button>
				)}
			</div>
		</CollapsibleSettingsSection>
	);
}

interface EnvVarsOverrideSectionProps {
	templateEnv: Record<string, string>;
	overrideEnv: Record<string, string>;
	onChange: (env: Record<string, string>) => void;
	onClear: () => void;
}

function EnvVarsOverrideSection({
	templateEnv,
	overrideEnv,
	onChange,
	onClear,
}: EnvVarsOverrideSectionProps) {
	const inheritedKeys = Object.keys(templateEnv);
	const overrideKeys = Object.keys(overrideEnv);
	const uniqueOverrideKeys = overrideKeys.filter((k) => !(k in templateEnv));
	const inheritedCount = inheritedKeys.length;
	const overrideCount = uniqueOverrideKeys.length;

	return (
		<CollapsibleSettingsSection
			title="Env Vars"
			badgeCount={inheritedCount + overrideCount}
			badgeText={sectionBadgeText(inheritedCount, overrideCount)}
		>
			<div data-testid="env-editor">
				{inheritedKeys.map((key) => (
					<div key={`inherited-${key}`} className="settings-item settings-item--inherited">
						<span className="settings-item__name">{key}</span>
						<span className="settings-item__value">= {templateEnv[key]}</span>
					</div>
				))}

				{overrideKeys.map((key) => (
					<div key={`override-${key}`} className="settings-item settings-item--override">
						<span className="settings-item__name">{key}</span>
						<span className="settings-item__value">= {overrideEnv[key]}</span>
					</div>
				))}

				<KeyValueEditor entries={overrideEnv} onChange={onChange} />

				{overrideKeys.length > 0 && (
					<button
						type="button"
						className="settings-item__clear-btn"
						onClick={onClear}
						aria-label="Clear Override"
					>
						Clear Override
					</button>
				)}
			</div>
		</CollapsibleSettingsSection>
	);
}
