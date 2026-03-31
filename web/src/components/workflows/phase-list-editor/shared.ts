import { GateType } from '@/gen/orc/v1/workflow_pb';

export interface PhaseOverrides {
	modelOverride?: string;
	thinkingOverride?: boolean;
	gateTypeOverride?: GateType;
	runtimeConfigOverride?: string;
}

export interface AddPhaseRequest {
	phaseTemplateId: string;
	sequence: number;
}

export const INHERIT_VALUE = '__inherit__';

export const MODEL_OPTIONS = [
	{ value: INHERIT_VALUE, label: 'Inherit (default)' },
	{ value: 'sonnet', label: 'Sonnet' },
	{ value: 'opus', label: 'Opus' },
	{ value: 'haiku', label: 'Haiku' },
];

export const GATE_TYPE_OPTIONS = [
	{ value: GateType.UNSPECIFIED, label: 'Inherit (default)' },
	{ value: GateType.AUTO, label: 'Auto' },
	{ value: GateType.HUMAN, label: 'Human' },
	{ value: GateType.SKIP, label: 'Skip' },
];
