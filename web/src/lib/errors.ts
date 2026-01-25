/**
 * Error handling utilities for the frontend.
 */

export class APIError extends Error {
	constructor(
		message: string,
		public statusCode?: number,
		public endpoint?: string
	) {
		super(message);
		this.name = 'APIError';
	}
}

export function handleStoreError(error: unknown, context: string): string {
	const message = error instanceof Error ? error.message : 'Unknown error';
	console.error(`[${context}]`, error);
	return message;
}
