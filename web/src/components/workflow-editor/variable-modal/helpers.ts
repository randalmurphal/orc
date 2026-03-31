import { VariableSourceType } from '@/gen/orc/v1/workflow_pb';
import { defaultSourceConfig, type SourceConfig } from './types';

export function normalizeVariableName(name: string): string {
	return name.trim().toUpperCase().replace(/[^A-Z0-9_]/g, '_');
}

export function parseSourceConfig(
	sourceType: VariableSourceType,
	configJson: string,
): SourceConfig {
	try {
		return JSON.parse(configJson);
	} catch {
		return defaultSourceConfig(sourceType);
	}
}
