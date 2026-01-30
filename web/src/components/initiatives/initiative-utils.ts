/**
 * Utility functions for initiative components.
 * Extracted from InitiativeCard for reuse across initiative-related components.
 */

import { InitiativeStatus } from '@/gen/orc/v1/initiative_pb';

export type InitiativeColorVariant = 'purple' | 'green' | 'amber' | 'blue';

/**
 * Extracts the first emoji from a string, or returns default.
 */
export function extractEmoji(text: string | undefined): string {
	if (!text) return '📋';

	// Match emoji including compound emojis (skin tones, ZWJ sequences)
	const emojiRegex =
		/(?:\p{Emoji_Presentation}|\p{Emoji}\uFE0F)(?:\p{Emoji_Modifier})?(?:\u200D(?:\p{Emoji_Presentation}|\p{Emoji}\uFE0F)(?:\p{Emoji_Modifier})?)*/u;
	const match = text.match(emojiRegex);

	return match ? match[0] : '📋';
}

/**
 * Maps initiative status to a color variant.
 */
export function getStatusColor(status: InitiativeStatus): InitiativeColorVariant {
	switch (status) {
		case InitiativeStatus.ACTIVE:
			return 'green';
		case InitiativeStatus.COMPLETED:
			return 'purple';
		case InitiativeStatus.ARCHIVED:
		case InitiativeStatus.DRAFT:
			return 'amber';
		default:
			return 'blue';
	}
}

/**
 * Maps initiative index to a color variant for visual variety.
 */
export function getIconColor(status: InitiativeStatus): InitiativeColorVariant {
	switch (status) {
		case InitiativeStatus.ACTIVE:
			return 'green';
		case InitiativeStatus.COMPLETED:
			return 'purple';
		case InitiativeStatus.ARCHIVED:
		case InitiativeStatus.DRAFT:
			return 'amber';
		default:
			return 'blue';
	}
}

/**
 * Checks if initiative should render with reduced opacity.
 */
export function isPaused(status: InitiativeStatus): boolean {
	return status === InitiativeStatus.ARCHIVED || status === InitiativeStatus.DRAFT;
}

/**
 * Get human-readable status label
 */
export function getStatusLabel(status: InitiativeStatus): string {
	switch (status) {
		case InitiativeStatus.ACTIVE:
			return 'Active';
		case InitiativeStatus.COMPLETED:
			return 'Completed';
		case InitiativeStatus.ARCHIVED:
			return 'Archived';
		case InitiativeStatus.DRAFT:
			return 'Draft';
		default:
			return 'Unknown';
	}
}
