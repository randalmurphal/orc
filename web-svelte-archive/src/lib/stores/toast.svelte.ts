/**
 * Toast notification store using Svelte 5 runes pattern
 */

export type ToastType = 'success' | 'error' | 'warning' | 'info';

export interface Toast {
	id: string;
	type: ToastType;
	message: string;
	title?: string;
	duration?: number;
	dismissible?: boolean;
}

interface ToastOptions {
	title?: string;
	duration?: number;
	dismissible?: boolean;
}

const DEFAULT_DURATION = 5000;

// Simple reactive store using closure
let toasts: Toast[] = $state([]);
let listeners: Set<() => void> = new Set();

function notify() {
	listeners.forEach((fn) => fn());
}

function generateId(): string {
	return `toast-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

function add(type: ToastType, message: string, options: ToastOptions = {}): string {
	const id = generateId();
	const toast: Toast = {
		id,
		type,
		message,
		title: options.title,
		duration: options.duration ?? DEFAULT_DURATION,
		dismissible: options.dismissible ?? true
	};

	toasts = [...toasts, toast];
	notify();

	// Auto-dismiss after duration
	if (toast.duration && toast.duration > 0) {
		setTimeout(() => {
			dismiss(id);
		}, toast.duration);
	}

	return id;
}

function dismiss(id: string) {
	toasts = toasts.filter((t) => t.id !== id);
	notify();
}

function clear() {
	toasts = [];
	notify();
}

// Public API
export const toast = {
	subscribe(fn: (toasts: Toast[]) => void): () => void {
		// Immediate callback with current state
		fn(toasts);
		// Create listener
		const listener = () => fn(toasts);
		listeners.add(listener);
		return () => listeners.delete(listener);
	},

	success(message: string, options?: ToastOptions): string {
		return add('success', message, options);
	},

	error(message: string, options?: ToastOptions): string {
		return add('error', message, { ...options, duration: options?.duration ?? 8000 });
	},

	warning(message: string, options?: ToastOptions): string {
		return add('warning', message, options);
	},

	info(message: string, options?: ToastOptions): string {
		return add('info', message, options);
	},

	dismiss,
	clear,

	// Get current toasts (for direct access)
	get all(): Toast[] {
		return toasts;
	}
};
