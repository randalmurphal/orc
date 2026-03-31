import { GateType } from '@/gen/orc/v1/workflow_pb';

export const GATE_TYPE_OPTIONS = [
	{ value: GateType.AUTO, label: 'Auto' },
	{ value: GateType.HUMAN, label: 'Human' },
	{ value: GateType.SKIP, label: 'Skip' },
];

export const VARIABLE_SUGGESTIONS = [
	'SPEC_CONTENT',
	'PROJECT_ROOT',
	'TASK_DESCRIPTION',
	'WORKTREE_PATH',
	'INITIATIVE_VISION',
	'INITIATIVE_DECISIONS',
	'RETRY_ATTEMPT',
	'RETRY_FROM_PHASE',
	'RETRY_REASON',
	'RETRY_FEEDBACK',
	'TDD_TEST_CONTENT',
	'BREAKDOWN_CONTENT',
];

export function slugify(name: string): string {
	return name
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '')
		.replace(/-+/g, '-');
}
