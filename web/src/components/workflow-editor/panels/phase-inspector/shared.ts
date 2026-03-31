import { create } from '@bufbuild/protobuf';
import { VariableSourceType } from '@/gen/orc/v1/workflow_pb';
import { GetMCPServerRequestSchema } from '@/gen/orc/v1/mcp_pb';
import { mcpClient } from '@/lib/client';

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

export async function fetchMCPServerConfig(name: string): Promise<Record<string, unknown> | undefined> {
	const response = await mcpClient.getMCPServer(
		create(GetMCPServerRequestSchema, { name }),
	);
	if (!response.server) {
		return undefined;
	}
	return {
		type: response.server.type,
		command: response.server.command,
		args: response.server.args,
		env: response.server.env,
		url: response.server.url,
		headers: response.server.headers,
		disabled: response.server.disabled,
	};
}

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
