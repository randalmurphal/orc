import { VariableSourceType, type WorkflowVariable } from '@/gen/orc/v1/workflow_pb';

export interface StaticConfig {
	value: string;
}

export interface EnvConfig {
	var: string;
	default?: string;
}

export interface ScriptConfig {
	path: string;
	args?: string[];
	workDir?: string;
	timeout?: number;
}

export interface ApiConfig {
	url: string;
	method?: string;
	headers?: Record<string, string>;
	jqFilter?: string;
	timeout?: number;
}

export interface PhaseOutputConfig {
	phase: string;
	field?: string;
}

export interface PromptFragmentConfig {
	path: string;
}

export type SourceConfig =
	| StaticConfig
	| EnvConfig
	| ScriptConfig
	| ApiConfig
	| PhaseOutputConfig
	| PromptFragmentConfig;

export interface VariableModalProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	workflowId: string;
	variable?: WorkflowVariable;
	availablePhases?: string[];
	onSuccess?: () => void;
}

export function defaultSourceConfig(sourceType: VariableSourceType): SourceConfig {
	switch (sourceType) {
		case VariableSourceType.STATIC:
			return { value: '' };
		case VariableSourceType.ENV:
			return { var: '' };
		case VariableSourceType.SCRIPT:
			return { path: '' };
		case VariableSourceType.API:
			return { url: '', method: 'GET' };
		case VariableSourceType.PHASE_OUTPUT:
			return { phase: '' };
		case VariableSourceType.PROMPT_FRAGMENT:
			return { path: '' };
		default:
			return { value: '' };
	}
}
