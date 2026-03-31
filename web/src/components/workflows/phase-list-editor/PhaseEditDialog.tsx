import { useCallback, useEffect, useMemo, useState } from 'react';
import * as RadixSelect from '@radix-ui/react-select';
import { Button, Icon } from '@/components/ui';
import { GateType, type PhaseTemplate, type WorkflowPhase } from '@/gen/orc/v1/workflow_pb';
import type { Hook, Skill } from '@/gen/orc/v1/config_pb';
import type { MCPServerInfo } from '@/gen/orc/v1/mcp_pb';
import { configClient, mcpClient } from '@/lib/client';
import {
	fetchMCPServerConfig,
	hydrateSelectedMCPServers,
	parseRuntimeConfig,
	serializeRuntimeConfig,
	type HookDefinition,
	type RuntimeConfigState,
} from '@/lib/runtimeConfigUtils';
import { RuntimeConfigSections } from './runtime-config-sections';
import { GATE_TYPE_OVERRIDE_OPTIONS, INHERIT_VALUE, MODEL_OPTIONS, type PhaseOverrides } from './shared';

interface PhaseEditDialogProps {
	phase: WorkflowPhase | null;
	getTemplate: (templateId: string) => PhaseTemplate | undefined;
	onSave: (phaseId: number, overrides: PhaseOverrides) => Promise<void>;
	onClose: () => void;
}

