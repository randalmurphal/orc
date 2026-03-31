import { VariableSourceType } from '@/gen/orc/v1/workflow_pb';

export { fetchMCPServerConfig } from '@/lib/runtimeConfigUtils';

export interface SectionState {
	subAgents: boolean;
	prompt: boolean;
	dataFlow: boolean;
	environment: boolean;
	advanced: boolean;
}

export interface FieldError {
	message: string;
	type: 'validation' | 'save' | 'load';
}

export interface FieldErrors {
	[key: string]: FieldError | null;
}

export type AutoSave = (field: string, value: unknown, immediate?: boolean) => void | Promise<void>;

export function formatSourceType(st: VariableSourceType): string {
	switch (st) {
		case VariableSourceType.STATIC:
			return 'static';
		case VariableSourceType.ENV:
			return 'env';
		case VariableSourceType.SCRIPT:
			return 'script';
		case VariableSourceType.API:
			return 'api';
		case VariableSourceType.PHASE_OUTPUT:
			return 'phase_output';
		case VariableSourceType.PROMPT_FRAGMENT:
			return 'prompt_fragment';
		default:
			return 'unknown';
	}
}
