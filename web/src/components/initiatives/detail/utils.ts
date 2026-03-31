import { InitiativeStatus } from '@/gen/orc/v1/initiative_pb';
import { TaskStatus } from '@/gen/orc/v1/task_pb';
import type { IconName } from '@/components/ui/Icon';
import { extractEmoji } from '@/components/initiatives/initiative-utils';

export type TaskFilter = 'all' | 'completed' | 'running' | 'planned';

export function getTaskStatusDisplay(status: TaskStatus): string {
	switch (status) {
		case TaskStatus.COMPLETED:
			return 'completed';
		case TaskStatus.RUNNING:
			return 'running';
		case TaskStatus.BLOCKED:
			return 'blocked';
		case TaskStatus.PAUSED:
			return 'paused';
		case TaskStatus.FAILED:
			return 'failed';
		case TaskStatus.PLANNED:
			return 'planned';
		case TaskStatus.CREATED:
			return 'created';
		case TaskStatus.CLASSIFYING:
			return 'classifying';
		case TaskStatus.FINALIZING:
			return 'finalizing';
		case TaskStatus.CLOSED:
			return 'closed';
		default:
			return 'pending';
	}
}

export function getInitiativeStatusDisplay(status: InitiativeStatus): string {
	switch (status) {
		case InitiativeStatus.DRAFT:
			return 'draft';
		case InitiativeStatus.ACTIVE:
			return 'active';
		case InitiativeStatus.COMPLETED:
			return 'completed';
		case InitiativeStatus.ARCHIVED:
			return 'archived';
		default:
			return 'unknown';
	}
}

export function getInitiativeEmoji(text: string, status?: InitiativeStatus): string {
	const emoji = extractEmoji(text);
	if (emoji !== '📋') {
		return emoji;
	}
	switch (status) {
		case InitiativeStatus.ACTIVE:
			return '🚀';
		case InitiativeStatus.COMPLETED:
			return '✅';
		case InitiativeStatus.ARCHIVED:
			return '📦';
		default:
			return '📋';
	}
}

export function stripLeadingEmoji(text: string): string {
	return text.replace(/^(\p{Emoji})\s*/u, '');
}

export function getTaskStatusIcon(status: TaskStatus): IconName {
	switch (status) {
		case TaskStatus.COMPLETED:
			return 'check-circle';
		case TaskStatus.RUNNING:
			return 'play-circle';
		case TaskStatus.FAILED:
			return 'x-circle';
		case TaskStatus.PAUSED:
			return 'pause-circle';
		case TaskStatus.BLOCKED:
			return 'alert-circle';
		default:
			return 'circle';
	}
}

export function getTaskStatusClass(status: TaskStatus): string {
	switch (status) {
		case TaskStatus.COMPLETED:
			return 'status-success';
		case TaskStatus.RUNNING:
			return 'status-running';
		case TaskStatus.FAILED:
			return 'status-danger';
		case TaskStatus.BLOCKED:
		case TaskStatus.PAUSED:
			return 'status-warning';
		default:
			return 'status-pending';
	}
}

export function getNoteTypeIcon(noteType: string): IconName {
	switch (noteType) {
		case 'pattern':
			return 'code';
		case 'warning':
			return 'alert-triangle';
		case 'learning':
			return 'brain';
		case 'handoff':
			return 'chevron-right';
		default:
			return 'message-square';
	}
}

export function getNoteTypeLabel(noteType: string): string {
	switch (noteType) {
		case 'pattern':
			return 'Patterns';
		case 'warning':
			return 'Warnings';
		case 'learning':
			return 'Learnings';
		case 'handoff':
			return 'Handoffs';
		default:
			return 'Notes';
	}
}