export function PhaseEditDialog({
	phase,
	getTemplate,
	onSave,
	onClose,
}: PhaseEditDialogProps) {
	const [editOverrides, setEditOverrides] = useState<PhaseOverrides>({});
	const [overrideHooks, setOverrideHooks] = useState<string[]>([]);
	const [overrideSkills, setOverrideSkills] = useState<string[]>([]);
	const [overrideMcpServers, setOverrideMcpServers] = useState<string[]>([]);
	const [overrideAllowedTools, setOverrideAllowedTools] = useState<string[]>([]);
	const [overrideDisallowedTools, setOverrideDisallowedTools] = useState<string[]>([]);
	const [overrideEnv, setOverrideEnv] = useState<Record<string, string>>({});
	const [overrideHookConfig, setOverrideHookConfig] = useState<Record<string, unknown>>({});
	const [overrideHookEventTypes, setOverrideHookEventTypes] = useState<Record<string, string>>({});
	const [overrideMcpServerData, setOverrideMcpServerData] = useState<Record<string, unknown>>({});
	const [jsonOverride, setJsonOverride] = useState('');
	const [jsonOverrideDirty, setJsonOverrideDirty] = useState(false);
	const [availableHooks, setAvailableHooks] = useState<Hook[]>([]);
	const [availableSkills, setAvailableSkills] = useState<Skill[]>([]);
	const [availableMcpServers, setAvailableMcpServers] = useState<MCPServerInfo[]>([]);

	useEffect(() => {
		if (!phase) {
			return;
		}

		let mounted = true;
		configClient.listHooks({}).then((response) => {
			if (mounted) {
				setAvailableHooks(response.hooks);
			}
		}).catch(() => {});
		configClient.listSkills({}).then((response) => {
			if (mounted) {
				setAvailableSkills(response.skills);
			}
		}).catch(() => {});
		mcpClient.listMCPServers({}).then((response) => {
			if (mounted) {
				setAvailableMcpServers(response.servers);
			}
		}).catch(() => {});
		return () => {
			mounted = false;
		};
	}, [phase]);

	useEffect(() => {
		if (!phase) {
			return;
		}

		setEditOverrides({
			modelOverride: phase.modelOverride || undefined,
			thinkingOverride: phase.thinkingOverride || undefined,
			gateTypeOverride: phase.gateTypeOverride,
		});

		const override = parseRuntimeConfig(phase.runtimeConfigOverride as string | undefined);
		setOverrideHooks(override.hooks);
		setOverrideSkills(override.skillRefs);
		setOverrideMcpServers(override.mcpServers);
		setOverrideAllowedTools(override.allowedTools);
		setOverrideDisallowedTools(override.disallowedTools);
		setOverrideEnv(override.env);
		setOverrideHookConfig(override.hookConfig ?? {});
		setOverrideHookEventTypes(override.hookEventTypes ?? {});
		setOverrideMcpServerData(override.mcpServerData ?? {});
		setJsonOverride(phase.runtimeConfigOverride || '');
		setJsonOverrideDirty(false);
	}, [phase]);

	useEffect(() => {
		if (!phase) {
			return;
		}
		let mounted = true;
		hydrateSelectedMCPServers(
			overrideMcpServers,
			overrideMcpServerData,
			fetchMCPServerConfig,
		).then((hydrated) => {
			if (mounted) {
				setOverrideMcpServerData(hydrated);
			}
		}).catch(() => {});
		return () => {
			mounted = false;
		};
	}, [phase, overrideMcpServers]);

	useEffect(() => {
		if (!phase || jsonOverrideDirty) {
			return;
		}
		setJsonOverride(
			serializeRuntimeConfig(
				{
					hooks: overrideHooks,
					skillRefs: overrideSkills,
					mcpServers: overrideMcpServers,
					allowedTools: overrideAllowedTools,
					disallowedTools: overrideDisallowedTools,
					env: overrideEnv,
					mcpServerData: overrideMcpServerData,
					hookConfig: overrideHookConfig,
					hookEventTypes: overrideHookEventTypes,
					extra: {},
				},
				{
					hookDefinitions: availableHooks.map((hook): HookDefinition => ({
						name: hook.name,
						eventType: hook.eventType,
					})),
				},
			),
		);
	}, [
		phase,
		jsonOverrideDirty,
		overrideHooks,
		overrideSkills,
		overrideMcpServers,
		overrideAllowedTools,
		overrideDisallowedTools,
		overrideEnv,
		overrideMcpServerData,
		overrideHookConfig,
		overrideHookEventTypes,
		availableHooks,
	]);

	const templateConfig = useMemo<RuntimeConfigState>(() => {
		if (!phase) return parseRuntimeConfig(undefined);
		const tmpl = phase.template;
		return parseRuntimeConfig((tmpl as Record<string, unknown> | undefined)?.runtimeConfig as string | undefined);
	}, [phase]);

	const buildRuntimeConfigOverride = useCallback(async (): Promise<string | undefined> => {
		let state: RuntimeConfigState = {
			hooks: overrideHooks,
			skillRefs: overrideSkills,
			mcpServers: overrideMcpServers,
			allowedTools: overrideAllowedTools,
			disallowedTools: overrideDisallowedTools,
			env: overrideEnv,
			mcpServerData: overrideMcpServerData,
			hookConfig: overrideHookConfig,
			hookEventTypes: overrideHookEventTypes,
			extra: {},
		};

		if (jsonOverrideDirty && jsonOverride.trim() !== '') {
			try {
				state = parseRuntimeConfig(jsonOverride);
			} catch {
				// Keep structured state when raw JSON is invalid.
			}
		}

		const serialized = serializeRuntimeConfig(
			{
				...state,
				mcpServerData: await hydrateSelectedMCPServers(
					state.mcpServers,
					state.mcpServerData ?? {},
					fetchMCPServerConfig,
				),
			},
			{
				hookDefinitions: availableHooks.map((hook): HookDefinition => ({
					name: hook.name,
					eventType: hook.eventType,
				})),
			},
		);
		if (serialized === '{}') return undefined;
		return serialized;
	}, [
		overrideHooks,
		overrideSkills,
		overrideMcpServers,
		overrideAllowedTools,
		overrideDisallowedTools,
		overrideEnv,
		overrideMcpServerData,
		overrideHookConfig,
		overrideHookEventTypes,
		jsonOverride,
		jsonOverrideDirty,
		availableHooks,
	]);

	const handleClearOverride = useCallback((section: string) => {
		setJsonOverrideDirty(false);
		switch (section) {
			case 'hooks':
				setOverrideHooks([]);
				setOverrideHookConfig({});
				setOverrideHookEventTypes({});
				break;
			case 'skills':
				setOverrideSkills([]);
				break;
			case 'mcpServers':
				setOverrideMcpServers([]);
				setOverrideMcpServerData({});
				break;
			case 'allowedTools':
				setOverrideAllowedTools([]);
				break;
			case 'disallowedTools':
				setOverrideDisallowedTools([]);
				break;
			case 'env':
				setOverrideEnv({});
				break;
		}
	}, []);

	const handleSavePhase = useCallback(async () => {
		if (!phase) return;
		const runtimeConfigOverride = await buildRuntimeConfigOverride();
		try {
			await onSave(phase.id, {
				...editOverrides,
				runtimeConfigOverride,
			});
		} catch {
			return;
		}
		onClose();
	}, [phase, editOverrides, buildRuntimeConfigOverride, onSave, onClose]);

	if (!phase) {
		return null;
	}

	return (
		<div className="phase-edit-dialog">
			<h4 className="phase-edit-title">
				Edit Phase: {getTemplate(phase.phaseTemplateId)?.name || phase.phaseTemplateId}
			</h4>

			<div className="form-group">
				<label id="phase-model-label" className="form-label">Model</label>
				<RadixSelect.Root
					value={editOverrides.modelOverride || INHERIT_VALUE}
					onValueChange={(value) => setEditOverrides((prev) => ({
						...prev,
						modelOverride: value === INHERIT_VALUE ? undefined : value,
					}))}
				>
					<RadixSelect.Trigger className="phase-template-trigger" aria-label="Model" aria-labelledby="phase-model-label">
						<RadixSelect.Value placeholder="Inherit (default)">
							{MODEL_OPTIONS.find((opt) => opt.value === (editOverrides.modelOverride || INHERIT_VALUE))?.label}
						</RadixSelect.Value>
						<RadixSelect.Icon className="phase-template-trigger-icon">
							<Icon name="chevron-down" size={12} />
						</RadixSelect.Icon>
					</RadixSelect.Trigger>
					<RadixSelect.Portal>
						<RadixSelect.Content className="phase-template-content" position="popper" sideOffset={4}>
							<RadixSelect.Viewport className="phase-template-viewport">
								{MODEL_OPTIONS.map((opt) => (
									<RadixSelect.Item key={opt.value} value={opt.value} className="phase-template-item">
										<RadixSelect.ItemText>{opt.label}</RadixSelect.ItemText>
									</RadixSelect.Item>
								))}
							</RadixSelect.Viewport>
						</RadixSelect.Content>
					</RadixSelect.Portal>
				</RadixSelect.Root>
			</div>

			<div className="form-group">
				<label className="form-checkbox">
					<input
						type="checkbox"
						checked={editOverrides.thinkingOverride || false}
						onChange={(e) => setEditOverrides((prev) => ({
							...prev,
							thinkingOverride: e.target.checked || undefined,
						}))}
						aria-label="Thinking"
					/>
					<span className="form-checkbox-label">Enable thinking mode</span>
				</label>
			</div>

			<div className="form-group">
				<label id="phase-gate-label" className="form-label">Gate Type</label>
				<RadixSelect.Root
					value={String(editOverrides.gateTypeOverride ?? GateType.UNSPECIFIED)}
					onValueChange={(value) => setEditOverrides((prev) => ({
						...prev,
						gateTypeOverride:
							Number(value) === GateType.UNSPECIFIED ? undefined : (Number(value) as GateType),
					}))}
				>
					<RadixSelect.Trigger className="phase-template-trigger" aria-label="Gate" aria-labelledby="phase-gate-label">
						<RadixSelect.Value placeholder="Inherit (default)">
							{GATE_TYPE_OVERRIDE_OPTIONS.find((opt) => opt.value === (editOverrides.gateTypeOverride ?? GateType.UNSPECIFIED))?.label}
						</RadixSelect.Value>
						<RadixSelect.Icon className="phase-template-trigger-icon">
							<Icon name="chevron-down" size={12} />
						</RadixSelect.Icon>
					</RadixSelect.Trigger>
					<RadixSelect.Portal>
						<RadixSelect.Content className="phase-template-content" position="popper" sideOffset={4}>
							<RadixSelect.Viewport className="phase-template-viewport">
								{GATE_TYPE_OVERRIDE_OPTIONS.map((opt) => (
									<RadixSelect.Item key={opt.value} value={String(opt.value)} className="phase-template-item">
										<RadixSelect.ItemText>{opt.label}</RadixSelect.ItemText>
									</RadixSelect.Item>
								))}
							</RadixSelect.Viewport>
						</RadixSelect.Content>
					</RadixSelect.Portal>
				</RadixSelect.Root>
			</div>

			<RuntimeConfigSections
				templateConfig={templateConfig}
				overrideHooks={overrideHooks}
				overrideSkills={overrideSkills}
				overrideMcpServers={overrideMcpServers}
				overrideAllowedTools={overrideAllowedTools}
				overrideDisallowedTools={overrideDisallowedTools}
				overrideEnv={overrideEnv}
				jsonOverride={jsonOverride}
				availableHookNames={availableHooks.map((hook) => hook.name)}
				availableSkillNames={availableSkills.map((skill) => skill.name)}
				availableMcpServerNames={availableMcpServers.map((server) => server.name)}
				onOverrideHooksChange={(value) => {
					setJsonOverrideDirty(false);
					setOverrideHooks(value);
				}}
				onOverrideSkillsChange={(value) => {
					setJsonOverrideDirty(false);
					setOverrideSkills(value);
				}}
				onOverrideMcpServersChange={(value) => {
					setJsonOverrideDirty(false);
					setOverrideMcpServers(value);
				}}
				onOverrideAllowedToolsChange={(value) => {
					setJsonOverrideDirty(false);
					setOverrideAllowedTools(value);
				}}
				onOverrideDisallowedToolsChange={(value) => {
					setJsonOverrideDirty(false);
					setOverrideDisallowedTools(value);
				}}
				onOverrideEnvChange={(value) => {
					setJsonOverrideDirty(false);
					setOverrideEnv(value);
				}}
				onJsonOverrideChange={(value) => {
					setJsonOverride(value);
					setJsonOverrideDirty(true);
				}}
				onClearOverride={handleClearOverride}
			/>

			<div className="phase-edit-actions">
				<Button variant="ghost" size="sm" onClick={onClose}>Cancel</Button>
				<Button variant="primary" size="sm" onClick={() => void handleSavePhase()}>
					Save Phase
				</Button>
			</div>
		</div>
	);
}
